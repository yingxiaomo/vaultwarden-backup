package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DBType         string
	SQLiteDBFile   string
	DataDir        string
	BackupDir      string
	ZipPassword    string
	AppriseURL     string
	AppriseAPIURL  string
	RcloneRemote   string
	MinDiskSpaceMB int
	BackupPrefix   string
	LocalKeepDays  int
	RcloneKeepDays int
	Lang           string
	DBHost         string
	DBPort         string
	DBUser         string
	DBPassword     string
	DBName         string
	HTTPProxy      string
	HTTPSProxy     string
	CronSchedule   string
	RunOnStartup   bool
}

func loadConfig() Config {
	c := Config{
		DBType:        getEnv("DB_TYPE", "sqlite"),
		SQLiteDBFile:  getEnv("SQLITE_DB_FILE", "db.sqlite3"),
		DataDir:       getEnv("DATA_DIR", "/data"),
		BackupDir:     getEnv("BACKUP_DIR", "/backup"),
		ZipPassword:   os.Getenv("ZIP_PASSWORD"),
		AppriseURL:    os.Getenv("APPRISE_URL"),
		AppriseAPIURL: strings.TrimRight(os.Getenv("APPRISE_API_URL"), "/"),
		RcloneRemote:  os.Getenv("RCLONE_REMOTE"),
		BackupPrefix:  getEnv("BACKUP_PREFIX", "vaultwarden_backup"),
		Lang:          getEnv("LANGUAGE", "zh"),
		DBHost:        getEnv("DB_HOST", "db"),
		DBPort:        os.Getenv("DB_PORT"),
		DBUser:        os.Getenv("DB_USER"),
		DBPassword:    os.Getenv("DB_PASSWORD"),
		DBName:        os.Getenv("DB_NAME"),
		HTTPProxy:     os.Getenv("HTTP_PROXY"),
		HTTPSProxy:    os.Getenv("HTTPS_PROXY"),
		CronSchedule:  getEnv("CRON_SCHEDULE", "0 2 * * *"),
		RunOnStartup:  getEnv("RUN_ON_STARTUP", "true") == "true",
	}
	c.MinDiskSpaceMB = getEnvInt("MIN_DISK_SPACE_MB", 5120)
	c.LocalKeepDays = getEnvInt("LOCAL_BACKUP_KEEP_DAYS", 15)
	c.RcloneKeepDays = getEnvInt("RCLONE_KEEP_DAYS", 0)
	if c.DBPort == "" {
		switch c.DBType {
		case "mysql":
			c.DBPort = "3306"
		case "postgres":
			c.DBPort = "5432"
		}
	}
	return c
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// ============================================================
// 多语言
// ============================================================

type LangMap map[string]string

var zhCN = LangMap{}
var enUS = LangMap{}

func init() {
	zhCN = LangMap{
		"startup":            "Vaultwarden 备份容器已启动",
		"cfg_db_type":        "数据库类型",
		"cfg_backup_dir":     "备份目录",
		"cfg_prefix":         "文件前缀",
		"cfg_keep_local":     "本地保留天数",
		"cfg_keep_remote":    "远端保留天数",
		"cfg_min_space":      "磁盘空间阈值",
		"cfg_rclone":         "远端同步",
		"cfg_apprise":        "通知方式",
		"cfg_apprise_url":    "Apprise 直连",
		"cfg_apprise_api":    "Apprise API",
		"cfg_proxy":          "代理",
		"cfg_no_proxy":       "无",
		"disk_check":         "检查磁盘空间",
		"disk_check_fail":    "无法获取磁盘空间信息，跳过检查",
		"disk_warn":          "磁盘空间不足！剩余 %s，低于阈值 %d MB",
		"disk_ok":            "磁盘空间充足",
		"db_backup":          "备份数据库（类型: %s）",
		"db_missing_tool":    "找不到 %s 命令，请检查安装",
		"db_sqlite_notfound": "找不到 SQLite 数据库文件 %s",
		"db_missing_mysql":   "MySQL 备份需要设置 DB_USER、DB_PASSWORD 和 DB_NAME",
		"db_missing_pg":      "PostgreSQL 备份需要设置 DB_USER、DB_PASSWORD 和 DB_NAME",
		"db_unsupported":     "不支持的数据库类型: %s",
		"db_ok":              "数据库备份成功",
		"db_fail":            "数据库备份失败！请检查数据库连接和权限",
		"staging":            "组装数据文件",
		"staging_copy":       "复制数据目录内容",
		"staging_clean":      "清理数据目录中的 SQLite 文件",
		"zip_start":          "打包中",
		"zip_unencrypted":    "非加密打包",
		"zip_encrypted":      "AES-256 加密打包",
		"zip_fail":           "打包失败",
		"zip_verify":         "校验压缩包完整性",
		"zip_verify_ok":      "完整性校验通过",
		"zip_verify_fail":    "压缩包损坏",
		"zip_size":           "压缩包大小",
		"rclone_check":       "检查 Rclone 远端配置",
		"rclone_check_ok":    "远端配置检查通过",
		"rclone_notfound":    "找不到 Rclone 远端配置: %s",
		"rclone_upload":      "上传到 %s",
		"rclone_retry":       "上传失败，%d 秒后重试（第 %d/%d 次）",
		"rclone_upload_ok":   "上传成功",
		"rclone_upload_fail": "上传失败（已重试 %d 次）",
		"rclone_cleanup":     "清理远端过期备份（保留 %d 天）",
		"rclone_cleanup_done": "远端清理完成",
		"rclone_skip":        "未配置 RCLONE_REMOTE，跳过云端上传",
		"local_cleanup":      "清理本地过期备份（保留 %d 天）",
		"local_cleanup_done": "本地清理完成",
		"cleanup_temp":       "清理临时文件",
		"notify_send":        "发送通知",
		"notify_apprise_api": "通过 Apprise API 发送",
		"notify_telegram":    "通过 Telegram 发送",
		"notify_webhook":     "通过 Webhook 发送",
		"notify_skip":        "未配置通知方式，跳过",
		"notify_fail":        "通知发送失败: %v",
		"success_title":      "Vaultwarden 备份成功",
		"success_body":       "类型: %s\n大小: %s\n文件: %s.zip",
		"fail_title":         "Vaultwarden 备份失败",
		"warn_title":         "Vaultwarden 备份警告",
		"warn_body":          "磁盘空间不足！剩余 %s，低于阈值 %d MB",
		"footer":             "备份完成",
		"err_create_dir":     "创建目录失败",
		"err_no_write_perm":  "备份目录没有写入权限",
	}

	enUS = LangMap{
		"startup":            "Vaultwarden Backup Container Started",
		"cfg_db_type":        "Database Type",
		"cfg_backup_dir":     "Backup Directory",
		"cfg_prefix":         "File Prefix",
		"cfg_keep_local":     "Local Retention (days)",
		"cfg_keep_remote":    "Remote Retention (days)",
		"cfg_min_space":      "Disk Space Threshold",
		"cfg_rclone":         "Remote Sync",
		"cfg_apprise":        "Notification",
		"cfg_apprise_url":    "Apprise Direct",
		"cfg_apprise_api":    "Apprise API",
		"cfg_proxy":          "Proxy",
		"cfg_no_proxy":       "None",
		"disk_check":         "Checking disk space",
		"disk_check_fail":    "Cannot determine disk space, skipping check",
		"disk_warn":          "Insufficient disk space! Available: %s, threshold: %d MB",
		"disk_ok":            "Disk space OK",
		"db_backup":          "Backing up database (type: %s)",
		"db_missing_tool":    "%s command not found, please check installation",
		"db_sqlite_notfound": "SQLite database file %s not found",
		"db_missing_mysql":   "MySQL backup requires DB_USER, DB_PASSWORD and DB_NAME",
		"db_missing_pg":      "PostgreSQL backup requires DB_USER, DB_PASSWORD and DB_NAME",
		"db_unsupported":     "Unsupported database type: %s",
		"db_ok":              "Database backup successful",
		"db_fail":            "Database backup failed! Please check connection and permissions",
		"staging":            "Assembling data files",
		"staging_copy":       "Copying data directory contents",
		"staging_clean":      "Cleaning SQLite files from data directory",
		"zip_start":          "Packaging",
		"zip_unencrypted":    "Unencrypted packaging",
		"zip_encrypted":      "AES-256 encrypted packaging",
		"zip_fail":           "Packaging failed",
		"zip_verify":         "Verifying zip integrity",
		"zip_verify_ok":      "Integrity check passed",
		"zip_verify_fail":    "Zip file corrupted",
		"zip_size":           "Backup size",
		"rclone_check":       "Checking Rclone remote configuration",
		"rclone_check_ok":    "Remote configuration OK",
		"rclone_notfound":    "Rclone remote not found: %s",
		"rclone_upload":      "Uploading to %s",
		"rclone_retry":       "Upload failed, retrying in %ds (attempt %d/%d)",
		"rclone_upload_ok":   "Upload successful",
		"rclone_upload_fail": "Upload failed (retried %d times)",
		"rclone_cleanup":     "Cleaning up remote expired backups (keeping %d days)",
		"rclone_cleanup_done": "Remote cleanup completed",
		"rclone_skip":        "RCLONE_REMOTE not configured, skipping cloud upload",
		"local_cleanup":      "Cleaning up local expired backups (keeping %d days)",
		"local_cleanup_done": "Local cleanup completed",
		"cleanup_temp":       "Cleaning up temporary files",
		"notify_send":        "Sending notification",
		"notify_apprise_api": "Via Apprise API",
		"notify_telegram":    "Via Telegram",
		"notify_webhook":     "Via Webhook",
		"notify_skip":        "No notification service configured, skipping",
		"notify_fail":        "Notification failed: %v",
		"success_title":      "Vaultwarden Backup Successful",
		"success_body":       "Type: %s\nSize: %s\nFile: %s.zip",
		"fail_title":         "Vaultwarden Backup Failed",
		"warn_title":         "Vaultwarden Backup Warning",
		"warn_body":          "Insufficient disk space! Available: %s, threshold: %d MB",
		"footer":             "Backup completed",
		"err_create_dir":     "Failed to create directory",
		"err_no_write_perm":  "Backup directory is not writable",
	}
}

func T(lang, key string, args ...interface{}) string {
	var m LangMap
	if lang == "en" {
		m = enUS
	} else {
		m = zhCN
	}
	tmpl, ok := m[key]
	if !ok {
		return key
	}
	if len(args) > 0 {
		return fmt.Sprintf(tmpl, args...)
	}
	return tmpl
}

func logf(cfg Config, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	t := time.Now().Format("2006-01-02 15:04:05")
	fmt.Printf("[%s] %s\n", t, msg)
}

// ============================================================
// 命令执行
// ============================================================

func runCmd(cmd string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	c := exec.Command(cmd, args...)
	c.Stdout = &stdout
	c.Stderr = &stderr
	err := c.Run()
	if err != nil {
		return stderr.String(), fmt.Errorf("%s: %w\n%s", cmd, err, stderr.String())
	}
	return stdout.String(), nil
}

func runCmdWithEnv(cmd string, args []string, extraEnv []string) (string, error) {
	var stdout, stderr bytes.Buffer
	c := exec.Command(cmd, args...)
	c.Stdout = &stdout
	c.Stderr = &stderr
	c.Env = append(os.Environ(), extraEnv...)
	err := c.Run()
	if err != nil {
		return stderr.String(), fmt.Errorf("%s: %w\n%s", cmd, err, stderr.String())
	}
	return stdout.String(), nil
}

func fileSize(path string) string {
	fi, err := os.Stat(path)
	if err != nil {
		return "?"
	}
	size := fi.Size()
	switch {
	case size > 1024*1024*1024:
		return fmt.Sprintf("%.1f GB", float64(size)/(1024*1024*1024))
	case size > 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
	case size > 1024:
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	default:
		return fmt.Sprintf("%d B", size)
	}
}

// ============================================================
// 磁盘空间检查
// ============================================================

func getFreeSpaceMB(dir string) (int, error) {
	out, err := runCmd("df", "-P", dir)
	if err != nil {
		return 0, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		return 0, fmt.Errorf("unexpected df output")
	}
	lastLine := lines[len(lines)-1]
	fields := strings.Fields(lastLine)
	if len(fields) < 5 {
		return 0, fmt.Errorf("unexpected df line")
	}
	availBlocks, err := strconv.ParseInt(fields[3], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("cannot parse available space")
	}
	return int(availBlocks * 512 / (1024 * 1024)), nil
}

func formatBytes(bytes int64) string {
	switch {
	case bytes > 1024*1024*1024:
		return fmt.Sprintf("%.1f GB", float64(bytes)/(1024*1024*1024))
	case bytes > 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	case bytes > 1024:
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// ============================================================
// 数据库备份
// ============================================================

func backupSQLite(cfg Config, output string) error {
	if _, err := exec.LookPath("sqlite3"); err != nil {
		return fmt.Errorf(T(cfg.Lang, "db_missing_tool"), "sqlite3")
	}
	dbPath := filepath.Join(cfg.DataDir, cfg.SQLiteDBFile)
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return fmt.Errorf(T(cfg.Lang, "db_sqlite_notfound"), dbPath)
	}
	_, err := runCmd("sqlite3", dbPath, fmt.Sprintf(".backup '%s'", output))
	return err
}

func backupMySQL(cfg Config, output string) error {
	if _, err := exec.LookPath("mysqldump"); err != nil {
		return fmt.Errorf(T(cfg.Lang, "db_missing_tool"), "mysqldump")
	}
	if cfg.DBUser == "" || cfg.DBPassword == "" || cfg.DBName == "" {
		return fmt.Errorf(T(cfg.Lang, "db_missing_mysql"))
	}
	args := []string{
		"--single-transaction", "--quick", "--opt",
		"-h", cfg.DBHost,
		"-P", cfg.DBPort,
		"-u", cfg.DBUser,
		cfg.DBName,
	}
	env := []string{fmt.Sprintf("MYSQL_PWD=%s", cfg.DBPassword)}
	out, err := runCmdWithEnv("mysqldump", args, env)
	if err != nil {
		return err
	}
	return os.WriteFile(output, []byte(out), 0644)
}

func backupPostgres(cfg Config, output string) error {
	if _, err := exec.LookPath("pg_dump"); err != nil {
		return fmt.Errorf(T(cfg.Lang, "db_missing_tool"), "pg_dump")
	}
	if cfg.DBUser == "" || cfg.DBPassword == "" || cfg.DBName == "" {
		return fmt.Errorf(T(cfg.Lang, "db_missing_pg"))
	}
	args := []string{
		"--no-owner", "--no-privileges", "--clean",
		"-h", cfg.DBHost,
		"-p", cfg.DBPort,
		"-U", cfg.DBUser,
		"-d", cfg.DBName,
		"-F", "p",
	}
	env := []string{fmt.Sprintf("PGPASSWORD=%s", cfg.DBPassword)}
	out, err := runCmdWithEnv("pg_dump", args, env)
	if err != nil {
		return err
	}
	return os.WriteFile(output, []byte(out), 0644)
}

// ============================================================
// 文件操作
// ============================================================

func copyDirContents(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	return copyDirContents(src, dst)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func removeSQLiteFiles(dir string) {
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && (strings.HasSuffix(info.Name(), ".sqlite3") ||
			strings.HasSuffix(info.Name(), ".sqlite") ||
			strings.HasSuffix(info.Name(), ".db-wal") ||
			strings.HasSuffix(info.Name(), ".db-shm")) {
			os.Remove(path)
		}
		return nil
	})
}

// ============================================================
// ZIP 打包
// ============================================================

func createZip(sourceDir, targetFile, password string) error {
	args := []string{"-r", "-q"}
	if password != "" {
		args = append(args, "-P", password)
	}
	args = append(args, targetFile, ".")
	args = append(args, "-x", ".*")

	cmd := exec.Command("zip", args...)
	cmd.Dir = sourceDir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("zip: %w\n%s", err, stderr.String())
	}
	return nil
}

