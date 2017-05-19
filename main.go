package main

import (
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/shadowsocks/go-shadowsocks2/core"
)

var config struct {
	Quiet      bool
	UDPTimeout time.Duration
}

func logf(f string, v ...interface{}) {
	if !config.Quiet {
		log.Printf(f, v...)
	}
}

func main() {

	var flags struct {
		Help      bool
		Client    string
		Server    string
		Port      int
		Cipher    string
		Key       string
		Password  string
		Keygen    int
		Socks     string
		RedirTCP  string
		RedirTCP6 string
		TCPTun    string
		UDPTun    string
	}

	flag.BoolVar(&flags.Help, "h", false, "show this help messages")
	flag.BoolVar(&config.Quiet, "q", false, "suppress status output")
	flag.StringVar(&flags.Cipher, "cipher", "AEAD_CHACHA20_POLY1305", "available ciphers: "+strings.Join(core.ListCipher(), " "))
	flag.StringVar(&flags.Key, "key", "", "base64url-encoded key (derive from password if empty)")
	flag.IntVar(&flags.Keygen, "keygen", 0, "generate a base64url-encoded random key of given length in byte")
	flag.StringVar(&flags.Password, "password", "Shadowsocks!Go", "password")
	flag.StringVar(&flags.Server, "s", "", "server listen address or url")
	flag.StringVar(&flags.Client, "c", "", "client connect address or url")
	flag.IntVar(&flags.Port, "p", 0, "listen port of server default: 8488")
	flag.StringVar(&flags.Socks, "socks", "", "(client-only) SOCKS listen address")
	flag.StringVar(&flags.RedirTCP, "redir", "", "(client-only) redirect TCP from this address")
	flag.StringVar(&flags.RedirTCP6, "redir6", "", "(client-only) redirect TCP IPv6 from this address")
	flag.StringVar(&flags.TCPTun, "tcptun", "", "(client-only) TCP tunnel (laddr1=raddr1,laddr2=raddr2,...)")
	flag.StringVar(&flags.UDPTun, "udptun", "", "(client-only) UDP tunnel (laddr1=raddr1,laddr2=raddr2,...)")
	flag.DurationVar(&config.UDPTimeout, "udptimeout", 5*time.Minute, "UDP tunnel timeout")
	flag.Parse()

	if flags.Help {
		flag.Usage()
		return
	}

	if flags.Keygen > 0 {
		key := make([]byte, flags.Keygen)
		io.ReadFull(rand.Reader, key)
		fmt.Println(base64.URLEncoding.EncodeToString(key))
		return
	}

	var key []byte
	if flags.Key != "" {
		k, err := base64.URLEncoding.DecodeString(flags.Key)
		if err != nil {
			log.Panicln(err)
		}
		key = k
	}

	// client mode
	var client = flags.Client
	if client != "" {
		if !strings.HasPrefix(client, "ss://") {
			client = fmt.Sprintf("ss://%s", client)
		}

		addr, cipher, password, err := parseURL(client, flags.Cipher, flags.Password, flags.Port)
		if err != nil {
			log.Panicln(err)
		}

		ciph, err := core.PickCipher(cipher, key, password)
		if err != nil {
			log.Panicln(err)
		}

		if flags.UDPTun != "" {
			for _, tun := range strings.Split(flags.UDPTun, ",") {
				p := strings.Split(tun, "=")
				go udpLocal(p[0], addr, p[1], ciph.PacketConn)
			}
		}

		if flags.TCPTun != "" {
			for _, tun := range strings.Split(flags.TCPTun, ",") {
				p := strings.Split(tun, "=")
				go tcpTun(p[0], addr, p[1], ciph.StreamConn)
			}
		}

		if flags.Socks != "" {
			go socksLocal(flags.Socks, addr, ciph.StreamConn)
		}

		if flags.RedirTCP != "" {
			go redirLocal(flags.RedirTCP, addr, ciph.StreamConn)
		}

		if flags.RedirTCP6 != "" {
			go redir6Local(flags.RedirTCP6, addr, ciph.StreamConn)
		}
	}

	// server mode
	server := flags.Server
	if !strings.HasPrefix(server, "ss://") {
		server = fmt.Sprintf("ss://%s", server)
	}

	addr, cipher, password, err := parseURL(server, flags.Cipher, flags.Password, flags.Port)
	if err != nil {
		log.Panicln(err)
	}

	ciph, err := core.PickCipher(cipher, key, password)
	if err != nil {
		log.Panicln(err)
	}

	go udpRemote(addr, ciph.PacketConn)
	go tcpRemote(addr, ciph.StreamConn)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}

func parseURL(addr, cipher, password string, port int) (string, string, string, error) {
	u, err := url.Parse(addr)
	if err != nil {
		return "", "", "", err
	}

	if port > 0 {
		addr = fmt.Sprintf("%s:%d", u.Hostname(), port)
	} else if u.Port() == "" {
		addr = fmt.Sprintf("%s:8488", u.Hostname())
	}
	if u.User != nil {
		if n := u.User.Username(); n != "" {
			cipher = u.User.Username()
		}
		if p, _ := u.User.Password(); p != "" {
			password = p
		}
	}
	return addr, cipher, password, nil
}
