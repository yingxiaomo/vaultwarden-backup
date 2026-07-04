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
DATA_DIR="${DATA_DIR:-/data}"         # Vaultwarden 数据目录
BACKUP_DIR="${BACKUP_DIR:-/backup}"   # 本地临时备份目录
ZIP_PASSWORD="${ZIP_PASSWORD:-}"      # 压缩包加密密码 (可选)
APPRISE_URL="${APPRISE_URL:-}"        # Apprise 通知 URL (直接使用命令行工具)
APPRISE_API_URL="${APPRISE_API_URL%/}"  # Apprise 服务 API 地址 (使用独立 Apprise 服务)
RCLONE_REMOTE="${RCLONE_REMOTE:-}"    # Rclone 远程路径，例如 myremote:/vaultwarden_backup
SQLITE_DB_FILE="${SQLITE_DB_FILE:-db.sqlite3}"  # SQLite 数据库文件名（可配置）

# 时间戳，用于生成唯一的文件名
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")    # 格式: 年月日_时分秒
# 备份文件前缀，可通过环境变量自定义
PREFIX="${BACKUP_PREFIX:-vaultwarden_backup}"
BACKUP_NAME="${PREFIX}_${TIMESTAMP}" # 备份文件基础名称
SQL_FILE="${BACKUP_DIR}/${BACKUP_NAME}.sql"   # 导出的 SQL 文件路径
ZIP_FILE="${BACKUP_DIR}/${BACKUP_NAME}.zip"   # 最终的加密压缩包路径
# 压缩包生成完毕标记
ZIP_DONE=0

# 无论脚本如何退出，都尝试删除临时文件
cleanup() {
  rm -f "$SQL_FILE"
  # 清理可能残留的中转组装目录
  rm -rf "${BACKUP_DIR}/staging_"*
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
        
        # 如果同时配置了 API 地址和具体的通知 URL (Stateless 模式)
        if [ -n "$APPRISE_URL" ]; then
            # 使用 jq 生成安全的 JSON
            local json=$(jq -n --arg title "$title" --arg body "$body" --arg urls "$APPRISE_URL" '{title: $title, body: $body, urls: $urls}')
            
            # 使用 -s 隐藏 curl 的进度条
            curl -s -X POST "$APPRISE_API_URL/notify" \
                 -H "Content-Type: application/json" \
                 -d "$json"
                
        # 在 API 地址里直接写了配置路径 (如 `http://apprise:8000/notify/mybot`) (Stateful 模式)
        else
            # 使用 --data-urlencode 提交，完美保留换行符并防止特殊字符截断表单
            curl -s -X POST "$APPRISE_API_URL" \
                 --data-urlencode "title=$title" \
                 --data-urlencode "body=$body"
        fi
        echo "" # 补一个换行避免日志粘在一起
        
    # 回退使用本地 Apprise 命令行工具
    elif [ -n "$APPRISE_URL" ]; then
        echo "使用本地 Apprise 命令行工具发送通知..."
        if ! apprise -t "$title" -b "$body" "$APPRISE_URL"; then
            echo "警告: Apprise 通知发送失败"
        fi
    else
        echo "未配置 APPRISE_URL 或 APPRISE_API_URL，跳过通知发送。"
    fi
}


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
# 使用 df -Pm 以 MB 为单位输出，避免在 32 位系统上发生整数溢出
FREE_SPACE_MB=$(df -Pm "$BACKUP_DIR" | awk '/^\// || /^[0-9]/ {print $(NF-2)}' | sed 's/[^0-9]//g')

# 如果磁盘空间获取失败（例如 df 输出格式异常），跳过检查
if [ -n "$FREE_SPACE_MB" ]; then
    # 5GB 转换为 MB
    MIN_SPACE_MB=$((5 * 1024))

    if [ "$FREE_SPACE_MB" -lt "$MIN_SPACE_MB" ]; then
        # 获取剩余空间
        HUMAN_FREE=$(df -h "$BACKUP_DIR" | tail -n 1 | awk '{print $4}')
        echo "警告: 备份目录所在磁盘空间不足，剩余空间为 $HUMAN_FREE，小于 5GB！"
        send_notification "Vaultwarden 备份警告 ⚠️" "备份目录所在磁盘空间不足，剩余空间为 $HUMAN_FREE，小于 5GB，可能导致备份失败。"
    fi
