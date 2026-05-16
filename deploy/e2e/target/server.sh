#!/bin/bash
# Naive single-connection HTTP server. Easy to overload.
while true; do
  printf 'HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nConnection: close\r\n\r\nok %s\n' "$(date)" \
    | nc -l -p 8080 -q 0
done
