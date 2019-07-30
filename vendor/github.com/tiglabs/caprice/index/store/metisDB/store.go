package metisDB

import (
	"github.com/tiglabs/caprice/index/store/metisDB/metis"
	"github.com/tiglabs/caprice/index/store"
)

type Store struct {
	db *metis.DB
}

func (s *Store) Writer() (store.KVWriter, error) {
	txn, err := s.NewTxn(true)
	if err != nil {
		return nil, err
	}
	return &Writer{txn: txn}, nil
}

func (s *Store) Reader() (store.KVReader, error) {
	txn, err := s.NewTxn(false)
	if err != nil {
		return nil, err
	}
	return &Reader{txn: txn}, nil
}

func New(config *Config) *Store {
	opts := metis.DefaultOptions
	applyConfig(&opts, config)
	db := metis.Open(opts)
	rv := Store{
		db: db,
	}
	return &rv
}

func (s *Store) NewTxn(update bool) (*Txn, error) {
	txn := s.db.NewTransaction(update)
	return &Txn{txn: txn}, nil
}

func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	return s.db.Close()
}

type Writer struct {
	txn *Txn
}

func (w *Writer) Set(key, val []byte) {
	w.txn.Set(key, val)
}

func (w *Writer) Delete(key []byte) {
	w.txn.Delete(key)
}

func (w *Writer) Commit() error {
	return w.txn.Commit()
}

func (w *Writer) Discard() {
	w.txn.Discard()
}

type Reader struct {
	txn *Txn
}

func (r *Reader) Get(key []byte) ([]byte, error) {
	return r.txn.Get(key)
}

func (r *Reader) PrefixIterator(prefix []byte) store.KVIterator {
	return r.txn.PrefixIterator(prefix)
}

func (r *Reader) RangeIterator(start, end []byte) store.KVIterator {
	return r.txn.RangeIterator(start, end)
}

func (r *Reader) Size() int64 {
	return r.txn.Size()
}

func (r *Reader) Close() error {
	r.txn.Discard()
	return nil
}
