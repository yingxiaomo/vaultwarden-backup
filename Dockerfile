
FROM alpine:latest

ENV RCLONE_CONFIG=/config/rclone/rclone.conf

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
COPY backup_en.sh /app/backup_en.sh

RUN chmod +x /app/backup.sh /app/backup_en.sh /app/entrypoint.sh

WORKDIR /app

CMD ["/app/entrypoint.sh"]
