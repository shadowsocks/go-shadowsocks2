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
	"testing"
	"time"

	"github.com/lxt1045/go-shadowsocks2/core"
	"github.com/lxt1045/go-shadowsocks2/socks"
)

var _ = func() bool {
	testing.Init()
	return true
}()

var config struct {
	Verbose    bool
	UDPTimeout time.Duration
}

/*
	ps -aux|grep "./ss2"
	./ss2 -s ss://AEAD_CHACHA20_POLY1305:123456@:38488 -verbose >/dev/null 2>&1 &
	./ss2 -c ss://AES-256-CFB:1tBzXtDJiuEgBIda@proxy.xwfintech.com:38388 -s ss://AEAD_CHACHA20_POLY1305:123456@:38489 -verbose > /dev/null 2>&1 &
*/
func main() {

	var flags struct {
		Client     string
		Server     string
		Jumper     string
		Cipher     string
		Key        string
		Password   string
		Keygen     int
		Socks      string
		RedirTCP   string
		RedirTCP6  string
		TCPTun     string
		UDPTun     string
		UDPSocks   bool
		Plugin     string
		PluginOpts string
	}

	flag.BoolVar(&config.Verbose, "verbose", false, "verbose mode")
	flag.StringVar(&flags.Cipher, "cipher", "AEAD_CHACHA20_POLY1305", "available ciphers: "+strings.Join(core.ListCipher(), " "))
	flag.StringVar(&flags.Key, "key", "", "base64url-encoded key (derive from password if empty)")
	flag.IntVar(&flags.Keygen, "keygen", 0, "generate a base64url-encoded random key of given length in byte")
	flag.StringVar(&flags.Password, "password", "", "password")
	flag.StringVar(&flags.Server, "s", "", "server listen address or url")
	flag.StringVar(&flags.Client, "c", "ss://AEAD_CHACHA20_POLY1305:123456@119.28.12.234:38488", "client connect address or url") //ss://AEAD_CHACHA20_POLY1305:123456@proxy.xwfintech.com:38488
	flag.StringVar(&flags.Jumper, "j", "", "jumper listen address or url")
	flag.StringVar(&flags.Socks, "socks", ":1081", "(client-only) SOCKS listen address")
	flag.BoolVar(&flags.UDPSocks, "u", false, "(client-only) Enable UDP support for SOCKS")
	flag.StringVar(&flags.RedirTCP, "redir", "", "(client-only) redirect TCP from this address")
	flag.StringVar(&flags.RedirTCP6, "redir6", "", "(client-only) redirect TCP IPv6 from this address")
	flag.StringVar(&flags.TCPTun, "tcptun", "", "(client-only) TCP tunnel (laddr1=raddr1,laddr2=raddr2,...)")
	flag.StringVar(&flags.UDPTun, "udptun", "", "(client-only) UDP tunnel (laddr1=raddr1,laddr2=raddr2,...)")
	flag.StringVar(&flags.Plugin, "plugin", "", "Enable SIP003 plugin. (e.g., v2ray-plugin)")
	flag.StringVar(&flags.PluginOpts, "plugin-opts", "", "Set SIP003 plugin options. (e.g., \"server;tls;host=mydomain.me\")")
	flag.DurationVar(&config.UDPTimeout, "udptimeout", 5*time.Minute, "UDP tunnel timeout")
	flag.Parse()

	config.Verbose = true
	//flags.Socks = ""

	if flags.Keygen > 0 {
		key := make([]byte, flags.Keygen)
		io.ReadFull(rand.Reader, key)
		fmt.Println(base64.URLEncoding.EncodeToString(key))
		return
	}

	if flags.Client == "" && flags.Server == "" {
		flag.Usage()
		return
	}

	var key []byte
	if flags.Key != "" {
		k, err := base64.URLEncoding.DecodeString(flags.Key)
		if err != nil {
			log.Fatal(err)
		}
		key = k
	}
	if flags.Client != "" && flags.Server != "" {
		var clientCiph core.Cipher
		var clientAddr string
		{
			addr := flags.Client
			cipher := flags.Cipher
			password := flags.Password
			var err error

			if strings.HasPrefix(addr, "ss://") {
				addr, cipher, password, err = parseURL(addr)
				if err != nil {
					log.Fatal(err)
				}
			}

			udpAddr := addr
			clientAddr = addr

			ciph, err := core.PickCipher(cipher, key, password)
			if err != nil {
				log.Fatal(err)
			}

			if false && flags.Socks != "" {
				socks.UDPEnabled = flags.UDPSocks

				//监听 1080 (socks5代理协议, 作为socks 服务端), 并将请求转发到远端 ss 服务端
				go socksLocal(flags.Socks, clientAddr, ciph.StreamConn)
				if flags.UDPSocks {
					go udpSocksLocal(flags.Socks, udpAddr, ciph.PacketConn)
				}
			}
			clientCiph = ciph
		}

		//
		// server
		//
		{
			addr := flags.Server
			cipher := flags.Cipher
			password := flags.Password
			var err error

			if strings.HasPrefix(addr, "ss://") {
				addr, cipher, password, err = parseURL(addr)
				if err != nil {
					log.Fatal(err)
				}
			}

			udpAddr := addr

			if flags.Plugin != "" {
				addr, err = startPlugin(flags.Plugin, flags.PluginOpts, addr, true)
				if err != nil {
					log.Fatal(err)
				}
			}

			ciph, err := core.PickCipher(cipher, key, password)
			if err != nil {
				log.Fatal(err)
			}

			go udpRemote(udpAddr, ciph.PacketConn)
			//监听本地 38388 (ss代理协议, 作为ss 服务端), 并转发到远端ss 代理服务器
			go tcpJumperRemote(addr, clientAddr, ciph.StreamConn, clientCiph.StreamConn)
		}
	}

	if flags.Client != "" && flags.Server == "" { // client mode
		addr := flags.Client
		cipher := flags.Cipher
		password := flags.Password
		var err error

		if strings.HasPrefix(addr, "ss://") {
			addr, cipher, password, err = parseURL(addr)
			if err != nil {
				log.Fatal(err)
			}
		}

		udpAddr := addr

		ciph, err := core.PickCipher(cipher, key, password)
		if err != nil {
			log.Fatal(err)
		}

		if flags.Plugin != "" {
			addr, err = startPlugin(flags.Plugin, flags.PluginOpts, addr, false)
			if err != nil {
				log.Fatal(err)
			}
		}

		if flags.UDPTun != "" {
			for _, tun := range strings.Split(flags.UDPTun, ",") {
				p := strings.Split(tun, "=")
				go udpLocal(p[0], udpAddr, p[1], ciph.PacketConn)
			}
		}

		if flags.TCPTun != "" {
			for _, tun := range strings.Split(flags.TCPTun, ",") {
				p := strings.Split(tun, "=")
				go tcpTun(p[0], addr, p[1], ciph.StreamConn)
			}
		}

		if flags.Socks != "" {
			socks.UDPEnabled = flags.UDPSocks
			go socksLocal(flags.Socks, addr, ciph.StreamConn)
			if flags.UDPSocks {
				go udpSocksLocal(flags.Socks, udpAddr, ciph.PacketConn)
			}
		}

		if flags.RedirTCP != "" {
			go redirLocal(flags.RedirTCP, addr, ciph.StreamConn)
		}

		if flags.RedirTCP6 != "" {
			go redir6Local(flags.RedirTCP6, addr, ciph.StreamConn)
		}
	}

	if flags.Client == "" && flags.Server != "" { // server mode
		addr := flags.Server
		cipher := flags.Cipher
		password := flags.Password
		var err error

		if strings.HasPrefix(addr, "ss://") {
			addr, cipher, password, err = parseURL(addr)
			if err != nil {
				log.Fatal(err)
			}
		}

		udpAddr := addr

		if flags.Plugin != "" {
			addr, err = startPlugin(flags.Plugin, flags.PluginOpts, addr, true)
			if err != nil {
				log.Fatal(err)
			}
		}

		ciph, err := core.PickCipher(cipher, key, password)
		if err != nil {
			log.Fatal(err)
		}

		go udpRemote(udpAddr, ciph.PacketConn)
		go tcpRemote(addr, ciph.StreamConn)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	killPlugin()
}

func parseURL(s string) (addr, cipher, password string, err error) {
	u, err := url.Parse(s)
	if err != nil {
		return
	}

	addr = u.Host
	if u.User != nil {
		cipher = u.User.Username()
		password, _ = u.User.Password()
	}
	return
}
