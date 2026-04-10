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
printenv | awk -F= '{print "export " $1 "='\''" substr($0, index($0,$2)) "'\''"}' | grep -v "no_proxy" > /app/env.sh

# 根据LANGUAGE环境变量选择备份脚本
LANGUAGE="${LANGUAGE:-zh}"
if [ "$LANGUAGE" = "en" ]; then
    BACKUP_SCRIPT="/app/backup_en.sh"
    echo "Language set to English, using backup_en.sh"
else
    BACKUP_SCRIPT="/app/backup.sh"
    echo "Language set to Chinese, using backup.sh"
fi

# 在 crontab 里先 source 环境变量，再执行备份脚本
echo "$CRON_SCHEDULE . /app/env.sh && $BACKUP_SCRIPT > /proc/1/fd/1 2>/proc/1/fd/2" | crontab -

# 启动即备份开关
if [ "${RUN_ON_STARTUP:-true}" = "true" ]; then
    echo "检测到启动即备份开关，正在执行首次备份任务..."
    $BACKUP_SCRIPT
fi

echo "Starting crond daemon..."
exec crond -f -l 2