else
    echo "警告: 无法获取磁盘空间信息，跳过空间检查。"
fi

echo "开始执行 Vaultwarden 备份任务..."

# 1. 数据库备份逻辑
echo "正在备份数据库 (类型: $DB_TYPE)..."

# 检查数据库备份工具是否存在
case "$DB_TYPE" in
    sqlite)
        if ! command -v sqlite3 > /dev/null 2>&1; then
            echo "错误: 找不到 sqlite3 命令，请确保已安装 SQLite 工具。"
            send_notification "Vaultwarden 备份失败 ❌" "找不到 sqlite3 命令，请确保已安装 SQLite 工具。"
            exit 1
        fi
        # SQLite 备份逻辑: 使用可配置的数据库文件名
        SQLITE_DB="${DATA_DIR}/${SQLITE_DB_FILE}"
        if [ -f "$SQLITE_DB" ]; then
            echo "正在使用 sqlite3 .backup 命令备份数据库..."
            if sqlite3 "$SQLITE_DB" ".backup '$SQL_FILE'"; then
                echo "✅ SQLite 数据库备份成功。"
            else
                echo "错误: SQLite 数据库备份失败！"
                send_notification "Vaultwarden 备份失败 ❌" "SQLite 数据库备份失败，请检查数据库文件权限。"
                exit 1
            fi
        else
            echo "错误: 找不到 SQLite 数据库文件 $SQLITE_DB"
            echo "提示: 可通过 SQLITE_DB_FILE 环境变量指定数据库文件名（默认: db.sqlite3）"
            send_notification "Vaultwarden 备份失败 ❌" "找不到 SQLite 数据库文件，请检查 DATA_DIR 和 SQLITE_DB_FILE 配置。"
            exit 1
        fi
        ;;
    mysql)
        if ! command -v mysqldump > /dev/null 2>&1; then
            echo "错误: 找不到 mysqldump 命令，请确保已安装 MySQL 客户端工具。"
            send_notification "Vaultwarden 备份失败 ❌" "找不到 mysqldump 命令，请确保已安装 MySQL 客户端工具。"
            exit 1
        fi
        if [ -z "$DB_USER" ] || [ -z "$DB_PASSWORD" ] || [ -z "$DB_NAME" ]; then
            echo "错误: MySQL 备份需要设置 DB_USER, DB_PASSWORD 和 DB_NAME 环境变量。"
            send_notification "Vaultwarden 备份失败 ❌" "MySQL 备份需要设置 DB_USER, DB_PASSWORD 和 DB_NAME 环境变量。"
            exit 1
        fi
        # MySQL/MariaDB 备份逻辑: 使用 --password 参数替代 MYSQL_PWD 环境变量
        echo "正在使用 mysqldump 备份 MySQL 数据库..."
        if mysqldump --single-transaction --quick --opt \
            -h "${DB_HOST:-db}" -P "${DB_PORT:-3306}" \
            -u "${DB_USER}" --password="${DB_PASSWORD}" \
            "${DB_NAME}" > "$SQL_FILE"; then
            echo "✅ MySQL 数据库备份成功。"
        else
            echo "错误: MySQL 数据库备份失败！"
            send_notification "Vaultwarden 备份失败 ❌" "MySQL 数据库备份失败，请检查数据库连接和权限。"
            exit 1
        fi
        ;;
    postgres)
        if ! command -v pg_dump > /dev/null 2>&1; then
            echo "错误: 找不到 pg_dump 命令，请确保已安装 PostgreSQL 客户端工具。"
            send_notification "Vaultwarden 备份失败 ❌" "找不到 pg_dump 命令，请确保已安装 PostgreSQL 客户端工具。"
            exit 1
        fi
        if [ -z "$DB_USER" ] || [ -z "$DB_PASSWORD" ] || [ -z "$DB_NAME" ]; then
            echo "错误: PostgreSQL 备份需要设置 DB_USER, DB_PASSWORD 和 DB_NAME 环境变量。"
            send_notification "Vaultwarden 备份失败 ❌" "PostgreSQL 备份需要设置 DB_USER, DB_PASSWORD 和 DB_NAME 环境变量。"
            exit 1
        fi
        # PostgreSQL 备份逻辑: 使用 pg_dump 导出
        export PGPASSWORD="${DB_PASSWORD}"
        echo "正在使用 pg_dump 备份 PostgreSQL 数据库..."
        if pg_dump --no-owner --no-privileges --clean \
            -h "${DB_HOST:-db}" -p "${DB_PORT:-5432}" \
            -U "${DB_USER}" -d "${DB_NAME}" -F p > "$SQL_FILE"; then
            echo "✅ PostgreSQL 数据库备份成功。"
        else
            echo "错误: PostgreSQL 数据库备份失败！"
            send_notification "Vaultwarden 备份失败 ❌" "PostgreSQL 数据库备份失败，请检查数据库连接和权限。"
            exit 1
        fi
        unset PGPASSWORD
        ;;
    *)
        echo "错误: 不支持的数据库类型 $DB_TYPE"
        send_notification "Vaultwarden 备份失败 ❌" "不支持的数据库类型: $DB_TYPE"
        exit 1
        ;;
