package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"embed"
)

//go:embed templates/*.gohtml
var templateFS embed.FS

// ============================================================
// Web 面板配置
// ============================================================

// WebConfig 完整的 Web 面板配置
type WebConfig struct {
	// 认证
	WebUser string `json:"WEB_USER"`
	WebPass string `json:"WEB_PASS"`

	// 数据库
	DBType       string `json:"DB_TYPE"`
	SQLiteDBFile string `json:"SQLITE_DB_FILE"`
	DBHost       string `json:"DB_HOST"`
	DBPort       string `json:"DB_PORT"`
	DBUser       string `json:"DB_USER"`
	DBPassword   string `json:"DB_PASSWORD"`
	DBName       string `json:"DB_NAME"`

	// 备份路径
	BackupPrefix string `json:"BACKUP_PREFIX"`
	DataDir      string `json:"DATA_DIR"`
	BackupDir    string `json:"BACKUP_DIR"`
	ZipPassword  string `json:"ZIP_PASSWORD"`

	// 定时任务
	CronSchedule  string `json:"CRON_SCHEDULE"`
	RunOnStartup  string `json:"RUN_ON_STARTUP"`
	LocalKeepDays int    `json:"LOCAL_BACKUP_KEEP_DAYS"`

	// 云存储
	RcloneRemote   string `json:"RCLONE_REMOTE"`
	RcloneKeepDays int    `json:"RCLONE_KEEP_DAYS"`

	// 通知
	AppriseURL    string `json:"APPRISE_URL"`
	AppriseAPIURL string `json:"APPRISE_API_URL"`

	// 系统
	TZ               string `json:"TZ"`
	HTTPProxy        string `json:"HTTP_PROXY"`
	HTTPSProxy       string `json:"HTTPS_PROXY"`
	CreateTempBackup string `json:"CREATE_TEMP_BACKUP"`

	MinDiskSpaceMB int `json:"MIN_DISK_SPACE_MB"`
}

// ConfigField 配置表单字段描述
type ConfigField struct {
	Key     string
	Label   string
	Type    string
	Opts    [][2]string
	Default string
	Desc    string
	Value   string
}

// CONFIG_FIELDS 配置表单字段列表
var CONFIG_FIELDS = []ConfigField{
	{Key: "DB_TYPE", Label: "数据库类型", Type: "select",
		Opts:    [][2]string{{"sqlite", "SQLite"}, {"mysql", "MySQL"}, {"postgres", "PostgreSQL"}},
		Default: "sqlite", Desc: "Vaultwarden 使用的数据库类型"},
	{Key: "SQLITE_DB_FILE", Label: "SQLite 文件名", Type: "text",
		Default: "db.sqlite3", Desc: "SQLite 数据库文件名（默认 db.sqlite3）"},
	{Key: "DB_HOST", Label: "数据库主机", Type: "text", Default: "db",
		Desc: "MySQL/PostgreSQL 主机地址"},
	{Key: "DB_PORT", Label: "数据库端口", Type: "text", Default: "3306",
		Desc: "MySQL/PostgreSQL 端口"},
	{Key: "DB_USER", Label: "数据库用户名", Type: "text", Default: "",
		Desc: "MySQL/PostgreSQL 用户名"},
	{Key: "DB_PASSWORD", Label: "数据库密码", Type: "password", Default: "",
		Desc: "MySQL/PostgreSQL 密码"},
	{Key: "DB_NAME", Label: "数据库名称", Type: "text", Default: "vaultwarden",
		Desc: "MySQL/PostgreSQL 数据库名"},
	{Key: "BACKUP_PREFIX", Label: "备份文件前缀", Type: "text",
		Default: "vaultwarden_backup", Desc: "备份 ZIP 文件名前缀"},
	{Key: "DATA_DIR", Label: "数据目录", Type: "text", Default: "/data",
		Desc: "Vaultwarden 数据挂载路径"},
	{Key: "BACKUP_DIR", Label: "备份目录", Type: "text", Default: "/backup",
		Desc: "备份文件存放路径"},
	{Key: "ZIP_PASSWORD", Label: "ZIP 加密密码", Type: "password", Default: "",
		Desc: "留空则不加密"},
	{Key: "CRON_SCHEDULE", Label: "定时任务 (Cron)", Type: "text",
		Default: "0 2 * * *", Desc: "例如 0 2 * * * 每天凌晨2点"},
	{Key: "RUN_ON_STARTUP", Label: "启动时立即备份", Type: "select",
		Opts:    [][2]string{{"true", "启用"}, {"false", "禁用"}},
		Default: "true", Desc: "容器启动时立即执行一次备份"},
	{Key: "LOCAL_BACKUP_KEEP_DAYS", Label: "本地保留天数", Type: "number",
		Default: "15", Desc: "本地备份保留天数"},
	{Key: "RCLONE_REMOTE", Label: "Rclone 远程路径", Type: "text",
		Default: "", Desc: "例如 my_onedrive:/vaultwarden_backup"},
	{Key: "RCLONE_KEEP_DAYS", Label: "远端保留天数", Type: "number",
		Default: "", Desc: "远端备份保留天数，留空不清理"},
	{Key: "APPRISE_URL", Label: "Apprise 通知 URL", Type: "text",
		Default: "", Desc: "例如 tgram://bottoken/ChatID"},
	{Key: "APPRISE_API_URL", Label: "Apprise API 地址", Type: "text",
		Default: "", Desc: "例如 http://apprise:8000"},
	{Key: "TZ", Label: "时区", Type: "text", Default: "Asia/Shanghai",
		Desc: "容器时区"},
	{Key: "HTTP_PROXY", Label: "HTTP 代理", Type: "text", Default: "",
		Desc: "例如 http://192.168.1.100:7890"},
	{Key: "HTTPS_PROXY", Label: "HTTPS 代理", Type: "text", Default: "",
		Desc: "例如 http://192.168.1.100:7890"},
	{Key: "CREATE_TEMP_BACKUP", Label: "恢复前创建临时备份", Type: "select",
		Opts:    [][2]string{{"true", "启用"}, {"false", "禁用"}},
		Default: "true", Desc: "恢复前备份当前数据以便回滚"},
}