func verifyZip(zipFile, password string) error {
	args := []string{"-t", "-q"}
	if password != "" {
		args = append(args, "-P", password)
	}
	args = append(args, zipFile)
	_, err := runCmd("unzip", args...)
	return err
}

// ============================================================
// Rclone 操作
// ============================================================

func extractRemoteName(remote string) string {
	idx := strings.Index(remote, ":")
	if idx < 0 {
		return remote
	}
	return remote[:idx]
}

func checkRcloneRemote(remote string) (bool, error) {
	out, err := runCmd("rclone", "listremotes")
	if err != nil {
		return false, err
	}
	remoteName := extractRemoteName(remote) + ":"
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == remoteName {
			return true, nil
		}
	}
	return false, nil
}

func uploadWithRetry(cfg Config, zipFile, remote string) error {
	maxRetries := 3
	delay := 5
	for i := 1; i <= maxRetries; i++ {
		_, err := runCmd("rclone", "copy", zipFile, remote)
		if err == nil {
			return nil
		}
		if i < maxRetries {
			logf(cfg, T(cfg.Lang, "rclone_retry"), delay, i, maxRetries)
			time.Sleep(time.Duration(delay) * time.Second)
			delay *= 2
		} else {
			return err
		}
	}
	return fmt.Errorf("upload failed after %d retries", maxRetries)
}

