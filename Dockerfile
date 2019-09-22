FROM golang:1.12.7-alpine3.10 AS builder

ENV GO111MODULE on
ENV GOPROXY https://goproxy.io

RUN apk upgrade \
    && apk add git \
    && go get github.com/shadowsocks/go-shadowsocks2

FROM alpine:3.10 AS dist

LABEL maintainer="mritd <mritd@linux.com>"

RUN apk upgrade \
    && apk add tzdata \
    && rm -rf /var/cache/apk/*

COPY --from=builder /go/bin/go-shadowsocks2 /usr/bin/shadowsocks

ENTRYPOINT ["shadowsocks"]
