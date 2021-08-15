module github.com/lxt1045/go-shadowsocks2

go 1.16

require (
	github.com/aead/chacha20 v0.0.0-20180709150244-8b13a72661da
	github.com/riobard/go-bloom v0.0.0-20200213042214-218e1707c495
	golang.org/x/crypto v0.0.0-20200128174031-69ecbb4d6d5d
)

replace (
	github.com/lxt1045/tools v0.0.0-20210525034748-d0cf567cec65 => github.com/lxt1045/tools v0.0.0-20210525034748-d0cf567cec65 // indirect
	golang.org/x/crypto v0.0.0-20190308221718-c2843e01d9a2 => github.com/golang/crypto v0.0.0-20190308221718-c2843e01d9a2
	golang.org/x/crypto v0.0.0-20190426145343-a29dc8fdc734 => github.com/golang/crypto v0.0.0-20190426145343-a29dc8fdc734
	golang.org/x/net v0.0.0-20190404232315-eb5bcb51f2a3 => github.com/golang/net v0.0.0-20190404232315-eb5bcb51f2a3
	golang.org/x/sys v0.0.0-20190215142949-d0b11bdaac8a => github.com/golang/sys v0.0.0-20190215142949-d0b11bdaac8a
	golang.org/x/sys v0.0.0-20190412213103-97732733099d => github.com/golang/sys v0.0.0-20190412213103-97732733099d
	golang.org/x/text v0.3.0 => github.com/golang/text v0.3.0
)
