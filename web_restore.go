package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ============================================================
// 恢复操作
// ============================================================

// restoreResult 恢复结果
type restoreResult struct {
	Success bool
	Message string
}

// doRestoreInternal 从最新备份恢复
func doRestoreInternal(cfg Config) restoreResult {
	pattern := filepath.Join(cfg.BackupDir, cfg.BackupPrefix+"_*.zip")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return restoreResult{Success: false, Message: "没有找到可恢复的备份文件"}
	}
	// 取最新的（按时间排序最后一个）
	backupPath := matches[len(matches)-1]
	return doRestoreFromPath(backupPath, cfg.DataDir, cfg.ZipPassword, cfg.DBType, cfg.SQLiteDBFile, true)
}

// doRestoreFromPath 从指定备份文件恢复
// 注意：需要手动停止 Vaultwarden 容器以确保数据一致性
func doRestoreFromPath(backupPath, dataDir, zipPassword, dbType, sqliteDbFile string, createTemp bool) restoreResult {
	tempBackup := filepath.Join(dataDir, "_restore_undo")

	// 1. 备份当前数据（用于回滚）
	if createTemp {
		_ = os.RemoveAll(tempBackup)
		if err := os.MkdirAll(tempBackup, 0755); err != nil {
			return restoreResult{Success: false, Message: "创建临时备份失败: " + err.Error()}
		}
		entries, err := os.ReadDir(dataDir)
		if err == nil {
			for _, entry := range entries {
				if entry.Name() == "_restore_undo" {
					continue
				}
				src := filepath.Join(dataDir, entry.Name())
				dst := filepath.Join(tempBackup, entry.Name())
				if err := os.Rename(src, dst); err != nil {
					_ = copyFile(src, dst)
					_ = os.RemoveAll(src)
				}
			}
		}
	}

	// 2. 解压备份到数据目录
	unzipArgs := []string{"-o", backupPath, "-d", dataDir}
	if zipPassword != "" {
		unzipArgs = []string{"-P", zipPassword, "-o", backupPath, "-d", dataDir}
	}
	if _, err := runCmd("unzip", unzipArgs...); err != nil {
		rollbackRestore(dataDir, tempBackup)
		return restoreResult{Success: false, Message: "解压备份失败: " + err.Error()}
	}

	// 3. SQLite 处理：将 .sql 文件映射回数据库文件
	if dbType == "sqlite" {
		sqlFiles, _ := filepath.Glob(filepath.Join(dataDir, "*.sql"))
		if len(sqlFiles) > 0 && strings.HasSuffix(sqlFiles[0], ".sql") {
			target := filepath.Join(dataDir, sqliteDbFile)
			_ = os.Remove(target + "-wal")
			_ = os.Remove(target + "-shm")
			if err := os.Rename(sqlFiles[0], target); err != nil {
				_ = copyFile(sqlFiles[0], target)
			}
		}
	}

	// 4. 清理临时备份
	_ = os.RemoveAll(tempBackup)

	return restoreResult{Success: true, Message: "恢复完成"}
}

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
