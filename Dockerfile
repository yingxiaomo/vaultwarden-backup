# Build stage
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod main.go ./
RUN go build -o vaultwarden-backup .

# Runtime stage
FROM alpine:3.21

ARG VERSION
LABEL org.opencontainers.image.title="Vaultwarden Backup" \
      org.opencontainers.image.description="自动备份 Vaultwarden 数据并同步到云存储" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.source="https://github.com/yingxiaomo/vaultwarden-backup"

ENV RCLONE_CONFIG=/config/rclone/rclone.conf
ENV APP_VERSION=${VERSION}

RUN apk add --no-cache \
    mariadb-client \
    sqlite \
    rclone \
    zip \
    unzip \
    python3 \
    py3-pip \
    tzdata \
    curl \
    postgresql-client \
    jq \
    && mkdir -p /app /vw_data /backup /config/rclone \
    && pip3 install --no-cache-dir --break-system-packages apprise \
    && rm -rf /usr/lib/python3.*/ensurepip \
              /usr/lib/python3.*/site-packages/pip \
              /root/.cache/pip

COPY --from=builder /app/vaultwarden-backup /app/vaultwarden-backup

WORKDIR /app
CMD ["/app/vaultwarden-backup"]
