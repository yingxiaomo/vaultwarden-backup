#!/bin/bash

# ==========================================
# 数据库操作函数库
# ==========================================

# 确保 utils.sh 被加载（提供 send_notification 等基础函数）
if ! type send_notification >/dev/null 2>&1; then
    source "$(dirname "${BASH_SOURCE[0]}")/utils.sh"
fi

# 备份 SQLite 数据库
backup_sqlite() {
    local sqlite_db="$1"
    local sql_file="$2"
    
    echo "正在使用 SQLite .backup 命令备份数据库..."
    if sqlite3 "$sqlite_db" ".backup '$sql_file'"; then
        echo "SQLite 数据库备份成功。"
        return 0
    else
        echo "错误: SQLite 数据库备份失败！"
        send_notification "Vaultwarden 备份失败" "SQLite 数据库备份失败，请检查数据库文件权限。"
        return 1
    fi
}

# 备份 MySQL 数据库 - 使用 --password 参数替代 MYSQL_PWD 环境变量
backup_mysql() {
    local db_host="$1"
    local db_port="$2"
    local db_user="$3"
    local db_password="$4"
    local db_name="$5"
    local sql_file="$6"
    
    echo "正在使用 mysqldump 备份 MySQL 数据库..."
    if mysqldump --single-transaction --quick --opt \
        -h "$db_host" -P "$db_port" \
        -u "$db_user" --password="$db_password" \
        "$db_name" > "$sql_file"; then
        echo "MySQL 数据库备份成功。"
        return 0
    else
        echo "错误: MySQL 数据库备份失败！"
        send_notification "Vaultwarden 备份失败" "MySQL 数据库备份失败，请检查数据库连接和权限。"
        return 1
    fi
}

# 备份 PostgreSQL 数据库
backup_postgres() {
    local db_host="$1"
    local db_port="$2"
    local db_user="$3"
    local db_password="$4"
    local db_name="$5"
    local sql_file="$6"
    
    export PGPASSWORD="$db_password"
    echo "正在使用 pg_dump 备份 PostgreSQL 数据库..."
    if pg_dump --no-owner --no-privileges --clean \
        -h "$db_host" -p "$db_port" \
        -U "$db_user" -d "$db_name" -F p > "$sql_file"; then
        echo "PostgreSQL 数据库备份成功。"
        unset PGPASSWORD
        return 0
    else
        echo "错误: PostgreSQL 数据库备份失败！"
        unset PGPASSWORD
        send_notification "Vaultwarden 备份失败" "PostgreSQL 数据库备份失败，请检查数据库连接和权限。"
        return 1
    fi
}

# 检查数据库备份工具是否存在
check_db_tools() {
    local db_type="$1"
    
    case "$db_type" in
        sqlite)
            if ! command -v sqlite3 > /dev/null 2>&1; then
                echo "错误: 找不到 sqlite3 命令，请确保已安装 SQLite 工具。"
                send_notification "Vaultwarden 备份失败" "找不到 sqlite3 命令，请确保已安装 SQLite 工具。"
                return 1
            fi
            ;;
        mysql)
            if ! command -v mysqldump > /dev/null 2>&1; then
                echo "错误: 找不到 mysqldump 命令，请确保已安装 MySQL 客户端工具。"
                send_notification "Vaultwarden 备份失败" "找不到 mysqldump 命令，请确保已安装 MySQL 客户端工具。"
                return 1
            fi
            ;;
        postgres)
            if ! command -v pg_dump > /dev/null 2>&1; then
                echo "错误: 找不到 pg_dump 命令，请确保已安装 PostgreSQL 客户端工具。"
                send_notification "Vaultwarden 备份失败" "找不到 pg_dump 命令，请确保已安装 PostgreSQL 客户端工具。"
                return 1
            fi
            ;;
        *)
            echo "错误: 不支持的数据库类型 $db_type"
            send_notification "Vaultwarden 备份失败" "不支持的数据库类型: $db_type"
            return 1
            ;;
    esac
    return 0
}

# 执行数据库备份
exec_db_backup() {
    local db_type="$1"
    local data_dir="$2"
    local sql_file="$3"
    
    case "$db_type" in
        sqlite)
            # 使用可配置的 SQLite 数据库文件名
            local sqlite_db_file="${SQLITE_DB_FILE:-db.sqlite3}"
            local sqlite_db="${data_dir}/${sqlite_db_file}"
            if [ -f "$sqlite_db" ]; then
                backup_sqlite "$sqlite_db" "$sql_file"
                return $?
            else
                echo "错误: 找不到 SQLite 数据库文件 $sqlite_db"
                echo "提示: 可通过 SQLITE_DB_FILE 环境变量指定数据库文件名（默认: db.sqlite3）"
                send_notification "Vaultwarden 备份失败" "找不到 SQLite 数据库文件，请检查 DATA_DIR 和 SQLITE_DB_FILE 配置。"
                return 1
            fi
            ;;
        mysql)
            if [ -z "$DB_USER" ] || [ -z "$DB_PASSWORD" ] || [ -z "$DB_NAME" ]; then
                echo "错误: MySQL 备份需要设置 DB_USER, DB_PASSWORD 和 DB_NAME 环境变量。"
                send_notification "Vaultwarden 备份失败" "MySQL 备份需要设置 DB_USER, DB_PASSWORD 和 DB_NAME 环境变量。"
                return 1
            fi
            backup_mysql "${DB_HOST:-db}" "${DB_PORT:-3306}" "$DB_USER" "$DB_PASSWORD" "$DB_NAME" "$sql_file"
            return $?
            ;;
        postgres)
            if [ -z "$DB_USER" ] || [ -z "$DB_PASSWORD" ] || [ -z "$DB_NAME" ]; then
                echo "错误: PostgreSQL 备份需要设置 DB_USER, DB_PASSWORD 和 DB_NAME 环境变量。"
                send_notification "Vaultwarden 备份失败" "PostgreSQL 备份需要设置 DB_USER, DB_PASSWORD 和 DB_NAME 环境变量。"
                return 1
            fi
            backup_postgres "${DB_HOST:-db}" "${DB_PORT:-5432}" "$DB_USER" "$DB_PASSWORD" "$DB_NAME" "$sql_file"
            return $?
            ;;
        *)
            echo "错误: 不支持的数据库类型 $db_type"
            send_notification "Vaultwarden 备份失败" "不支持的数据库类型: $db_type"
            return 1
            ;;
    esac
}
