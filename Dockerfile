# syntax=docker/dockerfile:1

# ─── Build stage ─────────────────────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

WORKDIR /src

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/docs-mcp ./cmd/server

# ─── Runtime stage ───────────────────────────────────────────────────────────
FROM alpine:3.21

RUN apk add --no-cache ca-certificates git

COPY --from=builder /bin/docs-mcp /usr/local/bin/docs-mcp

EXPOSE 8000

ENTRYPOINT ["docs-mcp"]
