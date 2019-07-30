package server

import (
	"io"
	"net"
	"runtime"
	"sync/atomic"

	. "git.jd.com/baudtime/baudtime/common/vars"
	"git.jd.com/baudtime/baudtime/util/worker"
	"github.com/go-kit/kit/log/level"
	"github.com/gogo/protobuf/proto"
)

type ReadWriteLoop struct {
	conn       *ProtoConn
	workerPool *worker.WorkerPool
	out        chan proto.Message
	closed     uint32
}

func (loop *ReadWriteLoop) loopWrite() {
	var err error
	var response proto.Message

	for !loop.isExited() {
		response = <-loop.out
		if response != nil {
			err = loop.conn.WriteMsg(response)
			if err != nil {
				if _, ok := err.(net.Error); ok || err == io.EOF || err == io.ErrUnexpectedEOF {
					loop.exit()
					return
				}

				level.Error(Logger).Log("msg", "responsing client failed", "err", err)
			}
		}
	}
}

func (loop *ReadWriteLoop) loopRead() {
	for !loop.isExited() {
		request, err := loop.conn.ReadMsg()
		if err != nil {
			if _, ok := err.(net.Error); ok || err == io.EOF || err == io.ErrUnexpectedEOF {
				loop.exit()
				return
			}

			level.Error(Logger).Log("msg", "reading request failed", "err", err)
			continue
		}

		loop.workerPool.Submit(request)
	}
}

func (loop *ReadWriteLoop) exit() {
	if atomic.CompareAndSwapUint32(&loop.closed, 0, 1) {
		loop.conn.Close()
		loop.workerPool.Shutdown()
		close(loop.out)
	}
}

func (loop *ReadWriteLoop) isExited() bool {
	return atomic.LoadUint32(&loop.closed) == 1
}

func NewReadWriteLoop(conn *ProtoConn, handle func(request proto.Message) proto.Message) *ReadWriteLoop {
	loop := &ReadWriteLoop{
		conn: conn,
		out:  make(chan proto.Message, 1024),
	}

	loop.workerPool = worker.NewWorkerPool("workers_"+conn.RemoteAddr().String(), 5, runtime.GOMAXPROCS(0), func(request interface{}) error {
		response := handle(request.(proto.Message))
		if response != nil {
			loop.out <- response
		}

		return nil
	}, Logger)

	return loop
}
