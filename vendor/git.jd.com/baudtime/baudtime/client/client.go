package client

import (
	"context"
	"io"
	"net"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	pb "git.jd.com/baudtime/baudtime/proxy/proxypb"
	"git.jd.com/baudtime/baudtime/server"
	"github.com/gogo/protobuf/proto"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
)

var timeoutErr = errors.New("timeout")

type future struct {
	opaque uint64
	ch     chan proto.Message
}

func newFuture(opaque uint64) *future {
	return &future{
		opaque: opaque,
		ch:     make(chan proto.Message),
	}
}

func (f *future) Get(ctx context.Context) (proto.Message, error) {
	select {
	case msg := <-f.ch:
		return msg, nil
	case <-ctx.Done():
		return nil, timeoutErr
	}
}

type futureTable struct {
	sync.RWMutex
	futures map[uint64]*future
}

func (ftable *futureTable) add(opaque uint64, f *future) {
	ftable.Lock()
	ftable.futures[opaque] = f
	ftable.Unlock()
}

func (ftable *futureTable) del(opaque uint64) {
	ftable.Lock()
	delete(ftable.futures, opaque)
	ftable.Unlock()
}

func (ftable *futureTable) get(opaque uint64) (*future, bool) {
	ftable.RLock()
	f, ok := ftable.futures[opaque]
	ftable.RUnlock()
	return f, ok
}

type conn struct {
	address string
	*server.ProtoConn
	queryFutures  *futureTable
	writeCallback func(*pb.WriteResponse) error
	exitChan      chan struct{}
	sync.Mutex
	sync.Once
}

func newConn(address string, writeCallback func(*pb.WriteResponse) error) (*conn, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return nil, err
	}

	tcpConn, err := net.DialTCP("tcp4", nil, tcpAddr)
	if err != nil {
		return nil, err
	}
	tcpConn.SetNoDelay(true)
	tcpConn.SetKeepAlive(true)
	tcpConn.SetKeepAlivePeriod(300 * time.Second)
	tcpConn.SetReadBuffer(65535)
	tcpConn.SetWriteBuffer(65535)

	cc := &conn{
		address:       address,
		ProtoConn:     server.NewProtoConn(tcpConn, pb.MsgTypeHandler{}),
		queryFutures:  &futureTable{futures: make(map[uint64]*future)},
		writeCallback: writeCallback,
		exitChan:      make(chan struct{}),
	}
	go cc.loopRead()
	return cc, nil
}

func (c *conn) write(msg proto.Message) error {
	c.Lock()
	err := c.WriteMsg(msg)
	c.Unlock()
	return err
}

func (c *conn) loopRead() {
	for {
		select {
		case <-c.exitChan:
			return
		default:
		}

		response, err := c.ReadMsg()
		if err != nil {
			if _, ok := err.(net.Error); ok || err == io.EOF || err == io.ErrUnexpectedEOF {
				c.close()
				return
			}

			continue
		}

		if response != nil {
			switch response := response.(type) {
			case *pb.QueryResponse:
				if f, ok := c.queryFutures.get(response.Opaque); ok {
					f.ch <- response
				}
			case *pb.WriteResponse:
				c.writeCallback(response)
			}
		}
	}
}

func (c *conn) close() error {
	var err error
	c.Do(func() {
		close(c.exitChan)
		for _, f := range c.queryFutures.futures {
			close(f.ch)
		}
		err = c.Close()
		c.queryFutures = nil
	})
	return err
}

func (c *conn) isClosed() bool {
	select {
	case _, ok := <-c.exitChan:
		return !ok
	default:
		return false
	}
}

type BaudClient struct {
	opaque     uint64
	conns      *sync.Map
	addrProv   ServiceAddrProvider
	onWriteErr func(*pb.WriteResponse) error
}

func New(onWriteErr func(*pb.WriteResponse) error, addrProvider ServiceAddrProvider) *BaudClient {
	c := &BaudClient{
		conns:      new(sync.Map),
		addrProv:   addrProvider,
		onWriteErr: onWriteErr,
	}

	go addrProvider.Watch()
	return c
}

func (cli *BaudClient) InstantQuery(ctx context.Context, query string, time string, timeout string) (*pb.QueryResponse, error) {
	req := &pb.InstantQueryRequest{
		Opaque:  atomic.AddUint64(&cli.opaque, 1),
		Time:    time,
		Timeout: timeout,
		Query:   query,
	}

	c, err := cli.getConn()
	if err != nil {
		return nil, err
	}

	f := newFuture(req.Opaque)
	c.queryFutures.add(req.Opaque, f)
	defer c.queryFutures.del(req.Opaque)

	err = c.write(req)
	if err != nil {
		cli.destroy(c)
		return nil, err
	}

	resp, err := f.Get(ctx)
	if err != nil {
		return nil, err
	}
	return resp.(*pb.QueryResponse), nil
}

func (cli *BaudClient) RangeQuery(ctx context.Context, query string, start string, end string, step string, timeout string) (*pb.QueryResponse, error) {
	req := &pb.RangeQueryRequest{
		Opaque:  atomic.AddUint64(&cli.opaque, 1),
		Start:   start,
		End:     end,
		Step:    step,
		Timeout: timeout,
		Query:   query,
	}

	c, err := cli.getConn()
	if err != nil {
		return nil, err
	}

	f := newFuture(req.Opaque)
	c.queryFutures.add(req.Opaque, f)
	defer c.queryFutures.del(req.Opaque)

	err = c.write(req)
	if err != nil {
		cli.destroy(c)
		return nil, err
	}

	resp, err := f.Get(ctx)
	if err != nil {
		return nil, err
	}
	return resp.(*pb.QueryResponse), nil
}

func (cli *BaudClient) Write(ctx context.Context, series ...*pb.Series) error {
	for _, s := range series {
		sort.Slice(s.Labels, func(i, j int) bool {
			return s.Labels[i].Name < s.Labels[j].Name
		})
	}

	req := &pb.WriteRequest{
		Opaque: atomic.AddUint64(&cli.opaque, 1),
		Series: series,
	}

	c, err := cli.getConn()
	if err != nil {
		return err
	}

	err = c.write(req)
	if err != nil {
		cli.destroy(c)
		return err
	}

	return nil
}

func (cli *BaudClient) Close() error {
	var multiErr error
	cli.conns.Range(func(key, value interface{}) bool {
		err := value.(*conn).close()
		if err != nil {
			multiErr = multierror.Append(multiErr, err)
		}
		return true
	})
	cli.conns = nil
	cli.addrProv.StopWatch()
	return multiErr
}

func (cli *BaudClient) getConn() (*conn, error) {
	address, err := cli.addrProv.GetServiceAddr()
	if err != nil {
		return nil, err
	}

	c, found := cli.conns.Load(address)
	if !found {
		newc, err := newConn(address, cli.onWriteErr)
		if err != nil {
			cli.addrProv.ServiceDown(address)
			return nil, err
		}

		var loaded bool
		c, loaded = cli.conns.LoadOrStore(address, newc)

		if loaded {
			newc.close()
		}
	}
	return c.(*conn), nil
}

func (cli *BaudClient) destroy(c *conn) error {
	cli.conns.Delete(c.address)
	return c.close()
}
