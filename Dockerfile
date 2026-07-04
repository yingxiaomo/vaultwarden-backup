FROM alpine:latest

ARG VERSION
LABEL org.opencontainers.image.title="Vaultwarden Backup" \
      org.opencontainers.image.description="Web 管理面板 - 自动备份 Vaultwarden 数据并同步到云存储" \
      org.opencontainers.image.version="" \
      org.opencontainers.image.source="https://github.com/yingxiaomo/vaultwarden-backup"

ENV RCLONE_CONFIG=/config/rclone/rclone.conf
ENV APP_VERSION=

RUN apk add --no-cache \
    mariadb-client \
    sqlite \
    rclone \
    jq \
    zip \
    unzip \
    python3 \
    py3-pip \
    bash \
    tzdata \
    curl \
    postgresql-client \
    && pip3 install --no-cache-dir --break-system-packages fastapi uvicorn jinja2 pyyaml python-multipart

RUN mkdir -p /app /vw_data /config/rclone /backup /app/config

# Web 面板
COPY src/app.py /app/src/app.py
COPY templates /app/templates

# 备份脚本
COPY lib /app/lib
COPY backup.sh /app/backup.sh
COPY backup_en.sh /app/backup_en.sh
COPY entrypoint.sh /app/entrypoint.sh

RUN chmod +x /app/lib/scripts/backup.sh /app/lib/scripts/entrypoint.sh /app/backup.sh /app/backup_en.sh /app/entrypoint.sh

WORKDIR /app
CMD ["/app/lib/scripts/entrypoint.sh"]
