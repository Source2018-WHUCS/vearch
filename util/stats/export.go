/*
Copyright 2017 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package stats

import (
	"sync"
	"time"

	"github.com/tiglabs/log"
)

type PushBackend interface {
	PushAll() error
}

var pushBackends = make(map[string]PushBackend)
var pushBackendsLock sync.Mutex
var once sync.Once

func RegisterPushBackend(name string, backend PushBackend, statsEmitPeriod string) {
	pushBackendsLock.Lock()
	defer pushBackendsLock.Unlock()
	if _, ok := pushBackends[name]; ok {
		//log.Fatalf("PushBackend %s already exists; can't register the same name multiple times", name)
		log.Error("PushBackend %s already exists; can't register the same name multiple times", name)
	}

	emitPeriod, err := time.ParseDuration(statsEmitPeriod)
	if err != nil {
		log.Error("pushbackend  period config error %s ", statsEmitPeriod)
		return
	}

	pushBackends[name] = backend
	once.Do(func() {
		go emitToBackend(emitPeriod)
	})

	log.Info("regist %s pushbackend %s ", name, statsEmitPeriod)
}

func emitToBackend(emitPeriod time.Duration) {
	ticker := time.NewTicker(emitPeriod)
	defer ticker.Stop()
	for range ticker.C {
		for name, backend := range pushBackends {
			go func(name string, backEnd PushBackend) {
				err := backEnd.PushAll()
				if err != nil {
					log.Warn("Pushing stats to backend %v failed: %v", name, err)
				}
			}(name, backend)
		}
	}
}
