#!/bin/bash

# 默认 Cron 表达式，如果未设置则默认每天凌晨 2 点执行
CRON_SCHEDULE="${CRON_SCHEDULE:-0 2 * * *}"

echo "=================================================="
echo "Vaultwarden Backup Container Started"
echo "Cron Schedule: $CRON_SCHEDULE"
echo "=================================================="
echo "$CRON_SCHEDULE /app/backup.sh > /proc/1/fd/1 2>/proc/1/fd/2" | crontab -

# 启动即备份开关
if [ "${RUN_ON_STARTUP:-true}" = "true" ]; then
    echo "检测到启动即备份开关，正在执行首次备份任务..."
    /app/backup.sh
fi

echo "Starting crond daemon..."
exec crond -f -l 2
