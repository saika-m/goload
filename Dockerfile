# Build stage
FROM golang:1.22-alpine AS builder

# Install build dependencies including git
RUN apk add --no-cache \
    make \
    gcc \
    musl-dev \
    git

# Set working directory
WORKDIR /app

# Copy go mod file first
COPY go.mod ./

# Initialize the module and download dependencies
RUN go mod download && \
    go mod verify

# Copy the rest of the source code
COPY . .

# Set up local module
RUN go mod edit -replace github.com/saika-m/goload=./

# Ensure all dependencies are downloaded and up to date
RUN go mod tidy

# Build application
RUN make build

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN adduser -D -u 1000 goload

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/bin/goload /app/

# Copy configuration directory
COPY --from=builder /app/config /app/config/

# Copy entrypoint script
COPY worker-entrypoint.sh /app/
RUN chmod +x /app/worker-entrypoint.sh

# Set ownership
RUN chown -R goload:goload /app

# Switch to non-root user
USER goload

# Set environment variables
ENV GO_ENV=production

# Expose ports
EXPOSE 8080 9090

# Health check
HEALTHCHECK --interval=30s --timeout=3s \
  CMD wget --quiet --tries=1 --spider http://localhost:8080/health || exit 1

# Default command (can be overridden)
CMD ["./goload"]