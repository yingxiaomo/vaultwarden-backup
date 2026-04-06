#!/bin/bash

if [ -n "$TZ" ] && [ -f "/usr/share/zoneinfo/$TZ" ]; then
    echo "Configuring timezone to $TZ..."
    cp "/usr/share/zoneinfo/$TZ" /etc/localtime
    echo "$TZ" > /etc/timezone
else
    echo "No TZ environment variable set or invalid timezone. Using default."
fi

# 默认 Cron 表达式，如果未设置则默认每天凌晨 2 点执行
CRON_SCHEDULE="${CRON_SCHEDULE:-0 2 * * *}"

echo "=================================================="
echo "Vaultwarden Backup Container Started"
echo "Cron Schedule: $CRON_SCHEDULE"
echo "=================================================="

# 将当前所有环境变量导出，并兼容 Alpine 的 sh 解析
export -p | grep -v "no_proxy" | sed 's/declare -x/export/g' > /app/env.sh

# 在 crontab 里先 source 环境变量，再执行备份脚本
echo "$CRON_SCHEDULE . /app/env.sh && /app/backup.sh > /proc/1/fd/1 2>/proc/1/fd/2" | crontab -

# 启动即备份开关
if [ "${RUN_ON_STARTUP:-true}" = "true" ]; then
    echo "检测到启动即备份开关，正在执行首次备份任务..."
    /app/backup.sh
fi

# 启动 FastAPI Web 服务
echo "Starting FastAPI Web service..."
uvicorn app:app --host 0.0.0.0 --port 8000 --reload &

# 启动 crond 守护进程
echo "Starting crond daemon..."
exec crond -f -l 2
