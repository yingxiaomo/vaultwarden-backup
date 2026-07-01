#!/bin/ash

# 设置时区（从环境变量读取，Docker 已传递 TZ 环境变量）
# 如果 TZ 未设置，默认为 Asia/Shanghai
export TZ="${TZ:-Asia/Shanghai}"

# 如果 env.sh 存在，先加载环境变量，确保 crond 继承正确的配置
if [ -f /app/env.sh ]; then
    . /app/env.sh
fi

# 启动 crond 守护进程
# -f: 前台运行（后台通过 & 实现）
# -l 8: 日志级别 8 (debug)
crond -f -l 8 &

# 运行 Web 面板
# 它会通过 lifespan 事件自动处理配置同步、env.sh 生成和 crontab 设置
exec python3 /app/src/app.py
