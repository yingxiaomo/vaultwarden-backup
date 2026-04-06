#!/bin/ash

# 启动 crond 守护进程
crond -f -l 8 &

# 运行 Web 面板（它会通过 startup 事件自动处理同步和启动备份）
python3 /app/src/app.py
