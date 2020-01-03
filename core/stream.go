package core

import (
	"bytes"
	"io"
	"net"

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
	return &ssconn{Conn: c, addr: addr}
}

type ssconn struct {
	net.Conn
	addr socks.Addr
}

func (c *ssconn) Write(b []byte) (int, error) {
	n, err := c.ReadFrom(bytes.NewBuffer(b))
	return int(n), err
}

func (c *ssconn) ReadFrom(r io.Reader) (int64, error) {
	if len(c.addr) > 0 {
		r = &readerWithAddr{Reader: r, b: c.addr}
		c.addr = nil
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
	if nc < len(r.b) {
		r.b = r.b[:nc]
		return nc, nil
	}
	r.b = nil
	nr, err := r.Reader.Read(b[nc:])
	return nc + nr, err
}