// WEB_CONFIG_FILE Web 面板配置文件路径
const WEB_CONFIG_FILE = "/app/config/config.json"
const WEB_LOG_FILE = "/app/config/backup.log"

// defaultWebConfig 返回默认配置
func defaultWebConfig() *WebConfig {
	return &WebConfig{
		DBType:           "sqlite",
		SQLiteDBFile:     "db.sqlite3",
		DBHost:           "db",
		DBPort:           "3306",
		DBName:           "vaultwarden",
		BackupPrefix:     "vaultwarden_backup",
		DataDir:          "/data",
		BackupDir:        "/backup",
		CronSchedule:     "0 2 * * *",
		RunOnStartup:     "true",
		LocalKeepDays:    15,
		CreateTempBackup: "true",
		TZ:               "Asia/Shanghai",
		MinDiskSpaceMB:   5120,
	}
}

// loadWebConfig 从 JSON 文件加载 Web 配置，空字段用环境变量兜底
func loadWebConfig() *WebConfig {
	wc := defaultWebConfig()
	data, err := os.ReadFile(WEB_CONFIG_FILE)
	if err != nil {
		loadEnvToWebConfig(wc)
		fixDBPort(wc)
		return wc
	}
	if err := json.Unmarshal(data, wc); err != nil {
		return defaultWebConfig()
	}
	fixDBPort(wc)
	return wc
}

// fixDBPort 根据数据库类型设置默认端口
func fixDBPort(wc *WebConfig) {
	if wc.DBPort == "" {
		switch wc.DBType {
		case "mysql":
			wc.DBPort = "3306"
		case "postgres":
			wc.DBPort = "5432"
		}
	}
}

