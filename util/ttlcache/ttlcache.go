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

package ttlcache

import (
	"sync"
	"time"
	"fmt"
)

const (
	TTLCACHE_MAX_CAPACITY = 1000000
	TTLCACHE_CLEAR_INTERVAL = 30 * time.Second
)

type TTLNode struct {
	content   interface{}
	deathTime time.Time
}

type TTLCache struct {
	destroyCh chan struct{}
	lock      sync.RWMutex
	timeout   time.Duration
	m         map[string]*TTLNode
}

func NewTTLCache(timeout time.Duration) *TTLCache {
	cache := &TTLCache{
		destroyCh: make(chan struct{}),
		timeout:   timeout,
		m:         make(map[string]*TTLNode),
	}

	//go func() {
	//	scheTimer := time.NewTimer(TTLCACHE_CLEAR_INTERVAL)
	//
	//	for {
	//		select {
	//		case <-cache.destroyCh:
	//			scheTimer.Stop()
	//			return
	//
	//		case <-scheTimer.C:
	//			curTime := time.Now()
	//			cache.lock.Lock()
	//			for k, v := range cache.m {
	//				if v.deathTime.Before(curTime) {
	//					delete(cache.m, k)
	//				}
	//			}
	//			cache.lock.Unlock()
	//		}
	//		scheTimer.Reset(TTLCACHE_CLEAR_INTERVAL)
	//	}
	//}()

	return cache
}

func (tc *TTLCache) Destroy() {
	close(tc.destroyCh)
}

func (tc *TTLCache) Size() int {
	tc.lock.RLock()
	defer tc.lock.RUnlock()

	return len(tc.m)
}

func (tc *TTLCache) Put(key string, obj interface{}) {
	tc.lock.Lock()
	defer tc.lock.Unlock()
	var node *TTLNode
	var find bool
	now := time.Now()
	if node, find = tc.m[key]; find {
		//未超时
		if node.deathTime.After(time.Now()) {
			node.deathTime = now.Add(tc.timeout)
			node.content = obj
		}
	}
	tc.m[key] = &TTLNode{
		content:    obj,
		deathTime:  now.Add(tc.timeout),
	}

	if len(tc.m) >= TTLCACHE_MAX_CAPACITY {
		fmt.Println("Out of ttlcache")
	}
}


func (tc *TTLCache) Get(key string) (interface{}, bool) {
	node, ok := tc.get(key)
	if ok {
		if node.deathTime.After(time.Now()) {
			return node.content, true
		}
		return func () (interface{}, bool){
			tc.lock.Lock()
			defer tc.lock.Unlock()
			node, ok := tc.m[key]
			if ok {
				if node.deathTime.After(time.Now()) {
					return node.content, true
				}
			}
			// delete(tc.m, key)
			return nil, false
		}()
	}
	return nil, false
}

func (tc *TTLCache) Delete(key string) {
	tc.lock.Lock()
	defer tc.lock.Unlock()
	delete(tc.m, key)
}

func (tc *TTLCache) Values() []interface{} {
	var values []interface{}
	var currentTime = time.Now()

	tc.lock.RLock()
	defer tc.lock.RUnlock()
	for _, node := range tc.m {
		if node.deathTime.After(currentTime) {
			values = append(values, node.content)
		}
	}
	return values
}

func (tc *TTLCache) Delay(key string) (interface{}, bool) {
	tc.lock.Lock()
	defer tc.lock.Unlock()
	var node *TTLNode
	var find bool
	node, find = tc.m[key]
	if find {
		node.deathTime = time.Now()
		return node.content, true
	} else {
		return nil, false
	}
}

func (tc *TTLCache) get(key string) (*TTLNode, bool) {
	tc.lock.RLock()
	defer tc.lock.RUnlock()
	node, ok := tc.m[key]
	if ok {
		return node, true
	}
	return nil, false
}

