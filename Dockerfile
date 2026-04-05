# 使用 postgres:16-alpine 作为基础镜像，自带 PostgreSQL 客户端工具，且体积小巧
FROM postgres:16-alpine

# 设置默认时区为 Asia/Shanghai
ENV TZ=Asia/Shanghai

# 更新 apk 缓存并安装必要的环境依赖
RUN apk update && apk add --no-cache \
    mariadb-client \
    sqlite \
    rclone \
    zip \
    python3 \
    py3-pip \
    bash \
    tzdata \
    curl \
    # 配置时区数据
    && cp /usr/share/zoneinfo/$TZ /etc/localtime \
    && echo $TZ > /etc/timezone \
    # 使用 pip 安装 apprise 通知工具
    # 在 alpine 中使用 --break-system-packages 允许在系统环境中安装 python 包
    && pip3 install --no-cache-dir --break-system-packages apprise

# 创建备份脚本存放目录和数据挂载点
RUN mkdir -p /app /vw_data /backup /config/rclone

# 将本地的 backup.sh 脚本复制到容器内的 /app 目录
COPY backup.sh /app/backup.sh

# 赋予脚本可执行权限
RUN chmod +x /app/backup.sh

# 设置工作目录
WORKDIR /app

# 默认执行命令，可以配合 cron 或直接运行脚本
CMD ["/app/backup.sh"]