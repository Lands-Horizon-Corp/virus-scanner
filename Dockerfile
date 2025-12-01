# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o virus-scanner main.go

# Runtime stage - use Ubuntu which handles permissions better
FROM ubuntu:22.04

# Install only essential ClamAV packages
RUN apt-get update && \
    apt-get install -y \
    clamav \
    clamav-freshclam \
    curl \
    ca-certificates && \
    rm -rf /var/lib/apt/lists/* && \
    apt-get clean

# Initialize ClamAV databases as root
RUN mkdir -p /var/lib/clamav && \
    chown clamav:clamav /var/lib/clamav && \
    freshclam --quiet || echo "Initial freshclam completed"

# Create app user and setup directories
RUN useradd -m -s /bin/bash appuser && \
    mkdir -p /app && \
    chown -R appuser:appuser /app

# Copy application
COPY --from=builder /app/virus-scanner /app/
COPY --from=builder /app/index.html /app/

# Switch to user immediately to avoid permission issues
USER appuser
WORKDIR /app

# Expose port
EXPOSE 8081

# Health check with longer timeout for memory-constrained environment
HEALTHCHECK --interval=60s --timeout=15s --start-period=90s --retries=2 \
  CMD curl -f http://localhost:8081/ || exit 1

# Start the application
CMD ["./virus-scanner"]