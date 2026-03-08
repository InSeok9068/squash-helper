# syntax=docker/dockerfile:1.7
ARG BROWSER_BASE_IMAGE=browser-base

############################
# 0) Chromium 전용 스테이지
# - 로컬 빌드 시에는 이 stage를 그대로 runtime base로 사용
# - CI에서는 이 stage를 별도 이미지(browser-base)로 미리 푸시해 재사용
############################
FROM debian:bookworm-slim AS browser-base
ENV DEBIAN_FRONTEND=noninteractive
ENV BROWSER_BIN=/usr/bin/chromium
# APT 캐시 마운트 (BuildKit 필요)
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    apt-get update; \
    apt-get install -y --no-install-recommends \
      chromium ca-certificates tzdata fonts-liberation \
      fonts-noto-cjk fonts-noto-color-emoji fonts-nanum \
    ; \
    rm -rf /var/lib/apt/lists/*

############################
# 1) Go 빌드 스테이지
############################
FROM --platform=$BUILDPLATFORM golang:1.24 AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG TARGETOS=linux
ARG TARGETARCH=arm64
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "-s -w" -o /out/squash-helper .

############################
# 2) 런타임 스테이지
# - 기본값은 같은 Dockerfile의 browser-base stage
# - CI에서는 build arg로 GHCR의 browser-base 이미지를 주입
############################
FROM ${BROWSER_BASE_IMAGE} AS runtime
WORKDIR /app
COPY --from=builder /out/squash-helper /app/squash-helper

EXPOSE 8080
ENTRYPOINT ["/app/squash-helper"]
CMD ["server"]
