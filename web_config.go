package main

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ============================================================
// ??????
// ============================================================

// Option ?? select ?????
type Option struct {
	Value    string
	Label    string
	Selected bool
}

// ConfigField ????????
type ConfigField struct {
	Key     string
	Label   string
	Type    string
	Opts    []Option
	Value   string
	Default string
	Desc    string
}

// ConfigGroup ????
type ConfigGroup struct {
	GroupID   string
	GroupName string
	Icon      string
	Fields    []ConfigField
}

// ConfigPageData ?????????
type ConfigPageData struct {
	Groups []ConfigGroup
	Ok     string
}

// ConfigFile ?? YAML ???????
type ConfigFile struct {
	WebUser string `yaml:"WEB_USER"`
	WebPass string `yaml:"WEB_PASS"`
	// ???? - ?????????
	DBType         string `yaml:"DB_TYPE"`
	SQLiteDBFile   string `yaml:"SQLITE_DB_FILE"`
	DBHost         string `yaml:"DB_HOST"`
	DBPort         string `yaml:"DB_PORT"`
	DBUser         string `yaml:"DB_USER"`
	DBPassword     string `yaml:"DB_PASSWORD"`
	DBName         string `yaml:"DB_NAME"`
	BackupPrefix   string `yaml:"BACKUP_PREFIX"`
	DataDir        string `yaml:"DATA_DIR"`
	BackupDir      string `yaml:"BACKUP_DIR"`
	ZipPassword    string `yaml:"ZIP_PASSWORD"`
	CronSchedule   string `yaml:"CRON_SCHEDULE"`
	RunOnStartup   string `yaml:"RUN_ON_STARTUP"`
	LocalKeepDays  string `yaml:"LOCAL_BACKUP_KEEP_DAYS"`
	RcloneRemote   string `yaml:"RCLONE_REMOTE"`
	RcloneKeepDays string `yaml:"RCLONE_KEEP_DAYS"`
	AppriseURL     string `yaml:"APPRISE_URL"`
	AppriseAPIURL  string `yaml:"APPRISE_API_URL"`
	TZ             string `yaml:"TZ"`
	HTTPProxy      string `yaml:"HTTP_PROXY"`
	HTTPSProxy     string `yaml:"HTTPS_PROXY"`
	StartupDelay   string `yaml:"STARTUP_DELAY"`
	MinDiskSpaceMB string `yaml:"MIN_DISK_SPACE_MB"`
}

// webConfigPath ? webLogPath ??????????
var webConfigPath = "/config/config.yaml"
var webLogPath = "/config/backup.log"

func init() {
	if v := os.Getenv("WEB_CONFIG_PATH"); v != "" {
		webConfigPath = v
	}
	if v := os.Getenv("WEB_LOG_PATH"); v != "" {
		webLogPath = v
	}
}

func getWebConfigPath() string {
	return webConfigPath
}

func getWebLogPath() string {
	return webLogPath
}

// ============================================================
// ??????????????
// ============================================================

