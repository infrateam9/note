# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go files
COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./

# Build the application for the target architecture
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build \
    -ldflags="-X main.Version=docker -X main.BuildTime=$(date -u +'%Y-%m-%dT%H:%M:%SZ') -X main.CommitHash=$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')" \
    -o note-app \
    .

# Runtime stage
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk add --no-cache ca-certificates

# Copy binary from builder
COPY --from=builder /build/note-app .

# Create note directory
RUN mkdir -p /note
RUN chown 8080:8080 /note

USER 8080:8080

# Expose port
EXPOSE 8080

# Set environment variables
ENV PORT=8080
ENV NOTE_DIR=/note

# Health check (uses PORT env var, defaults to 8080)
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --quiet --tries=1 --spider http://localhost:${PORT:-8080}/ || exit 1

# Run the application
CMD ["./note-app"]
