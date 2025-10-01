# Multi-stage build for Go binary
FROM golang:1.22-alpine AS builder

# Set working directory
WORKDIR /app

# Install git (required for some Go modules)
RUN apk add --no-cache git

# Copy go.mod and go.sum first for better Docker layer caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY pkg/ pkg/
COPY cmd/ cmd/

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o downloader ./cmd/downloader

# Final stage - minimal runtime image
FROM alpine:3.18

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Create downloads directory
RUN mkdir -p /downloads

# Create non-root user for security
RUN addgroup -g 1001 -S app && \
    adduser -u 1001 -S app -G app

WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/downloader .

# Make binary executable and set ownership
RUN chmod +x downloader && \
    chown -R app:app /app /downloads

# Switch to non-root user
USER app

# Set default download directory
ENV DOWNLOAD_DIR=/downloads

# Default command
ENTRYPOINT ["./downloader"]
CMD ["-help"]