# Build stage
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache make gcc musl-dev

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

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

# Copy binaries from builder
COPY --from=builder /app/bin/* /app/

# Copy configuration
COPY config/config.yaml /app/config/

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