#!/bin/bash
# Vaultwarden Backup — 基础测试
# 用 SQLite 在临时目录执行完整备份流程并校验产物

set -euo pipefail

TEST_DIR=$(mktemp -d)
echo "=== 测试目录: $TEST_DIR ==="

# 模拟数据
echo "CREATE TABLE test (id INT); INSERT INTO test VALUES (1);" > "$TEST_DIR/db.sqlite3"
sqlite3 "$TEST_DIR/db.sqlite3" < "$TEST_DIR/db.sqlite3" 2>/dev/null || true
# 创建一个真正的 SQLite 库
rm -f "$TEST_DIR/db.sqlite3"
sqlite3 "$TEST_DIR/db.sqlite3" "CREATE TABLE test (id INT); INSERT INTO test VALUES (1); INSERT INTO test VALUES (2);"

export DATA_DIR="$TEST_DIR"
export BACKUP_DIR="$TEST_DIR/backup"
export DB_TYPE="sqlite"
export SQLITE_DB_FILE="db.sqlite3"
export BACKUP_PREFIX="test_backup"
export LOCAL_BACKUP_KEEP_DAYS="30"
export RUN_ON_STARTUP="false"
export LANGUAGE="zh"

mkdir -p "$BACKUP_DIR"

# 准备 entrypoint 输出的环境文件
cat > "$TEST_DIR/env.sh" << EOF
export DATA_DIR="$TEST_DIR"
export BACKUP_DIR="$TEST_DIR/backup"
export DB_TYPE="sqlite"
export SQLITE_DB_FILE="db.sqlite3"
export BACKUP_PREFIX="test_backup"
export LOCAL_BACKUP_KEEP_DAYS="30"
EOF

echo "=== 执行备份 ==="
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
bash "$SCRIPT_DIR/backup.sh" 2>&1 || {
    echo "❌ 备份失败"
    rm -rf "$TEST_DIR"
    exit 1
}

echo "=== 校验备份产物 ==="
ZIP_COUNT=$(ls "$BACKUP_DIR"/test_backup_*.zip 2>/dev/null | wc -l)
if [ "$ZIP_COUNT" -eq 0 ]; then
    echo "❌ 未找到备份文件"
    rm -rf "$TEST_DIR"
    exit 1
fi

ZIP_FILE=$(ls "$BACKUP_DIR"/test_backup_*.zip | head -1)
echo "✅ 备份文件: $ZIP_FILE"

# 校验 ZIP 完整性
unzip -t "$ZIP_FILE" > /dev/null 2>&1 || {
    echo "❌ ZIP 完整性校验失败"
    rm -rf "$TEST_DIR"
    exit 1
}
echo "✅ ZIP 完整性通过"

# 解压检查 SQL 文件
unzip -o "$ZIP_FILE" -d "$TEST_DIR/extract" > /dev/null 2>&1
SQL_COUNT=$(ls "$TEST_DIR/extract"/*.sql 2>/dev/null | wc -l)
if [ "$SQL_COUNT" -eq 0 ]; then
    echo "❌ 备份中未包含 SQL 文件"
    rm -rf "$TEST_DIR"
    exit 1
fi
echo "✅ SQL 文件存在"

echo ""
echo "=== 全部测试通过 ✅ ==="
rm -rf "$TEST_DIR"