func cleanupRemote(remote, prefix string, keepDays int) error {
	_, err := runCmd("rclone", "delete", remote,
		"--filter", fmt.Sprintf("+ %s_*.zip", prefix),
		"--filter", "- *",
		"--min-age", fmt.Sprintf("%dd", keepDays),
	)
	return err
}

func cleanupLocal(backupDir, prefix string, keepDays int) error {
	pattern := filepath.Join(backupDir, fmt.Sprintf("%s_*.zip", prefix))
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	cutoff := time.Now().AddDate(0, 0, -keepDays)
	for _, match := range matches {
		fi, err := os.Stat(match)
		if err != nil {
			continue
		}
		if fi.ModTime().Before(cutoff) {
			os.Remove(match)
		}
	}
	return nil
}

// ============================================================
// 通知发送
// ============================================================

func sendNotification(cfg Config, title, body string) {
	if cfg.AppriseAPIURL != "" {
		logf(cfg, "%s (%s)", T(cfg.Lang, "notify_send"), T(cfg.Lang, "notify_apprise_api"))
		if err := notifyViaAppriseAPI(cfg.AppriseAPIURL, cfg.AppriseURL, title, body); err != nil {
			logf(cfg, T(cfg.Lang, "notify_fail"), err)
		}
		return
	}

	if cfg.AppriseURL != "" {
		if notified := notifyDirect(cfg, title, body); notified {
			return
		}
		logf(cfg, "%s (apprise CLI)...", T(cfg.Lang, "notify_send"))
		if _, err := runCmd("apprise", "-t", title, "-b", body, cfg.AppriseURL); err != nil {
			logf(cfg, T(cfg.Lang, "notify_fail"), err)
		}
		return
	}

	logf(cfg, "%s", T(cfg.Lang, "notify_skip"))
}

