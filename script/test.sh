#!/bin/bash

# Use [ -n "$TRAVIS" ] to test for running on Travis-CI.

# Run in the scripts directory.
cd "$( dirname "${BASH_SOURCE[0]}" )"

PLUGIN="v2ray-plugin"
SERVER_PORT="4433"
LOCAL_PORT="1080"
SOCKS="127.0.0.1:1080"
HTTP_PORT="8080"

if [[ -z "${GOPATH}" ]]; then
  GOPATH=~/go
fi

wait_server() {
    local port
    port=$1
    for i in {1..20}; do
        # sleep first because this maybe called immediately after server start
        sleep 0.1
        nc -z -w 4 127.0.0.1 $port && break
    done
}

start_http_server() {
    go build http.go
    ./http $HTTP_PORT &
    wait_server $HTTP_PORT
    http_pid=$!
}

stop_http_server() {
    kill -SIGTERM $http_pid
}

test_get() {
    local url
    local target
    local code
    url=$1
    target=$2
    code=$3

    if [[ -z $code ]]; then
        code="200"
    fi

    # -s silent to disable progress meter, but enable --show-error
    # -i to include http header
    # -L to follow redirect so we should always get HTTP 200
    cont=`curl -m 5 --socks5 $SOCKS -s --show-error -i -L $url 2>&1`
    ok=`echo $cont | grep -E -o "HTTP/1\.1 +$code"`
    html=`echo $cont | grep -E -o -i "$target"`
    if [[ -z $ok || -z $html ]] ; then
        echo "GET $url FAILED!!!"
        echo "$ok"
        echo "$html"
        echo $cont
        return 1
    fi
    return 0
}

test_shadowsocks() {
    local url
    local method
    local server_pid
    local local_pid
    local client_option
    local server_option
    local msg
    local expectation
    url=$1
    method=$2
    msg="Testing HTTP/GET $url $method w/o plugin"
    expectation=$3

    if [ $# -eq 4 ]; then
        server_option="-plugin $PLUGIN -plugin-opts server;"
        client_option="-plugin $PLUGIN"
        msg="Testing HTTP/GET $url $method with plugin"
    fi
    echo "============================================================"
    echo $msg

    go-shadowsocks2 -s "ss://$method:your-password@:$SERVER_PORT" -verbose $server_option &
    server_pid=$!
    wait_server $SERVER_PORT

    go-shadowsocks2 -c "ss://$method:your-password@127.0.0.1:$SERVER_PORT" \
        -verbose -socks :$LOCAL_PORT -u $client_option &
    local_pid=$!
    wait_server $LOCAL_PORT

    for i in {1..3}; do
        test_get $url "go-shadowsocks2"
        if ! [ $? -eq $expectation ]; then
            echo -e "STATUS:\033[31m FAIL \033[0m"
            kill -SIGTERM $server_pid
            kill -SIGTERM $local_pid
            stop_http_server
            exit 1
        fi
    done
    echo -e "STATUS:\033[32m PASS \033[0m"
    kill -SIGTERM $server_pid
    kill -SIGTERM $local_pid
    sleep 0.1
}

test_methods() {
    local url
    url=http://127.0.0.1:$HTTP_PORT/README.md
    
    test_shadowsocks $url AEAD_AES_128_GCM 0
    test_shadowsocks $url AEAD_AES_192_GCM 0
    test_shadowsocks $url AEAD_AES_256_GCM 0
    test_shadowsocks $url AEAD_CHACHA20_POLY1305 0
    test_shadowsocks $url AES-128-CFB 0
    test_shadowsocks $url AES-128-CTR 0
    test_shadowsocks $url AES-192-CFB 0
    test_shadowsocks $url AES-192-CTR 0
    test_shadowsocks $url AES-256-CFB 0
    test_shadowsocks $url AES-256-CTR 0
    test_shadowsocks $url CHACHA20-IETF 0
    test_shadowsocks $url XCHACHA20 0
}

test_plugin() {
    local url
    url=http://127.0.0.1:$HTTP_PORT/README.md

    test_shadowsocks $url AEAD_AES_128_GCM 0 plugin
    mv $GOPATH/bin/$PLUGIN ./
    test_shadowsocks $url AES-128-CFB 0 plugin
    mv ./$PLUGIN $GOPATH/bin/

    # chmod -x ./$PLUGIN
    # test_shadowsocks $url AEAD_AES_128_GCM 1 plugin
    # mv ./$PLUGIN $GOPATH/bin/
    # test_shadowsocks $url AES-128-CFB 1 plugin
    # chmod +x $GOPATH/bin/$PLUGIN
}

start_http_server

test_methods

test_plugin

stop_http_server
