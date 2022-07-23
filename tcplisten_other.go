//go:build !darwin
// +build !darwin

package main

import (
	"context"
	"net"
	"syscall"
)

// tcpListen binds a listening socket
func tcpListen(addr string) (net.Listener, error) {
	lc := net.ListenConfig{
		KeepAlive: config.TCPKeepAlive,
		Control: func(network, address string, c syscall.RawConn) error {
			var ctrlErr error
			if err := c.Control(func(fd uintptr) { ctrlErr = tcpSetListenOpts(fd) }); err != nil {
				return err
			}
			if ctrlErr != nil {
				logf("failed to set up listening socket: %s", ctrlErr)
			}
			return nil
		},
	}
	return lc.Listen(context.Background(), "tcp", addr)
}
