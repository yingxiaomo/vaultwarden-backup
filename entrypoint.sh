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

# 安全导出环境变量，正确处理单引号、$、等号等所有特殊字符
# 将值中的单引号替换为结束引号-转义-重新开始引号，再用单引号整体包裹
printenv | while IFS='=' read -r name rest; do
    # 跳过 no_proxy 相关变量（可能含通配符，影响 shell 解析）
    case "$name" in no_proxy|NO_PROXY) continue ;; esac
    # 将单引号替换为 '\''（结束单引号 + 转义单引号 + 重新开始单引号）
    rest_escaped="${rest//\'/\'\\\'\'}"
    printf "export %s='%s'\n" "$name" "$rest_escaped"
done > /app/env.sh

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