# Build stage
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum main.go ./
RUN apk add --no-cache upx && CGO_ENABLED=0 go build -trimpath -tags timetzdata -ldflags='-s -w -buildid=' -o vaultwarden-backup . && upx --best --no-progress --lzma vaultwarden-backup

# Runtime stage
FROM alpine:3.21

ARG VERSION
LABEL org.opencontainers.image.title="Vaultwarden Backup" org.opencontainers.image.description="自动备份 Vaultwarden 数据并同步到云存储" org.opencontainers.image.version="${VERSION}" org.opencontainers.image.source="https://github.com/yingxiaomo/vaultwarden-backup"

ENV RCLONE_CONFIG=/config/rclone/rclone.conf
ENV APP_VERSION=${VERSION}

RUN apk add --no-cache ca-certificates sqlite && mkdir -p /app /vw_data /backup /config/rclone

COPY --from=builder /app/vaultwarden-backup /app/vaultwarden-backup

WORKDIR /app
CMD ["/app/vaultwarden-backup"]
