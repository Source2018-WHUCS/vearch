/*
 * Copyright 2017 Dgraph Labs, Inc. and Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package metis

import (
	"sync"
)

// Values have their first byte being byteData or byteDelete. This helps us distinguish between
// a key that has never been seen and a key that has been explicitly deleted.
const (
	bitDelete                 byte = 1 << 0 // Set if the key has been deleted.
)

type Entry struct {
	Key       []byte
	Value     []byte
	meta      byte
}

func (e *Entry) estimateSize() int {
	return len(e.Key) + len(e.Value) + 1 // Meta
}

type request struct {
	// Input values
	Entries    []*Entry
	Wg         sync.WaitGroup
	Err        error

	done       bool
}

func (req *request) Done(err error) {
	if req.done {
		return
	}
	req.done = true
	req.Err = err
	req.Wg.Done()
}

func (req *request) Wait() error {
	req.Wg.Wait()
	req.Entries = nil
	req.done = false
	err := req.Err
	requestPool.Put(req)
	return err
}
