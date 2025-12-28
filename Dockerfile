# Build stage
FROM golang:1.24-alpine AS builder

ARG VERSION=0.0.1-alpha
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git make

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build optimized binary
# CGO_ENABLED=0: Static binary, no C dependencies
# -ldflags="-s -w": Strip debug info and symbol table (~30% size reduction)
# -trimpath: Remove file system paths (security + reproducible builds)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags "-s -w \
        -X 'main.version=${VERSION}' \
        -X 'main.commit=${COMMIT}' \
        -X 'main.buildDate=${BUILD_DATE}'" \
    -o zen-lock-webhook ./cmd/webhook

# Runtime stage - use scratch (empty) base for minimal size
# The binary is statically linked (CGO_ENABLED=0), so no libc needed
FROM scratch

# Copy CA certificates from Alpine for HTTPS/TLS support (needed for Kubernetes API)
# This is much smaller than the full Alpine base (~200KB vs 8MB)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy binary
COPY --from=builder /build/zen-lock-webhook /zen-lock-webhook

EXPOSE 8080 8081 9443

ENTRYPOINT ["/zen-lock-webhook"]
