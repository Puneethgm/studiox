# syntax=docker/dockerfile:1.6
# Build context: repo root.
#   docker build -f deploy/api.Dockerfile -t projectx-api .
#
# Final image bundles three binaries: server (the API), seed (super-admin
# bootstrap), and goose (DB migrations). The compose file invokes the
# right entrypoint per service.

# ---------- build ----------
FROM golang:1.24-alpine AS build
WORKDIR /src

# Cache deps separately from source.
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download

# Source.
COPY apps/api/ ./

ENV CGO_ENABLED=0 GOOS=linux

RUN go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server \
 && go build -trimpath -ldflags="-s -w" -o /out/seed   ./cmd/seed \
 && GOBIN=/out go install github.com/pressly/goose/v3/cmd/goose@v3.22.0

# ---------- runtime ----------
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /home/nonroot

COPY --from=build /out/server /usr/local/bin/server
COPY --from=build /out/seed   /usr/local/bin/seed
COPY --from=build /out/goose  /usr/local/bin/goose
COPY --from=build /src/migrations /migrations

EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/server"]
