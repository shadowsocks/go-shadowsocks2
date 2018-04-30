package net

import (
	"io"
	"log"
)

func copyOneWay(leftConn, rightConn DuplexConn) (int64, error) {
	n, err := io.Copy(leftConn, rightConn)
	// Send FIN to indicate EOF
	leftConn.CloseWrite()
	// Release reader resources
	rightConn.CloseRead()
	return n, err
}

// Relay copies between left and right bidirectionally. Returns number of
// bytes copied from right to left, from left to right, and any error occurred.
func Relay(leftConn, rightConn DuplexConn) (int64, int64, error) {
	type res struct {
		N   int64
		Err error
	}
	ch := make(chan res)

	go func() {
		n, err := copyOneWay(rightConn, leftConn)
		log.Printf("copyOneWay L->R done: %v %v", n, err)
		ch <- res{n, err}
	}()

	n, err := copyOneWay(leftConn, rightConn)
	log.Printf("copyOneWay L<-R done: %v %v", n, err)
	rs := <-ch

	if err == nil {
		err = rs.Err
	}
	return n, rs.N, err
}
