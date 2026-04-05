#!/bin/bash

# 默认 Cron 表达式，如果未设置则默认每天凌晨 2 点执行
CRON_SCHEDULE="${CRON_SCHEDULE:-0 2 * * *}"

echo "=================================================="
echo "Vaultwarden Backup Container Started"
echo "Cron Schedule: $CRON_SCHEDULE"
echo "=================================================="

# 配置 crontab，将输出重定向到容器的标准输出和标准错误，以便在 docker logs 中查看
echo "$CRON_SCHEDULE /app/backup.sh > /proc/1/fd/1 2>/proc/1/fd/2" | crontab -

# 在前台启动 crond 守护进程
# -f: 前台运行
# -l 2: 日志级别 (0-8, 默认 8, 2 表示打印基本信息)
echo "Starting crond daemon..."
exec crond -f -l 2
