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
APPRISE_URL="${APPRISE_URL:-}"        # Apprise 通知 URL (必填)
RCLONE_REMOTE="${RCLONE_REMOTE:-}"    # Rclone 远程路径，例如 myremote:/vaultwarden_backup

# 时间戳，用于生成唯一的文件名
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")    # 格式: 年月日_时分秒
BACKUP_NAME="vaultwarden_backup_${TIMESTAMP}" # 备份文件基础名称
SQL_FILE="${BACKUP_DIR}/${BACKUP_NAME}.sql"   # 导出的 SQL 文件路径
ZIP_FILE="${BACKUP_DIR}/${BACKUP_NAME}.zip"   # 最终的加密压缩包路径

# 发送通知的辅助函数
send_notification() {
    local title="$1"  # 通知标题
    local body="$2"   # 通知内容
    if [ -n "$APPRISE_URL" ]; then
        # 调用 apprise 命令行工具发送通知
        apprise -t "$title" -b "$body" "$APPRISE_URL"
    else
        echo "未配置 APPRISE_URL，跳过通知发送。"
    fi
}

# 检查必要参数
if [ -z "$ZIP_PASSWORD" ]; then
    echo "错误: 未设置 ZIP_PASSWORD 环境变量，无法进行加密打包！"
    exit 1
fi

# 确保备份目录存在
mkdir -p "$BACKUP_DIR"

echo "开始执行 Vaultwarden 备份任务..."

# 1. 数据库备份逻辑
echo "正在备份数据库 (类型: $DB_TYPE)..."
case "$DB_TYPE" in
    sqlite)
        # SQLite 备份逻辑: 使用 .backup 命令安全导出
        SQLITE_DB="${DATA_DIR}/db.sqlite3" # 默认 SQLite 数据库路径
        if [ -f "$SQLITE_DB" ]; then
            sqlite3 "$SQLITE_DB" ".backup '$SQL_FILE'"
        else
            echo "错误: 找不到 SQLite 数据库文件 $SQLITE_DB"
            send_notification "Vaultwarden 备份失败 ❌" "找不到 SQLite 数据库文件。"
            exit 1
        fi
        ;;
    mysql)
        # MySQL/MariaDB 备份逻辑: 使用 mysqldump 导出
        # 需要环境变量: DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME
        mysqldump -h "${DB_HOST:-db}" -P "${DB_PORT:-3306}" -u "${DB_USER}" -p"${DB_PASSWORD}" "${DB_NAME}" > "$SQL_FILE"
        ;;
    postgres)
        # PostgreSQL 备份逻辑: 使用 pg_dump 导出
        # 需要环境变量: DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME
        export PGPASSWORD="${DB_PASSWORD}" # pg_dump 需要通过环境变量传递密码
        pg_dump -h "${DB_HOST:-db}" -p "${DB_PORT:-5432}" -U "${DB_USER}" -d "${DB_NAME}" -F p > "$SQL_FILE"
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

# 2. 加密打包逻辑
echo "正在加密打包数据目录和数据库文件..."
# 使用 zip -P 进行 AES 加密压缩 (-r 递归, -q 静默)
# 注意: 将数据目录和刚导出的 SQL 文件一起打包
cd "$DATA_DIR" || exit 1 # 切换到数据目录以避免打包绝对路径
zip -r -P "$ZIP_PASSWORD" -q "$ZIP_FILE" ./* "$SQL_FILE"

# 检查打包是否成功
if [ $? -ne 0 ]; then
    echo "错误: 加密打包失败！"
    send_notification "Vaultwarden 备份失败 ❌" "使用 zip 加密打包过程中发生错误。"
    exit 1
fi

# 3. Rclone 上传逻辑 (如果配置了)
if [ -n "$RCLONE_REMOTE" ]; then
    echo "正在使用 Rclone 上传备份到 $RCLONE_REMOTE..."
    # 使用 rclone copy 上传文件
    rclone copy "$ZIP_FILE" "$RCLONE_REMOTE"
    
    if [ $? -ne 0 ]; then
        echo "错误: Rclone 上传失败！"
        send_notification "Vaultwarden 备份失败 ❌" "Rclone 上传到远程存储失败。"
        exit 1
    fi
else
    echo "未配置 RCLONE_REMOTE，跳过云端上传，备份仅保留在本地。"
fi

# 4. 清理临时文件
echo "正在清理临时文件..."
rm -f "$SQL_FILE" # 删除未加密的 SQL 文件
# 如果上传成功，可以选择删除本地 ZIP 包，这里保留以防万一，或者根据策略清理旧备份
# rm -f "$ZIP_FILE" 

# 5. 发送成功通知
echo "备份任务完成！"
send_notification "Vaultwarden 备份成功 ✅" "备份文件 $BACKUP_NAME.zip 已成功生成并处理。"

exit 0