esac

# 2. 打包逻辑
echo "正在组装文件并打包..."

# 创建一个临时中转目录，保证解压后的结构极度整洁
STAGING_DIR="${BACKUP_DIR}/staging_${TIMESTAMP}"
mkdir -p "${STAGING_DIR}/data"

# [步骤 1]: 将已经导出的 SQL 文件移动到中转目录的根部
mv "$SQL_FILE" "${STAGING_DIR}/"

# [步骤 2]: 将 /data 目录下的内容完整复制到中转目录的 data 文件夹中
shopt -s nullglob
cp -a "$DATA_DIR"/* "${STAGING_DIR}/data/" 2>/dev/null || true
shopt -u nullglob

# 删除物理复制过来的 sqlite 数据库文件，避免 zip 体积翻倍和恢复冲突
rm -f "${STAGING_DIR}/data/"*.sqlite3* 2>/dev/null || true

# [步骤 3]: 切换到中转目录执行打包
cd "${STAGING_DIR}" || exit 1

if [ -z "$ZIP_PASSWORD" ]; then
    echo "使用非加密方式打包..."
    # 排除隐藏文件（如 .DS_Store、.gitkeep 等），保持压缩包整洁
    zip -r -q "$ZIP_FILE" . -x ".*"
else
    echo "使用加密方式打包..."
    # 排除隐藏文件（如 .DS_Store、.gitkeep 等），保持压缩包整洁
    zip -r -P "$ZIP_PASSWORD" -q "$ZIP_FILE" . -x ".*"
fi

# 检查打包是否成功
ZIP_EXIT_CODE=$?
if [ $ZIP_EXIT_CODE -ne 0 ]; then
    echo "错误: 打包失败！"
    send_notification "Vaultwarden 备份失败 ❌" "使用 zip 打包过程中发生错误，请检查。"
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

# [步骤 4]: 打包完成后，功成身退，清理中转目录
cd "$BACKUP_DIR"
rm -rf "${STAGING_DIR}"

# 检查压缩包完整性
echo "正在校验压缩包完整性..."
if [ -z "$ZIP_PASSWORD" ]; then
    # 非加密打包的校验：使用 unzip -tq 静默测试
    if unzip -tq "$ZIP_FILE" > /dev/null 2>&1; then
        echo "✅ 压缩包完整性校验通过。"
    else
        echo "❌ 压缩包损坏！"
        send_notification "Vaultwarden 备份失败 ❌" "生成的压缩包校验失败，请检查磁盘空间。"
        exit 1
    fi
else
    # 加密打包的校验：显式传入 -P 密码进行测试
    if unzip -tq -P "$ZIP_PASSWORD" "$ZIP_FILE" > /dev/null 2>&1; then
        echo "✅ 压缩包完整性校验通过。"
    else
        echo "❌ 压缩包损坏！"
        send_notification "Vaultwarden 备份失败 ❌" "生成的压缩包密码校验失败，请检查。"
        exit 1
    fi
fi

# 获取压缩包大小
FILE_SIZE=$(du -h "$ZIP_FILE" | cut -f1)

# 3. Rclone 上传逻辑 (如果配置了)
if [ -n "$RCLONE_REMOTE" ]; then
    echo "正在检查 Rclone 远程配置..."
    if ! rclone listremotes | grep -q "^${RCLONE_REMOTE%%:*}:"; then
        echo "❌ 错误: 找不到 Rclone 远程配置: ${RCLONE_REMOTE%%:*}"
        send_notification "Vaultwarden 备份失败 ❌" "找不到 Rclone 远程配置: ${RCLONE_REMOTE%%:*}，请检查 Rclone 配置文件。"
        exit 1
    else
        echo "✅ Rclone 远程配置检查通过。"
    fi

    echo "正在使用 Rclone 上传备份到 $RCLONE_REMOTE..."

    # 带重试的上传逻辑（最多重试 3 次，应对网络波动）
    UPLOAD_SUCCESS=0
    MAX_RETRIES=3
    RETRY_DELAY=5

    for i in $(seq 1 $MAX_RETRIES); do
        echo "上传尝试 $i/$MAX_RETRIES..."
        if rclone copy "$ZIP_FILE" "$RCLONE_REMOTE"; then
            UPLOAD_SUCCESS=1
            break
        fi
        if [ $i -lt $MAX_RETRIES ]; then
            echo "上传失败，${RETRY_DELAY} 秒后重试..."
            sleep $RETRY_DELAY
            RETRY_DELAY=$((RETRY_DELAY * 2))
        fi
    done

    if [ $UPLOAD_SUCCESS -ne 1 ]; then
        echo "错误: Rclone 上传失败（已重试 $MAX_RETRIES 次）！"
        send_notification "Vaultwarden 备份失败 ❌" "Rclone 上传到远程存储失败（已重试 $MAX_RETRIES 次）。"
        exit 1
    fi

    # 4. 远端备份自动清理
    if [ -z "$RCLONE_KEEP_DAYS" ]; then
        echo "未设置 RCLONE_KEEP_DAYS，跳过远端清理。"
    else
        echo "正在清理远端过期备份（保留 $RCLONE_KEEP_DAYS 天）..."
        # 使用 filter 精确控制：只匹配备份文件前缀，排除其他文件
        # 注意：rclone filter 规则按顺序执行，最后匹配的规则决定文件命运
        rclone delete "$RCLONE_REMOTE" \
            --filter "+ ${PREFIX}_*.zip" \
            --filter "- *" \
            --min-age "${RCLONE_KEEP_DAYS}d" \
            --verbose 2>&1 | head -20
        echo "远端清理完成。"
    fi
else
    echo "未配置 RCLONE_REMOTE，跳过云端上传，备份仅保留在本地。"
fi

# 5. 清理临时文件并自动清理过期备份
echo "正在清理临时文件..."
rm -f "$SQL_FILE" # 删除未加密的 SQL 文件

echo "正在清理过期备份..."
# 使用 -delete 替代 -exec rm -f，性能更好
KEEP_DAYS=${LOCAL_BACKUP_KEEP_DAYS:-15}
find "$BACKUP_DIR" -name "${PREFIX}_*.zip" -mtime +$KEEP_DAYS -delete 2>/dev/null || \
    find "$BACKUP_DIR" -name "${PREFIX}_*.zip" -mtime +$KEEP_DAYS -exec rm -f {} \;
echo "已清理 $KEEP_DAYS 天前的旧本地备份。"

# 6. 发送成功通知
echo "备份任务完成！"
send_notification "Vaultwarden 备份成功 ✅" "类型: $DB_TYPE
大小: $FILE_SIZE
文件: $BACKUP_NAME.zip"

exit 0