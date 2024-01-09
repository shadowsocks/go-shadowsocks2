package main

//*
import (
	"net"

	"github.com/lxt1045/go-shadowsocks2/socks"
)

func proxy(server string, shadow func(net.Conn) net.Conn) {
	for {
		func() {
			defer func() {
				e := recover()
				if e != nil {
					logf("failed to connect to server %v: %v", server, e)
				}
			}()
			doProxy(server, shadow)
		}()
	}
}

func doProxy(server string, shadow func(net.Conn) net.Conn) {
	logf("SOCKS proxy %s <-> %s", "local", server)

	cmdConn, err := net.Dial("tcp", server)
	if err != nil {
		logf("failed to connect to server %v: %v", server, err)
		return
	}
	defer cmdConn.Close()
	cmdConn.(*net.TCPConn).SetKeepAlive(true)
	cmdConn = shadow(cmdConn) // 加解密

	if _, err = cmdConn.Write([]byte{socks.AtypProxy}); err != nil {
		logf("failed to send target address.cmd: %v", err)
		return
	}

	// start:
	for {
		// cmdConn 作为接受命令的连接
		tgt, err := socks.ReadAddr(cmdConn)
		if err != nil {
			logf("failed to get target address: %v", err)
			return
		}
		logf("get target address: %s", tgt.String())

		go func(server string, tgt socks.Addr) {
			// 建立新的连接
			remoteConn, err := net.Dial("tcp", server)
			if err != nil {
				logf("failed to connect to server %v: %v", server, err)
				return
			}
			// 和 remote 建立连接
			defer remoteConn.Close()
			remoteConn.(*net.TCPConn).SetKeepAlive(true)
			remoteConn = shadow(remoteConn) // 加解密

			tgtNew := append(tgt[:0:0], tgt...)
			switch tgtNew[0] {
			case socks.AtypDomainName:
				tgtNew[0] = socks.AtypProxyNewDomainName
			case socks.AtypIPv4:
				tgtNew[0] = socks.AtypProxyNewIPv4
			case socks.AtypIPv6:
				tgtNew[0] = socks.AtypProxyNewIPv6
			}
			if _, err = remoteConn.Write(tgtNew); err != nil {
				logf("failed to send target address.cmd: %v", err)
				return
			}

			// 本地连接
			logf("tgt: %s", tgt.String())
			localConn, err := net.Dial("tcp", tgt.String())
			if err != nil {
				logf("failed to connect to target: %s,err: %v", tgt.String(), err)
				return
			}
			defer localConn.Close()
			localConn.(*net.TCPConn).SetKeepAlive(true)

			logf("proxy %s <-> %s", server, tgt)

			_, _, err = relay(localConn, remoteConn)
			if err != nil {
				if err, ok := err.(net.Error); ok && err.Timeout() {
					return // ignore i/o timeout
				}
				logf("relay error: %v", err)
			}
		}(server, tgt)
	}
}

// Listen on addr for incoming connections.
func proxyRemote(addr string, shadow func(net.Conn) net.Conn) {
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

			// TODO  这里可以复用，用于反向代理
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
func kcpRemote(addr string) {
	/*
		//l, err := net.Listen("tcp", addr)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		recvCh, sendCh, err := udp.Listen(ctx, addr)
		if err != nil {
			logf("failed to listen on %s: %v", addr, err)
			return
		}

		logf("listening TCP on %s", addr)
		conns := make(map[uint64]chan udp.Pkg)
		for {
			// c, err := l.Accept()
			// if err != nil {
			// 	logf("failed to accept: %v", err)
			// 	continue
			// }
			pkgR := <-recvCh
			if ch, ok := conns[pkgR.ConnID]; ok {
				ch <- pkgR
				continue
			}
			ch := make(chan udp.Pkg, 8)
			conns[pkgR.ConnID] = ch

			go func(ch chan udp.Pkg, pkgR udp.Pkg) {
				tgt, err := socks.ReadAddr(bytes.NewBuffer(pkgR.Data))
				if err != nil {
					logf("failed to get target address: %v,pkgR.Data:%s",
						err, string(pkgR.Data))
					return
				}

				rc, err := net.Dial("tcp", tgt.String())
				if err != nil {
					logf("failed to connect to target: %v", err)
					return
				}
				defer rc.Close()
				rc.(*net.TCPConn).SetKeepAlive(true)

				logf("proxy %s <-> %s", pkgR.Addr, tgt)

				go func() {
					for {
						pkgR := <-ch
						rc.Write(pkgR.Data)
					}
				}()

				for {
					buf := make([]byte, 2048)
					n, err := rc.Read(buf)
					if err != nil {
						log.Println("exit", err)
						break
					}
					pkg := udp.Pkg{
						MsgType: 111,
						Guar:    true,
						Data:    buf[:n], //[]byte("hello word!"),
						ConnID:  pkgR.ConnID,
						//Addr:    to,
					}
					log.Println(pkg)
					sendCh <- pkg
				}
				if err != nil {
					if err, ok := err.(net.Error); ok && err.Timeout() {
						return // ignore i/o timeout
					}
					logf("relay error: %v", err)
				}
			}(ch, pkgR)
		}
		//*/
}

//*/
