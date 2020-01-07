package core

import (
	"bytes"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shadowsocks/go-shadowsocks2/socks"
)

type listener struct {
	net.Listener
	StreamConnCipher
}

func Listen(network, address string, ciph StreamConnCipher) (net.Listener, error) {
	l, err := net.Listen(network, address)
	return &listener{l, ciph}, err
}

func (l *listener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	return l.StreamConn(c), err
}

func Dial(network, address string, ciph StreamConnCipher) (net.Conn, error) {
	c, err := net.Dial(network, address)
	return ciph.StreamConn(c), err
}

// Connect sends the shadowsocks standard header to underlying ciphered connection
// in the next Write/ReadFrom call.
func Connect(c net.Conn, addr socks.Addr) net.Conn {
	return newSSConn(c, addr, 5 * time.Millisecond)
}

type ssconn struct {
	net.Conn
	addr socks.Addr
	done uint32
	m    sync.Mutex
	t    *time.Timer
}

func newSSConn(c net.Conn, addr socks.Addr, delay time.Duration) *ssconn {
	sc := &ssconn{Conn: c, addr: addr}
	if delay > 0 {
		sc.t = time.AfterFunc(delay, func() {
			sc.Write([]byte{})
		})
	}
	return sc
}

func (c *ssconn) Write(b []byte) (int, error) {
	n, err := c.ReadFrom(bytes.NewBuffer(b))
	return int(n), err
}

func (c *ssconn) ReadFrom(r io.Reader) (int64, error) {
	if atomic.LoadUint32(&c.done) == 0 {
		c.m.Lock()
		defer c.m.Unlock()
		if c.done == 0 {
			defer atomic.StoreUint32(&c.done, 1)
			if c.t != nil {
				c.t.Stop()
				c.t = nil
			}
			ra := readerWithAddr{Reader: r, b: c.addr}
			n, err := io.Copy(c.Conn, &ra)
			n -= int64(len(c.addr))
			if n < 0 { n = 0 }
			c.addr = nil
			return n, err
		}
	}
	return io.Copy(c.Conn, r)
}

func (c *ssconn) WriteTo(w io.Writer) (int64, error) {
	return io.Copy(w, c.Conn)
}

type readerWithAddr struct {
	io.Reader
	b []byte
}

func (r *readerWithAddr) Read(b []byte) (n int, err error) {
	nc := copy(b, r.b)
	r.b = r.b[nc:]
	if len(b) == nc {
		return nc, nil
	}
	nr, err := r.Reader.Read(b[nc:])
	return nc + nr, err
}
