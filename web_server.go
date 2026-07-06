package main

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

//go:embed templates/*
var templateFS embed.FS

var (
	templates     *template.Template
	templatesOnce sync.Once
)

// tmpl ???????????
func tmpl() *template.Template {
	templatesOnce.Do(func() {
		// ???????
		funcMap := template.FuncMap{
			"sub": func(a, b int) int { return a - b },
		}
		templates = template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/*.html"))
	})
	return templates
}

// ============================================================
// Session / Auth
// ============================================================

// hashCredentials ???? token (SHA256)
func hashCredentials(username, password string) string {
	h := sha256.Sum256([]byte(username + password))
	return hex.EncodeToString(h[:])
}

// verifyAuth ?????????
// ?? (ok bool, errorPage bool, redirectURL string)
func verifyAuth(r *http.Request) (bool, string) {
	cfg, err := loadYamlConfig()
	if err != nil || cfg.WebUser == "" || cfg.WebPass == "" {
		// ????
		return false, "/setup"
	}
	token := ""
	if cookie, err := r.Cookie("auth_token"); err == nil {
		token = cookie.Value
	}
	if token == "" {
		return false, "/login"
	}
	expected := hashCredentials(cfg.WebUser, cfg.WebPass)
	if token != expected {
		return false, "/login"
	}
	return true, ""
}

// redirectHTML ?????????? HTML ?????? GET ???
func redirectHTML(url string) string {
	return fmt.Sprintf(`<!DOCTYPE html><html><head><meta http-equiv="refresh" content="0;url=%s"></head><body></body></html>`, url)
}

// ============================================================
// HTTP ??? - ??
// ============================================================

func handleLogin(w http.ResponseWriter, r *http.Request) {
	errorMsg := r.URL.Query().Get("error")
	data := map[string]string{"Error": errorMsg}
	_ = tmpl().ExecuteTemplate(w, "login.html", data)
}

func handleDoLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	_ = r.ParseForm()
	username := r.FormValue("username")
	password := r.FormValue("password")

	cfg, err := loadYamlConfig()
	if err != nil || cfg.WebUser == "" || cfg.WebPass == "" {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}

	if username == cfg.WebUser && password == cfg.WebPass {
		token := hashCredentials(username, password)
		http.SetCookie(w, &http.Cookie{
			Name:     "auth_token",
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			MaxAge:   86400 * 7, // 7?
		})
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/login?error=????????", http.StatusSeeOther)
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// ============================================================
// HTTP ??? - ???
// ============================================================

func handleSetup(w http.ResponseWriter, r *http.Request) {
	errorMsg := r.URL.Query().Get("error")
	data := map[string]string{"Error": errorMsg}
	_ = tmpl().ExecuteTemplate(w, "setup.html", data)
}

func handleDoSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}
	_ = r.ParseForm()
	webUser := r.FormValue("web_user")
	webPass := r.FormValue("web_pass")

	if webUser == "" || webPass == "" {
		http.Redirect(w, r, "/setup?error=??????????", http.StatusSeeOther)
		return
	}

	cfg := &ConfigFile{
		WebUser: webUser,
		WebPass: webPass,
	}
	// ????????
	existing, err := loadYamlConfig()
	if err == nil {
		cfg.DBType = existing.DBType
		cfg.BackupDir = existing.BackupDir
		cfg.DataDir = existing.DataDir
		cfg.BackupPrefix = existing.BackupPrefix
		// ????????
		cfg.SQLiteDBFile = existing.SQLiteDBFile
		cfg.DBHost = existing.DBHost
		cfg.DBPort = existing.DBPort
		cfg.DBUser = existing.DBUser
		cfg.DBPassword = existing.DBPassword
		cfg.DBName = existing.DBName
		cfg.ZipPassword = existing.ZipPassword
		cfg.CronSchedule = existing.CronSchedule
		cfg.RunOnStartup = existing.RunOnStartup
		cfg.StartupDelay = existing.StartupDelay
		cfg.LocalKeepDays = existing.LocalKeepDays
		cfg.MinDiskSpaceMB = existing.MinDiskSpaceMB
		cfg.RcloneRemote = existing.RcloneRemote
		cfg.RcloneKeepDays = existing.RcloneKeepDays
		cfg.AppriseURL = existing.AppriseURL
		cfg.AppriseAPIURL = existing.AppriseAPIURL
		cfg.HTTPProxy = existing.HTTPProxy
		cfg.HTTPSProxy = existing.HTTPSProxy
		cfg.TZ = existing.TZ
	}

	if err := saveYamlConfig(cfg); err != nil {
		http.Redirect(w, r, "/setup?error=??????: "+err.Error(), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// ============================================================
// HTTP ??? - ???
// ============================================================

// BackupFileInfo ??????
type BackupFileInfo struct {
	Name  string
	Size  string
	MTime string
}

// DashboardData ???????
type DashboardData struct {
	Backups []BackupFileInfo
	Msg     string
}

func handleDashboard(w http.ResponseWriter, r *http.Request) {
	ok, redirect := verifyAuth(r)
	if !ok {
		http.Redirect(w, r, redirect, http.StatusSeeOther)
		return
	}

	msg := r.URL.Query().Get("msg")
	cfg, _ := loadYamlConfig()
	backupDir := getConfigValue(cfg, "BACKUP_DIR")
	if backupDir == "" {
		backupDir = "/backup"
	}
	prefix := getConfigValue(cfg, "BACKUP_PREFIX")
	if prefix == "" {
		prefix = "vaultwarden_backup"
	}

	var backups []BackupFileInfo
	pattern := filepath.Join(backupDir, prefix+"_*.zip")
	matches, err := filepath.Glob(pattern)
	if err == nil {
		// ????????????????
		type fileWithTime struct {
			path string
			info os.FileInfo
		}
		var files []fileWithTime
		for _, m := range matches {
			info, err := os.Stat(m)
			if err != nil {
				continue
			}
			files = append(files, fileWithTime{m, info})
		}
		// ???????
		for i := 0; i < len(files); i++ {
			for j := i + 1; j < len(files); j++ {
				if files[i].info.ModTime().Before(files[j].info.ModTime()) {
					files[i], files[j] = files[j], files[i]
				}
			}
		}
		// ??30?
		for i, f := range files {
			if i >= 30 {
				break
			}
			sizeStr := formatBytes(f.info.Size())
			backups = append(backups, BackupFileInfo{
				Name:  filepath.Base(f.path),
				Size:  sizeStr,
				MTime: f.info.ModTime().Format("2006-01-02 15:04:05"),
			})
		}
	}

	data := DashboardData{
		Backups: backups,
		Msg:     msg,
	}
	_ = tmpl().ExecuteTemplate(w, "dashboard.html", data)
}

// ============================================================
// HTTP ??? - ??
// ============================================================

func handleConfig(w http.ResponseWriter, r *http.Request) {
	ok, redirect := verifyAuth(r)
	if !ok {
		http.Redirect(w, r, redirect, http.StatusSeeOther)
		return
	}

	okParam := r.URL.Query().Get("ok")
	cfg, _ := loadYamlConfig()
	groups := buildConfigGroups(cfg)

	data := ConfigPageData{
		Groups: groups,
		Ok:     okParam,
	}
	_ = tmpl().ExecuteTemplate(w, "config.html", data)
}

func handleSaveConfig(w http.ResponseWriter, r *http.Request) {
	ok, redirect := verifyAuth(r)
	if !ok {
		http.Redirect(w, r, redirect, http.StatusSeeOther)
		return
	}
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/config", http.StatusSeeOther)
		return
	}

	_ = r.ParseForm()
	formMap := make(map[string]string)
	for k, v := range r.Form {
		if len(v) > 0 {
			formMap[k] = v[0]
		}
	}

	cfg := parseFormToConfig(formMap)
	if err := saveYamlConfig(cfg); err != nil {
		http.Error(w, "??????: "+err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/config?ok=1", http.StatusSeeOther)
}

// ============================================================
// HTTP ??? - ??
// ============================================================

func handleDoBackup(w http.ResponseWriter, r *http.Request) {
	ok, redirect := verifyAuth(r)
	if !ok {
		http.Redirect(w, r, redirect, http.StatusSeeOther)
		return
	}
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// ??? goroutine ??????
	go func() {
		cfg := loadConfig()
		// ?? rclone ?????
		ensureRcloneConfig()
		runBackup(cfg)
	}()

	http.Redirect(w, r, "/?msg=?????????????", http.StatusSeeOther)
}

// ============================================================
// HTTP ??? - ??
// ============================================================

func handleDoRestore(w http.ResponseWriter, r *http.Request) {
	ok, redirect := verifyAuth(r)
	if !ok {
		http.Redirect(w, r, redirect, http.StatusSeeOther)
		return
	}
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	_ = r.ParseForm()
	backupName := r.FormValue("backup_file")
	if backupName == "" {
		http.Redirect(w, r, "/?msg=???????????", http.StatusSeeOther)
		return
	}

	cfg := loadConfig()
	backupPath := filepath.Join(cfg.BackupDir, filepath.Base(backupName))

	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		http.Redirect(w, r, "/?msg=???????", http.StatusSeeOther)
		return
	}

	createTemp := true
	result := doRestoreFromPath(backupPath, cfg.DataDir, cfg.ZipPassword, cfg.DBType, cfg.SQLiteDBFile, createTemp)
	msg := result.Message
	if !result.Success {
		msg = "????: " + result.Message
	}
	http.Redirect(w, r, "/?msg="+msg, http.StatusSeeOther)
}

func handleDoUploadRestore(w http.ResponseWriter, r *http.Request) {
	ok, redirect := verifyAuth(r)
	if !ok {
		http.Redirect(w, r, redirect, http.StatusSeeOther)
		return
	}
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// ??????
	_ = r.ParseMultipartForm(100 << 20) // 100MB ??
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Redirect(w, r, "/?msg=????????: "+err.Error(), http.StatusSeeOther)
		return
	}
	defer file.Close()

	if !strings.HasSuffix(header.Filename, ".zip") {
		http.Redirect(w, r, "/?msg=??? .zip ???????", http.StatusSeeOther)
		return
	}

	cfg := loadConfig()
	savePath, err := saveUploadedFile(file, header.Filename, cfg.BackupDir)
	if err != nil {
		http.Redirect(w, r, "/?msg=????????: "+err.Error(), http.StatusSeeOther)
		return
	}

	createTemp := true
	result := doRestoreFromPath(savePath, cfg.DataDir, cfg.ZipPassword, cfg.DBType, cfg.SQLiteDBFile, createTemp)

	// ?????????
	_ = os.Remove(savePath)

	msg := result.Message
	if !result.Success {
		msg = "????: " + result.Message
	}
	http.Redirect(w, r, "/?msg="+msg, http.StatusSeeOther)
}

// ============================================================
// HTTP ??? - ??
// ============================================================

func handleLogs(w http.ResponseWriter, r *http.Request) {
	ok, _ := verifyAuth(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	logPath := getWebLogPath()
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"logs":""}`))
		return
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"logs":"??????"}`))
		return
	}

	// ????? 5000 ??
	content := string(data)
	if len(content) > 5000 {
		content = content[len(content)-5000:]
	}

	resp, _ := json.Marshal(map[string]string{"logs": content})
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(resp)
}

// ============================================================
// HTTP ??? - ????
// ============================================================

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// ============================================================
// ?????Web ??? CLI ???????
// ============================================================

// webLogWriter ??????? stdout ?????
func webLogWriter(logEntry string) {
	t := time.Now().Format("2006-01-02 15:04:05")
	line := fmt.Sprintf("[%s] %s", t, logEntry)
	fmt.Println(line)
	// ???????
	logPath := getWebLogPath()
	dir := filepath.Dir(logPath)
	_ = os.MkdirAll(dir, 0755)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(line + "\n")
}

// ============================================================
// ??????
// ============================================================

// setupRedirects ???????????
func setupRedirects(mux *http.ServeMux) {
	// ??????????????
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			handleDashboard(w, r)
			return
		}
		// ????????
		switch r.URL.Path {
		case "/login":
			handleLogin(w, r)
		case "/do_login":
			handleDoLogin(w, r)
		case "/logout":
			handleLogout(w, r)
		case "/setup":
			handleSetup(w, r)
		case "/do_setup":
			handleDoSetup(w, r)
		case "/config":
			handleConfig(w, r)
		case "/save_config":
			handleSaveConfig(w, r)
		case "/do_backup":
			handleDoBackup(w, r)
		case "/do_restore":
			handleDoRestore(w, r)
		case "/do_upload_restore":
			handleDoUploadRestore(w, r)
		case "/logs":
			handleLogs(w, r)
		case "/health":
			handleHealth(w, r)
		default:
			http.Redirect(w, r, "/", http.StatusSeeOther)
		}
	})
}

// ============================================================
// ?? Web ???
// ============================================================

func startWebServer() {
	log.Println("=== Vaultwarden Backup Web Panel ?? ===")

	// ????????
	cfg, err := loadYamlConfig()
	if err == nil && cfg.WebUser != "" && cfg.WebPass != "" {
		log.Println("???????????...")
		syncConfigToEnv(cfg)
		log.Println("??????")
	} else {
		log.Println("???????????? /setup ????")
	}

	mux := http.NewServeMux()
	setupRedirects(mux)

	// ??????
	webLogWriter("Web ??????")

	port := os.Getenv("WEB_PORT")
	if port == "" {
		port = "9876"
	}
	addr := fmt.Sprintf("0.0.0.0:%s", port)
	log.Printf("Web ??????: http://%s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Web ???????: %v", err)
	}
}