// loadEnvToWebConfig 用环境变量填充 WebConfig 中的空字段
// 便于 docker-compose 中已有的环境变量直接生效
func loadEnvToWebConfig(wc *WebConfig) {
	setEnvStr := func(field *string, key string) {
		if v := os.Getenv(key); v != "" {
			*field = v
		}
	}
	setEnvInt := func(field *int, key string) {
		if v := os.Getenv(key); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				*field = n
			}
		}
	}

	setEnvStr(&wc.WebUser, "WEB_USER")
	setEnvStr(&wc.WebPass, "WEB_PASS")
	setEnvStr(&wc.DBType, "DB_TYPE")
	setEnvStr(&wc.SQLiteDBFile, "SQLITE_DB_FILE")
	setEnvStr(&wc.DBHost, "DB_HOST")
	setEnvStr(&wc.DBPort, "DB_PORT")
	setEnvStr(&wc.DBUser, "DB_USER")
	setEnvStr(&wc.DBPassword, "DB_PASSWORD")
	setEnvStr(&wc.DBName, "DB_NAME")
	setEnvStr(&wc.BackupPrefix, "BACKUP_PREFIX")
	setEnvStr(&wc.DataDir, "DATA_DIR")
	setEnvStr(&wc.BackupDir, "BACKUP_DIR")
	setEnvStr(&wc.ZipPassword, "ZIP_PASSWORD")
	setEnvStr(&wc.CronSchedule, "CRON_SCHEDULE")
	setEnvStr(&wc.RunOnStartup, "RUN_ON_STARTUP")
	setEnvInt(&wc.LocalKeepDays, "LOCAL_BACKUP_KEEP_DAYS")
	setEnvStr(&wc.RcloneRemote, "RCLONE_REMOTE")
	setEnvInt(&wc.RcloneKeepDays, "RCLONE_KEEP_DAYS")
	setEnvStr(&wc.AppriseURL, "APPRISE_URL")
	setEnvStr(&wc.AppriseAPIURL, "APPRISE_API_URL")
	setEnvStr(&wc.TZ, "TZ")
	setEnvStr(&wc.HTTPProxy, "HTTP_PROXY")
	setEnvStr(&wc.HTTPSProxy, "HTTPS_PROXY")
	setEnvStr(&wc.CreateTempBackup, "CREATE_TEMP_BACKUP")
	setEnvInt(&wc.MinDiskSpaceMB, "MIN_DISK_SPACE_MB")
}

// saveWebConfig 保存 Web 配置到 JSON 文件
func saveWebConfig(wc *WebConfig) error {
	dir := filepath.Dir(WEB_CONFIG_FILE)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(wc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(WEB_CONFIG_FILE, data, 0644)
}

// webConfigToCLIConfig 将 WebConfig 转为 CLI Config（用于调用 runBackup）
func webConfigToCLIConfig(wc *WebConfig) Config {
	lang := "zh"
	cfg := Config{
		DBType:         wc.DBType,
		SQLiteDBFile:   wc.SQLiteDBFile,
		DataDir:        wc.DataDir,
		BackupDir:      wc.BackupDir,
		ZipPassword:    wc.ZipPassword,
		AppriseURL:     wc.AppriseURL,
		AppriseAPIURL:  strings.TrimRight(wc.AppriseAPIURL, "/"),
		RcloneRemote:   wc.RcloneRemote,
		BackupPrefix:   wc.BackupPrefix,
		Lang:           lang,
		DBHost:         wc.DBHost,
		DBPort:         wc.DBPort,
		DBUser:         wc.DBUser,
		DBPassword:     wc.DBPassword,
		DBName:         wc.DBName,
		HTTPProxy:      wc.HTTPProxy,
		HTTPSProxy:     wc.HTTPSProxy,
		LocalKeepDays:  wc.LocalKeepDays,
		RcloneKeepDays: wc.RcloneKeepDays,
		MinDiskSpaceMB: wc.MinDiskSpaceMB,
	}
	if cfg.DBPort == "" {
		switch cfg.DBType {
		case "mysql":
			cfg.DBPort = "3306"
		case "postgres":
			cfg.DBPort = "5432"
		}
	}
	return cfg
}

// ============================================================
// 模板数据
// ============================================================

// BackupFile 备份文件信息
type BackupFile struct {
	Name  string
	Size  string
	Mtime string
}

// DashboardData 仪表盘模板数据
type DashboardData struct {
	Backups []BackupFile
	Msg     string
}

// ConfigPageData 配置页模板数据
type ConfigPageData struct {
	Fields []ConfigField
	Ok     string
}

// AuthPageData 认证页面模板数据
type AuthPageData struct {
	Error string
}

// ============================================================
// 模板函数
// ============================================================

// templateFuncs 模板自定义函数
var templateFuncs = template.FuncMap{
	"substr": func(s string, start, end int) string {
		if start < 0 {
			start = 0
		}
		if end > len(s) {
			end = len(s)
		}
		if start >= end {
			return ""
		}
		return s[start:end]
	},
	"contains": strings.Contains,
}

// ============================================================
// 认证
// ============================================================

// hashToken 计算认证令牌
func hashToken(username, password string) string {
	h := sha256.Sum256([]byte(username + password))
	return fmt.Sprintf("%x", h)
}

// verifyWebAuth 验证 Web 认证
func verifyWebAuth(w http.ResponseWriter, r *http.Request) bool {
	wc := loadWebConfig()
	if wc.WebUser == "" || wc.WebPass == "" {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return false
	}
	token := ""
	if c, err := r.Cookie("auth_token"); err == nil {
		token = c.Value
	}
	if token == "" || token != hashToken(wc.WebUser, wc.WebPass) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return false
	}
	return true
}

