package metisDB

import (
	"bytes"
	"sync"

	"github.com/tiglabs/caprice/index/store/metisDB/metis"
)

type Iterator struct {
	close  sync.Once
	iter   *metis.Iterator
	prefix []byte
	start  []byte
	end    []byte
	valid  bool
	key    []byte
	val    []byte
}

func NewIterator(iter *metis.Iterator, prefix, start, end []byte) *Iterator {
	return &Iterator{iter: iter, prefix: prefix, start: start, end: end}
}

func (i *Iterator) updateValid() {
	i.valid = (i.key != nil)
	if i.valid {
		if i.prefix != nil {
			i.valid = bytes.HasPrefix(i.key, i.prefix)
		} else if i.end != nil {
			i.valid = bytes.Compare(i.key, i.end) < 0
		}
	}
}

func (i *Iterator) Seek(k []byte) {
	if i == nil {
		return
	}
	if i.start != nil && bytes.Compare(k, i.start) < 0 {
		k = i.start
	}
	if i.prefix != nil && !bytes.HasPrefix(k, i.prefix) {
		if bytes.Compare(k, i.prefix) < 0 {
			k = i.prefix
		} else {
			i.valid = false
			return
		}
	}
	var err error
	i.iter.Seek(k)
	if i.iter.Valid() {
		//i.valid = true
		item := i.iter.Item()
		i.key = item.Key()
		err = item.Value(func(val []byte) error {
			i.val = val
			return nil
		})
		if err != nil {
			i.key = nil
			i.val = nil
		}
	} else {
		i.key = nil
		i.val = nil
	}
	i.updateValid()
}

func (i *Iterator) Next() {
	if i == nil {
		return
	}
	var err error
	i.iter.Next()
	if i.iter.Valid() {
		item := i.iter.Item()
		i.key = item.Key()
		err = item.Value(func(val []byte) error {
			i.val = val
			return nil
		})
		if err != nil {
			i.key = nil
			i.val = nil
		}
	} else {
		i.key = nil
		i.val = nil
	}
	i.updateValid()
}

func (i *Iterator) Current() ([]byte, []byte, bool) {
	if i == nil {
		return nil, nil, false
	}
	return i.key, i.val, i.valid
}

func (i *Iterator) Key() []byte {
	if i == nil {
		return nil
	}
	return i.key
}

func (i *Iterator) Value() []byte {
	if i == nil {
		return nil
	}
	return i.val
}

func (i *Iterator) Valid() bool {
	if i == nil {
		return false
	}
	return i.valid
}

func (i *Iterator) Close() error {
	if i == nil {
		return nil
	}
	i.close.Do(func() {
		if i.iter != nil {
			i.iter.Close()
		}
	})

	return nil
}
