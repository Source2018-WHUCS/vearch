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
	"bytes"
	"encoding/hex"
	"sort"
	"sync/atomic"

	"github.com/pkg/errors"
	"github.com/tiglabs/caprice/index/store/metisDB/metis/y"
	"github.com/tiglabs/caprice/util"
)

type oracle struct {
	nextTxnTs uint64
}

func newOracle() *oracle {
	orc := &oracle{}
	return orc
}

func (o *oracle) readTs() uint64 {
	var readTs uint64
	readTs = atomic.LoadUint64(&o.nextTxnTs) - 1
	return readTs
}

func (o *oracle) newCommitTs() uint64 {
	var ts uint64
	// This is the general case, when user doesn't specify the read and commit ts.
	ts = atomic.AddUint64(&o.nextTxnTs, 1)
	return ts
}

func (o *oracle) doneCommit() {
	atomic.AddUint64(&o.nextTxnTs, 1)
}

type Txn struct {
	readTs   uint64
	commitTs uint64

	update bool // update is used to conditionally keep track of reads.

	pendingWrites map[string]*Entry // cache stores any writes done by txn.

	db        *DB
	discarded bool

	size         int64
	count        int64
	numIterators int32
}

type pendingWritesIterator struct {
	entries  []*Entry
	nextIdx  int
	readTs   uint64
	reversed bool
}

func (pi *pendingWritesIterator) Next() {
	pi.nextIdx++
}

func (pi *pendingWritesIterator) Rewind() {
	pi.nextIdx = 0
}

func (pi *pendingWritesIterator) Seek(key []byte) {
	key = y.ParseKey(key)
	pi.nextIdx = sort.Search(len(pi.entries), func(idx int) bool {
		cmp := bytes.Compare(pi.entries[idx].Key, key)
		if !pi.reversed {
			return cmp >= 0
		}
		return cmp <= 0
	})
}

func (pi *pendingWritesIterator) Key() []byte {
	y.AssertTrue(pi.Valid())
	entry := pi.entries[pi.nextIdx]
	return y.KeyWithTs(entry.Key, pi.readTs)
}

func (pi *pendingWritesIterator) Value() y.ValueStruct {
	y.AssertTrue(pi.Valid())
	entry := pi.entries[pi.nextIdx]
	return y.ValueStruct{
		Value:   entry.Value,
		Meta:    entry.meta,
		Version: pi.readTs,
	}
}

func (pi *pendingWritesIterator) Valid() bool {
	return pi.nextIdx < len(pi.entries)
}

func (pi *pendingWritesIterator) Close() error {
	return nil
}

func (txn *Txn) newPendingWritesIterator(reversed bool) *pendingWritesIterator {
	if !txn.update || len(txn.pendingWrites) == 0 {
		return nil
	}
	entries := make([]*Entry, 0, len(txn.pendingWrites))
	for _, e := range txn.pendingWrites {
		entries = append(entries, e)
	}
	// Number of pending writes per transaction shouldn't be too big in general.
	sort.Slice(entries, func(i, j int) bool {
		cmp := bytes.Compare(entries[i].Key, entries[j].Key)
		if !reversed {
			return cmp < 0
		}
		return cmp > 0
	})
	return &pendingWritesIterator{
		readTs:   txn.readTs,
		entries:  entries,
		reversed: reversed,
	}
}

func (txn *Txn) checkSize(e *Entry) error {
	count := txn.count + 1
	// Extra bytes for version in key.
	size := txn.size + int64(e.estimateSize()) + 10
	if count >= txn.db.opt.maxBatchCount || size >= txn.db.opt.maxBatchSize {
		return ErrTxnTooBig
	}
	txn.count, txn.size = count, size
	return nil
}

// Set adds a key-value pair to the database.
//
// It will return ErrReadOnlyTxn if update flag was set to false when creating the
// transaction.
//
// The current transaction keeps a reference to the key and val byte slice
// arguments. Users must not modify key and val until the end of the transaction.
func (txn *Txn) Set(key, val []byte) error {
	e := &Entry{
		Key:   y.SafeCopy(nil, key),
		Value: y.SafeCopy(nil, val),
	}
	return txn.SetEntry(e)
}

func exceedsSize(prefix string, max int64, key []byte) error {
	return errors.Errorf("%s with size %d exceeded %d limit. %s:\n%s",
		prefix, len(key), max, prefix, hex.Dump(key[:1<<10]))
}

func (txn *Txn) modify(e *Entry) error {
	const maxKeySize = 65000

	switch {
	case !txn.update:
		return ErrReadOnlyTxn
	case txn.discarded:
		return ErrDiscardedTxn
	case len(e.Key) == 0:
		return ErrEmptyKey
	case len(e.Key) > maxKeySize:
		// Key length can't be more than uint16, as determined by table::header.  To
		// keep things safe and allow badger move prefix and a timestamp suffix, let's
		// cut it down to 65000, instead of using 65536.
		return exceedsSize("Key", maxKeySize, e.Key)
	}
	if atomic.LoadInt32(&txn.numIterators) > 1 {
		return ErrWriteReject
	}

	if err := txn.checkSize(e); err != nil {
		return err
	}
	txn.pendingWrites[string(e.Key)] = e
	return nil
}

