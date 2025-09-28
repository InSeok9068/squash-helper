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
# 2) 런타임 스테이지 (Debian bookworm-slim)
############################
FROM debian:bookworm-slim AS runtime
ENV DEBIAN_FRONTEND=noninteractive

# 크로미움 + 최소 필수 패키지
RUN apt-get update && apt-get install -y --no-install-recommends \
      chromium ca-certificates tzdata \
      fonts-liberation \
    && rm -rf /var/lib/apt/lists/*


# (옵션) 한글 폰트가 필요하면 주석 해제
# RUN apt-get update && apt-get install -y --no-install-recommends fonts-noto-cjk && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /out/squash-helper /app/squash-helper

EXPOSE 8080
ENTRYPOINT ["/app/squash-helper"]
CMD ["server"]