// ============================================================
// 模板渲染
// ============================================================

var templates *template.Template

func initTemplates() {
	var err error
	templates, err = template.New("").Funcs(templateFuncs).ParseFS(templateFS, "templates/*.gohtml")
	if err != nil {
		log.Printf("模板解析失败: %v", err)
	}
}

func renderTemplate(w http.ResponseWriter, name string, data interface{}) {
	if templates == nil {
		initTemplates()
	}
	if err := templates.ExecuteTemplate(w, name+".gohtml", data); err != nil {
		log.Printf("模板渲染失败 %s: %v", name, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// ============================================================
// Web 服务入口
// ============================================================

var (
	cronMu       sync.Mutex
	cronTicker   *time.Ticker
	cronStopChan chan struct{}
)

// cronMatcher 简单 cron 表达式匹配（分 时 日 月 周）
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

// startCronScheduler 启动定时任务调度器
func startCronScheduler() {
	cronMu.Lock()
	defer cronMu.Unlock()

	if cronTicker != nil {
		cronTicker.Stop()
		if cronStopChan != nil {
			close(cronStopChan)
		}
	}

	cronStopChan = make(chan struct{})
	cronTicker = time.NewTicker(30 * time.Second)

	go func() {
		wc := loadWebConfig()
		if wc.RunOnStartup == "true" {
			go func() {
				log.Println("启动时备份...")
				cfg := webConfigToCLIConfig(wc)
				runBackup(cfg)
				log.Println("启动时备份完成")
			}()
		}

		for {
			select {
			case <-cronStopChan:
				return
			case t := <-cronTicker.C:
				wc := loadWebConfig()
				if wc.CronSchedule == "" {
					continue
				}
				if cronMatcher(wc.CronSchedule, t) {
					cfg := webConfigToCLIConfig(wc)
					log.Printf("触发定时备份 (cron: %s)", wc.CronSchedule)
					runBackup(cfg)
				}
			}
		}
	}()
}

// ============================================================
// HTTP 路由处理
// ============================================================

// runWebServer 启动 Web 服务
func runWebServer() {
	initTemplates()

	mux := http.NewServeMux()

	// 认证相关
	mux.HandleFunc("/login", handleLogin)
	mux.HandleFunc("/do_login", handleDoLogin)
	mux.HandleFunc("/logout", handleLogout)
	mux.HandleFunc("/setup", handleSetup)
	mux.HandleFunc("/do_setup", handleDoSetup)

	// 需认证的路由
	mux.HandleFunc("/", handleDashboard)
	mux.HandleFunc("/config", handleConfig)
	mux.HandleFunc("/save_config", handleSaveConfig)
	mux.HandleFunc("/do_backup", handleDoBackup)
	mux.HandleFunc("/do_restore", handleDoRestore)
	mux.HandleFunc("/do_upload_restore", handleDoUploadRestore)
	mux.HandleFunc("/logs", handleLogs)
	mux.HandleFunc("/health", handleHealth)

	startCronScheduler()

	log.Println("Vaultwarden Backup Web 面板启动，监听端口 :9876")
	log.Println("访问 http://0.0.0.0:9876 打开管理面板")
	if err := http.ListenAndServe(":9876", mux); err != nil {
		log.Fatalf("Web 服务启动失败: %v", err)
	}
}

// ============================================================
// 路由处理器
// ============================================================

// handleLogin 登录页
func handleLogin(w http.ResponseWriter, r *http.Request) {
	wc := loadWebConfig()
	if wc.WebUser == "" || wc.WebPass == "" {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}
	error := r.URL.Query().Get("error")
	renderTemplate(w, "login", AuthPageData{Error: error})
}

// handleDoLogin 处理登录
func handleDoLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	r.ParseForm()
	username := r.FormValue("username")
	password := r.FormValue("password")

	wc := loadWebConfig()
	if username == wc.WebUser && password == wc.WebPass {
		token := hashToken(username, password)
		http.SetCookie(w, &http.Cookie{
			Name:     "auth_token",
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			MaxAge:   86400 * 30,
		})
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/login?error=用户名或密码错误", http.StatusSeeOther)
}

// handleLogout 退出登录
func handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:   "auth_token",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// handleSetup 初始化页
func handleSetup(w http.ResponseWriter, r *http.Request) {
	wc := loadWebConfig()
	if wc.WebUser != "" && wc.WebPass != "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	error := r.URL.Query().Get("error")
	renderTemplate(w, "setup", AuthPageData{Error: error})
}

// handleDoSetup 处理初始化
func handleDoSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}
	r.ParseForm()
	webUser := r.FormValue("web_user")
	webPass := r.FormValue("web_pass")

	if webUser == "" || webPass == "" {
		http.Redirect(w, r, "/setup?error=用户名和密码不能为空", http.StatusSeeOther)
		return
	}

	wc := loadWebConfig()
	wc.WebUser = webUser
	wc.WebPass = webPass
	if err := saveWebConfig(wc); err != nil {
		http.Redirect(w, r, "/setup?error=保存配置失败", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// handleDashboard 仪表盘
func handleDashboard(w http.ResponseWriter, r *http.Request) {
	if !verifyWebAuth(w, r) {
		return
	}
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	wc := loadWebConfig()
	backupDir := wc.BackupDir
	prefix := wc.BackupPrefix
	msg := r.URL.Query().Get("msg")

	backups := []BackupFile{}
	if info, err := os.Stat(backupDir); err == nil && info.IsDir() {
		entries, err := os.ReadDir(backupDir)
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) && strings.HasSuffix(entry.Name(), ".zip") {
					fi, err := entry.Info()
					if err != nil {
						continue
					}
					size := fi.Size()
					sizeStr := fmt.Sprintf("%.1f KB", float64(size)/1024)
					if size >= 1024*1024 {
						sizeStr = fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
					}
					backups = append(backups, BackupFile{
						Name:  entry.Name(),
						Size:  sizeStr,
						Mtime: fi.ModTime().Format("2006-01-02 15:04:05"),
					})
				}
			}
		}
	}
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Mtime > backups[j].Mtime
	})
	if len(backups) > 30 {
		backups = backups[:30]
	}

	renderTemplate(w, "dashboard", DashboardData{Backups: backups, Msg: msg})
}

