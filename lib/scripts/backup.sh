#!/bin/bash

# ==========================================
# Vaultwarden 备份脚本 (支持 SQLite/MySQL/PostgreSQL + Apprise 通知)
# ==========================================

# 导入模块化函数库
source "$(dirname "$0")/../utils.sh"
source "$(dirname "$0")/../db_ops.sh"
source "$(dirname "$0")/../upload_ops.sh"

# 强制设置网络代理，确保 Apprise 和 Rclone 能够顺畅出海 (如果不需要代理，请在环境变量中留空)
export HTTP_PROXY="${HTTP_PROXY:-}"   # HTTP 代理地址，例如 http://192.168.1.100:7890
export HTTPS_PROXY="${HTTPS_PROXY:-}" # HTTPS 代理地址，例如 http://192.168.1.100:7890
export ALL_PROXY="${ALL_PROXY:-}"     # SOCKS5 代理地址，例如 socks5://192.168.1.100:7890

# 日志文件路径
LOG_FILE="/app/config/backup.log"
# 确保日志目录存在
mkdir -p "$(dirname "$LOG_FILE")"
# 清空旧日志，保留最新记录
> "$LOG_FILE"
# 重定向输出到日志文件
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

# 基础配置变量 (可通过环境变量覆盖)
DB_TYPE="${DB_TYPE:-sqlite}"          # 数据库类型: sqlite, mysql, postgres
DATA_DIR="${DATA_DIR:-/data}"      # Vaultwarden 数据目录
BACKUP_DIR="${BACKUP_DIR:-/backup}"   # 本地临时备份目录
ZIP_PASSWORD="${ZIP_PASSWORD:-}"      # 压缩包加密密码
APPRISE_URL="${APPRISE_URL:-}"        # Apprise 通知 URL
# 读取 API URL，并使用 %/ 自动移除末尾可能多余的斜杠，包容用户的书写习惯
APPRISE_API_URL="${APPRISE_API_URL%/}"  # Apprise 服务 API 地址
RCLONE_REMOTE="${RCLONE_REMOTE:-}"    # Rclone 远程路径，例如 myremote:/vaultwarden_backup

# 时间戳，用于生成唯一的文件名
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")    # 格式: 年月日_时分秒
# 备份文件前缀，可通过环境变量自定义
PREFIX="${BACKUP_PREFIX:-vaultwarden_backup}"
BACKUP_NAME="${PREFIX}_${TIMESTAMP}" # 备份文件基础名称

# 根据数据库类型使用不同的备份文件后缀
if [ "$DB_TYPE" = "sqlite" ]; then
    SQL_FILE="${BACKUP_DIR}/${BACKUP_NAME}.db"   # SQLite 二进制备份文件路径
else
    SQL_FILE="${BACKUP_DIR}/${BACKUP_NAME}.sql"   # MySQL/PostgreSQL SQL 文本文件路径
fi

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
# 5GB 转换为 MB
MIN_SPACE_MB=$((5 * 1024))
check_disk_space "$BACKUP_DIR" "$MIN_SPACE_MB"

echo "开始执行 Vaultwarden 备份任务..."

# 1. 数据库备份逻辑
echo "正在备份数据库 (类型: $DB_TYPE)..."

# 检查数据库备份工具是否存在
if ! check_db_tools "$DB_TYPE"; then
    exit 1
fi

# 执行数据库备份
if ! exec_db_backup "$DB_TYPE" "$DATA_DIR" "$SQL_FILE"; then
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

# [核心修复]：分两步打包，既保留附件目录结构，又防止数据库文件出现“路径套娃”
echo "正在打包数据目录..."

# 为数据库文件创建一个固定的临时名称，方便恢复时精准匹配
TEMP_SQL_FILE="${BACKUP_DIR}/db_dump_temp.sql"
if [ "$DB_TYPE" = "sqlite" ]; then
    TEMP_SQL_FILE="${BACKUP_DIR}/db_dump_temp.db"
fi

# 复制数据库文件到临时文件
cp "$SQL_FILE" "$TEMP_SQL_FILE"

if [ -z "$ZIP_PASSWORD" ]; then
    # 非加密打包
    # 精准排除数据库文件，避免误伤附件目录中的文件
    zip -r -q "$ZIP_FILE" . -x "db.sqlite3" "db_dump_temp.db" "db_dump_temp.sql" # 打包当前目录，排除可能存在的旧备份
    echo "正在追加数据库备份..."
    zip -j -q "$ZIP_FILE" "$TEMP_SQL_FILE" # 使用 -j 仅针对数据库文件，使其位于压缩包根目录
else
    # 加密打包
    # 精准排除数据库文件，避免误伤附件目录中的文件
    zip -r -P "$ZIP_PASSWORD" -q "$ZIP_FILE" . -x "db.sqlite3" "db_dump_temp.db" "db_dump_temp.sql" # 打包当前目录，排除可能存在的旧备份
    echo "正在追加数据库备份..."
    zip -j -P "$ZIP_PASSWORD" -q "$ZIP_FILE" "$TEMP_SQL_FILE" # 使用 -j 仅针对数据库文件，使其位于压缩包根目录
fi

# 清理临时文件
rm -f "$TEMP_SQL_FILE"

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
if ! exec_rclone_ops "$ZIP_FILE" "$RCLONE_REMOTE" "$PREFIX" "$RCLONE_KEEP_DAYS"; then
    exit 1
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
cleanup_old_files "$BACKUP_DIR" "${PREFIX}_*.zip" "$KEEP_DAYS"

# 6. 发送成功通知
echo "备份任务完成！"
send_notification "Vaultwarden 备份成功 ✅" "类型: $DB_TYPE
大小: $FILE_SIZE
文件: $BACKUP_NAME.zip"

exit 0