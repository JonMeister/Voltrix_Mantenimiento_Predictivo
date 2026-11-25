# =============================================================================
# Multi-stage Dockerfile — Voltrix Energy · Mantenimiento Predictivo (Go)
# Imagen final reproducible, usuario no-root, tamaño < 25 MB.
# =============================================================================

# Etapa 1: Compilación estática en Go
FROM golang:1.22-alpine AS builder

WORKDIR /src

COPY go.mod ./
COPY . .

RUN go mod tidy && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /out/mantenimiento-api ./cmd/api

# Etapa 2: Runtime minimalista Alpine
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata wget && \
    addgroup -S voltrix && adduser -S -G voltrix voltrix

WORKDIR /app
COPY --from=builder /out/mantenimiento-api /app/mantenimiento-api

USER voltrix

EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/health || exit 1

ENTRYPOINT ["/app/mantenimiento-api"]

