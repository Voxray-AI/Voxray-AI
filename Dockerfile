# Multi-stage build for Voxray-Go server
# Build stage
FROM golang:1.25-alpine AS builder
WORKDIR /app

# Copy module files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /voxray ./cmd/voxray

# Run stage: minimal image
FROM alpine:3.20
RUN adduser -D -g "" voxray
USER voxray
WORKDIR /app

# Config can be mounted or provided via VOXRAY_CONFIG env
ENV VOXRAY_CONFIG=/app/config.json

COPY --from=builder /voxray /voxray

EXPOSE 8080

ENTRYPOINT ["/voxray"]
CMD ["-config", "/app/config.json"]
