#!/bin/bash
set -euo pipefail

# 时区配置
if [ -n "$TZ" ] && [ -f "/usr/share/zoneinfo/$TZ" ]; then
    echo "[entrypoint] 配置时区: $TZ"
    cp "/usr/share/zoneinfo/$TZ" /etc/localtime
    echo "$TZ" > /etc/timezone
else
    echo "[entrypoint] TZ 未设置或无效，使用默认时区"
fi

# 默认值
CRON_SCHEDULE="${CRON_SCHEDULE:-0 2 * * *}"

echo "=================================================="
echo "  Vaultwarden Backup Container Started"
echo "  Cron: $CRON_SCHEDULE"
echo "=================================================="

# 安全导出环境变量到文件，供 crontab 使用
# 正确处理含特殊字符的值（空格、引号、$ 等）
printenv | while IFS="=" read -r name rest; do
    case "$name" in
        no_proxy|NO_PROXY) continue ;;
    esac
    # 使用 printf %q 让 shell 自动处理转义
    printf "export %s=%s\n" "$name" "$(printf "%q" "$rest")"
done > /app/env.sh

# 根据 LANGUAGE 选择备份脚本
LANGUAGE="${LANGUAGE:-zh}"
if [ "$LANGUAGE" = "en" ]; then
    BACKUP_SCRIPT="/app/backup_en.sh"
    echo "[entrypoint] 语言: English"
else
    BACKUP_SCRIPT="/app/backup.sh"
    echo "[entrypoint] 语言: 中文"
fi

# 写入 crontab
echo "$CRON_SCHEDULE . /app/env.sh && $BACKUP_SCRIPT > /proc/1/fd/1 2>/proc/1/fd/2" | crontab -

# 启动时立即备份
if [ "${RUN_ON_STARTUP:-true}" = "true" ]; then
    echo "[entrypoint] RUN_ON_STARTUP=true，执行首次备份..."
    $BACKUP_SCRIPT
fi

echo "[entrypoint] 启动 crond 守护进程..."
exec crond -f -l 2
