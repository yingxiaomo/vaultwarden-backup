FROM alpine:latest

# 版本信息 — 构建时通过 --build-arg VERSION= 传入
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
    bash \
    tzdata \
    curl \
    postgresql-client \
    jq \
    && pip3 install --no-cache-dir --break-system-packages apprise

RUN mkdir -p /app /vw_data /backup /config/rclone

COPY backup.sh /app/backup.sh
COPY backup_en.sh /app/backup_en.sh
COPY entrypoint.sh /app/entrypoint.sh

RUN chmod +x /app/backup.sh /app/backup_en.sh /app/entrypoint.sh

WORKDIR /app

CMD ["/app/entrypoint.sh"]
