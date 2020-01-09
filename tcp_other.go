// +build !linux

package main

import (
	"net"
	"syscall"
)

func redirLocal(d net.Dialer, addr, server string, shadow func(net.Conn) net.Conn) {
	logf("TCP redirect not supported")
}

func redir6Local(d net.Dialer, addr, server string, shadow func(net.Conn) net.Conn) {
	logf("TCP6 redirect not supported")
}

func getTCPControl(fastopen bool) func(_, _ string, _ syscall.RawConn) error {
	return nil
}
