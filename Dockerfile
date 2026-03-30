# syntax=docker/dockerfile:1
# Multi-stage production image: compile static binary, run as non-root in minimal runtime.

ARG GO_VERSION=1.23
ARG TARGETOS=linux
ARG TARGETARCH=amd64

# --- deps: modules only (better layer cache) ---
FROM golang:${GO_VERSION}-bookworm AS deps
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# --- build ---
FROM deps AS builder
ARG VERSION=dev
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY . .
ENV CGO_ENABLED=0
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/web-scraper .

# --- runtime: distroless static non-root ---
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=builder /out/web-scraper /app/web-scraper
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/web-scraper"]
