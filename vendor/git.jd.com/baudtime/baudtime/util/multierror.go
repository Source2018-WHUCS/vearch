package util

import (
	"github.com/hashicorp/go-multierror"
	"sync"
)

type MultiError struct {
	err multierror.Error
	sync.RWMutex
}

func (mErr *MultiError) ErrorOrNil() error {
	mErr.RLock()
	defer mErr.RUnlock()
	return mErr.err.ErrorOrNil()
}

func (mErr *MultiError) Append(errs ...error) error {
	mErr.Lock()
	defer mErr.Unlock()
	return multierror.Append(&mErr.err, errs...)
}

func (mErr *MultiError) Error() string {
	mErr.RLock()
	defer mErr.RUnlock()
	return mErr.err.Error()
}