var configFieldDefs = []ConfigField{
	{Key: "DB_TYPE", Label: "数据库类型", Type: "select",
		Opts:    []Option{{Value: "sqlite", Label: "SQLite"}, {Value: "mysql", Label: "MySQL"}, {Value: "postgres", Label: "PostgreSQL"}},
		Default: "sqlite", Desc: "Vaultwarden 使用的数据库类型"},
	{Key: "SQLITE_DB_FILE", Label: "SQLite 文件名", Type: "text", Default: "db.sqlite3", Desc: "SQLite 数据库文件名"},
	{Key: "DB_HOST", Label: "数据库主机", Type: "text", Default: "db", Desc: "MySQL/PostgreSQL 主机地址"},
	{Key: "DB_PORT", Label: "数据库端口", Type: "text", Default: "3306", Desc: "MySQL/PostgreSQL 端口"},
	{Key: "DB_USER", Label: "数据库用户名", Type: "text", Default: "", Desc: "MySQL/PostgreSQL 用户名"},
	{Key: "DB_PASSWORD", Label: "数据库密码", Type: "password", Default: "", Desc: "MySQL/PostgreSQL 密码"},
	{Key: "DB_NAME", Label: "数据库名称", Type: "text", Default: "vaultwarden", Desc: "MySQL/PostgreSQL 数据库名"},
	{Key: "BACKUP_PREFIX", Label: "备份文件前缀", Type: "text", Default: "vaultwarden_backup", Desc: "备份 ZIP 文件名前缀"},
	{Key: "DATA_DIR", Label: "数据目录", Type: "text", Default: "/data", Desc: "Vaultwarden 数据挂载路径"},
	{Key: "BACKUP_DIR", Label: "备份目录", Type: "text", Default: "/backup", Desc: "备份文件存放路径"},
	{Key: "ZIP_PASSWORD", Label: "ZIP 加密密码", Type: "password", Default: "", Desc: "留空则不加密"},
	{Key: "CRON_SCHEDULE", Label: "定时任务 (Cron)", Type: "text", Default: "0 2 * * *", Desc: "例如 0 2 * * * 每天凌晨2点"},
	{Key: "RUN_ON_STARTUP", Label: "启动时立即备份", Type: "select",
		Opts:    []Option{{Value: "true", Label: "启用"}, {Value: "false", Label: "禁用"}},
		Default: "true", Desc: "容器启动时立即执行一次备份"},
	{Key: "STARTUP_DELAY", Label: "启动延迟(秒)", Type: "number", Default: "15", Desc: "启动后等待N秒再执行备份"},
	{Key: "LOCAL_BACKUP_KEEP_DAYS", Label: "本地保留天数", Type: "number", Default: "15", Desc: "本地备份文件保留天数"},
	{Key: "MIN_DISK_SPACE_MB", Label: "磁盘空间阈值(MB)", Type: "number", Default: "5120", Desc: "低于此值则跳过备份"},
	{Key: "RCLONE_REMOTE", Label: "Rclone 远程路径", Type: "text", Default: "", Desc: "例如 my_onedrive:/vaultwarden_backup"},
	{Key: "RCLONE_KEEP_DAYS", Label: "远端保留天数", Type: "number", Default: "", Desc: "远端备份保留天数，留空不清理"},
	{Key: "TZ", Label: "时区", Type: "text", Default: "Asia/Shanghai", Desc: "容器时区设置"},
	{Key: "APPRISE_URL", Label: "Apprise 通知 URL", Type: "text", Default: "", Desc: "例如 tgram://bottoken/ChatID"},
	{Key: "APPRISE_API_URL", Label: "Apprise API 地址", Type: "text", Default: "", Desc: "例如 http://apprise:8000"},
	{Key: "HTTP_PROXY", Label: "HTTP 代理", Type: "text", Default: "", Desc: "例如 http://192.168.1.100:7890"},
	{Key: "HTTPS_PROXY", Label: "HTTPS 代理", Type: "text", Default: "", Desc: "例如 http://192.168.1.100:7890"},
}

// ============================================================
// YAML ????
// ============================================================

