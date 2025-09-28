# syntax=docker/dockerfile:1

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
# 2) 런타임 스테이지 (Ubuntu 24.04)
############################
FROM ubuntu:24.04 AS runtime
ENV DEBIAN_FRONTEND=noninteractive

# Headless Chromium/rod 실행에 필요한 런타임 의존성
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
      ca-certificates curl unzip tzdata \
      # t64 리네임 반영
      libglib2.0-0t64 \
      libnss3 libnspr4 \
      libx11-6t64 libxcomposite1 libxdamage1 libxrandr2 \
      libxkbcommon0 libgtk-3-0t64 libdrm2 libgbm1 \
      libasound2 libxshmfence1 libcups2 \
      libatk-bridge2.0-0 libpangocairo-1.0-0 \
      fonts-liberation \
    ; rm -rf /var/lib/apt/lists/*

# (옵션) 한글 폰트가 필요하면 주석 해제
# RUN apt-get update && apt-get install -y --no-install-recommends fonts-noto-cjk && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /out/squash-helper /app/squash-helper

EXPOSE 8080
ENTRYPOINT ["/app/squash-helper"]
CMD ["server"]
