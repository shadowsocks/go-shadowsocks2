package core

import (
	"testing"
	"net/http"
	"golang.org/x/net/proxy"
	"github.com/shadowsocks/go-shadowsocks2/socks"
	"io/ioutil"
	"net"
	"io"
	"time"
	"strings"
)

func TestDialerInterface(t *testing.T) {
	var dialer proxy.Dialer
	dialer, err := NewDialer("198.51.100.254:65534", "aes-128-gcm", "dialer-test")
	if err != nil {
		t.Fatal("Unable to create new dialer:", err.Error())
		return
	}
	// AES-128 key is 16 bytes long.
	// This key is just for interface testing only, so zeroKey doesn't matter.
	zeroKey := make([]byte, 16)
	dialer, err = NewDialerFromKey("195.51.100.254:65533", "aes-128-gcm", zeroKey)
	if err != nil {
		t.Fatal("Unable to create new dialer with pre-generated key:", err.Error())
		return
	}
	_ = dialer
	t.Log("Dialer interface works.")
}

func TestDialerFunctionality(t *testing.T) {
	// First let's launch a dummy shadowsocks server.
	ciph, err := PickCipher("aes-128-gcm", nil, "dialer-test")
	if err != nil {
		t.Fatal("Unable to pick cipher", ciph, ":", err.Error())
		return
	}
	go tcpRemote("127.0.0.1:60001", ciph.StreamConn)
	dialer, err := NewDialer("127.0.0.1:60001", "aes-128-gcm", "dialer-test")
	if err != nil {
		t.Fatal("Unable to create new dialer:", err.Error())
		return
	}
	client := &http.Client {
		Transport: &http.Transport {
			Dial: dialer.Dial,
		},
	}
	resp, err := client.Get("http://ifconfig.co")
	if err != nil {
		t.Fatal("Unable to connect ifconfig.co through shadowsocks server:", err.Error())
		return
	}
	t.Log("http.Get ifconfig.co =", resp.Status)
	body, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	ip := strings.TrimSpace(string(body))
	t.Log("My IP address is:", ip)
	if ipp := net.ParseIP(ip); ipp == nil {
		t.Errorf("Responsed IP address is not a valid IP.")
	} else {
		t.Log("IP address looks good.")
	}

}

// Code lines below are copied from main package, since go can't import main package.
func tcpRemote(addr string, shadow func(net.Conn) net.Conn) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return
	}
	for {
		c, err := l.Accept()
		if err != nil {
			continue
		}

		go func() {
			defer c.Close()
			c.(*net.TCPConn).SetKeepAlive(true)
			c = shadow(c)

			tgt, err := socks.ReadAddr(c)
			if err != nil {
				return
			}

			rc, err := net.Dial("tcp", tgt.String())
			if err != nil {
				return
			}
			defer rc.Close()
			rc.(*net.TCPConn).SetKeepAlive(true)

			_, _, err = relay(c, rc)
			if err != nil {
				if err, ok := err.(net.Error); ok && err.Timeout() {
					return // ignore i/o timeout
				}
			}
		}()
	}
}

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