// loadYamlConfig ? YAML ??????
func loadYamlConfig() (*ConfigFile, error) {
	cfg := &ConfigFile{}
	data, err := os.ReadFile(getWebConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// saveYamlConfig ????? YAML ?????????
func saveYamlConfig(cfg *ConfigFile) error {
	dir := filepath.Dir(getWebConfigPath())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.WriteFile(getWebConfigPath(), data, 0644); err != nil {
		return err
	}
	syncConfigToEnv(cfg)
	return nil
}

// syncConfigToEnv ? YAML ?????????
func syncConfigToEnv(cfg *ConfigFile) {
	setEnv := func(k, v string) {
		if v != "" {
			os.Setenv(k, v)
		}
	}
	setEnv("DB_TYPE", cfg.DBType)
	setEnv("SQLITE_DB_FILE", cfg.SQLiteDBFile)
	setEnv("DB_HOST", cfg.DBHost)
	setEnv("DB_PORT", cfg.DBPort)
	setEnv("DB_USER", cfg.DBUser)
	setEnv("DB_PASSWORD", cfg.DBPassword)
	setEnv("DB_NAME", cfg.DBName)
	setEnv("BACKUP_PREFIX", cfg.BackupPrefix)
	setEnv("DATA_DIR", cfg.DataDir)
	setEnv("BACKUP_DIR", cfg.BackupDir)
	setEnv("ZIP_PASSWORD", cfg.ZipPassword)
	setEnv("CRON_SCHEDULE", cfg.CronSchedule)
	setEnv("RUN_ON_STARTUP", cfg.RunOnStartup)
	setEnv("STARTUP_DELAY", cfg.StartupDelay)
	setEnv("LOCAL_BACKUP_KEEP_DAYS", cfg.LocalKeepDays)
	setEnv("MIN_DISK_SPACE_MB", cfg.MinDiskSpaceMB)
	setEnv("RCLONE_REMOTE", cfg.RcloneRemote)
	setEnv("RCLONE_KEEP_DAYS", cfg.RcloneKeepDays)
	setEnv("APPRISE_URL", cfg.AppriseURL)
	setEnv("APPRISE_API_URL", cfg.AppriseAPIURL)
	setEnv("HTTP_PROXY", cfg.HTTPProxy)
	setEnv("TZ", cfg.TZ)
	setEnv("HTTPS_PROXY", cfg.HTTPSProxy)
}

// getConfigValue ? YAML ????????????
func getConfigValue(cfg *ConfigFile, key string) string {
	if cfg != nil {
		v := getConfigValueFromFile(cfg, key)
		if v != "" {
			return v
		}
	}
	return os.Getenv(key)
}

func getConfigValueFromFile(cfg *ConfigFile, key string) string {
	switch key {
	case "DB_TYPE":
		return cfg.DBType
	case "SQLITE_DB_FILE":
		return cfg.SQLiteDBFile
	case "DB_HOST":
		return cfg.DBHost
	case "DB_PORT":
		return cfg.DBPort
	case "DB_USER":
		return cfg.DBUser
	case "DB_PASSWORD":
		return cfg.DBPassword
	case "DB_NAME":
		return cfg.DBName
	case "BACKUP_PREFIX":
		return cfg.BackupPrefix
	case "DATA_DIR":
		return cfg.DataDir
	case "BACKUP_DIR":
		return cfg.BackupDir
	case "ZIP_PASSWORD":
		return cfg.ZipPassword
	case "CRON_SCHEDULE":
		return cfg.CronSchedule
	case "RUN_ON_STARTUP":
		return cfg.RunOnStartup
	case "STARTUP_DELAY":
		return cfg.StartupDelay
	case "LOCAL_BACKUP_KEEP_DAYS":
		return cfg.LocalKeepDays
	case "MIN_DISK_SPACE_MB":
		return cfg.MinDiskSpaceMB
	case "RCLONE_REMOTE":
		return cfg.RcloneRemote
	case "RCLONE_KEEP_DAYS":
		return cfg.RcloneKeepDays
	case "APPRISE_URL":
		return cfg.AppriseURL
	case "APPRISE_API_URL":
		return cfg.AppriseAPIURL
	case "HTTP_PROXY":
		return cfg.HTTPProxy
	case "HTTPS_PROXY":
		return cfg.HTTPSProxy
	}
	return ""
}

func setConfigValueByKey(cfg *ConfigFile, key, value string) {
	switch key {
	case "DB_TYPE":
		cfg.DBType = value
	case "SQLITE_DB_FILE":
		cfg.SQLiteDBFile = value
	case "DB_HOST":
		cfg.DBHost = value
	case "DB_PORT":
		cfg.DBPort = value
	case "DB_USER":
		cfg.DBUser = value
	case "DB_PASSWORD":
		cfg.DBPassword = value
	case "DB_NAME":
		cfg.DBName = value
	case "BACKUP_PREFIX":
		cfg.BackupPrefix = value
	case "DATA_DIR":
		cfg.DataDir = value
	case "BACKUP_DIR":
		cfg.BackupDir = value
	case "ZIP_PASSWORD":
		cfg.ZipPassword = value
	case "CRON_SCHEDULE":
		cfg.CronSchedule = value
	case "RUN_ON_STARTUP":
		cfg.RunOnStartup = value
	case "STARTUP_DELAY":
		cfg.StartupDelay = value
	case "LOCAL_BACKUP_KEEP_DAYS":
		cfg.LocalKeepDays = value
	case "MIN_DISK_SPACE_MB":
		cfg.MinDiskSpaceMB = value
	case "RCLONE_REMOTE":
		cfg.RcloneRemote = value
	case "RCLONE_KEEP_DAYS":
		cfg.RcloneKeepDays = value
	case "APPRISE_URL":
		cfg.AppriseURL = value
	case "APPRISE_API_URL":
		cfg.AppriseAPIURL = value
	case "HTTP_PROXY":
		cfg.HTTPProxy = value
	case "TZ":
		cfg.TZ = value
	case "HTTPS_PROXY":
		cfg.HTTPSProxy = value
	}
}

// ============================================================
// ????????????
// ============================================================

func buildConfigGroups(cfg *ConfigFile) []ConfigGroup {
	getVal := func(key string) string {
		return getConfigValue(cfg, key)
	}

	makeField := func(def ConfigField) ConfigField {
		f := def
		f.Value = getVal(def.Key)
		if f.Value == "" {
			f.Value = f.Default
		}
		if f.Type == "select" && len(f.Opts) > 0 {
			updated := make([]Option, len(f.Opts))
			for i, opt := range f.Opts {
				opt.Selected = opt.Value == f.Value
				updated[i] = opt
			}
			f.Opts = updated
		}
		return f
	}

	dbFields := []ConfigField{}
	pathFields := []ConfigField{}
	cronFields := []ConfigField{}
	cloudFields := []ConfigField{}

	dbKeys := map[string]bool{"DB_TYPE": true, "SQLITE_DB_FILE": true, "DB_HOST": true, "DB_PORT": true, "DB_USER": true, "DB_PASSWORD": true, "DB_NAME": true}
	pathKeys := map[string]bool{"BACKUP_PREFIX": true, "DATA_DIR": true, "BACKUP_DIR": true, "ZIP_PASSWORD": true}
	cronKeys := map[string]bool{"CRON_SCHEDULE": true, "RUN_ON_STARTUP": true, "STARTUP_DELAY": true, "LOCAL_BACKUP_KEEP_DAYS": true, "MIN_DISK_SPACE_MB": true, "TZ": true}
	cloudKeys := map[string]bool{"RCLONE_REMOTE": true, "RCLONE_KEEP_DAYS": true, "APPRISE_URL": true, "APPRISE_API_URL": true, "HTTP_PROXY": true, "HTTPS_PROXY": true}

	for _, def := range configFieldDefs {
		f := makeField(def)
		if dbKeys[f.Key] {
			dbFields = append(dbFields, f)
		} else if pathKeys[f.Key] {
			pathFields = append(pathFields, f)
		} else if cronKeys[f.Key] {
			cronFields = append(cronFields, f)
		} else if cloudKeys[f.Key] {
			cloudFields = append(cloudFields, f)
		}
	}

	return []ConfigGroup{
		{GroupID: "database", GroupName: "数据库设置", Icon: "bi-database", Fields: dbFields},
		{GroupID: "path", GroupName: "路径与加密", Icon: "bi-folder", Fields: pathFields},
		{GroupID: "schedule", GroupName: "定时计划", Icon: "bi-clock", Fields: cronFields},
		{GroupID: "cloud", GroupName: "云端与通知", Icon: "bi-cloud", Fields: cloudFields},
	}
}

func getCurrentConfigValue(key string) string {
	cfg, err := loadYamlConfig()
	if err == nil {
		return getConfigValue(cfg, key)
	}
	return os.Getenv(key)
}

func parseFormToConfig(form map[string]string) *ConfigFile {
	cfg := &ConfigFile{}
	for k, v := range form {
		switch k {
		case "WEB_USER":
			cfg.WebUser = v
		case "WEB_PASS":
			cfg.WebPass = v
		default:
			setConfigValueByKey(cfg, k, v)
		}
	}
	if cfg.WebUser == "" || cfg.WebPass == "" {
		existing, err := loadYamlConfig()
		if err == nil {
			if cfg.WebUser == "" {
				cfg.WebUser = existing.WebUser
			}
			if cfg.WebPass == "" {
				cfg.WebPass = existing.WebPass
			}
		}
	}
	return cfg
}
