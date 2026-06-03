# ── Build stage ────────────────────────────────────────
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go module files first for better caching
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download

# Copy all source files
COPY apps/api/ .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o /api-server ./cmd/server

# ── Runtime stage ───────────────────────────────────────
FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /api-server /app/api-server

# Create uploads directory
RUN mkdir -p /app/uploads

EXPOSE 8080

CMD ["/app/api-server"]
