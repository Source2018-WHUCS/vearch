package mutex

import "time"

type Releaser func()

func (m Releaser) Release() {
	if m != nil {
		m()
	}
}

type ParallelMutex interface {
	Len() int
	Cap() int
	TryMutex(timeout time.Duration) (f Releaser, success bool)
	Mutex() Releaser
}

type parallelMutex struct {
	parallelCh chan struct{}
}

func (m *parallelMutex) TryMutex(timeout time.Duration) (f Releaser, success bool) {
	var timer = time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case m.parallelCh <- struct{}{}:
		f = func() {
			<-m.parallelCh
		}
		success = true
		return
	case <-timer.C:
		f = nil
		success = false
		return
	}

}

func (m *parallelMutex) Mutex() Releaser {
	m.parallelCh <- struct{}{}
	return func() {
		<-m.parallelCh
	}
}

func (m *parallelMutex) Len() int {
	return len(m.parallelCh)
}

func (m *parallelMutex) Cap() int {
	return cap(m.parallelCh)
}

func NewParallelMutex(parallelism int) ParallelMutex {
	if parallelism <= 0 {
		panic("parallelism must be larger than 0")
	}
	return &parallelMutex{parallelCh: make(chan struct{}, parallelism)}
}
