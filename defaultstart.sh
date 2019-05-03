#!/bin/sh
echo "Starting Shadowsocks Server..."
echo "Use Method: ${SS_METHOD}; Password:${SS_PASSWORD}"
shadowsocks -s ss://${SS_METHOD}:${SS_PASSWORD}@:8080 -verbose