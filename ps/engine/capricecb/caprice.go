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
	"fmt"
	_ "github.com/blevesearch/bleve/config"
	"github.com/blevesearch/bleve/registry"
	"github.com/tiglabs/baudengine/ps/engine"
	"github.com/tiglabs/baudengine/ps/engine/mapping"
	"github.com/tiglabs/baudengine/ps/engine/register"
	"github.com/tiglabs/baudengine/proto/entity"
	"github.com/tiglabs/baudengine/proto/pspb"
	"github.com/tiglabs/baudengine/util/baudlog"
	"github.com/tiglabs/caprice"
	"github.com/tiglabs/log"
	"sync"
)

const Name = "caprice"
const NameGuiXu = "guixu"

var _ engine.Engine = &capriceEngine{}

func init() {
	register.Register(Name, New)
	register.Register(NameGuiXu, New)
}

func New(cfg register.EngineConfig) (engine.Engine, error) {

	// init schema make mapping begin
	indexMapping, err := mapping.Space2Mapping(cfg.Space)
	if err != nil {
		return nil, err
	}

	indexConfig := &caprice.IndexConfig{
		Path: cfg.Path,
	}
	if v, ok := cfg.ExtraOptions["disable_auto_compactions"]; ok {
		indexConfig.DisableAutoCompactions = v.(bool)
	}

	index, err := caprice.NewIndexImpl(indexConfig)

	if err != nil {
		log.Error("new caprice failed, unknown  %s", err.Error())
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	ce := &capriceEngine{
		ctx:          ctx,
		cancel:       cancel,
		index:        index,
		indexMapping: indexMapping,
		indexConfig:  indexConfig,
		space:        cfg.Space,
		partitionID:  cfg.PartitionID,
		cache:        registry.NewCache(),
	}

	ce.reader = &readerImpl{capriceEngine: ce, indexReaderConfig: caprice.NewDefaultIndexReaderConfig()}

	indexWriteConfig := caprice.NewDefaultIndexWriterConfig()
	indexWriteConfig.Parallelism = cfg.DWPTNum
	indexWriteConfig.Analyzer = NewCapriceAnalyzer(indexMapping)

	writer, err := index.NewIndexWriter(indexWriteConfig)
	flushEntityC := make(chan *entity.FlushEntity, 1000)

	ce.writer = &writerImpl{capriceEngine: ce, writer: writer, docCache: NewDocCache(), flushEntityC: flushEntityC}

	ce.rtReader = NewRTReader(ce.writer)

	ce.writer.StartHandleCommitJobs()

	return ce, nil
}

type capriceEngine struct {
	ctx          context.Context
	cancel       context.CancelFunc
	index        caprice.Index
	indexMapping *mapping.IndexMapping
	indexConfig  *caprice.IndexConfig
	reader       *readerImpl
	writer       *writerImpl
	rtReader     *rtReaderImpl
	space        *entity.Space
	partitionID  entity.PartitionID
	cache        *registry.Cache
	lock         sync.RWMutex
}

func (ce *capriceEngine) GetSpace() *entity.Space {
	return ce.space
}

func (ce *capriceEngine) GetPartitionID() entity.PartitionID {
	return ce.partitionID
}

func (c *capriceEngine) Reader() engine.Reader {
	return c.reader
}

func (c *capriceEngine) RTReader() engine.RTReader {
	return c.rtReader
}

func (c *capriceEngine) Writer() engine.Writer {
	return c.writer
}

func (c *capriceEngine) MapDocument(doc *pspb.DocCmd) ([]*pspb.Field, map[string]pspb.FieldType, error) {
	return c.indexMapping.MapDocument(doc.Source)
}

func (c *capriceEngine) UpdateMapping(space *entity.Space) error {
	if c.space != nil && c.space.Version > space.Version {
		err := fmt.Errorf("update schema version not right, old:[%d] new:[%d] ", c.space.Version, space.Version)
		log.Warn(err.Error())
		return err
	}

	indexMapping, err := mapping.Space2Mapping(space)
	if err != nil {
		return err
	}
	c.space = space
	c.indexMapping = indexMapping
	return nil
}

func (c *capriceEngine) GetMapping() *mapping.IndexMapping {
	return c.indexMapping
}

func (c *capriceEngine) ApplySnapshot(iter engine.Iterator, sn int64) error {
	panic("TODO ANSJ")
}

func (c *capriceEngine) NewSnapshot() (engine.Snapshot, error) {
	panic("TODO ANSJ")
}

func (c *capriceEngine) Close() {
	c.cancel()
	log.Info("capriceEngine closed, partitionId:[%d]", c.partitionID)
	baudlog.CloseIfNotNil(c.index) //make sure only close this
}

func (c *capriceEngine) Optimize() error {
	err := c.index.Optimize()
	if err != nil {
		return err
	}
	return nil
}
