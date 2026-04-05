
FROM postgres:16-alpine

# 设置默认时区和 Rclone 全局配置文件路径
ENV TZ=Asia/Shanghai \
    RCLONE_CONFIG=/config/rclone/rclone.conf

# 直接使用 --no-cache 安装，无需冗余的 apk update
RUN apk add --no-cache \
    mariadb-client \
    sqlite \
    rclone \
    zip \
    python3 \
    py3-pip \
    bash \
    tzdata \
    curl \
    && cp /usr/share/zoneinfo/$TZ /etc/localtime \
    && echo $TZ > /etc/timezone \
    && pip3 install --no-cache-dir --break-system-packages apprise

RUN mkdir -p /app /vw_data /backup /config/rclone

COPY backup.sh /app/backup.sh
COPY entrypoint.sh /app/entrypoint.sh

RUN chmod +x /app/backup.sh /app/entrypoint.sh

WORKDIR /app

CMD ["/app/entrypoint.sh"]