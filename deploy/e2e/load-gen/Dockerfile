FROM alpine:3.21

# stress-ng lives in the community repository on Alpine.
RUN apk add --no-cache bash curl coreutils stress-ng

COPY generate-load.sh /app/generate-load.sh
RUN chmod +x /app/generate-load.sh

CMD ["/app/generate-load.sh"]
