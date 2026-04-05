#!/bin/bash

# ==========================================
# Vaultwarden 备份脚本 (支持 SQLite/MySQL/PostgreSQL + Apprise 通知)
# ==========================================

# 强制设置网络代理，确保 Apprise 和 Rclone 能够顺畅出海 (如果不需要代理，请在环境变量中留空)
export HTTP_PROXY="${HTTP_PROXY:-}"   # HTTP 代理地址，例如 http://192.168.1.100:7890
export HTTPS_PROXY="${HTTPS_PROXY:-}" # HTTPS 代理地址，例如 http://192.168.1.100:7890
export ALL_PROXY="${ALL_PROXY:-}"     # SOCKS5 代理地址，例如 socks5://192.168.1.100:7890

# 基础配置变量 (可通过环境变量覆盖)
DB_TYPE="${DB_TYPE:-sqlite}"          # 数据库类型: sqlite, mysql, postgres
DATA_DIR="${DATA_DIR:-/vw_data}"      # Vaultwarden 数据目录
BACKUP_DIR="${BACKUP_DIR:-/backup}"   # 本地临时备份目录
ZIP_PASSWORD="${ZIP_PASSWORD:-}"      # 压缩包加密密码 (必填)
APPRISE_URL="${APPRISE_URL:-}"        # Apprise 通知 URL (直接使用命令行工具)
# 读取 API URL，并使用 %/ 自动移除末尾可能多余的斜杠，包容用户的书写习惯
APPRISE_API_URL="${APPRISE_API_URL%/}"  # Apprise 服务 API 地址 (使用独立 Apprise 服务)
RCLONE_REMOTE="${RCLONE_REMOTE:-}"    # Rclone 远程路径，例如 myremote:/vaultwarden_backup

# 时间戳，用于生成唯一的文件名
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")    # 格式: 年月日_时分秒
# 备份文件前缀，可通过环境变量自定义
PREFIX="${BACKUP_PREFIX:-vaultwarden_backup}"
BACKUP_NAME="${PREFIX}_${TIMESTAMP}" # 备份文件基础名称
SQL_FILE="${BACKUP_DIR}/${BACKUP_NAME}.sql"   # 导出的 SQL 文件路径
ZIP_FILE="${BACKUP_DIR}/${BACKUP_NAME}.zip"   # 最终的加密压缩包路径

# 压缩包生成完毕标志
ZIP_DONE=0
# 无论脚本如何退出，都尝试删除临时文件
cleanup() {
  rm -f "$SQL_FILE"
  # 只有在打包"还没完成"时就异常退出，才去清理残缺的 zip 文件
  if [ "$ZIP_DONE" -ne 1 ] && [ -f "$ZIP_FILE" ]; then
      rm -f "$ZIP_FILE"
  fi
}
trap cleanup EXIT

# 发送通知的辅助函数
send_notification() {
    local title="$1"  # 通知标题
    local body="$2"   # 通知内容
    
    # 优先使用独立的 Apprise 服务 API
    if [ -n "$APPRISE_API_URL" ]; then
        echo "使用独立 Apprise 服务发送通知..."
        
        # 玩法 1：如果用户同时配置了 API 地址和具体的通知 URL (Stateless 模式)
        if [ -n "$APPRISE_URL" ]; then
            # 解决 JSON 换行符报错问题：将实际换行符替换为文本 "\n"
            local safe_body="${body//$'\n'/\\n}"
            # 【新增优化】防止标题或内容中包含双引号导致 JSON 结构破坏
            safe_body="${safe_body//\"/\\\"}"
            local safe_title="${title//\"/\\\"}"
            
            # 使用 -s 隐藏 curl 的进度条
            curl -s -X POST "$APPRISE_API_URL/notify" \
                 -H "Content-Type: application/json" \
                 -d "{\"title\": \"$safe_title\", \"body\": \"$safe_body\", \"urls\": \"$APPRISE_URL\"}"
                
        # 玩法 2：用户在 API 地址里直接写了配置路径 (如 `http://apprise:8000/notify/mybot)`  (Stateful 模式)
        else
            # 使用 --data-urlencode 提交，完美保留换行符并防止特殊字符截断表单
            curl -s -X POST "$APPRISE_API_URL" \
                 --data-urlencode "title=$title" \
                 --data-urlencode "body=$body"
        fi
        echo "" # 补一个换行避免日志粘连
        
    # 回退使用本地 Apprise 命令行工具
    elif [ -n "$APPRISE_URL" ]; then
        echo "使用本地 Apprise 命令行工具发送通知..."
        apprise -t "$title" -b "$body" "$APPRISE_URL"
    else
        echo "未配置 APPRISE_URL 或 APPRISE_API_URL，跳过通知发送。"
    fi
}

