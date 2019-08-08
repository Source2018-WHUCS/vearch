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
	_ "github.com/blevesearch/bleve/config"
	"github.com/tiglabs/baudengine/config"
	"github.com/tiglabs/baudengine/proto/entity"
	"github.com/tiglabs/baudengine/proto/pspb"
	"github.com/tiglabs/baudengine/ps/engine"
	"github.com/tiglabs/baudengine/ps/engine/mapping"
	"github.com/tiglabs/baudengine/ps/engine/register"
	"github.com/tiglabs/log"
	"io/ioutil"
	"sync"
	"time"
	"unsafe"
)

const Name = "gamma"

var _ engine.Engine = &gammaEngine{}

var indexLocker sync.Mutex

func init() {
	register.Register(Name, New)
}

var logInitOnce sync.Once

func New(cfg register.EngineConfig) (engine.Engine, error) {

	//set log dir
	logInitOnce.Do(func() {
		if rep := C.SetLogDictionary(byteArrayStr(config.Conf().GetLogDir(config.PS))); rep != 0 {
			log.Error("init gamma log has err")
		}
	})

	// init schema make mapping begin
	indexMapping, err := mapping.Space2Mapping(cfg.Space)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	table, e := mapping2Table(cfg, indexMapping)
	if e != nil {
		return nil, e
	}

	defer C.DestroyFieldInfos(table.fields, table.fields_num)
	defer C.DestroyVectorInfos(table.vectors_info, table.vectors_num)

	gammaConfig := C.MakeConfig(byteArrayStr(cfg.Path), C.int(cfg.Space.Engine.MaxSize))
	defer C.DestroyConfig(gammaConfig)
	ge := &gammaEngine{
		ctx:          ctx,
		cancel:       cancel,
		indexMapping: indexMapping,
		space:        cfg.Space,
		partitionID:  cfg.PartitionID,
		gamma:        C.Init(gammaConfig),
	}
	ge.reader = &readerImpl{engine: ge, path: cfg.Path}
	ge.writer = &writerImpl{engine: ge, path: cfg.Path}

	infos, _ := ioutil.ReadDir(cfg.Path)
	if len(infos) == 0 {
		log.Info("to create table for gamma by path:[%s]", cfg.Path)
		if resp := C.CreateTable(ge.gamma, table); resp != 0 {
			return nil, fmt.Errorf("create gamma table has err:[%d]", int(resp))
		}
	}

	go ge.autoCreateIndex()

	if log.IsDebugEnabled() {
		go func() {
			for {
				log.Debug("gamma use memory is:[%d]", C.GetMemoryBytes(ge.gamma))
				time.Sleep(10 * time.Second)
			}
		}()
	}

	return ge, nil
}

type gammaEngine struct {
	ctx          context.Context
	cancel       context.CancelFunc
	indexMapping *mapping.IndexMapping
	space        *entity.Space
	partitionID  entity.PartitionID

	gamma  unsafe.Pointer
	reader *readerImpl
	writer *writerImpl

	buildIndexOnce sync.Once
}

func (ge *gammaEngine) GetSpace() *entity.Space {
	return ge.space
}

func (ge *gammaEngine) GetPartitionID() entity.PartitionID {
	return ge.partitionID
}

func (ge *gammaEngine) Reader() engine.Reader {
	return ge.reader
}

func (ge *gammaEngine) RTReader() engine.RTReader {
	return ge.reader
}

func (ge *gammaEngine) Writer() engine.Writer {
	return ge.writer
}

func (ge *gammaEngine) UpdateMapping(space *entity.Space) error {
	return fmt.Errorf("not support update mapping in gamma")
}

func (ge *gammaEngine) GetMapping() *mapping.IndexMapping {
	return ge.indexMapping
}

func (ge *gammaEngine) MapDocument(doc *pspb.DocCmd) ([]*pspb.Field, map[string]pspb.FieldType, error) {
	return ge.indexMapping.MapDocument(doc.Source)
}

func (ge *gammaEngine) NewSnapshot() (engine.Snapshot, error) {
	panic("implement me")
}

func (ge *gammaEngine) ApplySnapshot(iter engine.Iterator, sn int64) error {
	panic("implement me")
}

func (ge *gammaEngine) Optimize() error {
	go func() {
		ge.buildIndexOnce.Do(func() {
			log.Info("build index:[%d] begin", ge.partitionID)
			if e1 := ge.BuildIndex(); e1 != nil {
				log.Error("build index:[%d] has err ", ge.partitionID, e1.Error())
			}
			log.Info("build index:[%d] end", ge.partitionID)
		})
	}()
	return nil
}

func (ge *gammaEngine) BuildIndex() error {
	indexLocker.Lock()
	defer indexLocker.Unlock()

	//UNINDEXED = 0, INDEXING, INDEXED
	go func() {
		rc := C.BuildIndex(ge.gamma)
		if rc != 0 {
			log.Error("build index:[%d] err response code:[%d]", ge.partitionID, rc)
		}
	}()
	for {
		s := C.GetIndexStatus(ge.gamma)
		log.Info("index:[%d] status is %d", ge.partitionID, int(s))

		if int(s) == 2 {
			log.Info("index:[%d] ok", ge.partitionID)
			break
		}

		time.Sleep(3 * time.Second)
	}

	return nil
}

func (ge *gammaEngine) Close() {
	C.Close(ge.gamma)
}

func (ge *gammaEngine) autoCreateIndex() {

	if ge.space.Engine.IndexSize <= 0 {
		return
	}

	for {
		if u, err := ge.reader.DocCount(ge.ctx); err != nil {
			log.Error("auto create index err :[%s]", err.Error())
		} else if int64(u) >= ge.space.Engine.IndexSize {
			if err := ge.Optimize(); err != nil {
				log.Error("auto create index err :[%s]", err.Error())
			}
			break
		}
		time.Sleep(1 * time.Second)
	}
}