func notifyDirect(cfg Config, title, body string) bool {
	if strings.HasPrefix(cfg.AppriseURL, "tgram://") || strings.HasPrefix(cfg.AppriseURL, "telegram://") {
		schemeEnd := strings.Index(cfg.AppriseURL, "://")
		if schemeEnd < 0 {
			return false
		}
		rest := cfg.AppriseURL[schemeEnd+3:]
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
			return false
		}
		botToken := parts[0]
		chatID := parts[1]

		logf(cfg, "%s (%s)", T(cfg.Lang, "notify_send"), T(cfg.Lang, "notify_telegram"))
		if err := sendTelegram(botToken, chatID, title, body); err != nil {
			logf(cfg, T(cfg.Lang, "notify_fail"), err)
			return false
		}
		return true
	}

	if u, err := url.Parse(cfg.AppriseURL); err == nil {
		if u.Scheme == "discord" || u.Scheme == "slack" || u.Scheme == "webhook" {
			logf(cfg, "%s (%s)", T(cfg.Lang, "notify_send"), T(cfg.Lang, "notify_webhook"))
			if err := sendWebhook(cfg.AppriseURL, title, body); err != nil {
				logf(cfg, T(cfg.Lang, "notify_fail"), err)
			}
			return true
		}
	}

	return false
}

