package main

import (
	"fmt"
	"net"
	"runtime"

	"golang.org/x/sys/windows"
)

const (
	// https://github.com/shadowsocks/shadowsocks-libev/blob/89b5f987d6a5329de9713704615581d363f0cfed/src/winsock.h#L82
	TCP_FASTOPEN = 15
)

func redirLocal(addr, server string, shadow func(net.Conn) net.Conn) {
	logf("TCP redirect not supported on %s-%s", runtime.GOOS, runtime.GOARCH)
}

func redir6Local(addr, server string, shadow func(net.Conn) net.Conn) {
	logf("TCP6 redirect not supported on %s-%s", runtime.GOOS, runtime.GOARCH)
}

// tcpSetListenOpts sets listening socket options.
func tcpSetListenOpts(fd uintptr) error {
	if config.TCPFastOpen {
		if err := windows.SetsockoptInt(windows.Handle(fd), windows.IPPROTO_TCP, TCP_FASTOPEN, 1); err != nil {
			return fmt.Errorf("failed to set TCP_FASTOPEN: %s", err)
		}
	}
	return nil
}

// tcpSetDialOpts sets dialing socket options.
func tcpSetDialOpts(fd uintptr) error {
	if config.TCPFastOpen {
		if err := windows.SetsockoptInt(windows.Handle(fd), windows.IPPROTO_TCP, TCP_FASTOPEN, 1); err != nil {
			return fmt.Errorf("failed to set TCP_FASTOPEN: %s", err)
		}
	}
	return nil
}
