ARG GO_VERSION=1.24
ARG ALPINE_VERSION=3.21.3

# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS builder
WORKDIR /src
RUN apk add --no-cache build-base        # Compiler/Linker für CGO

COPY go.mod go.sum ./
RUN go mod download
COPY . .

# CGO aktiv (passt Flags bei Bedarf an)
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /target/app .

FROM alpine:${ALPINE_VERSION}
# Laufzeit-Libs je nach Bedarf (Beispiele):
RUN apk add --no-cache ca-certificates libstdc++ tzdata

COPY --from=builder /target/app /app

ENTRYPOINT ["/app"]