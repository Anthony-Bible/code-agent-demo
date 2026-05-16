# Stage 1: Build the Go binary
FROM golang:1.24.11-bookworm AS builder

WORKDIR /build

# Copy dependency files first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /code-agent-demo ./cmd/cli

# Stage 2: Minimal runtime image
FROM alpine:3.21

# Install runtime dependencies needed for AI investigation tool execution
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    bash \
    coreutils \
    curl \
    jq

# Create non-root user (UID 10001 to avoid conflicts with system UIDs)
RUN adduser -D -u 10001 agent

# OCI image labels
ARG REVISION=unknown
ARG CREATED=unknown
LABEL org.opencontainers.image.source="https://github.com/Anthony-Bible/code-agent-demo" \
      org.opencontainers.image.description="AI coding agent webhook server" \
      org.opencontainers.image.revision="${REVISION}" \
      org.opencontainers.image.created="${CREATED}"

# Copy binary from builder
COPY --from=builder /code-agent-demo /app/code-agent-demo

# Copy skills and agents directories if they exist
COPY --from=builder /build/skills/ /app/skills/
COPY --from=builder /build/agents/ /app/agents/
COPY --from=builder /build/config/alert-sources.example.yaml /app/config/alert-sources.example.yaml

# Create writable directories the app needs at runtime
RUN mkdir -p /app/.agent /app/.agent/investigations && \
    chown -R agent:agent /app

WORKDIR /app

EXPOSE 8080

USER agent

ENTRYPOINT ["/app/code-agent-demo"]
CMD ["serve"]
