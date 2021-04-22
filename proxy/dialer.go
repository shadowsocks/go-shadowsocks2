package core

import (
	"net"
	"errors"
	"github.com/shadowsocks/go-shadowsocks2/socks"
	"github.com/shadowsocks/go-shadowsocks2/core"
)

type ProxyDialer struct {
	core.StreamConnCipher
	addr string
}

func NewDialer(addr string, algo string, password string) (s *ProxyDialer, err error) {
	cipher, err := core.PickCipher(algo, nil, password)
	if err != nil {
		return
	}
	s = new(ProxyDialer)
	s.StreamConnCipher = cipher
	s.addr = addr
	return
}

func NewDialerFromKey(addr string, algo string, key []byte) (s *ProxyDialer, err error) {
	cipher, err := core.PickCipher(algo, key, "")
	if err != nil {
		return
	}
	s = new(ProxyDialer)
	s.StreamConnCipher = cipher
	s.addr = addr
	return
}

func (dialer *ProxyDialer) Dial(network, addr string) (c net.Conn, err error) {
	target := socks.ParseAddr(addr)
	if target == nil {
		return nil, errors.New("Unable to parse address: " + addr)
	}
	conn, err := net.Dial("tcp", dialer.addr)
	if err != nil {
		return nil, err
	}
	conn.(*net.TCPConn).SetKeepAlive(true)
	c = dialer.StreamConn(conn)
	if _, err = c.Write(target); err != nil {
		c.Close()
		return nil, err
	}
	return
}
