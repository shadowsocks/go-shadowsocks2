package main

//*
import (
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lxt1045/go-shadowsocks2/socks"
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
		c, err := l.Accept()
		if err != nil {
			logf("failed to accept: %s", err)
			continue
		}

		go func() {
			defer c.Close()
			c.(*net.TCPConn).SetKeepAlive(true)
			tgt, err := getAddr(c)
			if err != nil {

				// UDP: keep the connection until disconnect then free the UDP socket
				if err == socks.InfoUDPAssociate {
					buf := make([]byte, 1)
					// block here
					for {
						_, err := c.Read(buf)
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

			rc, err := net.Dial("tcp", server)
			if err != nil {
				logf("failed to connect to server %v: %v", server, err)
				return
			}
			defer rc.Close()
			rc.(*net.TCPConn).SetKeepAlive(true)
			rc = shadow(rc) // 加解密

			if _, err = rc.Write(tgt); err != nil {
				logf("failed to send target address: %v", err)
				return
			}

			logf("proxy %s <-> %s <-> %s", c.RemoteAddr(), server, tgt)
			_, _, err = relay(rc, c)
			if err != nil {
				if err, ok := err.(net.Error); ok && err.Timeout() {
					return // ignore i/o timeout
				}
				logf("relay error: %v", err)
			}
		}()
	}
}

// Listen on addr for incoming connections.
func tcpRemote0(addr string, shadow func(net.Conn) net.Conn) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		logf("failed to listen on %s: %v", addr, err)
		return
	}

	logf("listening TCP on %s", addr)
	for {
		c, err := l.Accept()
		if err != nil {
			logf("failed to accept: %v", err)
			continue
		}

		go func() {
			defer c.Close()
			c.(*net.TCPConn).SetKeepAlive(true)
			c = shadow(c)

			tgt, err := socks.ReadAddr(c)
			if err != nil {
				logf("failed to get target address: %v", err)
				return
			}

			rc, err := net.Dial("tcp", tgt.String())
			if err != nil {
				logf("failed to connect to target: %v", err)
				return
			}
			defer rc.Close()
			rc.(*net.TCPConn).SetKeepAlive(true)

			logf("proxy %s <-> %s", c.RemoteAddr(), tgt)
			_, _, err = relay(c, rc)
			if err != nil {
				if err, ok := err.(net.Error); ok && err.Timeout() {
					return // ignore i/o timeout
				}
				logf("relay error: %v", err)
			}
		}()
	}
}

// Listen on addr for incoming connections.
func tcpRemote(addr string, shadow func(net.Conn) net.Conn) {
	l, err := net.Listen("tcp4", addr)
	if err != nil {
		logf("failed to listen on %s: %v", addr, err)
		return
	}

	type Conn struct {
		conn     net.Conn
		deadline int64
	}

	var proxyCMDConn Conn
	connCache := map[string]Conn{}
	lock := sync.Mutex{}
	nextReflesh := time.Now().Unix() + 60

	logf("listening TCP on %s", addr)
	for {
		c, err := l.Accept()
		if err != nil {
			logf("failed to accept: %v", err)
			continue
		}

		go func(c net.Conn) {
			c.(*net.TCPConn).SetKeepAlive(true)
			c = shadow(c)

			// TODO  这里可以复用，用于反向代理
			tgt, err := socks.ReadAddr(c)
			if err != nil {
				switch {
				case errors.Is(socks.ErrCMDConn, err):
					proxyCMDConn = Conn{
						conn: c,
					}
				case errors.Is(socks.ErrCMDConnNew, err):
					logf("ErrCMDConnNew: %s", tgt.String())
					lock.Lock()
					remoteConn := connCache[tgt.String()]
					delete(connCache, tgt.String())
					lock.Unlock()
					if remoteConn.conn == nil {
						return
					}
					defer remoteConn.conn.Close()

					logf("proxy %s <-> %s", c.RemoteAddr(), tgt)
					_, _, err = relay(c, remoteConn.conn)
					if err != nil {
						if err, ok := err.(net.Error); ok && err.Timeout() {
							return // ignore i/o timeout
						}
						logf("relay error: %v", err)
					}
				default:
					logf("failed to get target address: %v", err)
					c.Close()
				}
				return
			}

			if proxyCMDConn.conn == nil {
				defer c.Close()
				rc, err := net.Dial("tcp", tgt.String())
				if err != nil {
					logf("failed to connect to target: %v", err)
					return
				}
				defer rc.Close()
				rc.(*net.TCPConn).SetKeepAlive(true)

				logf("proxy %s <-> %s", c.RemoteAddr(), tgt)
				_, _, err = relay(c, rc)
				if err != nil {
					if err, ok := err.(net.Error); ok && err.Timeout() {
						return // ignore i/o timeout
					}
					logf("relay error: %v", err)
				}
				return
			}

			func() {
				lock.Lock()
				defer lock.Unlock()
				connCache[tgt.String()] = Conn{
					conn:     c,
					deadline: time.Now().Unix() + 600,
				}
			}()
			logf("proxyCMDConn.conn.Write: %s", tgt.String())
			proxyCMDConn.conn.Write(tgt)
		}(c)

		// 清理 connCache
		tsNextReflesh, tsNow := atomic.LoadInt64(&nextReflesh), time.Now().Unix()
		if tsNextReflesh < tsNow && atomic.CompareAndSwapInt64(&nextReflesh, tsNextReflesh, tsNow+60) {
			func() {
				lock.Lock()
				defer lock.Unlock()
				for k, v := range connCache {
					if v.deadline < tsNow {
						delete(connCache, k)
						if v.conn != nil {
							v.conn.Close()
						}
					}
				}
			}()
		}
	}
}

