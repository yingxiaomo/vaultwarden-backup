package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

// restoreResult 恢复操作的结果
type restoreResult struct {
	Success bool
	Message string
}

// doRestore 执行恢复操作
// backupZip 为空时自动查找最新的备份
func doRestore(cfg Config, backupZip string) restoreResult {
	// 1. 确定要恢复的备份文件
	if backupZip == "" {
		pattern := filepath.Join(cfg.BackupDir, cfg.BackupPrefix+"_*.zip")
		matches, err := filepath.Glob(pattern)
		if err != nil || len(matches) == 0 {
			return restoreResult{false, "没有找到可恢复的备份文件"}
		}
		backupZip = matches[len(matches)-1]
	}

	logf(cfg, "恢复备份: %s", filepath.Base(backupZip))

	if _, err := os.Stat(backupZip); os.IsNotExist(err) {
		return restoreResult{false, "备份文件不存在: " + backupZip}
	}

	// 2. 创建临时目录
	tmpDir := filepath.Join(cfg.BackupDir, fmt.Sprintf(".restore_%d", time.Now().Unix()))
	defer os.RemoveAll(tmpDir)

	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return restoreResult{false, "创建临时目录失败: " + err.Error()}
	}

	// 3. 解压备份
	logf(cfg, "解压备份文件...")
	unzipArgs := []string{"-o", backupZip, "-d", tmpDir}
	if cfg.ZipPassword != "" {
		unzipArgs = []string{"-P", cfg.ZipPassword, "-o", backupZip, "-d", tmpDir}
	}
	if _, err := runCmd("unzip", unzipArgs...); err != nil {
		return restoreResult{false, "解压备份失败: " + err.Error()}
	}

	// 4. 查找 .sql 文件
	var sqlFile string
	entries, _ := os.ReadDir(tmpDir)
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			sqlFile = filepath.Join(tmpDir, e.Name())
			break
		}
	}

	// 5. 备份当前数据（回滚保护）
	undoDir := filepath.Join(cfg.DataDir, ".pre_restore_backup")
	_ = os.RemoveAll(undoDir)
	if err := copyDir(cfg.DataDir, undoDir); err != nil {
		logf(cfg, "警告: 无法创建数据回滚备份: %v", err)
	}
	defer os.RemoveAll(undoDir)

	// 6. 执行数据库恢复
	if sqlFile != "" {
		logf(cfg, "恢复数据库 (%s)...", cfg.DBType)
		if err := restoreDatabase(cfg, sqlFile); err != nil {
			// 回滚数据目录
			_ = os.RemoveAll(cfg.DataDir)
			_ = copyDir(undoDir, cfg.DataDir)
			return restoreResult{false, "数据库恢复失败（已回滚）: " + err.Error()}
		}
		logf(cfg, "✅ 数据库恢复完成")
	} else {
		logf(cfg, "未找到 SQL 文件，跳过数据库恢复")
	}

	// 7. 恢复 data/ 目录文件
	dataRestore := filepath.Join(tmpDir, "data")
	if info, err := os.Stat(dataRestore); err == nil && info.IsDir() {
		logf(cfg, "恢复数据文件...")
		entries, _ := os.ReadDir(dataRestore)
		for _, e := range entries {
			src := filepath.Join(dataRestore, e.Name())
			dst := filepath.Join(cfg.DataDir, e.Name())
			if e.IsDir() {
				_ = os.RemoveAll(dst)
				_ = copyDir(src, dst)
			} else {
				_ = copyFile(src, dst)
			}
		}
		logf(cfg, "✅ 数据文件恢复完成")
	}

	return restoreResult{true, "恢复完成"}
}

// restoreDatabase 根据数据库类型执行 SQL 恢复
func restoreDatabase(cfg Config, sqlFilePath string) error {
	switch cfg.DBType {
	case "sqlite":
		return restoreSQLite(cfg, sqlFilePath)
	case "mysql":
		return restoreMySQL(cfg, sqlFilePath)
	case "postgres":
		return restorePostgres(cfg, sqlFilePath)
	default:
		return fmt.Errorf("不支持的数据库类型: %s", cfg.DBType)
	}
}

