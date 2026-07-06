package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ============================================================
// Web ??????
// ============================================================

// restoreResult ???????
type restoreResult struct {
	Success bool
	Message string
}

// doRestoreInternal ??????
// 1. ?? Vaultwarden ??
// 2. ?????????
// 3. ???????????
// 4. ?? SQLite ????
// 5. ?? Vaultwarden ??
// ?????????
func doRestoreInternal(cfg Config) restoreResult {
	backupDir := cfg.BackupDir
	dataDir := cfg.DataDir
	zipPassword := cfg.ZipPassword
	dbType := cfg.DBType
	sqliteDbFile := cfg.SQLiteDBFile

	// ????? .zip ??
	pattern := filepath.Join(backupDir, cfg.BackupPrefix+"_*.zip")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return restoreResult{Success: false, Message: "???????"}
	}
	// ??????
	backupPath := matches[len(matches)-1]
	return doRestoreFromPath(backupPath, dataDir, zipPassword, dbType, sqliteDbFile, true)
}

// doRestoreFromPath ?????????
func doRestoreFromPath(backupPath, dataDir, zipPassword, dbType, sqliteDbFile string, createTemp bool) restoreResult {
	tempBackup := filepath.Join(dataDir, "_restore_undo")

	// ?? Vaultwarden
	if output, err := runCmd("docker", "stop", "vaultwarden"); err != nil {
		// ????????????????
		_ = output
	}

	// ??????????
	if createTemp {
		_ = os.RemoveAll(tempBackup)
		if err := os.MkdirAll(tempBackup, 0755); err != nil {
			return restoreResult{Success: false, Message: "????????: " + err.Error()}
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
					// ?????????????
					_ = copyFile(src, dst)
					_ = os.RemoveAll(src)
				}
			}
		}
	}

	// ????
	unzipArgs := []string{"-o", backupPath, "-d", dataDir}
	if zipPassword != "" {
		unzipArgs = []string{"-P", zipPassword, "-o", backupPath, "-d", dataDir}
	}
	if _, err := runCmd("unzip", unzipArgs...); err != nil {
		// ?????????
		rollbackRestore(dataDir, tempBackup)
		runCmd("docker", "start", "vaultwarden")
		return restoreResult{Success: false, Message: "????????: " + err.Error()}
	}

	// SQLite ?????? .sql ??????????
	if dbType == "sqlite" {
		// ?? .sql ??
		sqlFiles, _ := filepath.Glob(filepath.Join(dataDir, "*.sql"))
		if len(sqlFiles) > 0 && strings.HasSuffix(sqlFiles[0], ".sql") {
			target := filepath.Join(dataDir, sqliteDbFile)
			// ?? WAL/SHM ??
			_ = os.Remove(target + "-wal")
			_ = os.Remove(target + "-shm")
			// ? .sql ??????????
			if err := os.Rename(sqlFiles[0], target); err != nil {
				_ = copyFile(sqlFiles[0], target)
			}
		}
	}

	// ?? Vaultwarden
	if output, err := runCmd("docker", "start", "vaultwarden"); err != nil {
		_ = output
		// ?????????????????
	}

	// ??????
	_ = os.RemoveAll(tempBackup)

	return restoreResult{Success: true, Message: "????"}
}

func rollbackRestore(dataDir, tempBackup string) {
	// ?????????
	entries, err := os.ReadDir(dataDir)
	if err == nil {
		for _, entry := range entries {
			if entry.Name() == "_restore_undo" {
				continue
			}
			_ = os.RemoveAll(filepath.Join(dataDir, entry.Name()))
		}
	}
	// ???????
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

// doUploadRestore ????????
func doUploadRestore(savePath, dataDir, zipPassword, dbType, sqliteDbFile string, createTemp bool) restoreResult {
	return doRestoreFromPath(savePath, dataDir, zipPassword, dbType, sqliteDbFile, createTemp)
}

// saveUploadedFile ???????
func saveUploadedFile(fileBody io.Reader, filename, backupDir string) (string, error) {
	savePath := filepath.Join(backupDir, "_upload_"+filepath.Base(filename))
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("??????: %w", err)
	}
	out, err := os.Create(savePath)
	if err != nil {
		return "", fmt.Errorf("??????: %w", err)
	}
	defer out.Close()
	if _, err := io.Copy(out, fileBody); err != nil {
		return "", fmt.Errorf("??????: %w", err)
	}
	return savePath, nil
}