// handleConfig 配置页
func handleConfig(w http.ResponseWriter, r *http.Request) {
	if !verifyWebAuth(w, r) {
		return
	}

	wc := loadWebConfig()
	fields := make([]ConfigField, len(CONFIG_FIELDS))
	for i, f := range CONFIG_FIELDS {
		field := f
		val := ""
		switch f.Key {
		case "DB_TYPE":
			val = wc.DBType
		case "SQLITE_DB_FILE":
			val = wc.SQLiteDBFile
		case "DB_HOST":
			val = wc.DBHost
		case "DB_PORT":
			val = wc.DBPort
		case "DB_USER":
			val = wc.DBUser
		case "DB_PASSWORD":
			val = wc.DBPassword
		case "DB_NAME":
			val = wc.DBName
		case "BACKUP_PREFIX":
			val = wc.BackupPrefix
		case "DATA_DIR":
			val = wc.DataDir
		case "BACKUP_DIR":
			val = wc.BackupDir
		case "ZIP_PASSWORD":
			val = wc.ZipPassword
		case "CRON_SCHEDULE":
			val = wc.CronSchedule
		case "RUN_ON_STARTUP":
			val = wc.RunOnStartup
		case "LOCAL_BACKUP_KEEP_DAYS":
			val = strconv.Itoa(wc.LocalKeepDays)
		case "RCLONE_REMOTE":
			val = wc.RcloneRemote
		case "RCLONE_KEEP_DAYS":
			if wc.RcloneKeepDays > 0 {
				val = strconv.Itoa(wc.RcloneKeepDays)
			}
		case "APPRISE_URL":
			val = wc.AppriseURL
		case "APPRISE_API_URL":
			val = wc.AppriseAPIURL
		case "TZ":
			val = wc.TZ
		case "HTTP_PROXY":
			val = wc.HTTPProxy
		case "HTTPS_PROXY":
			val = wc.HTTPSProxy
		case "CREATE_TEMP_BACKUP":
			val = wc.CreateTempBackup
		case "MIN_DISK_SPACE_MB":
			val = strconv.Itoa(wc.MinDiskSpaceMB)
		}
		if val == "" {
			val = f.Default
		}
		field.Value = val
		fields[i] = field
	}

	ok := r.URL.Query().Get("ok")
	renderTemplate(w, "config", ConfigPageData{Fields: fields, Ok: ok})
}

