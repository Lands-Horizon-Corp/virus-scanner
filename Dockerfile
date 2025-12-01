# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o virus-scanner main.go

# Runtime stage with ClamAV
FROM clamav/clamav:latest

# Install curl for health checks
RUN apk add --no-cache curl

# Create app directory
WORKDIR /app

# Copy the built application
COPY --from=builder /app/virus-scanner .
COPY --from=builder /app/index.html .

# Create a non-root user
RUN adduser -D -s /bin/sh appuser && \
    chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose port 8081
EXPOSE 8081

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
  CMD curl -f http://localhost:8081/ || exit 1

# Start the virus scanner
CMD ["./virus-scanner"]
