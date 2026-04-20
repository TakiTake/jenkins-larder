# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ ./cmd/
COPY src/ ./src/

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /larder ./cmd/larder

# Runtime stage
FROM alpine:3.19

RUN apk --no-cache add ca-certificates && \
    adduser -D -u 1000 larder

COPY --from=builder /larder /usr/local/bin/larder
COPY config/default.yaml /etc/jenkins-larder/config.yaml

RUN mkdir -p /var/cache/jenkins-plugins && \
    chown larder:larder /var/cache/jenkins-plugins

USER larder

EXPOSE 8080 8081 9090

ENV CONFIG_PATH=/etc/jenkins-larder/config.yaml

ENTRYPOINT ["larder"]
