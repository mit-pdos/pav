package advrpc

// advrpc provides a basic RPC lib on top of an adversarial network.
// for testing, it returns the right bytes from the right rpc id.
// however, its formal model says that rpc calls return arbitrary bytes.

import (
	"github.com/mit-pdos/pav/marshalutil"
	"github.com/mit-pdos/pav/netffi"
	"github.com/tchajed/marshal"
)

// # Server

type Server struct {
	handlers map[uint64]func([]byte, *[]byte)
}

func (s *Server) handle(conn *netffi.Conn, rpcId uint64, data []byte) {
	resp := new([]byte)
	f, ok0 := s.handlers[rpcId]
	if !ok0 {
		// adv gave bad rpcId.
		return
	}
	f(data, resp)
	// ignore errors. if err, client will timeout, then retry.
	conn.Send(*resp)
}

func (s *Server) read(conn *netffi.Conn) {
	for {
		req, err0 := conn.Receive()
		if err0 {
			// connection done. quit thread.
			break
		}
		rpcId, data, err1 := marshalutil.ReadInt(req)
		if err1 {
			// adv didn't even give rpcId.
			continue
		}
		go func() {
			s.handle(conn, rpcId, data)
		}()
	}
}

func (s *Server) Serve(addr uint64) {
	l := netffi.Listen(addr)
	go func() {
		for {
			conn := l.Accept()
			go func() {
				s.read(conn)
			}()
		}
	}()
}

func NewServer(handlers map[uint64]func([]byte, *[]byte)) *Server {
	return &Server{handlers: handlers}
}

// # Client

// Client is meant for exclusive use.
type Client struct {
	conn *netffi.Conn
}

func Dial(addr uint64) (*Client, bool) {
	c, err := netffi.Dial(addr)
	if err {
		return nil, true
	}
	return &Client{conn: c}, false
}

// Call does an rpc, and returns error on fail.
func (c *Client) Call(rpcId uint64, args []byte, reply *[]byte) bool {
	req0 := make([]byte, 0, 8+len(args))
	req1 := marshal.WriteInt(req0, rpcId)
	req2 := marshal.WriteBytes(req1, args)
	if c.conn.Send(req2) {
		return true
	}

	resp, err0 := c.conn.Receive()
	if err0 {
		return true
	}
	*reply = resp
	return false
}
