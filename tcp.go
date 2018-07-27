package main

import (
	"net"

	ssnet "github.com/shadowsocks/go-shadowsocks2/net"
	"github.com/shadowsocks/go-shadowsocks2/socks"
)

// Create a SOCKS server listening on addr and proxy to server.
func socksLocal(addr, server string, shadow func(net.Conn) net.Conn) {
	logf("SOCKS proxy %s <-> %s", addr, server)
	tcpLocal(addr, server, shadow, func(c net.Conn) (socks.Addr, error) { return socks.Handshake(c) })
}

// Create a TCP tunnel from addr to target via server.
func tcpTun(addr, server, target string, shadow func(net.Conn) net.Conn) {
	tgt := socks.ParseAddr(target)
	if tgt == nil {
		logf("invalid target address %q", target)
		return
	}
	logf("TCP tunnel %s <-> %s <-> %s", addr, server, target)
	tcpLocal(addr, server, shadow, func(net.Conn) (socks.Addr, error) { return tgt, nil })
}

// Listen on addr and proxy to server to reach target from getAddr.
func tcpLocal(addr, server string, shadow func(net.Conn) net.Conn, getAddr func(net.Conn) (socks.Addr, error)) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		logf("failed to listen on %s: %v", addr, err)
		return
	}

	for {
		clientConn, err := l.(*net.TCPListener).AcceptTCP()
		if err != nil {
			logf("failed to accept: %s", err)
			continue
		}

		go func() {
			defer clientConn.Close()
			clientConn.SetKeepAlive(true)
			tgt, err := getAddr(clientConn)
			defer logf("done with %v", tgt)
			if err != nil {

				// UDP: keep the connection until disconnect then free the UDP socket
				if err == socks.InfoUDPAssociate {
					buf := []byte{}
					// block here
					for {
						_, err := clientConn.Read(buf)
						if err, ok := err.(net.Error); ok && err.Timeout() {
							continue
						}
						logf("UDP Associate End.")
						return
					}
				}

				logf("failed to get target address: %v", err)
				return
			}

			c, err := net.Dial("tcp", server)
			if err != nil {
				logf("failed to connect to server %v: %v", server, err)
				return
			}
			proxyConn := c.(*net.TCPConn)
			defer proxyConn.Close()
			proxyConn.SetKeepAlive(true)
			shadowConn := shadow(proxyConn)

			if _, err = shadowConn.Write(tgt); err != nil {
				logf("failed to send target address: %v", err)
				return
			}

			logf("proxy %s <-> %s <-> %s", clientConn.RemoteAddr(), server, tgt)
			_, _, err = ssnet.Relay(clientConn, shadowConn)
			if err != nil {
				logf("relay error: %v", err)
			}
		}()
	}
}

// Listen on addr for incoming connections.
func tcpRemote(addr string, shadow func(net.Conn) net.Conn) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		logf("failed to listen on %s: %v", addr, err)
		return
	}

	logf("listening TCP on %s", addr)
	for {
		clientConn, err := l.(*net.TCPListener).AcceptTCP()
		if err != nil {
			logf("failed to accept: %v", err)
			continue
		}

		go func() {
			defer logf("done with %s", clientConn.RemoteAddr())
			defer clientConn.Close()
			clientConn.SetKeepAlive(true)
			shadowConn := shadow(clientConn)

			tgt, err := socks.ReadAddr(shadowConn)
			if err != nil {
				logf("failed to get target address: %v", err)
				return
			}

			c, err := net.Dial("tcp", tgt.String())
			if err != nil {
				logf("failed to connect to target: %v", err)
				return
			}
			tgtConn := c.(*net.TCPConn)
			defer tgtConn.Close()
			tgtConn.SetKeepAlive(true)

			logf("proxy %s <-> %s", clientConn.RemoteAddr(), tgt)
			_, _, err = ssnet.Relay(shadowConn, tgtConn)
			if err != nil {
				logf("relay error: %v", err)
			}
		}()
	}
}