func sendTelegram(botToken, chatID, title, body string) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       escapeMarkdownV2(fmt.Sprintf("*%s*\n%s", title, body)),
		"parse_mode": "MarkdownV2",
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := http.Post(apiURL, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Telegram API %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func escapeMarkdownV2(text string) string {
	special := []string{"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}
	result := text
	for _, ch := range special {
		result = strings.ReplaceAll(result, ch, "\\"+ch)
	}
	return result
}

func sendWebhook(webhookURL, title, body string) error {
	payload := map[string]interface{}{
		"title":   title,
		"body":    body,
		"message": fmt.Sprintf("%s\n\n%s", title, body),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Post(webhookURL, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func notifyViaAppriseAPI(apiURL, appriseURL, title, body string) error {
	apiURL = strings.TrimRight(apiURL, "/")

	if appriseURL != "" {
		payload := map[string]interface{}{
			"title": title,
			"body":  body,
			"urls":  appriseURL,
		}
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Post(apiURL+"/notify", "application/json", bytes.NewReader(data))
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			b, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("Apprise API %d: %s", resp.StatusCode, string(b))
		}
		return nil
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.PostForm(apiURL, url.Values{
		"title": {title},
		"body":  {body},
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Apprise API %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// ============================================================
// Cron 调度
// ============================================================

func cronMatcher(expr string, t time.Time) bool {
	fields := strings.Fields(expr)
	if len(fields) < 5 {
		return false
	}
	return cronFieldMatch(fields[0], t.Minute()) &&
		cronFieldMatch(fields[1], t.Hour()) &&
		cronFieldMatch(fields[2], t.Day()) &&
		cronFieldMatch(fields[3], int(t.Month())) &&
		cronFieldMatch(fields[4], int(t.Weekday()))
}

func cronFieldMatch(pattern string, value int) bool {
	if pattern == "*" {
		return true
	}
	for _, part := range strings.Split(pattern, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "-") && !strings.Contains(part, "/") {
			rangeParts := strings.SplitN(part, "-", 2)
			start, err1 := strconv.Atoi(rangeParts[0])
			end, err2 := strconv.Atoi(rangeParts[1])
			if err1 == nil && err2 == nil && value >= start && value <= end {
				return true
			}
			continue
		}
		if strings.Contains(part, "/") {
			sub := strings.SplitN(part, "/", 2)
			step, err := strconv.Atoi(sub[1])
			if err != nil {
				continue
			}
			if sub[0] == "*" {
				if value%step == 0 {
					return true
				}
			} else {
				start, err := strconv.Atoi(sub[0])
				if err != nil {
					continue
				}
				if value >= start && (value-start)%step == 0 {
					return true
				}
			}
			continue
		}
		if v, err := strconv.Atoi(part); err == nil && v == value {
			return true
		}
	}
	return false
}

// ============================================================
// 主备份流程
// ============================================================

func runBackup(cfg Config) int {
	if cfg.HTTPProxy != "" {
		os.Setenv("HTTP_PROXY", cfg.HTTPProxy)
	}
	if cfg.HTTPSProxy != "" {
		os.Setenv("HTTPS_PROXY", cfg.HTTPSProxy)
	}

	logf(cfg, "━━━ %s ━━━", T(cfg.Lang, "startup"))
	logf(cfg, "%s: %s", T(cfg.Lang, "cfg_db_type"), cfg.DBType)
	logf(cfg, "%s: %s", T(cfg.Lang, "cfg_backup_dir"), cfg.BackupDir)
	logf(cfg, "%s: %s", T(cfg.Lang, "cfg_prefix"), cfg.BackupPrefix)
	logf(cfg, "%s: %d", T(cfg.Lang, "cfg_keep_local"), cfg.LocalKeepDays)
	if cfg.RcloneRemote != "" {
		logf(cfg, "%s: %s", T(cfg.Lang, "cfg_rclone"), cfg.RcloneRemote)
		if cfg.RcloneKeepDays > 0 {
			logf(cfg, "%s: %d", T(cfg.Lang, "cfg_keep_remote"), cfg.RcloneKeepDays)
		}
	}
	if cfg.AppriseAPIURL != "" {
		logf(cfg, "%s: %s (%s)", T(cfg.Lang, "cfg_apprise"), T(cfg.Lang, "cfg_apprise_api"), cfg.AppriseAPIURL)
	} else if cfg.AppriseURL != "" {
		logf(cfg, "%s: %s", T(cfg.Lang, "cfg_apprise"), T(cfg.Lang, "cfg_apprise_url"))
	}
	if cfg.HTTPProxy != "" {
		logf(cfg, "%s: HTTP_PROXY=%s", T(cfg.Lang, "cfg_proxy"), cfg.HTTPProxy)
	} else {
		logf(cfg, "%s: %s", T(cfg.Lang, "cfg_proxy"), T(cfg.Lang, "cfg_no_proxy"))
	}

	var (
		zipDone    bool
		zipFile    string
		fileSz     string
		backupName string
	)

	ts := time.Now().Format("20060102_150405")
	safePrefix := cfg.BackupPrefix
	if safePrefix == "" {
		safePrefix = "vaultwarden_backup"
	}
	backupName = fmt.Sprintf("%s_%s", safePrefix, ts)
	zipFile = filepath.Join(cfg.BackupDir, backupName+".zip")

	defer func() {
		if !zipDone {
			os.Remove(zipFile)
		}
		entries, _ := filepath.Glob(filepath.Join(cfg.BackupDir, "staging_*"))
		for _, e := range entries {
			os.RemoveAll(e)
		}
	}()

	if err := os.MkdirAll(cfg.BackupDir, 0755); err != nil {
		logf(cfg, "%s: %v", T(cfg.Lang, "err_create_dir"), err)
		sendNotification(cfg, T(cfg.Lang, "fail_title"), fmt.Sprintf("%s: %v", T(cfg.Lang, "err_create_dir"), err))
		return 1
	}

	testFile := filepath.Join(cfg.BackupDir, ".write_test")
	if err := os.WriteFile(testFile, []byte{}, 0644); err != nil {
		logf(cfg, "%s", T(cfg.Lang, "err_no_write_perm"))
		sendNotification(cfg, T(cfg.Lang, "fail_title"), T(cfg.Lang, "err_no_write_perm"))
		return 1
	}
	os.Remove(testFile)

	logf(cfg, "%s...", T(cfg.Lang, "disk_check"))
	freeMB, err := getFreeSpaceMB(cfg.BackupDir)
	if err != nil {
		logf(cfg, "%s", T(cfg.Lang, "disk_check_fail"))
	} else if freeMB < cfg.MinDiskSpaceMB {
		humanFree := formatBytes(int64(freeMB) * 1024 * 1024)
		logf(cfg, T(cfg.Lang, "disk_warn"), humanFree, cfg.MinDiskSpaceMB)
		sendNotification(cfg, T(cfg.Lang, "warn_title"),
			fmt.Sprintf(T(cfg.Lang, "warn_body"), humanFree, cfg.MinDiskSpaceMB))
	} else {
		logf(cfg, "%s (%d MB)", T(cfg.Lang, "disk_ok"), freeMB)
	}

	logf(cfg, T(cfg.Lang, "db_backup"), cfg.DBType)

	stagingDir := filepath.Join(cfg.BackupDir, fmt.Sprintf("staging_%s", ts))
	if err := os.MkdirAll(stagingDir, 0755); err != nil {
		logf(cfg, "%s: %v", T(cfg.Lang, "err_create_dir"), err)
		sendNotification(cfg, T(cfg.Lang, "fail_title"), fmt.Sprintf("%s: %v", T(cfg.Lang, "err_create_dir"), err))
		return 1
	}

	sqlFile := filepath.Join(stagingDir, backupName+".sql")

	switch cfg.DBType {
	case "sqlite":
		if err := backupSQLite(cfg, sqlFile); err != nil {
			logf(cfg, "%s: %v", T(cfg.Lang, "db_fail"), err)
			sendNotification(cfg, T(cfg.Lang, "fail_title"), fmt.Sprintf("%s: %v", T(cfg.Lang, "db_fail"), err))
			return 1
		}
	case "mysql":
		if err := backupMySQL(cfg, sqlFile); err != nil {
			logf(cfg, "%s: %v", T(cfg.Lang, "db_fail"), err)
			sendNotification(cfg, T(cfg.Lang, "fail_title"), fmt.Sprintf("%s: %v", T(cfg.Lang, "db_fail"), err))
			return 1
		}
	case "postgres":
		if err := backupPostgres(cfg, sqlFile); err != nil {
			logf(cfg, "%s: %v", T(cfg.Lang, "db_fail"), err)
			sendNotification(cfg, T(cfg.Lang, "fail_title"), fmt.Sprintf("%s: %v", T(cfg.Lang, "db_fail"), err))
			return 1
		}
	default:
		logf(cfg, T(cfg.Lang, "db_unsupported"), cfg.DBType)
		sendNotification(cfg, T(cfg.Lang, "fail_title"), fmt.Sprintf(T(cfg.Lang, "db_unsupported"), cfg.DBType))
		return 1
	}
	logf(cfg, "✅ %s", T(cfg.Lang, "db_ok"))

	logf(cfg, "%s...", T(cfg.Lang, "staging"))

	dataDir := filepath.Join(stagingDir, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		logf(cfg, "%s: %v", T(cfg.Lang, "err_create_dir"), err)
		sendNotification(cfg, T(cfg.Lang, "fail_title"), fmt.Sprintf("%s: %v", T(cfg.Lang, "err_create_dir"), err))
		return 1
	}

	logf(cfg, "%s...", T(cfg.Lang, "staging_copy"))
	if err := copyDirContents(cfg.DataDir, dataDir); err != nil {
		logf(cfg, "复制数据目录失败: %v", err)
	}

	logf(cfg, "%s...", T(cfg.Lang, "staging_clean"))
	removeSQLiteFiles(dataDir)

	logf(cfg, "%s...", T(cfg.Lang, "zip_start"))
	if cfg.ZipPassword != "" {
		logf(cfg, "%s", T(cfg.Lang, "zip_encrypted"))
	} else {
		logf(cfg, "%s", T(cfg.Lang, "zip_unencrypted"))
	}

	if err := createZip(stagingDir, zipFile, cfg.ZipPassword); err != nil {
		logf(cfg, "%s: %v", T(cfg.Lang, "zip_fail"), err)
		sendNotification(cfg, T(cfg.Lang, "fail_title"), fmt.Sprintf("%s: %v", T(cfg.Lang, "zip_fail"), err))
		return 1
	}

	if _, err := os.Stat(zipFile); os.IsNotExist(err) {
		logf(cfg, "%s", T(cfg.Lang, "zip_fail"))
		sendNotification(cfg, T(cfg.Lang, "fail_title"), T(cfg.Lang, "zip_fail"))
		return 1
	}

	zipDone = true
	os.RemoveAll(stagingDir)

	logf(cfg, "%s...", T(cfg.Lang, "zip_verify"))
	if err := verifyZip(zipFile, cfg.ZipPassword); err != nil {
		logf(cfg, "%s: %v", T(cfg.Lang, "zip_verify_fail"), err)
		sendNotification(cfg, T(cfg.Lang, "fail_title"), fmt.Sprintf("%s: %v", T(cfg.Lang, "zip_verify_fail"), err))
		return 1
	}
	logf(cfg, "✅ %s", T(cfg.Lang, "zip_verify_ok"))

	fileSz = fileSize(zipFile)
	logf(cfg, "%s: %s", T(cfg.Lang, "zip_size"), fileSz)

	if cfg.RcloneRemote != "" {
		logf(cfg, "%s...", T(cfg.Lang, "rclone_check"))
		ok, err := checkRcloneRemote(cfg.RcloneRemote)
		if err != nil || !ok {
			logf(cfg, T(cfg.Lang, "rclone_notfound"), extractRemoteName(cfg.RcloneRemote))
			sendNotification(cfg, T(cfg.Lang, "fail_title"),
				fmt.Sprintf(T(cfg.Lang, "rclone_notfound"), extractRemoteName(cfg.RcloneRemote)))
			return 1
		}
		logf(cfg, "✅ %s", T(cfg.Lang, "rclone_check_ok"))

		logf(cfg, T(cfg.Lang, "rclone_upload"), cfg.RcloneRemote)
		if err := uploadWithRetry(cfg, zipFile, cfg.RcloneRemote); err != nil {
			logf(cfg, "%s: %v", T(cfg.Lang, "rclone_upload_fail"), err)
			sendNotification(cfg, T(cfg.Lang, "fail_title"),
				fmt.Sprintf("%s: %v", T(cfg.Lang, "rclone_upload_fail"), err))
			return 1
		}
		logf(cfg, "✅ %s", T(cfg.Lang, "rclone_upload_ok"))

		if cfg.RcloneKeepDays > 0 {
			logf(cfg, T(cfg.Lang, "rclone_cleanup"), cfg.RcloneKeepDays)
			if err := cleanupRemote(cfg.RcloneRemote, safePrefix, cfg.RcloneKeepDays); err != nil {
				logf(cfg, "远端清理失败: %v", err)
			} else {
				logf(cfg, "%s", T(cfg.Lang, "rclone_cleanup_done"))
			}
		}
	} else {
		logf(cfg, "%s", T(cfg.Lang, "rclone_skip"))
	}

	logf(cfg, T(cfg.Lang, "local_cleanup"), cfg.LocalKeepDays)
	if err := cleanupLocal(cfg.BackupDir, safePrefix, cfg.LocalKeepDays); err != nil {
		logf(cfg, "本地清理失败: %v", err)
	} else {
		logf(cfg, "%s", T(cfg.Lang, "local_cleanup_done"))
	}

	logf(cfg, "%s...", T(cfg.Lang, "notify_send"))
	successBody := fmt.Sprintf(T(cfg.Lang, "success_body"), cfg.DBType, fileSz, backupName)
	sendNotification(cfg, T(cfg.Lang, "success_title"), successBody)

	logf(cfg, "✅ %s", T(cfg.Lang, "footer"))
	return 0
}

// ============================================================
// 调度器
// ============================================================

func startScheduler() {
	cfg := loadConfig()

	log.Printf("启动后台调度模式，按 CRON_SCHEDULE 定时备份...")
	log.Printf("提示：使用 --once 执行一次性备份")
	log.Printf("Cron 表达式: %s", cfg.CronSchedule)
	log.Printf("启动时备份: %v", cfg.RunOnStartup)

	if cfg.RunOnStartup {
		log.Println("执行启动时备份...")
		cfg := loadConfig()
		runBackup(cfg)
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	lastCheckedMin := -1

	for range ticker.C {
		now := time.Now()
		currentMin := now.Hour()*100 + now.Minute()

		if currentMin == lastCheckedMin {
			continue
		}
		lastCheckedMin = currentMin

		cfg := loadConfig()
		if cfg.CronSchedule == "" {
			continue
		}

		if cronMatcher(cfg.CronSchedule, now) {
			log.Printf("触发定时备份 (cron: %s)", cfg.CronSchedule)
			runBackup(cfg)
		}
	}
}

// ============================================================
// 入口
// ============================================================

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "--once" || arg == "-once" || arg == "--oneshot" {
			cfg := loadConfig()
			os.Exit(runBackup(cfg))
			return
		}
	}

	startScheduler()
	select {}
}
