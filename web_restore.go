package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ============================================================
// Web 面板恢复 - 入口（底层恢复逻辑见 restore.go）
// ============================================================

// restoreResult 恢复结果
type restoreResult struct {
	Success bool
	Message string
}

// doRestoreInternal 从最新备份恢复（Web 面板使用）
func doRestoreInternal(cfg Config) restoreResult {
	return doRestore(cfg, "")
}

// doRestoreFromPath 从指定备份文件恢复（Web 面板使用）
// 使用 restore.go 的 doRestore 引擎，支持所有数据库类型
func doRestoreFromPath(backupPath, dataDir, zipPassword, dbType, sqliteDbFile string, createTemp bool) restoreResult {
	// 构建临时 Config
	cfg := loadConfig()
	cfg.ZipPassword = zipPassword
	cfg.DataDir = dataDir
	cfg.DBType = dbType
	cfg.SQLiteDBFile = sqliteDbFile

	return doRestore(cfg, backupPath)
}

// rollbackRestore 回滚恢复操作（保留备用）
func rollbackRestore(dataDir, tempBackup string) {
	entries, err := os.ReadDir(dataDir)
	if err == nil {
		for _, entry := range entries {
			if entry.Name() == "_restore_undo" {
				continue
			}
			_ = os.RemoveAll(filepath.Join(dataDir, entry.Name()))
		}
	}
	entries, err = os.ReadDir(tempBackup)
	if err == nil {
		for _, entry := range entries {
			src := filepath.Join(tempBackup, entry.Name())
			dst := filepath.Join(dataDir, entry.Name())
			_ = os.Rename(src, dst)
		}
	}
	_ = os.RemoveAll(tempBackup)
}

// saveUploadedFile 保存用户上传的文件到备份目录
func saveUploadedFile(fileBody io.Reader, filename, backupDir string) (string, error) {
	savePath := filepath.Join(backupDir, "_upload_"+filepath.Base(filename))
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("创建备份目录失败: %w", err)
	}
	out, err := os.Create(savePath)
	if err != nil {
		return "", fmt.Errorf("创建文件失败: %w", err)
	}
	defer out.Close()
	if _, err := io.Copy(out, fileBody); err != nil {
		return "", fmt.Errorf("保存文件失败: %w", err)
	}
	return savePath, nil
}

// 兼容：doUploadRestore 转化为 doRestoreFromPath 调用
func doUploadRestore(savePath, dataDir, zipPassword, dbType, sqliteDbFile string, createTemp bool) restoreResult {
	return doRestoreFromPath(savePath, dataDir, zipPassword, dbType, sqliteDbFile, createTemp)
}
