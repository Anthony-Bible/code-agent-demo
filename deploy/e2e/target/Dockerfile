FROM alpine:3.21

# netcat-openbsd needed for `nc -l -p ... -q 0` (BusyBox nc lacks -q).
RUN apk add --no-cache bash curl coreutils netcat-openbsd prometheus-node-exporter

COPY server.sh /app/server.sh
COPY start.sh /app/start.sh
RUN chmod +x /app/start.sh /app/server.sh

EXPOSE 9100 8080
CMD ["/app/start.sh"]
