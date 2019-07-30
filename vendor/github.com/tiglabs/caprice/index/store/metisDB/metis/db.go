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
	"github.com/tiglabs/caprice/index/store/metisDB/metis/skl"
	"github.com/tiglabs/caprice/index/store/metisDB/metis/y"
)

// DB provides the various functions required to interact with Badger.
// DB is thread-safe.
type DB struct {
	sync.RWMutex // Guards list of in-memory tables, not individual reads and writes.

	mt      *skl.Skiplist   // Our latest (actively written) in-memory table
	imm     []*skl.Skiplist // Add here only AFTER pushing to flushChan.
	opt     Options

	orc        *oracle
}

// Open returns a new DB object.
func Open(opt Options) (db *DB) {
	opt.maxBatchSize = (15 * opt.MaxTableSize) / 100
	opt.maxBatchCount = opt.maxBatchSize / int64(skl.MaxNodeSize)

	db = &DB{
		imm:     make([]*skl.Skiplist, 0, 5),
		opt:     opt,
		orc:     newOracle(),
	}

	db.mt = skl.NewSkiplist(arenaSize(opt))
	db.orc.nextTxnTs++

	return db
}

// Close closes a DB. It's crucial to call it to ensure all the pending updates
// make their way to disk. Calling DB.Close() multiple times is not safe and would
// cause panic.
func (db *DB) Close() (err error) {
	db.Lock()
	defer db.Unlock()
	if db.mt != nil {
		db.mt.DecrRef()
		db.mt = nil
	}
	for _, mt := range db.imm {
		mt.DecrRef()
	}
	db.imm = db.imm[:0]
	return err
}

// getMemtables returns the current memtables and get references.
func (db *DB) getMemTables() ([]*skl.Skiplist, func()) {
	db.RLock()
	defer db.RUnlock()

	tables := make([]*skl.Skiplist, len(db.imm)+1)

	// Get mutable memtable.
	tables[0] = db.mt
	tables[0].IncrRef()

	// Get immutable memtables.
	if len(db.imm) > 0 {
		last := len(db.imm) - 1
		for i := range db.imm {
			tables[i+1] = db.imm[last-i]
			tables[i+1].IncrRef()
		}
	}

	return tables, func() {
		for _, tbl := range tables {
			tbl.DecrRef()
		}
	}
}

// get returns the value in memtable or disk for given key.
// Note that value will include meta byte.
//
// IMPORTANT: We should never write an entry with an older timestamp for the same key, We need to
// maintain this invariant to search for the latest value of a key, or else we need to search in all
// tables and find the max version among them.  To maintain this invariant, we also need to ensure
// that all versions of a key are always present in the same table from level 1, because compaction
// can push any table down.
//
// Update (Sep 22, 2018): To maintain the above invariant, and to allow keys to be moved from one
// value log to another (while reclaiming space during value log GC), we have logically moved this
// need to write "old versions after new versions" to the badgerMove keyspace. Thus, for normal
// gets, we can stop going down the LSM tree once we find any version of the key (note however that
// we will ALWAYS skip versions with ts greater than the key version).  However, if that key has
// been moved, then for the corresponding movekey, we'll look through all the levels of the tree
// to ensure that we pick the highest version of the movekey present.
func (db *DB) get(key []byte) (y.ValueStruct, error) {
	tables, decr := db.getMemTables() // Lock should be released.
	defer decr()

	var maxVs *y.ValueStruct
	var version uint64
	for i := 0; i < len(tables); i++ {
		vs := tables[i].Get(key)
		if vs.Meta == 0 && vs.Value == nil {
			continue
		}
		// Found a version of the key. For user keyspace, return immediately. For move keyspace,
		// continue iterating, unless we found a version == given key version.
		if maxVs == nil || vs.Version == version {
			return vs, nil
		}
		if maxVs.Version < vs.Version {
			*maxVs = vs
		}
	}
	return y.ValueStruct{}, nil
}

func (db *DB) Size() (size int64) {
	tables, decr := db.getMemTables() // Lock should be released.
	defer decr()
	for _, m := range tables {
		size += m.MemSize()
	}
	return
}

var requestPool = sync.Pool{
	New: func() interface{} {
		return new(request)
	},
}

func (db *DB) writeToLSM(b *request) error {
	for _, entry := range b.Entries {
		db.mt.Put(entry.Key,
			y.ValueStruct{
				Value: entry.Value,
				Meta:  entry.meta,
			})
	}
	return nil
}

func (db *DB) write(entries []*Entry) error {
	var count, size int64
	for _, e := range entries {
		size += int64(e.estimateSize())
		count++
	}
	if count >= db.opt.maxBatchCount || size >= db.opt.maxBatchSize {
		return ErrTxnTooBig
	}

	// We can only service one request because we need each txn to be stored in a contigous section.
	// Txns should not interleave among other txns or rewrites.
	req := requestPool.Get().(*request)
	req.Entries = entries
	defer func() {
		requestPool.Put(req)
	}()
	db.ensureRoomForWrite()
	return db.writeToLSM(req)
}

// ensureRoomForWrite is always called serially.
func (db *DB) ensureRoomForWrite() {
	db.Lock()
	defer db.Unlock()
	if db.mt.MemSize() < db.opt.MaxTableSize {
		return
	}
	y.AssertTrue(db.mt != nil) // A nil mt indicates that DB is being closed.
	db.imm = append(db.imm, db.mt)
	db.mt = skl.NewSkiplist(arenaSize(db.opt))
	return
}

func arenaSize(opt Options) int64 {
	return opt.MaxTableSize + opt.maxBatchSize + opt.maxBatchCount*int64(skl.MaxNodeSize)
}
