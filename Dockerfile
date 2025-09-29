# syntax=docker/dockerfile:1.7

############################
# 0) Chromium 전용 스테이지 (변경 적음 → 캐시 오래감)
############################
FROM debian:bookworm-slim AS chromium
ENV DEBIAN_FRONTEND=noninteractive
# APT 캐시 마운트 (BuildKit 필요)
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    apt-get update; \
    apt-get install -y --no-install-recommends \
      chromium ca-certificates tzdata fonts-liberation \
    ; \
    rm -rf /var/lib/apt/lists/*

############################
# 1) Go 빌드 스테이지
############################
FROM golang:1.24 AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG TARGETOS=linux
ARG TARGETARCH=arm64
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "-s -w" -o /out/squash-helper .

############################
# 2) 런타임 스테이지 (Chromium stage 재사용)
############################
FROM chromium AS runtime
WORKDIR /app
COPY --from=builder /out/squash-helper /app/squash-helper

EXPOSE 8080
ENTRYPOINT ["/app/squash-helper"]
CMD ["server"]
