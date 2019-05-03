FROM golang:1.12.4-alpine3.9 AS builder

WORKDIR /shadowsocks2

COPY . .

RUN apk add git \
    && CGO_ENABLED=0 go build -ldflags '-w -s' -o shadowsocks2

FROM alpine:3.9

LABEL maintainer="mritd <mritd@linux.com>"

RUN apk upgrade \
    && apk add tzdata \
    && rm -rf /var/cache/apk/*

COPY --from=builder /shadowsocks2/shadowsocks2 /usr/bin/shadowsocks2
COPY defaultstart.sh /shadowsocks2/start.sh

ENV SS_PASSWORD password
ENV SS_METHOD aes-256-cfb

WORKDIR /shadowsocks2

CMD ["sh", "start.sh"]