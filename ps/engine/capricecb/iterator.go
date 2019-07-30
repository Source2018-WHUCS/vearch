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
	"github.com/tiglabs/caprice/index/store"
)

var _ engine.Iterator = &Iterator{}

type Filter interface {
	// return true for filter key
	Filter(key []byte) bool
}

type Iterator struct {
	kvIterator store.KVIterator
	filters    []Filter
}

func (iter *Iterator) Close() error {
	if iter == nil {
		return nil
	}
	return iter.kvIterator.Close()
}

func (iter *Iterator) Next() {
	if iter == nil {
		return
	}
Loop:
	iter.kvIterator.Next()
	for _, f := range iter.filters {
		if f.Filter(iter.kvIterator.Key()) {
			goto Loop
		}
	}
}

func (iter *Iterator) Valid() bool {
	if iter == nil {
		return false
	}
	return iter.kvIterator.Valid()
}

func (iter *Iterator) Key() []byte {
	if iter == nil {
		return nil
	}
	return iter.kvIterator.Key()
}

func (iter *Iterator) Value() []byte {
	if iter == nil {
		return nil
	}
	return iter.kvIterator.Value()
}
