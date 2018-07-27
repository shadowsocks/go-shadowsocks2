// +build !linux

package main

import "net"

func redirLocal(addr, server string, shadow func(net.Conn) net.Conn) {
	logf("TCP redirect not supported")
}

func redir6Local(addr, server string, shadow func(net.Conn) net.Conn) {
	logf("TCP6 redirect not supported")
}
