FROM alpine:latest

ARG VERSION
LABEL org.opencontainers.image.title="Vaultwarden Backup Web Panel" \
      org.opencontainers.image.description="Web 管理面板 — 自动备份 Vaultwarden 数据并同步到云存储" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.source="https://github.com/yingxiaomo/vaultwarden-backup"

ENV RCLONE_CONFIG=/config/rclone/rclone.conf
ENV APP_VERSION=${VERSION}

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
    && pip3 install --no-cache-dir --break-system-packages apprise fastapi uvicorn jinja2 docker pyyaml python-multipart

RUN mkdir -p /app /vw_data /config/rclone /backup

COPY lib /app/lib
COPY src /app/src
COPY app_config /app/app_config
COPY routes /app/routes
COPY services /app/services
COPY utils /app/utils
COPY templates /app/templates
COPY static /app/static
COPY config.yaml.example /app/config.yaml.example

RUN chmod +x /app/lib/scripts/backup.sh /app/lib/scripts/entrypoint.sh

WORKDIR /app

CMD ["/app/lib/scripts/entrypoint.sh"]
