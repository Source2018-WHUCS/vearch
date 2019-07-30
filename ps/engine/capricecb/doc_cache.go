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
	"sync"

	"github.com/tiglabs/baudengine/proto/pspb"
)

type documentCache struct {
	lhs, rhs map[string]*pspb.DocCmd
	rwMutex  sync.RWMutex
	flushMu  sync.Mutex
	lhsInSrv bool
	flushing bool
}

func NewDocCache() *documentCache {
	return &documentCache{
		lhs:      make(map[string]*pspb.DocCmd),
		rhs:      make(map[string]*pspb.DocCmd),
		lhsInSrv: true,
	}
}

func (dc *documentCache) Get(docId string) (*pspb.DocCmd, bool) {
	dc.rwMutex.RLock()
	defer dc.rwMutex.RUnlock()
	if dc.isFlushing() {
		docCmd, hit := dc.tmpCache()[docId]
		if hit {
			return docCmd, hit
		}
	}
	docCmd, hit := dc.servingCache()[docId]
	return docCmd, hit
}

func (dc *documentCache) Put(docId string, docCmd *pspb.DocCmd) {
	dc.rwMutex.Lock()
	defer dc.rwMutex.Unlock()
	if dc.isFlushing() {
		dc.tmpCache()[docId] = docCmd
		return
	}
	dc.servingCache()[docId] = docCmd
}

func (dc *documentCache) Size() int {
	dc.rwMutex.RLock()
	defer dc.rwMutex.RUnlock()
	if dc.flushing {
		return len(dc.servingCache()) + len(dc.tmpCache())
	}
	return len(dc.servingCache())
}

// shift si not thread safe, but shift only called in thread safe function onFlushSucess
func (dc *documentCache) shift() {
	defer func() {
		dc.close(dc.servingCache())
		dc.lhsInSrv = !dc.lhsInSrv
	}()

	if dc.lhsInSrv {
		dc.lhs = make(map[string]*pspb.DocCmd)
		return
	}

	dc.rhs = make(map[string]*pspb.DocCmd)
}

func (dc *documentCache) servingCache() map[string]*pspb.DocCmd {
	if dc.lhsInSrv {
		return dc.lhs
	}
	return dc.rhs
}

func (dc *documentCache) nonServingCache() map[string]*pspb.DocCmd {
	if dc.lhsInSrv {
		return dc.rhs
	}
	return dc.lhs
}

// tmpCache used in flush process, tmp cache is non serving cache
// tmpCache is not thread safe, tmpCache should be called in thread safe function
func (dc *documentCache) tmpCache() map[string]*pspb.DocCmd {
	if !dc.isFlushing() {
		panic("tmp cache called out of flushing process")
	}
	return dc.nonServingCache()
}

func (dc *documentCache) clearTmpCache() {
	if !dc.isFlushing() {
		panic("tmp cache called out of flushing process")
	}
	if dc.lhsInSrv {
		dc.rhs = make(map[string]*pspb.DocCmd)
		return
	}
	dc.lhs = make(map[string]*pspb.DocCmd)
}

func (dc *documentCache) isFlushing() bool {
	return dc.flushing
}

func (dc *documentCache) BeforeFlush() {
	dc.flushMu.Lock()
	dc.rwMutex.Lock()
	defer dc.rwMutex.Unlock()
	dc.flushing = true
}

func (dc *documentCache) OnFlushSuccess() {
	dc.rwMutex.Lock()
	defer dc.rwMutex.Unlock()
	defer dc.flushMu.Unlock()
	dc.flushing = false
	dc.shift()
}

func (dc *documentCache) OnFlushFailed() {
	dc.rwMutex.Lock()
	defer dc.rwMutex.Unlock()
	defer dc.flushMu.Unlock()
	tmpCache := dc.tmpCache()
	servingCache := dc.servingCache()
	for docId, docCmd := range tmpCache {
		servingCache[docId] = docCmd
	}
	dc.clearTmpCache()
	dc.flushing = false
}

func (dc *documentCache) close(cmds map[string]*pspb.DocCmd) {
	for _, cmd := range cmds {
		if cmd != nil {
			cmd.Reset()
		}
	}
}
