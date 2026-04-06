#!/bin/bash

if [ -n "$TZ" ] && [ -f "/usr/share/zoneinfo/$TZ" ]; then
    echo "正在配置时区为 $TZ..."
    cp "/usr/share/zoneinfo/$TZ" /etc/localtime
    echo "$TZ" > /etc/timezone
else
    echo "未设置 TZ 环境变量或时区无效。使用默认时区。"
fi

# 默认 Cron 表达式，如果未设置则默认每天凌晨 2 点执行
CRON_SCHEDULE="${CRON_SCHEDULE:-0 2 * * *}"

echo "=================================================="
echo "Vaultwarden 备份容器已启动"
echo "Cron 执行计划: $CRON_SCHEDULE"
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
echo "正在启动 FastAPI Web 服务..."
uvicorn app:app --host 0.0.0.0 --port 9876 --reload &

# 启动 crond 守护进程
echo "正在启动 crond 守护进程..."
exec crond -f -l 2
