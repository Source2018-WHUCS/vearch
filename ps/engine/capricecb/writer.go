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

package capricecb

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/spf13/cast"
	"github.com/tiglabs/baudengine/ps/engine"
	"github.com/tiglabs/baudengine/proto"
	"github.com/tiglabs/baudengine/proto/entity"
	"github.com/tiglabs/baudengine/proto/pspb"
	"github.com/tiglabs/baudengine/proto/response"
	"github.com/tiglabs/baudengine/util"
	"github.com/tiglabs/baudengine/util/ioutil2"
	"github.com/tiglabs/caprice"
	"github.com/tiglabs/log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
)

const indexSn = "sn"

var _ engine.Writer = &writerImpl{}

type writerImpl struct {
	*capriceEngine
	docCache     *documentCache
	writer       caprice.IndexWriter
	flushEntityC chan *entity.FlushEntity
}

func (wi *writerImpl) Write(ctx context.Context, doc *pspb.DocCmd) *response.DocResult {
	if doc == nil || doc.Type == pspb.OpType_NOOP {
		log.Error("you put a nil doc cmd or noop is zero , make sure it a bug")
		if doc == nil {
			doc = pspb.GetDocCmd()
		}
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

	update := docCmd.Version >= 0

	eDoc, err := DocCmd2Document(docCmd)
	if err != nil {
		return response.NewErrDocResult(docCmd.DocId, err)
	}

	doc := wi.getDocForUpdate(ctx, docCmd.DocId)
	if doc.Found == true {
		return response.NewErrDocResult(docCmd.DocId, pkg.ErrDocumentExist)
	}

	if update {
		err = wi.writer.Update(eDoc)
	} else {
		err = wi.writer.Create(eDoc)
	}

	if err != nil {
		return response.NewErrDocResult(docCmd.DocId, err)
	}

	wi.docCache.Put(docCmd.DocId, docCmd)

	return wi.DocCmd2DocResult(docCmd)
}

func (wi *writerImpl) Update(ctx context.Context, docCmd *pspb.DocCmd) *response.DocResult {
	var (
		err     error
		replace bool
	)
	switch docCmd.Type {
	case pspb.OpType_REPLACE:
		replace, err = wi.replace(ctx, docCmd)
	case pspb.OpType_MERGE:
		err = wi.merge(ctx, docCmd)
	}
	if err != nil {
		return response.NewErrDocResult(docCmd.DocId, err)
	}
	result := wi.DocCmd2DocResult(docCmd)
	result.Replace = replace
	return result
}

func (wi *writerImpl) Delete(ctx context.Context, docCmd *pspb.DocCmd) *response.DocResult {
	if docCmd.Version == 0 {
		return response.NewErrDocResult(docCmd.DocId, pkg.ErrDocDelVersionNotSpecified)
	}

	if docCmd.Version < 0 {
		if err := wi.writer.Delete(docCmd.DocId); err != nil {
			return response.NewErrDocResult(docCmd.DocId, err)
		}
	}

	if docCmd.Version > 0 {
		doc := wi.getDocForUpdate(ctx, docCmd.DocId)
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
		if err := wi.writer.Delete(docCmd.DocId); err != nil {
			return response.NewErrDocResult(docCmd.DocId, err)
		}
	}

	wi.docCache.Put(docCmd.DocId, docCmd)

	return wi.DocCmd2DocResult(docCmd)
}

func (wi *writerImpl) replace(ctx context.Context, docCmd *pspb.DocCmd) (bool, error) {
	if docCmd.Version < 0 {
		eDoc, err := DocCmd2Document(docCmd)
		if err != nil {
			return false, err
		}
		doc := wi.getDocForUpdate(ctx, docCmd.DocId)
		err = wi.writer.Update(eDoc)
		if err != nil {
			return doc.Found, err
		}
		wi.docCache.Put(docCmd.DocId, docCmd)

		if err != nil {
			return false, err
		}
		return doc.Found, nil
	}

	if docCmd.Version == 0 {
		return false, pkg.ErrDocReplaceVersionNotSpecified
	}

	doc := wi.getDocForUpdate(ctx, docCmd.DocId)
	if !doc.Found {
		return false, pkg.ErrDocumentNotExist
	}

	if docCmd.Version > 0 && docCmd.Version != doc.Version {
		if docCmd.PulloutVersion {
			return false, pkg.ErrDocPulloutVersionNotMatch
		} else {
			return false, fmt.Errorf("document version not same new:[%d] old:[%d]", docCmd.Version, doc.Version)
		}
	}

	docCmd.Version = doc.Version + 1
	eDoc, err := DocCmd2Document(docCmd)
	if err != nil {
		return false, err
	}
	if err := wi.writer.Update(eDoc); err != nil {
		return false, err
	}
	wi.docCache.Put(docCmd.DocId, docCmd)

	return true, nil
}

func (wi *writerImpl) merge(ctx context.Context, cmd *pspb.DocCmd) error {
	doc := wi.getDocForUpdate(ctx, cmd.DocId)
	if !doc.Found {
		return pkg.ErrDocumentNotExist
	}
	if doc.Source == nil {
		return pkg.ErrDocumentMustHasSource
	}

	if cmd.Version == 0 {
		return pkg.ErrDocumentMergeVersionNotSpecified
	}

	if cmd.Version > 0 && cmd.Version != doc.Version {
		if cmd.PulloutVersion {
			return pkg.ErrDocPulloutVersionNotMatch
		} else {
			return fmt.Errorf("document version not same new:[%d] old:[%d]", cmd.Version, doc.Version)
		}
	}
	cmd.Version = doc.Version + 1

	src := make(map[string]interface{})

	if err := json.Unmarshal(cmd.Source, &src); err != nil {
		log.Error("Unmarshal document err by id %d , content %s ", cmd.DocId, cmd.Source)
		return err
	}

	dest := make(map[string]interface{})
	if err := json.Unmarshal(doc.Source, &dest); err != nil {
		log.Error("Unmarshal old document err by id %d , content %s ", cmd.DocId, cmd.Source)
		return err
	}

	util.MergeMap(dest, src)

	if bytes, e := json.Marshal(dest); e != nil {
		log.Error("marshal document err by id %v ", cmd.DocId)
		return fmt.Errorf("marshal document err by id %v ", cmd.DocId)
	} else {
		cmd.Source = bytes
	}

	fields, _, err := wi.indexMapping.MapDocument(cmd.Source)
	if err != nil {
		return err
	}
	cmd.Fields = fields
	eDoc, err := DocCmd2Document(cmd)
	if err != nil {
		return err
	}
	if err := wi.writer.Update(eDoc); err != nil {
		return err
	}
	wi.docCache.Put(cmd.DocId, cmd)

	if err != nil {
		return err
	}
	return nil
}

func (wi *writerImpl) getDocForUpdate(ctx context.Context, docId string) *response.DocResult {
	lastDocCmd, hit := wi.docCache.Get(docId)
	if !hit {
		return wi.reader.GetDoc(ctx, docId)
	}

	if lastDocCmd.Type == pspb.OpType_DELETE {
		return response.NewErrDocResult(docId, pkg.ErrDocumentNotExist)
	}

	if lastDocCmd.Type == pspb.OpType_CREATE ||
		lastDocCmd.Type == pspb.OpType_REPLACE ||
		lastDocCmd.Type == pspb.OpType_MERGE {
		return wi.DocCmd2DocResult(lastDocCmd)
	}

	panic(fmt.Sprintf("unreachable value:[%v] type:[%d]",lastDocCmd, lastDocCmd.Type))
}

func (wi *writerImpl) Flush(ctx context.Context, sn int64) error {
	wi.lock.Lock()
	defer wi.lock.Unlock()
	wi.docCache.BeforeFlush()

	fileName := filepath.Join(wi.indexConfig.Path, indexSn)
	err := ioutil2.WriteFileAtomic(fileName, []byte(string(strconv.FormatInt(sn, 10))), os.ModePerm)
	if err != nil {
		wi.docCache.OnFlushFailed()
		return err
	}

	if err := wi.writer.Flush(); err != nil {
		wi.docCache.OnFlushFailed()
		return err
	}
	wi.docCache.OnFlushSuccess()
	return nil
}

func (wi *writerImpl) Commit(ctx context.Context, sn int64) (chan error, error) {
	wi.lock.Lock()
	defer wi.lock.Unlock()

	fileName := filepath.Join(wi.indexConfig.Path, indexSn)
	err := ioutil2.WriteFileAtomic(fileName, []byte(string(strconv.FormatInt(sn, 10))), os.ModePerm)
	if err != nil {
		return nil, err
	}

	f := wi.writer.Commit()
	flushC := make(chan error, 1)
	wi.flushEntityC <- &entity.FlushEntity{F: f, FlushC: flushC}
	return flushC, nil
}

func (wi *writerImpl) StartHandleCommitJobs() {
	go func() {
		defer func() {
			if i := recover(); i != nil {
				log.Error(string(debug.Stack()))
				log.Error(cast.ToString(i))
			}
		}()

		for {
			select {
			case <-wi.ctx.Done():
				log.Info("StartHandleCommitJobs done, partitionId: [%d], space: [%v]", wi.partitionID, wi.space)
				return
			case fe, ok := <-wi.flushEntityC:
				if ok {
					wi.docCache.BeforeFlush()
					err := fe.F()
					if err != nil {
						wi.docCache.OnFlushFailed()
					} else {
						wi.docCache.OnFlushSuccess()
					}
					fe.FlushC <- err
				}
			}
		}
	}()
}
