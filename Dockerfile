# Stage 1: Build the binary
FROM golang:1.24-alpine AS builder

# Build arguments
ARG VERSION=dev
ARG BUILD_TIME
ARG GO_VERSION

# Install build dependencies securely
RUN apk add --no-cache ca-certificates git

WORKDIR /build

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./

# Download dependencies with proper verification
# Using specific versions to prevent supply chain attacks
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build the binary with security hardening and version injection
RUN GO_VERSION_DETECTED=$(go version | awk '{print $3}') && \
    CGO_ENABLED=0 GOOS=linux go build \
    -buildvcs=false \
    -ldflags="-w -s -extldflags '-static' -X main.version=${VERSION:-dev} -X main.buildTime=${BUILD_TIME:-$(date -u +%Y-%m-%dT%H:%M:%SZ)} -X main.goVersion=${GO_VERSION:-$GO_VERSION_DETECTED}" \
    -tags 'netgo,osusergo' \
    -trimpath \
    -o gofs ./cmd/gofs

# Verify the binary works
RUN ./gofs --version

# Stage 2: Create the minimal runtime image using Chainguard's hardened image
FROM cgr.dev/chainguard/static:latest

# Security labels for compliance and traceability
LABEL org.opencontainers.image.vendor="gofs" \
      org.opencontainers.image.title="gofs" \
      org.opencontainers.image.description="Secure file server written in Go" \
      org.opencontainers.image.url="https://github.com/samzong/gofs" \
      org.opencontainers.image.source="https://github.com/samzong/gofs" \
      org.opencontainers.image.licenses="MIT" \
      security.scan.enabled="true" \
      security.hardening.enabled="true" \
      security.nonroot="true"

# Copy the statically linked binary
COPY --from=builder /build/gofs /gofs

# Set security context - Chainguard images run as nonroot by default
USER 65532:65532

# Set working directory and home for security
WORKDIR /
ENV HOME=/tmp

# Expose the configurable port
EXPOSE 8000

# Health check using HTTP endpoint (requires wget/curl in container)
# For scratch images, health checks should be handled by orchestrator
# HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
#     CMD wget --no-verbose --tries=1 --spider http://localhost:8000/healthz || exit 1

# Default configuration optimized for containers
ENTRYPOINT ["/gofs"]
CMD ["--host", "0.0.0.0", "--port", "8000", "--dir", "/data"]