# 检查必要参数（ZIP_PASSWORD 现在是可选项）
if [ -z "$ZIP_PASSWORD" ]; then
    echo "未设置 ZIP_PASSWORD 环境变量，将进行非加密打包。"
else
    echo "已设置 ZIP_PASSWORD 环境变量，将进行加密打包。"
fi

# 确保备份目录存在
mkdir -p "$BACKUP_DIR"

# 检查备份目录权限
if [ ! -w "$BACKUP_DIR" ]; then
    echo "错误: 备份目录 $BACKUP_DIR 没有写入权限！"
    send_notification "Vaultwarden 备份失败 ❌" "备份目录没有写入权限，请检查权限设置。"
    exit 1
fi

# 检查磁盘空间
FREE_SPACE=$(df -h "$BACKUP_DIR" | tail -n 1 | awk '{print $4}' | sed 's/G//')
if [ "$FREE_SPACE" -lt 5 ]; then
    echo "警告: 备份目录所在磁盘空间不足，剩余空间小于 5GB！"
    send_notification "Vaultwarden 备份警告 ⚠️" "备份目录所在磁盘空间不足，剩余空间小于 5GB，可能导致备份失败。"
fi

echo "开始执行 Vaultwarden 备份任务..."

# 1. 数据库备份逻辑
echo "正在备份数据库 (类型: $DB_TYPE)..."