// handleSaveConfig 保存配置
func handleSaveConfig(w http.ResponseWriter, r *http.Request) {
	if !verifyWebAuth(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/config", http.StatusSeeOther)
		return
	}
	r.ParseForm()

	wc := loadWebConfig()
	wc.DBType = r.FormValue("DB_TYPE")
	wc.SQLiteDBFile = r.FormValue("SQLITE_DB_FILE")
	wc.DBHost = r.FormValue("DB_HOST")
	wc.DBPort = r.FormValue("DB_PORT")
	wc.DBUser = r.FormValue("DB_USER")
	wc.DBPassword = r.FormValue("DB_PASSWORD")
	wc.DBName = r.FormValue("DB_NAME")
	wc.BackupPrefix = r.FormValue("BACKUP_PREFIX")
	wc.DataDir = r.FormValue("DATA_DIR")
	wc.BackupDir = r.FormValue("BACKUP_DIR")
	wc.ZipPassword = r.FormValue("ZIP_PASSWORD")
	wc.CronSchedule = r.FormValue("CRON_SCHEDULE")
	wc.RunOnStartup = r.FormValue("RUN_ON_STARTUP")
	wc.LocalKeepDays, _ = strconv.Atoi(r.FormValue("LOCAL_BACKUP_KEEP_DAYS"))
	wc.RcloneRemote = r.FormValue("RCLONE_REMOTE")
	wc.RcloneKeepDays, _ = strconv.Atoi(r.FormValue("RCLONE_KEEP_DAYS"))
	wc.AppriseURL = r.FormValue("APPRISE_URL")
	wc.AppriseAPIURL = r.FormValue("APPRISE_API_URL")
	wc.TZ = r.FormValue("TZ")
	wc.HTTPProxy = r.FormValue("HTTP_PROXY")
	wc.HTTPSProxy = r.FormValue("HTTPS_PROXY")
	wc.CreateTempBackup = r.FormValue("CREATE_TEMP_BACKUP")

	if wc.DBPort == "" {
		switch wc.DBType {
		case "mysql":
			wc.DBPort = "3306"
		case "postgres":
			wc.DBPort = "5432"
		}
	}

	if err := saveWebConfig(wc); err != nil {
		log.Printf("保存配置失败: %v", err)
		http.Error(w, "保存配置失败", http.StatusInternalServerError)
		return
	}

	startCronScheduler()
	http.Redirect(w, r, "/config?ok=1", http.StatusSeeOther)
}

