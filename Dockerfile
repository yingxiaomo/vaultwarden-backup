FROM alpine:3.21

# 版本信息 — 构建时通过 --build-arg VERSION= 传入
ARG VERSION
LABEL org.opencontainers.image.title="Vaultwarden Backup" \
      org.opencontainers.image.description="自动备份 Vaultwarden 数据并同步到云存储" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.source="https://github.com/yingxiaomo/vaultwarden-backup"

ENV RCLONE_CONFIG=/config/rclone/rclone.conf
ENV APP_VERSION=${VERSION}

# 安装依赖 + 清理 pip 自身和缓存，大幅减小镜像体积
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
    && mkdir -p /app /vw_data /backup /config/rclone \
    && pip3 install --no-cache-dir --break-system-packages apprise \
    && rm -rf /usr/lib/python3.*/ensurepip \
              /usr/lib/python3.*/site-packages/pip \
              /root/.cache/pip \
              /root/.cache

# 复制脚本
COPY backup.sh backup_en.sh entrypoint.sh /app/

RUN chmod +x /app/backup.sh /app/backup_en.sh /app/entrypoint.sh

WORKDIR /app

CMD ["/app/entrypoint.sh"]
