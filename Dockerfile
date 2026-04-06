
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
    && pip3 install --no-cache-dir --break-system-packages apprise fastapi uvicorn jinja2 docker pyyaml python-multipart

RUN mkdir -p /app /vw_data /backup /config/rclone

COPY lib /app/lib
COPY src /app/src
COPY templates /app/templates
COPY static /app/static
COPY config.yaml.example /app/config.yaml.example

RUN chmod +x /app/lib/scripts/backup.sh /app/lib/scripts/entrypoint.sh

WORKDIR /app

CMD ["/app/lib/scripts/entrypoint.sh"]