package main

import (
	"context"
	"net"
	"syscall"
)

// tcpListen binds a listening socket
func tcpListen(addr string) (net.Listener, error) {
	var rawConn syscall.RawConn
	lc := net.ListenConfig{
		KeepAlive: config.TCPKeepAlive,
		Control: func(network, address string, c syscall.RawConn) error {
			rawConn = c
			return nil
		},
	}

	l, err := lc.Listen(context.Background(), "tcp", addr)
	if err != nil {
		return nil, err
	}

	// On MacOS we have to call Control() after the bind() and listen() are complete,
	// otherwise setsockopt(TCP_FASTOPEN) fails with EINVAL (invalid argument).
	// See https://github.com/h2o/h2o/commit/ec58f59f5e9a6c6a8a38087eb87fdc4b1763f080
	var ctrlErr error
	if err := rawConn.Control(func(fd uintptr) { ctrlErr = tcpSetListenOpts(fd) }); err != nil {
		l.Close()
		return nil, err
	}

	if ctrlErr != nil {
		logf("failed to set up listening socket: %s", ctrlErr)
	}

	return l, nil
}
