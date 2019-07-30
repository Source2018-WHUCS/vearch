package metisDB

import (
	"github.com/tiglabs/caprice/index/store/metisDB/metis"
)

type Txn struct {
	txn *metis.Txn
}

func (t *Txn) Set(key, val []byte) error {
	return t.txn.Set(key, val)
}

// Delete removes the specified key
// the key []byte may be reused as soon as this call returns
func (t *Txn) Delete(key []byte) error {
	return t.txn.Delete(key)
}

func (t *Txn) Commit() error {
	return t.txn.Commit()
}

func (t *Txn) Discard() {
	t.txn.Discard()
}

// Get returns the value associated with the key
// If the key does not exist, nil is returned.
// The caller owns the bytes returned.
func (t *Txn) Get(key []byte) ([]byte, error) {
	it, err := t.txn.Get(key)
	if err == metis.ErrKeyNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return it.ValueCopy([]byte(nil))
}

// PrefixIterator returns a KVIterator that will
// visit all K/V pairs with the provided prefix
func (t *Txn) PrefixIterator(prefix []byte) *Iterator {
	opts := metis.DefaultIteratorOptions
	it := t.txn.NewIterator(opts)
	rv := NewIterator(it, prefix, nil, nil)

	rv.Seek(prefix)
	return rv
}

// RangeIterator returns a KVIterator that will
// visit all K/V pairs >= start AND < end
func (t *Txn) RangeIterator(start, end []byte) *Iterator {
	opts := metis.DefaultIteratorOptions
	it := t.txn.NewIterator(opts)
	rv := NewIterator(it, nil, start, end)
	rv.Seek(start)
	return rv
}

func (t *Txn) Size() int64 {
	return t.txn.Size()
}
