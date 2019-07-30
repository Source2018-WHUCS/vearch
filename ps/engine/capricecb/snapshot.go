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
	"github.com/tiglabs/baudengine/ps/engine"
	"github.com/tiglabs/caprice"
	"github.com/tiglabs/caprice/index/store"
)

var _ engine.Snapshot = &Snapshot{}

type Snapshot struct {
	sn       uint64
	index    caprice.Index
	kvReader store.KVReader
}

// filter raft log key
func (snapshot *Snapshot) NewIterator() engine.Iterator {
	rangeIterator := snapshot.kvReader.RangeIterator(nil, nil)
	return &Iterator{
		kvIterator: rangeIterator,
		filters:    []Filter{&RaftFilter{}},
	}
}

func (snapshot *Snapshot) Close() error {
	return snapshot.kvReader.Close()
}

// RaftFilter for iterator
type RaftFilter struct{}

func (f *RaftFilter) Filter(key []byte) bool {
	return false
}

type FileSlice struct {
	Dir      string
	FileName string
	Offset   uint64
	Size     uint64
	Buffer   []byte
	Crc32    string
	IsLast   bool
}
