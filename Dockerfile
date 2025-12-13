# TODO (T066): Create production-ready Dockerfile

# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY cmd/ ./cmd/
COPY src/ ./src/

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /jenkins-mirror ./cmd/mirror

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /jenkins-mirror .

# Copy default config
COPY config/default.yaml /etc/jenkins-mirror/config.yaml

# Create cache directory
RUN mkdir -p /var/cache/jenkins-plugins

# Expose ports
EXPOSE 8080 8081 9090

# Set environment variable for config
ENV CONFIG_PATH=/etc/jenkins-mirror/config.yaml

CMD ["./jenkins-mirror"]
