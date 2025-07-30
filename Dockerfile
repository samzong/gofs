# Stage 1: Build the binary
FROM golang:1.24-alpine AS builder

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

# Build the binary with security hardening
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -a -installsuffix cgo \
    -ldflags="-w -s -extldflags '-static'" \
    -tags netgo \
    -trimpath \
    -o gofs ./cmd/gofs

# Verify the binary works
RUN ./gofs --version

# Stage 2: Create the minimal runtime image
FROM scratch

# Copy essential certificates from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the statically linked binary
COPY --from=builder /build/gofs /gofs

# Create minimal filesystem structure
COPY --from=builder --chown=65534:65534 /tmp /tmp

# Use nobody user (65534) for maximum security
USER 65534:65534

# Expose the configurable port
EXPOSE 8000

# Health check using the built-in health check functionality
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/gofs", "--health-check"] || exit 1

# Default configuration optimized for containers
ENTRYPOINT ["/gofs"]
CMD ["--host", "0.0.0.0", "--port", "8000", "--dir", "/data"]