// Listen on addr for incoming connections.
func tcpJumperRemote(addr, jumpServer string, shadow func(net.Conn) net.Conn, clientShadow func(net.Conn) net.Conn) {
	l, err := net.Listen("tcp4", addr)
	if err != nil {
		logf("failed to listen on %s: %v", addr, err)
		return
	}

	logf("listening TCP on %s", addr)
	for {
		c, err := l.Accept()
		if err != nil {
			logf("failed to accept: %v", err)
			continue
		}

		go func() {
			defer c.Close()
			c.(*net.TCPConn).SetKeepAlive(true)
			c = shadow(c)

			tgt, err := socks.ReadAddr(c)
			if err != nil {
				logf("failed to get target address: %v", err)
				return
			}

			logf("TCP get addr: %v", tgt)

			// rc, err := net.Dial("tcp", tgt.String())
			// if err != nil {
			// 	logf("failed to connect to target: %v", err)
			// 	return
			// }
			// defer rc.Close()
			// rc.(*net.TCPConn).SetKeepAlive(true)
			//
			// logf("proxy %s <-> %s", c.RemoteAddr(), tgt)

			rc, err := net.Dial("tcp", jumpServer)
			if err != nil {
				logf("failed to connect to server %v: %v", jumpServer, err)
				return
			}
			defer rc.Close()
			rc.(*net.TCPConn).SetKeepAlive(true)
			rc = clientShadow(rc)

			if _, err = rc.Write(tgt); err != nil {
				logf("failed to send target address: %v", err)
				return
			}
			logf("proxy %s <-> %s <-> %s", c.RemoteAddr(), jumpServer, tgt)

			_, _, err = relay(c, rc)
			if err != nil {
				if err, ok := err.(net.Error); ok && err.Timeout() {
					return // ignore i/o timeout
				}
				logf("relay error: %v", err)
			}
		}()
	}
}

// relay copies between left and right bidirectionally. Returns number of
// bytes copied from right to left, from left to right, and any error occurred.
func relay(left, right net.Conn) (int64, int64, error) {
	type res struct {
		N   int64
		Err error
	}
	ch := make(chan res)

	go func() {
		n, err := io.Copy(right, left)
		right.SetDeadline(time.Now()) // wake up the other goroutine blocking on right
		left.SetDeadline(time.Now())  // wake up the other goroutine blocking on left
		ch <- res{n, err}
	}()

	n, err := io.Copy(left, right)
	right.SetDeadline(time.Now()) // wake up the other goroutine blocking on right
	left.SetDeadline(time.Now())  // wake up the other goroutine blocking on left
	rs := <-ch

	if err == nil {
		err = rs.Err
	}
	return n, rs.N, err
}
