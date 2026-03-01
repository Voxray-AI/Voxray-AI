# Multi-stage build for Voila-Go server
# Build stage
FROM golang:1.25-alpine AS builder
WORKDIR /app

# Copy module files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /voila ./cmd/voila

# Run stage: minimal image
FROM alpine:3.20
RUN adduser -D -g "" voila
USER voila
WORKDIR /app

# Config can be mounted or provided via VOILA_CONFIG env
ENV VOILA_CONFIG=/app/config.json

COPY --from=builder /voila /voila

EXPOSE 8080

ENTRYPOINT ["/voila"]
CMD ["-config", "/app/config.json"]
