# go-shadowsocks2

A fresh implementation of Shadowsocks in Go.

GoDoc at https://godoc.org/github.com/shadowsocks/go-shadowsocks2/

![Build and test](https://github.com/shadowsocks/go-shadowsocks2/workflows/Build%20and%20test/badge.svg)


## Features

- [x] SOCKS5 proxy with UDP Associate
- [x] Support for Netfilter TCP redirect on Linux (IPv6 should work but not tested)
- [x] Support for Packet Filter TCP redirect on MacOS/Darwin (IPv4 only)
- [x] UDP tunneling (e.g. relay DNS packets)
- [x] TCP tunneling (e.g. benchmark with iperf3)
- [x] SIP003 plugins
- [x] Replay attack mitigation


## Install

Pre-built binaries for common platforms are available at https://github.com/shadowsocks/go-shadowsocks2/releases

Install from source

```sh
go get -u -v github.com/shadowsocks/go-shadowsocks2
```


## Basic Usage

### Server

Start a server listening on port 8488 using `AEAD_CHACHA20_POLY1305` AEAD cipher with password `your-password`.

```sh
go-shadowsocks2 -s 'ss://AEAD_CHACHA20_POLY1305:your-password@:8488' -verbose
```


### Client

Start a client connecting to the above server. The client listens on port 1080 for incoming SOCKS5 
connections, and tunnels both UDP and TCP on port 8053 and port 8054 to 8.8.8.8:53 and 8.8.4.4:53 
respectively. 

```sh
go-shadowsocks2 -c 'ss://AEAD_CHACHA20_POLY1305:your-password@[server_address]:8488' \
    -verbose -socks :1080 -u -udptun :8053=8.8.8.8:53,:8054=8.8.4.4:53 \
                             -tcptun :8053=8.8.8.8:53,:8054=8.8.4.4:53
```

Replace `[server_address]` with the server's public address.

The server offers `-U` option to disable TCP relay to aviod websocket proxy port conflict.

## Advanced Usage


### Netfilter TCP redirect on Linux

The client offers `-redir` and `-redir6` (for IPv6) options to handle TCP connections 
redirected by Netfilter on Linux. The feature works similar to `ss-redir` from `shadowsocks-libev`.


Start a client listening on port 1082 for redirected TCP connections and port 1083 for redirected
TCP IPv6 connections.

```sh
go-shadowsocks2 -c 'ss://AEAD_CHACHA20_POLY1305:your-password@[server_address]:8488' -redir :1082 -redir6 :1083
```


### TCP tunneling

The client offers `-tcptun [local_addr]:[local_port]=[remote_addr]:[remote_port]` option to tunnel TCP.
For example it can be used to proxy iperf3 for benchmarking.

Start iperf3 on the same machine with the server.

```sh
iperf3 -s
```

By default iperf3 listens on port 5201.

Start a client on the same machine with the server. The client listens on port 1090 for incoming connections
and tunnels to localhost:5201 where iperf3 is listening.

```sh
go-shadowsocks2 -c 'ss://AEAD_CHACHA20_POLY1305:your-password@[server_address]:8488' -tcptun :1090=localhost:5201
```

Start iperf3 client to connect to the tunneld port instead

```sh
iperf3 -c localhost -p 1090
```

### SIP003 Plugins (Experimental)

Both client and server support SIP003 plugins.
Use `-plugin` and `-plugin-opts` parameters to enable.

Client:

```sh
go-shadowsocks2 -c 'ss://AEAD_CHACHA20_POLY1305:your-password@[server_address]:8488' \
    -verbose -socks :1080 -u -plugin v2ray
```
Server:

```sh
go-shadowsocks2 -s 'ss://AEAD_CHACHA20_POLY1305:your-password@:8488' -verbose \
    -plugin v2ray -plugin-opts "server"
```
Note:

It will look for the plugin in the current directory first, then `$PATH`.

UDP connections will not be affected by SIP003.

### Replay Attack Mitigation

By default a [Bloom filter](https://en.wikipedia.org/wiki/Bloom_filter) is deployed to defend against [replay attacks](https://en.wikipedia.org/wiki/Replay_attack).
Use the following environment variables to fine-tune the mechanism:

- `SHADOWSOCKS_SF_CAPACITY`: Number of recent connections to track. Default `1e6` (one million). Setting it to 0 disables the feature.
- `SHADOWSOCKS_SF_FPR`: False positive rate of the Bloom filter. Default `1e-6` (0.0001%). This should be enough for most cases.
- `SHADOWSOCKS_SF_SLOT`: The Bloom filter is divided into a number (default `10`) of slots. When the Bloom filter is full, the
  oldest slot will be cleared for recycling. In general you should not change this number unless you understand what you are doing.

```sh
SHADOWSOCKS_SF_CAPACITY=1e6 SHADOWSOCKS_SF_FPR=1e-6 SHADOWSOCKS_SF_SLOT=10 go-shadowsocks2 ...
```

## Design Principles

The code base strives to

- be idiomatic Go and well organized;
- use fewer external dependences as reasonably possible;
- only include proven modern ciphers;
