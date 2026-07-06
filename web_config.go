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
	WebUser  string `yaml:"WEB_USER"`
	WebPass  string `yaml:"WEB_PASS"`
	// ???? - ?????????
	DBType           string `yaml:"DB_TYPE"`
	SQLiteDBFile     string `yaml:"SQLITE_DB_FILE"`
	DBHost           string `yaml:"DB_HOST"`
	DBPort           string `yaml:"DB_PORT"`
	DBUser           string `yaml:"DB_USER"`
	DBPassword       string `yaml:"DB_PASSWORD"`
	DBName           string `yaml:"DB_NAME"`
	BackupPrefix     string `yaml:"BACKUP_PREFIX"`
	DataDir          string `yaml:"DATA_DIR"`
	BackupDir        string `yaml:"BACKUP_DIR"`
	ZipPassword      string `yaml:"ZIP_PASSWORD"`
	CronSchedule     string `yaml:"CRON_SCHEDULE"`
	RunOnStartup     string `yaml:"RUN_ON_STARTUP"`
	LocalKeepDays    string `yaml:"LOCAL_BACKUP_KEEP_DAYS"`
	RcloneRemote     string `yaml:"RCLONE_REMOTE"`
	RcloneKeepDays   string `yaml:"RCLONE_KEEP_DAYS"`
	AppriseURL       string `yaml:"APPRISE_URL"`
	AppriseAPIURL    string `yaml:"APPRISE_API_URL"`
	TZ               string `yaml:"TZ"`
	HTTPProxy        string `yaml:"HTTP_PROXY"`
	HTTPSProxy       string `yaml:"HTTPS_PROXY"`
	StartupDelay     string `yaml:"STARTUP_DELAY"`
	MinDiskSpaceMB   string `yaml:"MIN_DISK_SPACE_MB"`
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
	{Key: "DB_TYPE", Label: "?????", Type: "select",
		Opts:    []Option{{Value: "sqlite", Label: "SQLite"}, {Value: "mysql", Label: "MySQL"}, {Value: "postgres", Label: "PostgreSQL"}},
		Default: "sqlite", Desc: "Vaultwarden ????????"},
	{Key: "SQLITE_DB_FILE", Label: "SQLite ???", Type: "text", Default: "db.sqlite3", Desc: "SQLite ??????"},
	{Key: "DB_HOST", Label: "?????", Type: "text", Default: "db", Desc: "MySQL/PostgreSQL ????"},
	{Key: "DB_PORT", Label: "?????", Type: "text", Default: "3306", Desc: "MySQL/PostgreSQL ??"},
	{Key: "DB_USER", Label: "??????", Type: "text", Default: "", Desc: "MySQL/PostgreSQL ???"},
	{Key: "DB_PASSWORD", Label: "?????", Type: "password", Default: "", Desc: "MySQL/PostgreSQL ??"},
	{Key: "DB_NAME", Label: "?????", Type: "text", Default: "vaultwarden", Desc: "MySQL/PostgreSQL ????"},
	{Key: "BACKUP_PREFIX", Label: "??????", Type: "text", Default: "vaultwarden_backup", Desc: "?? ZIP ?????"},
	{Key: "DATA_DIR", Label: "????", Type: "text", Default: "/data", Desc: "Vaultwarden ??????"},
	{Key: "BACKUP_DIR", Label: "????", Type: "text", Default: "/backup", Desc: "????????"},
	{Key: "ZIP_PASSWORD", Label: "ZIP ????", Type: "password", Default: "", Desc: "??????"},
	{Key: "CRON_SCHEDULE", Label: "???? (Cron)", Type: "text", Default: "0 2 * * *", Desc: "?? 0 2 * * * ????2?"},
	{Key: "RUN_ON_STARTUP", Label: "???????", Type: "select",
		Opts: []Option{{Value: "true", Label: "??"}, {Value: "false", Label: "??"}},
		Default: "true", Desc: "?????????????"},
	{Key: "STARTUP_DELAY", Label: "????(?)", Type: "number", Default: "15", Desc: "???????????????"},
	{Key: "LOCAL_BACKUP_KEEP_DAYS", Label: "??????", Type: "number", Default: "15", Desc: "????????"},
	{Key: "MIN_DISK_SPACE_MB", Label: "??????(MB)", Type: "number", Default: "5120", Desc: "???????????"},
	{Key: "RCLONE_REMOTE", Label: "Rclone ????", Type: "text", Default: "", Desc: "?? my_onedrive:/vaultwarden_backup"},
	{Key: "RCLONE_KEEP_DAYS", Label: "??????", Type: "number", Default: "", Desc: "??????????????"},
	{Key: "APPRISE_URL", Label: "Apprise ?? URL", Type: "text", Default: "", Desc: "?? tgram://bottoken/ChatID"},
	{Key: "APPRISE_API_URL", Label: "Apprise API ??", Type: "text", Default: "", Desc: "?? http://apprise:8000"},
	{Key: "HTTP_PROXY", Label: "HTTP ??", Type: "text", Default: "", Desc: "?? http://192.168.1.100:7890"},
	{Key: "HTTPS_PROXY", Label: "HTTPS ??", Type: "text", Default: "", Desc: "?? http://192.168.1.100:7890"},
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
	cronKeys := map[string]bool{"CRON_SCHEDULE": true, "RUN_ON_STARTUP": true, "STARTUP_DELAY": true, "LOCAL_BACKUP_KEEP_DAYS": true, "MIN_DISK_SPACE_MB": true}
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
		{GroupID: "database", GroupName: "?????", Icon: "bi-database", Fields: dbFields},
		{GroupID: "path", GroupName: "?????", Icon: "bi-folder", Fields: pathFields},
		{GroupID: "schedule", GroupName: "????", Icon: "bi-clock", Fields: cronFields},
		{GroupID: "cloud", GroupName: "??????", Icon: "bi-cloud", Fields: cloudFields},
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