// handleDoBackup 立即执行备份
func handleDoBackup(w http.ResponseWriter, r *http.Request) {
	if !verifyWebAuth(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	wc := loadWebConfig()
	cfg := webConfigToCLIConfig(wc)

	go func() {
		log.Println("Web 面板触发手动备份...")
		runBackup(cfg)
		log.Println("手动备份完成")
	}()

	http.Redirect(w, r, "/?msg=备份任务已启动，请查看日志了解进度", http.StatusSeeOther)
}

// handleDoRestore 恢复备份
func handleDoRestore(w http.ResponseWriter, r *http.Request) {
	if !verifyWebAuth(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	r.ParseForm()
	backupFile := r.FormValue("backup_file")
	if backupFile == "" {
		http.Error(w, "未指定备份文件", http.StatusBadRequest)
		return
	}

	wc := loadWebConfig()
	backupPath := filepath.Join(wc.BackupDir, filepath.Base(backupFile))
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		http.Error(w, "备份文件不存在", http.StatusNotFound)
		return
	}

	err := doRestoreInternal(wc, backupPath)
	if err != nil {
		http.Redirect(w, r, "/?msg=恢复失败（已回滚）: "+err.Error(), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/?msg=恢复成功", http.StatusSeeOther)
}

// handleDoUploadRestore 上传并恢复
func handleDoUploadRestore(w http.ResponseWriter, r *http.Request) {
	if !verifyWebAuth(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	r.ParseMultipartForm(100 << 20)
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "读取上传文件失败", http.StatusBadRequest)
		return
	}
	defer file.Close()

	if !strings.HasSuffix(header.Filename, ".zip") {
		http.Error(w, "请上传 .zip 格式的备份文件", http.StatusBadRequest)
		return
	}

	wc := loadWebConfig()
	savePath := filepath.Join(wc.BackupDir, "_upload_"+header.Filename)
	os.MkdirAll(wc.BackupDir, 0755)

	dst, err := os.Create(savePath)
	if err != nil {
		http.Error(w, "保存上传文件失败", http.StatusInternalServerError)
		return
	}
	defer dst.Close()
	if _, err := io.Copy(dst, file); err != nil {
		os.Remove(savePath)
		http.Error(w, "保存上传文件失败", http.StatusInternalServerError)
		return
	}
	dst.Close()

	err = doRestoreInternal(wc, savePath)
	os.Remove(savePath)
	if err != nil {
		http.Redirect(w, r, "/?msg=从上传文件恢复失败（已回滚）: "+err.Error(), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/?msg=从上传文件恢复成功", http.StatusSeeOther)
}

// handleLogs 获取日志
func handleLogs(w http.ResponseWriter, r *http.Request) {
	if !verifyWebAuth(w, r) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := os.Stat(WEB_LOG_FILE); os.IsNotExist(err) {
		fmt.Fprintf(w, `{"logs": "暂无日志"}`)
		return
	}

	data, err := os.ReadFile(WEB_LOG_FILE)
	if err != nil {
		fmt.Fprintf(w, `{"logs": "读取日志失败"}`)
		return
	}
	content := string(data)
	if len(content) > 5000 {
		content = content[len(content)-5000:]
	}
	// 使用 json.Marshal 来安全转义
	resp := map[string]string{"logs": content}
	jsonData, _ := json.Marshal(resp)
	w.Write(jsonData)
}

// handleHealth 健康检查
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status": "ok"}`)
}

// ============================================================
// 恢复逻辑
// ============================================================

// doRestoreInternal 执行恢复操作
func doRestoreInternal(wc *WebConfig, backupPath string) error {
	dataDir := wc.DataDir
	zipPassword := wc.ZipPassword
	dbType := wc.DBType
	sqliteDBFile := wc.SQLiteDBFile
	createTemp := wc.CreateTempBackup == "true"

	tempBackup := filepath.Join(dataDir, "_restore_undo")

	exec.Command("docker", "stop", "vaultwarden").Run()

	if createTemp {
		os.RemoveAll(tempBackup)
		os.MkdirAll(tempBackup, 0755)
		if entries, err := os.ReadDir(dataDir); err == nil {
			for _, entry := range entries {
				if entry.Name() != "_restore_undo" {
					src := filepath.Join(dataDir, entry.Name())
					dst := filepath.Join(tempBackup, entry.Name())
					os.Rename(src, dst)
				}
			}
		}
	}

	unzipArgs := []string{"-o", backupPath, "-d", dataDir}
	if zipPassword != "" {
		unzipArgs = []string{"-P", zipPassword, "-o", backupPath, "-d", dataDir}
	}
	if _, err := runCmd("unzip", unzipArgs...); err != nil {
		if createTemp {
			restoreUndo(dataDir, tempBackup)
		}
		exec.Command("docker", "start", "vaultwarden").Run()
		return fmt.Errorf("解压失败: %w", err)
	}

	if dbType == "sqlite" {
		tempDB := filepath.Join(dataDir, "db_dump_temp.db")
		if _, err := os.Stat(tempDB); err == nil {
			target := filepath.Join(dataDir, sqliteDBFile)
			for _, f := range []string{target + "-wal", target + "-shm"} {
				os.Remove(f)
			}
			os.Rename(tempDB, target)
		}
	}

	exec.Command("docker", "start", "vaultwarden").Run()
	os.RemoveAll(tempBackup)
	return nil
}

// restoreUndo 回滚数据
func restoreUndo(dataDir, tempBackup string) {
	if entries, err := os.ReadDir(tempBackup); err == nil {
		for _, entry := range entries {
			oldPath := filepath.Join(dataDir, entry.Name())
			os.RemoveAll(oldPath)
			os.Rename(filepath.Join(tempBackup, entry.Name()), oldPath)
		}
	}
	os.RemoveAll(tempBackup)
}