// SetEntry takes an Entry struct and adds the key-value pair in the struct,
// along with other metadata to the database.
//
// The current transaction keeps a reference to the entry passed in argument.
// Users must not modify the entry until the end of the transaction.
func (txn *Txn) SetEntry(e *Entry) error {
	return txn.modify(e)
}

// Delete deletes a key.
//
// This is done by adding a delete marker for the key at commit timestamp.  Any
// reads happening before this timestamp would be unaffected. Any reads after
// this commit would see the deletion.
//
// The current transaction keeps a reference to the key byte slice argument.
// Users must not modify the key until the end of the transaction.
func (txn *Txn) Delete(key []byte) error {
	e := &Entry{
		Key:  y.SafeCopy(nil, key),
		meta: bitDelete,
	}
	return txn.modify(e)
}

// Get looks for key and returns corresponding Item.
// If key is not found, ErrKeyNotFound is returned.
func (txn *Txn) Get(key []byte) (item *Item, rerr error) {
	if len(key) == 0 {
		return nil, ErrEmptyKey
	} else if txn.discarded {
		return nil, ErrDiscardedTxn
	}

	if txn.update {
		item, rerr = txn.getFromPendingWrites(key)
		if rerr != nil {
			return
		}
		if item != nil {
			return
		}
	}

	seek := y.KeyWithTs(key, txn.readTs)
	vs, err := txn.db.get(seek)
	if err != nil {
		return nil, util.StackErrorFrom(err)
	}
	if vs.Value == nil && vs.Meta == 0 {
		return nil, ErrKeyNotFound
	}
	if isDeleted(vs.Meta) {
		return nil, ErrKeyNotFound
	}
	item = new(Item)
	item.key = key
	item.version = vs.Version
	item.meta = vs.Meta
	// copy
	item.val = y.SafeCopy(item.val, vs.Value) // TODO: Do we need to copy this over?
	return item, nil
}

func (txn *Txn) getFromPendingWrites(key []byte) (item *Item, rerr error) {
	if e, has := txn.pendingWrites[string(key)]; has && bytes.Equal(key, e.Key) {
		if isDeleted(e.meta) {
			return nil, ErrKeyNotFound
		}
		item = new(Item)
		// Fulfill from cache.
		item.meta = e.meta
		item.val = y.SafeCopy(item.val, e.Value)
		item.key = key
		item.version = txn.readTs
		// We probably don't need to set db on item here.
		return item, nil
	}
	return
}

// Discard discards a created transaction. This method is very important and must be called. Commit
// method calls this internally, however, calling this multiple times doesn't cause any issues. So,
// this can safely be called via a defer right when transaction is created.
//
// NOTE: If any operations are run on a discarded transaction, ErrDiscardedTxn is returned.
func (txn *Txn) Discard() {
	if txn.discarded { // Avoid a re-run.
		return
	}
	if atomic.LoadInt32(&txn.numIterators) > 0 {
		panic("Unclosed iterator at time of Txn.Discard.")
	}
	txn.discarded = true
	txn.pendingWrites = nil
}

func (txn *Txn) prepare() ([]*Entry, error) {
	orc := txn.db.orc

	commitTs := orc.newCommitTs()
	if commitTs == 0 {
		return nil, ErrConflict
	}

	entries := make([]*Entry, 0, len(txn.pendingWrites)+1)
	for _, e := range txn.pendingWrites {
		// fmt.Fprintf(&b, "[%q : %q], ", e.Key, e.Value)

		// Suffix the keys with commit ts, so the key versions are sorted in
		// descending order of commit timestamp.
		e.Key = y.KeyWithTs(e.Key, commitTs)
		entries = append(entries, e)
	}
	return entries, nil
}

func (txn *Txn) Commit() error {
	if txn.discarded {
		panic("Trying to commit a discarded txn")
	}
	defer txn.Discard()

	if len(txn.pendingWrites) == 0 {
		return nil // Nothing to do.
	}
	entries, err := txn.prepare()
	if err != nil {
		return err
	}
	defer func() { txn.db.orc.doneCommit() }()
	return txn.db.write(entries)
}

// ReadTs returns the read timestamp of the transaction.
func (txn *Txn) ReadTs() uint64 {
	return txn.readTs
}

func (txn *Txn) Size() int64 {
	return txn.db.Size()
}

func (db *DB) NewTransaction(update bool) *Txn {
	return db.newTransaction(update)
}

func (db *DB) newTransaction(update bool) (txn *Txn) {
	txn = &Txn{
		update: update,
		db:     db,
	}
	if update {
		txn.pendingWrites = make(map[string]*Entry)
	}

	txn.readTs = db.orc.readTs()

	return
}

func (db *DB) View(fn func(txn *Txn) error) error {
	var txn *Txn
	txn = db.NewTransaction(false)
	defer txn.Discard()

	return fn(txn)
}

// Update executes a function, creating and managing a read-write transaction
// for the user. Error returned by the function is relayed by the Update method.
// Update cannot be used with managed transactions.
func (db *DB) Update(fn func(txn *Txn) error) error {
	txn := db.NewTransaction(true)
	defer txn.Discard()

	if err := fn(txn); err != nil {
		return err
	}

	return txn.Commit()
}
