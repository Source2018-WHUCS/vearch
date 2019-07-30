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

package util

import (
	"context"
	"sync"
	"time"
	"math"
	"math/rand"
)

var retryPool = &sync.Pool{
	New: func() interface{} {
		return new(Retry)
	},
}

var retryClosedCh = func() <-chan time.Time {
	c := make(chan time.Time)
	close(c)
	return c
}()

// DefaultRetryOption reusable default configuration
var DefaultRetryOption = RetryOption{
	MaxRetries:  3,
	InitBackoff: 50 * time.Millisecond,
	MaxBackoff:  2 * time.Second,
	MaskBackoff: 2,
	RandFactor:  0.15,
}

// RetryOption provides reusable configuration of Retry objects.
type RetryOption struct {
	MaxRetries  int
	InitBackoff time.Duration
	MaxBackoff  time.Duration
	MaskBackoff float64
	RandFactor  float64
	Context     context.Context
}

// Retry implements  an exponential backoff retry loop.
type Retry struct {
	option  RetryOption
	current int
	isStart bool
}

// NewRetry returns a new Retry initialized to some default values.
func NewRetry(opt *RetryOption) *Retry {
	if opt.InitBackoff == 0 {
		opt.InitBackoff = 50 * time.Millisecond
	}
	if opt.MaxBackoff == 0 {
		opt.MaxBackoff = 2 * time.Second
	}
	if opt.MaskBackoff == 0 {
		opt.MaskBackoff = 2
	}
	if opt.RandFactor == 0 {
		opt.RandFactor = 0.15
	}
	if opt.Context == nil {
		opt.Context = context.Background()
	}

	r := retryPool.Get().(*Retry)
	r.option = *opt
	r.Reset()
	return r
}

// Reset reset to initial state
func (r *Retry) Reset() {
	select {
	case <-r.option.Context.Done():
		return

	default:
		r.current = 0
		r.isStart = true
	}
}

// Stop stop retry loop
func (r *Retry) Stop() {
	r.isStart = false
}

func (r *Retry) Close() error {
	r.Stop()
	retryPool.Put(r)
	return nil
}

// Next returns whether the retry loop should continue and the current retry times.
func (r *Retry) Next() (bool, int) {
	if !r.isStart {
		return false, r.current
	} else if r.current == 0 {
		r.current++
		return true, r.current
	}
	if r.option.MaxRetries > 0 && r.current >= r.option.MaxRetries {
		return false, r.current
	}

	// Wait before retry.
	select {
	case <-time.After(r.retryInterval()):
		r.current++
		return true, r.current
	case <-r.option.Context.Done():
		return false, r.current
	}
}

// NextCh returns a channel which will receive when the next retry interval has expired.
func (r *Retry) NextCh() (<-chan time.Time, int) {
	if r.isStart == false || (r.option.MaxRetries > 0 && r.current >= r.option.MaxRetries) {
		return nil, r.current
	}

	r.current++
	if r.current == 1 {
		return retryClosedCh, r.current
	}
	return time.After(r.retryInterval()), r.current
}

// RetryMaxAttempt is a helper that runs fn N times and collects the last err.
func RetryMaxAttempt(opt *RetryOption, fn func() error) error {
	var err error
	r := NewRetry(opt)
	ok, _ := r.Next()

	for ok {
		err = fn()
		if err == nil {
			return nil
		}
		ok, _ = r.Next()
	}

	r.Close()
	return err
}

func RetryUntilSuccess(opt *RetryOption, fn func() error) error {
	r := NewRetry(opt)
	var err error
	ok, _ := r.Next()
	for ok {
		if err = fn();err == nil {
			break
		}
		ok, _ = r.Next()
	}
	return err
}

// RetryDuration will retry the given function until the given duration has elapsed.
func RetryDuration(duration time.Duration, fn func() error) error {
	deadline := time.Now().Add(duration)
	var lastErr error
	for wait := time.Duration(1); time.Now().Before(deadline); wait *= 2 {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		if wait > time.Second {
			wait = time.Second
		}
		time.Sleep(wait)
	}
	return lastErr
}
//retryInterval get the interval for next retry
func (r *Retry) retryInterval() time.Duration {
	backoff := float64(r.option.InitBackoff) * math.Pow(r.option.MaskBackoff, float64(r.current))
	if maxBackoff := float64(r.option.MaxBackoff); backoff > maxBackoff {
		backoff = maxBackoff
	}

	delta := r.option.RandFactor * backoff
	return time.Duration(backoff - delta + rand.Float64()*(2*delta+1))
}
