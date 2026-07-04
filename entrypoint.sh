#!/bin/sh
# ============================================================
# Vaultwarden Backup 入口脚本
# ============================================================

# 设置时区
if [ -n "$TZ" ] && [ -f "/usr/share/zoneinfo/$TZ" ]; then
    echo "设置时区: $TZ"
    cp "/usr/share/zoneinfo/$TZ" /etc/localtime
    echo "$TZ" > /etc/timezone
fi

# 默认行为：后台调度模式（常驻容器，按 CRON_SCHEDULE 定时备份）
# 与旧版 crond 容器的行为完全一致
echo "启动 Vaultwarden Backup..."
exec /app/vaultwarden-backup