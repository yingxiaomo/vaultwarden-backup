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

echo "正在同步 Web 面板配置到底层运行环境..."
# 让 Python 读取 config.yaml 并强制生成最新的 /app/env.sh
# 直接使用 get_env_vars() 会触发配置初始化，然后通过 startup_event 生成 env.sh
python3 -c "from app import get_env_vars; get_env_vars()"
# 手动生成 env.sh
python3 -c "from app import get_env_vars; env_vars = get_env_vars(); import os; f = open('/app/env.sh', 'w'); [f.write(f'export {k}={repr(v)}\n') for k, v in env_vars.items() if v is not None]; f.close()"

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