# 检查数据库备份工具是否存在
case "$DB_TYPE" in
    sqlite)
        # 检查 sqlite3 命令是否存在
        if ! command -v sqlite3 > /dev/null 2>&1; then
            echo "错误: 找不到 sqlite3 命令，请确保已安装 SQLite 工具。"
            send_notification "Vaultwarden 备份失败 ❌" "找不到 sqlite3 命令，请确保已安装 SQLite 工具。"
            exit 1
        fi
        # SQLite 备份逻辑: 使用 .backup 命令安全导出
        SQLITE_DB="${DATA_DIR}/db.sqlite3" # 默认 SQLite 数据库路径
        if [ -f "$SQLITE_DB" ]; then
            echo "正在使用 SQLite .backup 命令备份数据库..."
            if sqlite3 "$SQLITE_DB" ".backup '$SQL_FILE'"; then
                echo "✅ SQLite 数据库备份成功。"
            else
                echo "错误: SQLite 数据库备份失败！"
                send_notification "Vaultwarden 备份失败 ❌" "SQLite 数据库备份失败，请检查数据库文件权限。"
                exit 1
            fi
        else
            echo "错误: 找不到 SQLite 数据库文件 $SQLITE_DB"
            send_notification "Vaultwarden 备份失败 ❌" "找不到 SQLite 数据库文件。"
            exit 1
        fi
        ;;
    mysql)
        # 检查 mysqldump 命令是否存在
        if ! command -v mysqldump > /dev/null 2>&1; then
            echo "错误: 找不到 mysqldump 命令，请确保已安装 MySQL 客户端工具。"
            send_notification "Vaultwarden 备份失败 ❌" "找不到 mysqldump 命令，请确保已安装 MySQL 客户端工具。"
            exit 1
        fi
        # 检查必要的环境变量
        if [ -z "$DB_USER" ] || [ -z "$DB_PASSWORD" ] || [ -z "$DB_NAME" ]; then
            echo "错误: MySQL 备份需要设置 DB_USER, DB_PASSWORD 和 DB_NAME 环境变量。"
            send_notification "Vaultwarden 备份失败 ❌" "MySQL 备份需要设置 DB_USER, DB_PASSWORD 和 DB_NAME 环境变量。"
            exit 1
        fi
        # MySQL/MariaDB 备份逻辑: 使用 mysqldump 导出
        # 添加 --single-transaction 防止锁表，不影响 Vaultwarden 写入
        export MYSQL_PWD="${DB_PASSWORD}" # 注入环境变量，避免日志报警
        echo "正在使用 mysqldump 备份 MySQL 数据库..."
        if mysqldump --single-transaction --quick --opt -h "${DB_HOST:-db}" -P "${DB_PORT:-3306}" -u "${DB_USER}" "${DB_NAME}" > "$SQL_FILE"; then
            echo "✅ MySQL 数据库备份成功。"
        else
            echo "错误: MySQL 数据库备份失败！"
            send_notification "Vaultwarden 备份失败 ❌" "MySQL 数据库备份失败，请检查数据库连接和权限。"
            exit 1
        fi
        ;;
    postgres)
        # 检查 pg_dump 命令是否存在
        if ! command -v pg_dump > /dev/null 2>&1; then
            echo "错误: 找不到 pg_dump 命令，请确保已安装 PostgreSQL 客户端工具。"
            send_notification "Vaultwarden 备份失败 ❌" "找不到 pg_dump 命令，请确保已安装 PostgreSQL 客户端工具。"
            exit 1
        fi
        # 检查必要的环境变量
        if [ -z "$DB_USER" ] || [ -z "$DB_PASSWORD" ] || [ -z "$DB_NAME" ]; then
            echo "错误: PostgreSQL 备份需要设置 DB_USER, DB_PASSWORD 和 DB_NAME 环境变量。"
            send_notification "Vaultwarden 备份失败 ❌" "PostgreSQL 备份需要设置 DB_USER, DB_PASSWORD 和 DB_NAME 环境变量。"
            exit 1
        fi
        # PostgreSQL 备份逻辑: 使用 pg_dump 导出
        # 添加 --no-owner 和 --no-privileges 让备份文件在恢复到不同用户环境时更省心
        export PGPASSWORD="${DB_PASSWORD}" # pg_dump 需要通过环境变量传递密码
        echo "正在使用 pg_dump 备份 PostgreSQL 数据库..."
        if pg_dump --no-owner --no-privileges --clean -h "${DB_HOST:-db}" -p "${DB_PORT:-5432}" -U "${DB_USER}" -d "${DB_NAME}" -F p > "$SQL_FILE"; then
            echo "✅ PostgreSQL 数据库备份成功。"
        else
            echo "错误: PostgreSQL 数据库备份失败！"
            send_notification "Vaultwarden 备份失败 ❌" "PostgreSQL 数据库备份失败，请检查数据库连接和权限。"
            exit 1
        fi
        ;;
    *)
        echo "错误: 不支持的数据库类型 $DB_TYPE"
        send_notification "Vaultwarden 备份失败 ❌" "不支持的数据库类型: $DB_TYPE"
        exit 1
        ;;
esac

# 检查数据库备份是否成功
if [ $? -ne 0 ]; then
    echo "错误: 数据库备份失败！"
    send_notification "Vaultwarden 备份失败 ❌" "数据库 ($DB_TYPE) 导出过程中发生错误。"
    exit 1
fi

# 2. 打包逻辑
echo "正在打包数据目录和数据库文件..."
# 切换到数据目录以避免打包绝对路径
cd "$DATA_DIR" || exit 1

# 检查 SQL 文件是否存在
if [ ! -f "$SQL_FILE" ]; then
    echo "错误: 数据库备份文件 $SQL_FILE 不存在！"
    send_notification "Vaultwarden 备份失败 ❌" "数据库备份文件不存在，可能是数据库导出失败。"
    exit 1
fi

# 根据是否设置了密码决定是否加密打包
if [ -z "$ZIP_PASSWORD" ]; then
    # 非加密打包，添加 -m 参数减少内存使用
    echo "使用非加密方式打包..."
    zip -r -m -q "$ZIP_FILE" . "$SQL_FILE"
else
    # 加密打包，添加 -m 参数减少内存使用
    echo "使用加密方式打包..."
    zip -r -m -P "$ZIP_PASSWORD" -q "$ZIP_FILE" . "$SQL_FILE"
fi

# 检查打包是否成功
if [ $? -ne 0 ]; then
    echo "错误: 打包失败！"
    send_notification "Vaultwarden 备份失败 ❌" "使用 zip 打包过程中发生错误，请检查磁盘空间和权限。"
    exit 1
fi

