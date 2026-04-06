
FROM postgres:16-alpine

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
    && pip3 install --no-cache-dir --break-system-packages apprise fastapi uvicorn jinja2 python-docker

RUN mkdir -p /app /vw_data /backup /config/rclone

COPY backup.sh /app/backup.sh
COPY entrypoint.sh /app/entrypoint.sh
COPY app.py /app/app.py
COPY templates /app/templates
COPY static /app/static

RUN chmod +x /app/backup.sh /app/entrypoint.sh

WORKDIR /app

CMD ["/app/entrypoint.sh"]