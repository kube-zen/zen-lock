# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the webhook binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -trimpath -o zen-lock-webhook ./cmd/webhook

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /build/zen-lock-webhook .

# Run as non-root user
RUN addgroup -g 1000 zen-lock && \
    adduser -D -u 1000 -G zen-lock zen-lock && \
    chown -R zen-lock:zen-lock /root

USER zen-lock

ENTRYPOINT ["./zen-lock-webhook"]