# 检查压缩包是否存在
if [ ! -f "$ZIP_FILE" ]; then
    echo "错误: 压缩包文件 $ZIP_FILE 不存在！"
    send_notification "Vaultwarden 备份失败 ❌" "压缩包文件不存在，可能是打包过程中发生错误。"
    exit 1
fi

# 标记打包成功，此后哪怕上传失败，本地压缩包也不会被 trap 误删
ZIP_DONE=1

# 检查压缩包完整性
echo "正在校验压缩包完整性..."
if [ -z "$ZIP_PASSWORD" ]; then
    # 非加密打包的校验
    if zip -T "$ZIP_FILE" 2>&1; then
        echo "✅ 压缩包完整性校验通过。"
    else
        echo "❌ 压缩包损坏！"
        send_notification "Vaultwarden 备份失败 ❌" "生成的压缩包校验失败，请检查磁盘空间和权限。"
        exit 1
    fi
else
    # 加密打包的校验
    if zip -T -P "$ZIP_PASSWORD" "$ZIP_FILE" 2>&1; then
        echo "✅ 压缩包完整性校验通过。"
    else
        echo "❌ 压缩包损坏！"
        send_notification "Vaultwarden 备份失败 ❌" "生成的压缩包校验失败，请检查磁盘空间和权限。"
        exit 1
    fi
fi

# 获取压缩包大小
FILE_SIZE=$(du -h "$ZIP_FILE" | cut -f1)

# 3. Rclone 上传逻辑 (如果配置了)
if [ -n "$RCLONE_REMOTE" ]; then
    # 测试 Rclone 远程连接 (全局变量已生效，无需指定 --config)
    echo "正在测试 Rclone 远程连接..."
    if ! rclone listremotes | grep -q "^${RCLONE_REMOTE%%:*}:"; then
        echo "❌ 警告: 找不到 Rclone 配置的远程端: ${RCLONE_REMOTE%%:*}"
        send_notification "Vaultwarden 备份警告 ⚠️" "找不到 Rclone 配置的远程端: ${RCLONE_REMOTE%%:*}，将尝试继续上传。"
    else
        echo "✅ Rclone 远程连接测试通过。"
    fi
    
    echo "正在使用 Rclone 上传备份到 $RCLONE_REMOTE..."
    # 使用 rclone copy 上传文件
    rclone copy "$ZIP_FILE" "$RCLONE_REMOTE"
    
    if [ $? -ne 0 ]; then
        echo "错误: Rclone 上传失败！"
        send_notification "Vaultwarden 备份失败 ❌" "Rclone 上传到远程存储失败。"
        exit 1
    fi
    
    # 4. 远端备份自动清理
    if [ -z "$RCLONE_KEEP_DAYS" ]; then
        echo "未设置 RCLONE_KEEP_DAYS，跳过远端清理。"
    else
        echo "正在清理远端过期备份（保留 $RCLONE_KEEP_DAYS 天）..."
        # 加入 --include 过滤，绝对防止误删用户网盘里的其他私人文件！
        rclone delete "$RCLONE_REMOTE" --include "${PREFIX}_*.zip" --min-age "${RCLONE_KEEP_DAYS}d"
    fi
else
    echo "未配置 RCLONE_REMOTE，跳过云端上传，备份仅保留在本地。"
fi

# 4. 清理临时文件
echo "正在清理临时文件..."
rm -f "$SQL_FILE" # 删除未加密的 SQL 文件
# 如果上传成功，可以选择删除本地 ZIP 包，这里保留以防万一，或者根据策略清理旧备份
# rm -f "$ZIP_FILE" 

# 5. 自动清理过期备份
echo "正在清理过期备份..."
# 自动清理过期的本地备份文件，防止磁盘塞满
KEEP_DAYS=${LOCAL_BACKUP_KEEP_DAYS:-15}
find "$BACKUP_DIR" -name "${PREFIX}_*.zip" -mtime +$KEEP_DAYS -exec rm {} \;
echo "已清理 $KEEP_DAYS 天前的旧本地备份。"

# 6. 发送成功通知
echo "备份任务完成！"
send_notification "Vaultwarden 备份成功 ✅" "类型: $DB_TYPE
大小: $FILE_SIZE
文件: $BACKUP_NAME.zip"

exit 0