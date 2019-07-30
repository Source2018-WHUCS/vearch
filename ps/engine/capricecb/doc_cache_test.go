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
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/tiglabs/baudengine/proto/pspb"
	"github.com/tiglabs/baudengine/util/atomic"
)

func TestDocumentCache(t *testing.T) {
	times := atomic.NewAtomicInt64(0)
	finish := atomic.NewAtomicBool(false)
	docCache := NewDocCache()

	wg := sync.WaitGroup{}

	for p := 0; p < 5; p++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tick := time.NewTicker(time.Microsecond)
			for {
				select {
				case <-tick.C:
					docCache.BeforeFlush()
					if rand.Int()%5 == 0 {
						docCache.OnFlushFailed()
					} else {
						times.Add(1)
						docCache.OnFlushSuccess()
					}
					if finish.Get() {
						return
					}
				}
			}
		}()
	}

	for p := 0; p < 20; p++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer finish.Set(true)
			for i := 0; i < 100000; i++ {
				before := times.Get()
				docCache.Put(strconv.Itoa(i%10000), &pspb.DocCmd{DocId: strconv.Itoa(i % 10000)})
				docCmd, hit := docCache.Get(strconv.Itoa(i % 10000))
				if !hit {
					after := times.Get()
					if before == after {
						t.Errorf("doc cache hit failed, i：%v, doc cmd :%v", i, docCmd)
					}
				}
			}
		}()
	}
	wg.Wait()
}
