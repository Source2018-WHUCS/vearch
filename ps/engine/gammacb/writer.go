// Copyright 2018 The ChuBao Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package gammacb

/*
#cgo CFLAGS : -Ilib/include
#cgo LDFLAGS: -Llib/lib -lgamma

#include "gamma_api.h"
*/
import "C"
import (
	"context"
	"fmt"
	"github.com/spf13/cast"
	pkg "github.com/tiglabs/baudengine/proto"
	"github.com/tiglabs/baudengine/proto/pspb"
	"github.com/tiglabs/baudengine/proto/response"
	"github.com/tiglabs/baudengine/ps/engine"
	"github.com/tiglabs/baudengine/util/baudlog"
	"github.com/tiglabs/baudengine/util/ioutil2"
	"github.com/tiglabs/log"
	"go.uber.org/atomic"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"sync"
	"unsafe"
)

var _ engine.Writer = &writerImpl{}

type writerImpl struct {
	engine  *gammaEngine
	path    string
	lock    sync.RWMutex
	lastSn  atomic.Int64
	running bool
}

func (wi *writerImpl) Write(ctx context.Context, doc *pspb.DocCmd) *response.DocResult {
	if doc == nil || doc.Type == pspb.OpType_NOOP {
		log.Error("you put a nil doc cmd or noop is zero , make sure it a bug")
		return response.NewErrDocResult(doc.DocId, fmt.Errorf("you put a nil doc cmd , make sure it a bug"))
	}
	var result *response.DocResult
	switch doc.Type {
	case pspb.OpType_MERGE, pspb.OpType_REPLACE:
		result = wi.Update(ctx, doc)
	case pspb.OpType_CREATE:
		result = wi.Create(ctx, doc)
	case pspb.OpType_DELETE:
		result = wi.Delete(ctx, doc)
	default:
		result = response.NewErrDocResult(doc.DocId, fmt.Errorf("not found op type:[%d]", doc.Type))
	}
	return result
}

func (wi *writerImpl) Create(ctx context.Context, docCmd *pspb.DocCmd) *response.DocResult {
	cDoc, err := DocCmd2Document(docCmd)
	if err != nil {
		return response.NewErrDocResult(docCmd.DocId, err)
	}
	defer C.DestroyFields(cDoc.fields, cDoc.fields_num)

	if resp := C.AddDoc(wi.engine.gamma, (*C.struct_Doc)(unsafe.Pointer(cDoc))); resp != 0 {
		return response.NewErrDocResult(docCmd.DocId, fmt.Errorf("gamma create doc err code:[%d]", int(resp)))
	}

	return wi.engine.DocCmd2WriteResult(docCmd)
}

func (wi *writerImpl) Update(ctx context.Context, docCmd *pspb.DocCmd) *response.DocResult {
	cDoc, err := DocCmd2Document(docCmd)
	if err != nil {
		return response.NewErrDocResult(docCmd.DocId, err)
	}
	defer func() {
		C.DestroyFields(cDoc.fields, cDoc.fields_num)
	}()

	if resp := C.AddOrUpdateDoc(wi.engine.gamma, (*C.struct_Doc)(unsafe.Pointer(cDoc))); resp != 0 {
		return response.NewErrDocResult(docCmd.DocId, fmt.Errorf("gamma create doc err code:[%d]", int(resp)))
	}

	return wi.engine.DocCmd2WriteResult(docCmd)
}

func (wi *writerImpl) Delete(ctx context.Context, docCmd *pspb.DocCmd) *response.DocResult {

	if docCmd.Version == 0 {
		return response.NewErrDocResult(docCmd.DocId, pkg.ErrDocDelVersionNotSpecified)
	}

	if docCmd.Version < 0 {
		if code := C.DelDoc(wi.engine.gamma, byteArrayStr(docCmd.DocId)); code != 0 {
			return response.NewErrDocResult(docCmd.DocId, fmt.Errorf("delete document err"))
		}
	}

	if docCmd.Version > 0 {
		doc := wi.engine.reader.GetDoc(ctx, docCmd.DocId)
		if doc == nil {
			return response.NewNotFoundDocResult(docCmd.DocId)
		}
		if !doc.Found {
			return response.NewErrDocResult(docCmd.DocId, pkg.ErrDocumentNotExist)
		}

		if docCmd.Version != doc.Version {
			if docCmd.PulloutVersion {
				return response.NewErrDocResult(docCmd.DocId, pkg.ErrDocPulloutVersionNotMatch)
			} else {
				return response.NewErrDocResult(docCmd.DocId, fmt.Errorf("document version not same new:[%d] old:[%d]", docCmd.Version, doc.Version))
			}
		}
		if code := C.DelDoc(wi.engine.gamma, byteArrayStr(docCmd.DocId)); code != 0 {
			return response.NewErrDocResult(docCmd.DocId, fmt.Errorf("delete document err"))
		}
	}

	return wi.engine.reader.GetDoc(ctx, docCmd.DocId)
}

func (wi *writerImpl) Flush(ctx context.Context, sn int64) error {
	wi.lastSn.Store(sn)
	if code := C.Dump(wi.engine.gamma); code != 0 {
		return fmt.Errorf("dump index err response code :[%d]", code)
	}
	return nil
}

func (wi *writerImpl) Commit(ctx context.Context, snx int64) (chan error, error) {

	flushC := make(chan error, 1)

	go func(fc chan error, sn int64) {

		wi.lock.Lock()
		if wi.running {
			log.Info("commit is running , so skip this request")
			fc <- nil
			return
		}
		wi.running = true
		wi.lock.Unlock()

		defer func() {
			wi.lock.Lock()
			wi.running = false
			wi.lock.Unlock()

			if i := recover(); i != nil {
				log.Error(string(debug.Stack()))
				log.Error(cast.ToString(i))
			}
		}()

		log.Info("begin dump data for gamma")

		wi.lastSn.Store(sn)

		fileName := filepath.Join(wi.path, indexSn)
		err := ioutil2.WriteFileAtomic(fileName, []byte(string(strconv.FormatInt(sn, 10))), os.ModePerm)
		if err != nil {
			return
		}

		if code := C.Dump(wi.engine.gamma); code != 0 {
			fc <- baudlog.LogErrAndReturn(fmt.Errorf("dump index err response code :[%d]", code))
		} else {
			fc <- nil
		}

		log.Info("end dump data for gamma")
	}(flushC, snx)

	return flushC, nil
}
