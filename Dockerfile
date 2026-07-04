# ============================================================
# 阶段一：编译 Go 二进制文件
# ============================================================
FROM golang:1.23-alpine AS builder

WORKDIR /build
COPY go.mod main.go web.go ./
COPY templates/ ./templates/

RUN go build -ldflags="-s -w" -o vaultwarden-backup .

# ============================================================
# 阶段二：最小运行镜像
# ============================================================
FROM alpine:3.21

ARG VERSION=0.0.0
LABEL org.opencontainers.image.title="Vaultwarden Backup" \
      org.opencontainers.image.description="Vaultwarden 数据备份工具 — 支持数据库导出、加密压缩、云同步和 Web 管理面板" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.source="https://github.com/yingxiaomo/vaultwarden-backup"

ENV RCLONE_CONFIG=/config/rclone/rclone.conf

# 安装运行时依赖（仅备份所需的 CLI 工具）
RUN apk add --no-cache \
    mariadb-client \
    sqlite \
    rclone \
    zip \
    unzip \
    tzdata \
    ca-certificates \
    curl \
    postgresql-client \
    && rm -rf /var/cache/apk/*

# 创建必要的目录结构
RUN mkdir -p /app /data /backup /config/rclone /app/config

# 复制编译好的二进制文件和入口脚本
COPY --from=builder /build/vaultwarden-backup /app/vaultwarden-backup
COPY entrypoint.sh /app/entrypoint.sh

RUN chmod +x /app/entrypoint.sh

WORKDIR /app

# 默认以 Web 面板模式启动
CMD ["/app/entrypoint.sh"]