package net

import (
	"io"
	"net"
)

type closeWriter interface{ CloseWrite() error }
type closeReader interface{ CloseRead() error }

func copyOneWay(leftConn, rightConn net.Conn) (int64, error) {
	n, err := io.Copy(leftConn, rightConn)
	// Send FIN to indicate EOF
	leftConn.(closeWriter).CloseWrite()
	// Release reader resources
	rightConn.(closeReader).CloseRead()
	return n, err
}

// Relay copies between left and right bidirectionally. Returns number of
// bytes copied from right to left, from left to right, and any error occurred.
// Relay allows for half-closed connections: if one side is done writing, it can
// still read all remaning data from its peer.
func Relay(leftConn, rightConn net.Conn) (int64, int64, error) {
	type res struct {
		N   int64
		Err error
	}
	ch := make(chan res)

	go func() {
		n, err := copyOneWay(rightConn, leftConn)
		ch <- res{n, err}
	}()

	n, err := copyOneWay(leftConn, rightConn)
	rs := <-ch

	if err == nil {
		err = rs.Err
	}
	return n, rs.N, err
}
