package main

import (
	"fmt"
	"net"
	"runtime"

	"github.com/shadowsocks/go-shadowsocks2/pfutil"
	"github.com/shadowsocks/go-shadowsocks2/socks"
	"golang.org/x/sys/unix"
)

const (
	// https://github.com/apple/darwin-xnu/blob/a1babec6b135d1f35b2590a1990af3c5c5393479/bsd/netinet/tcp_var.h#L1483
	TCP_FASTOPEN_SERVER = 1
	// https://github.com/apple/darwin-xnu/blob/a1babec6b135d1f35b2590a1990af3c5c5393479/bsd/netinet/tcp_var.h#L1484
	TCP_FASTOPEN_CLIENT = 2
)

func redirLocal(addr, server string, shadow func(net.Conn) net.Conn) {
	tcpLocal(addr, server, shadow, natLookup)
}

func redir6Local(addr, server string, shadow func(net.Conn) net.Conn) {
	logf("TCP6 redirect not supported on %s-%s", runtime.GOOS, runtime.GOARCH)
}

func natLookup(c net.Conn) (socks.Addr, error) {
	if tc, ok := c.(*net.TCPConn); ok {
		addr, err := pfutil.NatLookup(tc)
		return socks.ParseAddr(addr.String()), err
	}
	panic("not TCP connection")
}

// tcpSetListenOpts sets listening socket options.
func tcpSetListenOpts(fd uintptr) error {
	if config.TCPFastOpen {
		if err := unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_FASTOPEN, TCP_FASTOPEN_SERVER); err != nil {
			return fmt.Errorf("failed to set TCP_FASTOPEN: %s", err)
		}
	}
	return nil
}

// tcpSetDialOpts sets dialing socket options.
func tcpSetDialOpts(fd uintptr) error {
	if config.TCPFastOpen {
		if err := unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_FASTOPEN, TCP_FASTOPEN_CLIENT); err != nil {
			return fmt.Errorf("failed to set TCP_FASTOPEN: %s", err)
		}
	}
	return nil
}