// restoreSQLite 恢复 SQLite：通过 sqlite3 .restore 导入备份文件
func restoreSQLite(cfg Config, sqlFilePath string) error {
	if _, err := runCmd("sqlite3", "--version"); err != nil {
		return fmt.Errorf("找不到 sqlite3 命令")
	}
	target := filepath.Join(cfg.DataDir, cfg.SQLiteDBFile)
	// 使用 sqlite3 的 .restore 命令从备份文件恢复
	_, err := runCmd("sqlite3", target, fmt.Sprintf(".restore '%s'", sqlFilePath))
	if err != nil {
		// .restore 失败时回退到直接复制文件
		return copyFile(sqlFilePath, target)
	}
	return nil
}

// restoreMySQL 通过 Go SQL 驱动执行 SQL 恢复
func restoreMySQL(cfg Config, sqlFilePath string) error {
	if cfg.DBUser == "" || cfg.DBPassword == "" || cfg.DBName == "" {
		return fmt.Errorf("MySQL 恢复需要设置 DB_USER、DB_PASSWORD、DB_NAME")
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&timeout=60s&interpolateParams=true&multiStatements=true",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("连接 MySQL 失败: %w", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		return fmt.Errorf("MySQL 连接测试失败: %w", err)
	}

	return executeSQLFile(db, sqlFilePath, "mysql")
}

// restorePostgres 通过 Go SQL 驱动执行 SQL 恢复
func restorePostgres(cfg Config, sqlFilePath string) error {
	if cfg.DBUser == "" || cfg.DBPassword == "" || cfg.DBName == "" {
		return fmt.Errorf("PostgreSQL 恢复需要设置 DB_USER、DB_PASSWORD、DB_NAME")
	}
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable connect_timeout=30",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("连接 PostgreSQL 失败: %w", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		return fmt.Errorf("PostgreSQL 连接测试失败: %w", err)
	}

	return executeSQLFile(db, sqlFilePath, "postgres")
}

// executeSQLFile 读取 SQL 文件并逐语句执行
func executeSQLFile(db *sql.DB, filePath, dbType string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("打开 SQL 文件失败: %w", err)
	}
	defer f.Close()

	// 按分号分割 SQL 语句
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024) // 10MB 缓冲区

	var statement strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		// 跳过注释行
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// 处理 PostgreSQL 的特殊反斜杠
		if dbType == "postgres" {
			line = strings.ReplaceAll(line, `\`, `\\`)
		}
		statement.WriteString(line + "\n")

		// 完整语句以分号结束（且不在字符串内）
		if strings.HasSuffix(strings.TrimSpace(statement.String()), ";") {
			sql := statement.String()
			if _, err := db.Exec(sql); err != nil {
				// 跳过可忽略的错误（如已存在的索引）
				if !isIgnorableError(err) {
					return fmt.Errorf("执行 SQL 失败: %w\nSQL: %.200s", err, sql)
				}
			}
			statement.Reset()
		}
	}

	// 执行最后一条没有分号的语句（如果有）
	if strings.TrimSpace(statement.String()) != "" {
		if _, err := db.Exec(statement.String()); err != nil {
			if !isIgnorableError(err) {
				return fmt.Errorf("执行 SQL 失败: %w", err)
			}
		}
	}

	return scanner.Err()
}

// isIgnorableError 判断是否可以跳过的错误
func isIgnorableError(err error) bool {
	msg := err.Error()
	// MySQL: 表已存在、索引已存在
	if strings.Contains(msg, "already exists") || strings.Contains(msg, "Duplicate key name") {
		return true
	}
	// PostgreSQL: 表已存在、唯一约束已存在
	if strings.Contains(msg, "already exists") || strings.Contains(msg, "duplicate") {
		return true
	}
	return false
}
