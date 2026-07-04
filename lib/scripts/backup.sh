#!/bin/bash

# ==========================================
# Vaultwarden 备份脚本 (支持 SQLite/MySQL/PostgreSQL + Apprise 通知)
# ==========================================

# 导入模块化函数库
source "$(dirname "$0")/../utils.sh"
source "$(dirname "$0")/../db_ops.sh"
source "$(dirname "$0")/../upload_ops.sh"

# 强制设置网络代理，确保 Apprise 和 Rclone 能够顺畅出海
export HTTP_PROXY="${HTTP_PROXY:-}"
export HTTPS_PROXY="${HTTPS_PROXY:-}"
export ALL_PROXY="${ALL_PROXY:-}"

# 日志文件路径
LOG_FILE="/app/config/backup.log"
mkdir -p "$(dirname "$LOG_FILE")"
exec > >(tee -a "$LOG_FILE") 2>&1

# 检查是否已初始化
if [ -z "$WEB_USER" ] || [ -z "$WEB_PASS" ]; then
    echo "错误: 系统未初始化，请先完成初始化配置。"
    exit 1
fi

# 检查数据库配置
if [ "$DB_TYPE" = "mysql" ] || [ "$DB_TYPE" = "postgres" ]; then
    if [ -z "$DB_USER" ] || [ -z "$DB_PASSWORD" ] || [ -z "$DB_NAME" ]; then
        echo "错误: ${DB_TYPE} 数据库配置不完整，请检查配置文件。"
        exit 1
    fi
fi

# 基础配置变量
DB_TYPE="${DB_TYPE:-sqlite}"
DATA_DIR="${DATA_DIR:-/data}"
BACKUP_DIR="${BACKUP_DIR:-/backup}"
ZIP_PASSWORD="${ZIP_PASSWORD:-}"
APPRISE_URL="${APPRISE_URL:-}"
APPRISE_API_URL="${APPRISE_API_URL%/}"
RCLONE_REMOTE="${RCLONE_REMOTE:-}"
RCLONE_KEEP_DAYS="${RCLONE_KEEP_DAYS:-}"
SQLITE_DB_FILE="${SQLITE_DB_FILE:-db.sqlite3}"

TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
PREFIX="${BACKUP_PREFIX:-vaultwarden_backup}"
BACKUP_NAME="${PREFIX}_${TIMESTAMP}"

if [ "$DB_TYPE" = "sqlite" ]; then
    SQL_FILE="${BACKUP_DIR}/${BACKUP_NAME}.db"
else
    SQL_FILE="${BACKUP_DIR}/${BACKUP_NAME}.sql"
fi

ZIP_FILE="${BACKUP_DIR}/${BACKUP_NAME}.zip"
ZIP_DONE=0

cleanup() {
  rm -f "$SQL_FILE"
  rm -rf "${BACKUP_DIR}/staging_"*
  if [ "$ZIP_DONE" -ne 1 ] && [ -f "$ZIP_FILE" ]; then
      rm -f "$ZIP_FILE"
  fi
}
trap cleanup EXIT

if [ -z "$ZIP_PASSWORD" ]; then
    echo "未设置 ZIP_PASSWORD 环境变量，将进行非加密打包。"
else
    echo "已设置 ZIP_PASSWORD 环境变量，将进行加密打包。"
fi

mkdir -p "$BACKUP_DIR"

if [ ! -w "$BACKUP_DIR" ]; then
    echo "错误: 备份目录 $BACKUP_DIR 没有写入权限！"
    send_notification "Vaultwarden 备份失败" "备份目录没有写入权限，请检查权限设置。"
    exit 1
fi

# 检查磁盘空间
MIN_SPACE_MB=$((5 * 1024))
check_disk_space "$BACKUP_DIR" "$MIN_SPACE_MB"

echo "开始执行 Vaultwarden 备份任务..."

# 1. 数据库备份
echo "正在备份数据库 (类型: $DB_TYPE)..."
if ! check_db_tools "$DB_TYPE"; then
    exit 1
fi
if ! exec_db_backup "$DB_TYPE" "$DATA_DIR" "$SQL_FILE"; then
    exit 1
fi

# 2. 打包逻辑 - 使用中转目录方式
echo "正在组装文件并打包..."

STAGING_DIR="${BACKUP_DIR}/staging_${TIMESTAMP}"
mkdir -p "${STAGING_DIR}/data"

# 移动 SQL 文件到中转目录根部
mv "$SQL_FILE" "${STAGING_DIR}/"

# 复制 /data 目录内容到中转目录
shopt -s nullglob
cp -a "$DATA_DIR"/* "${STAGING_DIR}/data/" 2>/dev/null || true
shopt -u nullglob

# 删除复制过来的 sqlite 数据库文件，避免体积翻倍
rm -f "${STAGING_DIR}/data/"*.sqlite3* 2>/dev/null || true

# 切换到中转目录打包
cd "${STAGING_DIR}" || exit 1

if [ -z "$ZIP_PASSWORD" ]; then
    echo "使用非加密方式打包..."
    zip -r -q "$ZIP_FILE" . -x ".*"
else
    echo "使用加密方式打包..."
    zip -r -P "$ZIP_PASSWORD" -q "$ZIP_FILE" . -x ".*"
fi

ZIP_EXIT_CODE=$?
cd "$BACKUP_DIR"
rm -rf "${STAGING_DIR}"

if [ $ZIP_EXIT_CODE -ne 0 ]; then
    echo "错误: 打包失败！"
    send_notification "Vaultwarden 备份失败" "使用 zip 打包过程中发生错误，请检查。"
    exit 1
fi

if [ ! -f "$ZIP_FILE" ]; then
    echo "错误: 压缩包文件 $ZIP_FILE 不存在！"
    send_notification "Vaultwarden 备份失败" "压缩包文件不存在，可能是打包过程中发生错误。"
    exit 1
fi

ZIP_DONE=1

# 校验压缩包完整性
echo "正在校验压缩包完整性..."
if [ -z "$ZIP_PASSWORD" ]; then
    if unzip -tq "$ZIP_FILE" > /dev/null 2>&1; then
        echo "压缩包完整性校验通过。"
    else
        echo "压缩包损坏！"
        send_notification "Vaultwarden 备份失败" "生成的压缩包校验失败，请检查磁盘空间。"
        exit 1
    fi
else
    if unzip -tq -P "$ZIP_PASSWORD" "$ZIP_FILE" > /dev/null 2>&1; then
        echo "压缩包完整性校验通过。"
    else
        echo "压缩包损坏！"
        send_notification "Vaultwarden 备份失败" "生成的压缩包密码校验失败，请检查。"
        exit 1
    fi
fi

FILE_SIZE=$(du -h "$ZIP_FILE" | cut -f1)

# 3. Rclone 上传
if ! exec_rclone_ops "$ZIP_FILE" "$RCLONE_REMOTE" "$PREFIX" "$RCLONE_KEEP_DAYS"; then
    exit 1
fi

# 4. 清理临时文件
echo "正在清理临时文件..."
rm -f "$SQL_FILE"

# 5. 自动清理过期备份
echo "正在清理过期备份..."
KEEP_DAYS=${LOCAL_BACKUP_KEEP_DAYS:-15}
cleanup_old_files "$BACKUP_DIR" "${PREFIX}_*.zip" "$KEEP_DAYS"

# 6. 发送成功通知
echo "备份任务完成！"
send_notification "Vaultwarden 备份成功" "类型: $DB_TYPE
大小: $FILE_SIZE
文件: $BACKUP_NAME.zip"

exit 0
