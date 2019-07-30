package worker

import (
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

type worker struct {
	ch      chan interface{}
	handle  func(interface{}) error
	belong  *WorkerPool
	running uint32
}

func (w *worker) run() {
	var begin time.Time
	for t := range w.ch {
		if !w.isRunning() {
			return
		}

		begin = time.Now()
		err := w.handle(t)
		if err != nil {
			//w.ch <- t //write sample back for handling again by other worker, but when connection was reset, we should not write back
			//log.WithError(err).Warn("msg was writen back")
			continue
		}
		w.belong.messagesOut.incr(1)
		w.belong.messagesOutDuration.incr(int64(time.Since(begin)))
	}
}

func (w *worker) exit() {
	if atomic.CompareAndSwapUint32(&w.running, 1, 0) {
		close(w.ch)
	}
}

func (w *worker) isRunning() bool {
	return atomic.LoadUint32(&w.running) == 1
}

var (
	ewmaWeight    = 0.2
	adaptDuration = 15 * time.Second

	// Allow 30% too many shards before scaling down.
	workerToleranceFraction = 0.3
)

type WorkerPool struct {
	name                string
	workers             []*worker
	maxWorkerNum        int
	coreWorkerNum       int
	iter                int
	exitCh              chan struct{}
	New                 func() *worker
	messagesIn          *ewmaRate
	messagesOut         *ewmaRate
	messagesOutDuration *ewmaRate
	integralAccumulator float64
	wg                  sync.WaitGroup
	sync.Mutex
	logger log.Logger
}

func NewWorkerPool(name string, coreWorkerNum, maxWorkerNum int, handle func(interface{}) error, logger log.Logger) *WorkerPool {
	if maxWorkerNum > runtime.GOMAXPROCS(0) {
		maxWorkerNum = runtime.GOMAXPROCS(0)
	}

	if coreWorkerNum <= 0 || coreWorkerNum > maxWorkerNum/2 {
		coreWorkerNum = maxWorkerNum / 2
	}

	pool := &WorkerPool{
		name:                name,
		workers:             make([]*worker, 0, maxWorkerNum),
		maxWorkerNum:        maxWorkerNum,
		coreWorkerNum:       coreWorkerNum,
		exitCh:              make(chan struct{}),
		messagesIn:          newEWMARate(ewmaWeight, adaptDuration),
		messagesOut:         newEWMARate(ewmaWeight, adaptDuration),
		messagesOutDuration: newEWMARate(ewmaWeight, adaptDuration),
		logger:              logger,
	}

	pool.New = func() *worker {
		return &worker{
			ch:      make(chan interface{}, 1024),
			handle:  handle,
			belong:  pool,
			running: 1,
		}
	}

	for pool.workerNum() < coreWorkerNum {
		pool.addWorker()
	}

	if maxWorkerNum > 1 {
		go pool.selfAdapt()
	}

	return pool
}

func (pool *WorkerPool) Submit(task interface{}) {
	pool.messagesIn.incr(1)

	pool.Lock()
	workNum := len(pool.workers)
	if workNum > 0 {
		pool.iter = (pool.iter + 1) % workNum
		pool.workers[pool.iter].ch <- task
	}
	pool.Unlock()
}

func (pool *WorkerPool) Shutdown() {
	close(pool.exitCh)
	pool.wg.Wait()
	level.Info(pool.logger).Log("msg", "worker pool exit", "poolName", pool.name)
}

func (pool *WorkerPool) selfAdapt() {
	t := time.NewTicker(adaptDuration)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			pool.messagesIn.tick()
			pool.messagesOut.tick()
			pool.messagesOutDuration.tick()

			// We use the number of incoming samples as a prediction of how much work we
			// will need to do next iteration.  We add to this any pending samples
			// (received - send) so we can catch up with any backlog. We use the average
			// outgoing batch latency to work out how many shards we need.
			var (
				messagesIn          = pool.messagesIn.rate()
				messagesOut         = pool.messagesOut.rate()
				messagesPending     = messagesIn - messagesOut
				messagesOutDuration = pool.messagesOutDuration.rate()
			)

			// We use an integral accumulator, like in a PID, to help dampen oscillation.
			pool.integralAccumulator = pool.integralAccumulator + (messagesPending * 0.1)

			if messagesOut <= 0 {
				continue
			}

			var (
				timePerResponse = messagesOutDuration / messagesOut
				desiredWorkers  = (timePerResponse * (messagesIn + messagesPending + pool.integralAccumulator)) / float64(time.Second)
			)

			currentWorkerNum := pool.workerNum()

			// Changes in the number of shards must be greater than workerToleranceFraction.
			var (
				lowerBound = float64(currentWorkerNum) * (1. - workerToleranceFraction)
				upperBound = float64(currentWorkerNum) * (1. + workerToleranceFraction)
			)

			if lowerBound <= desiredWorkers && desiredWorkers <= upperBound {
				//level.Debug(pool.logger).Log(
				//	"lowerBound", lowerBound,
				//	"desiredWorkers", desiredWorkers,
				//	"upperBound", upperBound,
				//)
				continue
			}

			desiredNum := int(math.Ceil(desiredWorkers))
			if desiredNum > pool.maxWorkerNum {
				desiredNum = pool.maxWorkerNum
			} else if desiredNum < pool.coreWorkerNum {
				desiredNum = pool.coreWorkerNum
			}

			// Resharding can take some time, and we want this loop
			// to stay close to adaptDuration.
			if currentWorkerNum < desiredNum {
				//level.Debug(pool.logger).Log("msg", "currentWorkerNum < desiredNum", "desiredNum", desiredNum)
				for pool.workerNum() < desiredNum {
					pool.addWorker()
				}
			} else if currentWorkerNum > desiredNum {
				//level.Debug(pool.logger).Log("msg", "currentWorkerNum > desiredNum", "desiredNum", desiredNum)
				for pool.workerNum() > desiredNum {
					pool.removeWorker()
				}
			} else {
				//level.Debug(pool.logger).Log("msg", "currentWorkerNum = desiredNum", "desiredNum", desiredNum)
			}
		case <-pool.exitCh:
			pool.Lock()
			for _, wk := range pool.workers {
				wk.exit()
			}
			pool.workers = nil
			pool.Unlock()
			return
		}
	}
}

func (pool *WorkerPool) workerNum() int {
	pool.Lock()
	num := len(pool.workers)
	pool.Unlock()
	return num
}

func (pool *WorkerPool) addWorker() {
	wk := pool.New()

	pool.wg.Add(1)
	go func() {
		wk.run()
		pool.wg.Done()
	}()

	pool.Lock()
	pool.workers = append(pool.workers, wk)
	pool.Unlock()

	//log.Debugf("worker added, executor:%v, count: %v", pool.name, pool.workerNum())
}

func (pool *WorkerPool) removeWorker() {
	var wk *worker

	pool.Lock()
	i := len(pool.workers)
	if i > 0 {
		wk = pool.workers[i-1]
		pool.workers = pool.workers[:i-1]
	}
	pool.Unlock()

	if wk != nil {
		wk.exit()
		wk = nil
	}

	//log.Debugf("worker removed, executor:%v, count: %v", pool.name, pool.workerNum())
}
