package server

import (
	"net"
	"sync"
	"sync/atomic"
	"time"

	. "git.jd.com/baudtime/baudtime/common/vars"
	"github.com/go-kit/kit/log/level"
)

type TcpServerObserver interface {
	OnStart() error
	OnStop() error
	OnAccept(*net.TCPConn) *ReadWriteLoop
}

const (
	TcpAcceptMinSleep = 10 * time.Millisecond
	TcpAcceptMaxSleep = 1 * time.Second
)

type TcpServer struct {
	port        string
	maxConnNum  int
	tcpListener *net.TCPListener
	srvObserver TcpServerObserver
	loops       map[*ReadWriteLoop]struct{}
	running     uint32
	wg          sync.WaitGroup
	mtx         sync.Mutex
}

func NewTcpServer(port string, maxConnNum int, observer TcpServerObserver) *TcpServer {
	initialCap := maxConnNum
	if initialCap <= 0 {
		initialCap = 10000
	}
	return &TcpServer{
		port:        port,
		maxConnNum:  maxConnNum,
		srvObserver: observer,
		loops:       make(map[*ReadWriteLoop]struct{}, initialCap),
		running:     1,
	}
}

func (s *TcpServer) Run() {
	tcpAddr, err := net.ResolveTCPAddr("tcp", ":"+s.port)
	if err != nil {
		level.Error(Logger).Log("msg", "invalid tcp address")
		return
	}
	s.tcpListener, err = net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		level.Error(Logger).Log("msg", "failed to listen", "err", err)
		return
	}

	err = s.srvObserver.OnStart()
	if err != nil {
		return
	}

	tmpDelay := TcpAcceptMinSleep
	for s.isRunning() {
		tcpConn, err := s.tcpListener.AcceptTCP()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				level.Error(Logger).Log("msg", "temporary error occured to accept conn", "err", ne, "sleepingMs", tmpDelay/time.Millisecond)
				time.Sleep(tmpDelay)
				tmpDelay *= 2
				if tmpDelay > TcpAcceptMaxSleep {
					tmpDelay = TcpAcceptMaxSleep
				}
			} else if s.isRunning() {
				level.Error(Logger).Log("msg", "failed to accept conn", "err", err)
			}
			continue
		}
		tmpDelay = TcpAcceptMinSleep

		if s.maxConnReached() {
			continue
		}

		l := s.srvObserver.OnAccept(tcpConn)
		if l != nil {
			s.addLoop(l)

			s.wg.Add(1)
			go func() {
				l.loopWrite()
				s.removeLoop(l)
				s.wg.Done()
			}()

			s.wg.Add(1)
			go func() {
				l.loopRead()
				s.removeLoop(l)
				s.wg.Done()
			}()
		}
	}

	s.wg.Wait()

	s.srvObserver.OnStop()
}

func (s *TcpServer) Shutdown() {
	if atomic.CompareAndSwapUint32(&s.running, 1, 0) {
		s.tcpListener.Close()
		s.mtx.Lock()
		for loop := range s.loops {
			loop.exit()
		}
		s.loops = nil
		s.mtx.Unlock()
		s.wg.Wait()
	}
}

func (s *TcpServer) isRunning() bool {
	return atomic.LoadUint32(&s.running) == 1
}

func (s *TcpServer) addLoop(loop *ReadWriteLoop) {
	s.mtx.Lock()
	if s.loops != nil {
		s.loops[loop] = struct{}{}
	}
	s.mtx.Unlock()
}

func (s *TcpServer) removeLoop(loop *ReadWriteLoop) {
	s.mtx.Lock()
	if s.loops != nil {
		delete(s.loops, loop)
	}
	s.mtx.Unlock()
}

func (s *TcpServer) maxConnReached() bool {
	s.mtx.Lock()
	reached := s.maxConnNum > 0 && len(s.loops) >= s.maxConnNum
	s.mtx.Unlock()
	return reached
}
