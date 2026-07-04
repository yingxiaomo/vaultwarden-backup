import hashlib
import os
import shutil
import shlex
import subprocess
import tempfile
from contextlib import asynccontextmanager
from datetime import datetime
from pathlib import Path

from fastapi import FastAPI, Request, Form
from fastapi.responses import HTMLResponse, RedirectResponse
from jinja2 import Environment, FileSystemLoader, select_autoescape

CONFIG_FILE = "/app/config/config.yaml"
ENV_FILE = "/app/env.sh"
BACKUP_LOG = "/app/config/backup.log"
TEMPLATES_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "templates")
_jinja_env = Environment(
    loader=FileSystemLoader(TEMPLATES_DIR),
    autoescape=select_autoescape(["html", "xml"]),
    auto_reload=False,
    cache_size=0,
)


def _render(name: str, request: Request, **context):
    tpl = _jinja_env.get_template(name)
    return HTMLResponse(tpl.render({"request": request, **context}))


def load_config():
    if not os.path.exists(CONFIG_FILE):
        return {}
    try:
        import yaml
        with open(CONFIG_FILE, "r", encoding="utf-8") as f:
            return yaml.safe_load(f) or {}
    except Exception as e:
        print(f"load config error: {e}")
        return {}


def save_config(env_vars: dict):
    os.makedirs(os.path.dirname(CONFIG_FILE), exist_ok=True)
    import yaml as _yaml
    filtered = {k: v for k, v in env_vars.items() if v is not None}
    with open(CONFIG_FILE, "w", encoding="utf-8") as f:
        _yaml.dump(filtered, f, allow_unicode=True, default_flow_style=False, sort_keys=False)

    os.makedirs(os.path.dirname(ENV_FILE), exist_ok=True)
    with open(ENV_FILE, "w", encoding="utf-8") as f:
        for key, value in env_vars.items():
            if value is not None:
                f.write(f"export {key}={shlex.quote(str(value))}\n")

    cron = env_vars.get("CRON_SCHEDULE", "0 2 * * *")
    cron_line = f"{cron} . {ENV_FILE} && /app/lib/scripts/backup.sh > /proc/1/fd/1 2>/proc/1/fd/2\n"
    tmp = tempfile.NamedTemporaryFile(mode="w", delete=False)
    try:
        tmp.write(cron_line)
        tmp.close()
        subprocess.run(["crontab", tmp.name], check=True, capture_output=True, timeout=10)
    except Exception as e:
        print(f"crontab update error: {e}")
    finally:
        os.unlink(tmp.name)


CONFIG_FIELDS = [
    {"key": "DB_TYPE", "label": "数据库类型", "type": "select",
     "opts": [("sqlite", "SQLite"), ("mysql", "MySQL"), ("postgres", "PostgreSQL")],
     "default": "sqlite", "desc": "Vaultwarden 使用的数据库类型"},
    {"key": "SQLITE_DB_FILE", "label": "SQLite 文件名", "type": "text",
     "default": "db.sqlite3", "desc": "SQLite 数据库文件名（默认 db.sqlite3）"},
    {"key": "DB_HOST", "label": "数据库主机", "type": "text", "default": "db",
     "desc": "MySQL/PostgreSQL 主机地址"},
    {"key": "DB_PORT", "label": "数据库端口", "type": "text", "default": "3306",
     "desc": "MySQL/PostgreSQL 端口"},
    {"key": "DB_USER", "label": "数据库用户名", "type": "text", "default": "",
     "desc": "MySQL/PostgreSQL 用户名"},
    {"key": "DB_PASSWORD", "label": "数据库密码", "type": "password", "default": "",
     "desc": "MySQL/PostgreSQL 密码"},
    {"key": "DB_NAME", "label": "数据库名称", "type": "text", "default": "vaultwarden",
     "desc": "MySQL/PostgreSQL 数据库名"},
    {"key": "BACKUP_PREFIX", "label": "备份文件前缀", "type": "text",
     "default": "vaultwarden_backup", "desc": "备份 ZIP 文件名前缀"},
    {"key": "DATA_DIR", "label": "数据目录", "type": "text", "default": "/data",
     "desc": "Vaultwarden 数据挂载路径"},
    {"key": "BACKUP_DIR", "label": "备份目录", "type": "text", "default": "/backup",
     "desc": "备份文件存放路径"},
    {"key": "ZIP_PASSWORD", "label": "ZIP 加密密码", "type": "password", "default": "",
     "desc": "留空则不加密"},
    {"key": "CRON_SCHEDULE", "label": "定时任务 (Cron)", "type": "text",
     "default": "0 2 * * *", "desc": "例如 0 2 * * * 每天凌晨2点"},
    {"key": "RUN_ON_STARTUP", "label": "启动时立即备份", "type": "select",
     "opts": [("true", "启用"), ("false", "禁用")],
     "default": "true", "desc": "容器启动时立即执行一次备份"},
    {"key": "LOCAL_BACKUP_KEEP_DAYS", "label": "本地保留天数", "type": "number",
     "default": "15", "desc": "本地备份保留天数"},
    {"key": "RCLONE_REMOTE", "label": "Rclone 远程路径", "type": "text",
     "default": "", "desc": "例如 my_onedrive:/vaultwarden_backup"},
    {"key": "RCLONE_KEEP_DAYS", "label": "远端保留天数", "type": "number",
     "default": "", "desc": "远端备份保留天数，留空不清理"},
    {"key": "APPRISE_URL", "label": "Apprise 通知 URL", "type": "text",
     "default": "", "desc": "例如 tgram://bottoken/ChatID"},
    {"key": "APPRISE_API_URL", "label": "Apprise API 地址", "type": "text",
     "default": "", "desc": "例如 http://apprise:8000"},
    {"key": "TZ", "label": "时区", "type": "text", "default": "Asia/Shanghai",
     "desc": "容器时区"},
    {"key": "HTTP_PROXY", "label": "HTTP 代理", "type": "text", "default": "",
     "desc": "例如 http://192.168.1.100:7890"},
    {"key": "HTTPS_PROXY", "label": "HTTPS 代理", "type": "text", "default": "",
     "desc": "例如 http://192.168.1.100:7890"},
    {"key": "CREATE_TEMP_BACKUP", "label": "恢复前创建临时备份", "type": "select",
     "opts": [("true", "启用"), ("false", "禁用")],
     "default": "true", "desc": "恢复前备份当前数据以便回滚"},
]


def verify_auth(request: Request):
    config = load_config()
    web_user = config.get("WEB_USER", "")
    web_pass = config.get("WEB_PASS", "")
    if not web_user or not web_pass:
        raise RequiresSetupException()
    token = request.cookies.get("auth_token")
    if not token:
        raise RequiresLoginException()
    expected = hashlib.sha256((web_user + web_pass).encode()).hexdigest()
    if token != expected:
        raise RequiresLoginException()
    return True


class RequiresSetupException(Exception):
    pass

class RequiresLoginException(Exception):
    pass


@asynccontextmanager
async def lifespan(app: FastAPI):
    print("=== Vaultwarden Backup Web Panel 启动 ===")
    config = load_config()
    if config.get("WEB_USER") and config.get("WEB_PASS"):
        print("系统已初始化，同步配置...")
        save_config(config)
        print("配置同步完成")
    else:
        print("系统未初始化，等待用户在 /setup 完成配置")
    yield

app = FastAPI(lifespan=lifespan)


# -- 登录 --
@app.get("/login", response_class=HTMLResponse)
async def login_page(request: Request, error: str = ""):
    return _render("login.html", request, error=error)

@app.post("/do_login")
async def do_login(request: Request, username: str = Form(...), password: str = Form(...)):
    config = load_config()
    if username == config.get("WEB_USER") and password == config.get("WEB_PASS"):
        token = hashlib.sha256((username + password).encode()).hexdigest()
        resp = RedirectResponse(url="/", status_code=303)
        resp.set_cookie("auth_token", token, httponly=True)
        return resp
    return RedirectResponse(url="/login?error=用户名或密码错误", status_code=303)

@app.get("/logout")
async def logout():
    resp = RedirectResponse(url="/login", status_code=303)
    resp.delete_cookie("auth_token")
    return resp

# -- 初始化 --
@app.get("/setup", response_class=HTMLResponse)
async def setup_page(request: Request, error: str = ""):
    return _render("setup.html", request, error=error)

@app.post("/do_setup")
async def do_setup(
    request: Request,
    web_user: str = Form(...),
    web_pass: str = Form(...),
    db_type: str = Form("sqlite"),
    tz: str = Form("Asia/Shanghai"),
    cron: str = Form("0 2 * * *"),
):
    config = {
        "WEB_USER": web_user, "WEB_PASS": web_pass,
        "DB_TYPE": db_type, "TZ": tz, "CRON_SCHEDULE": cron,
        "BACKUP_PREFIX": "vaultwarden_backup",
        "DATA_DIR": "/data", "BACKUP_DIR": "/backup",
        "LOCAL_BACKUP_KEEP_DAYS": "15", "RUN_ON_STARTUP": "true",
        "SQLITE_DB_FILE": "db.sqlite3",
    }
    save_config(config)
    return RedirectResponse(url="/login", status_code=303)

# -- 仪表盘 --
@app.get("/", response_class=HTMLResponse)
async def dashboard(request: Request, msg: str = ""):
    verify_auth(request)
    config = load_config()
    backup_dir = config.get("BACKUP_DIR", "/backup")
    prefix = config.get("BACKUP_PREFIX", "vaultwarden_backup")

    backups = []
    if os.path.exists(backup_dir):
        try:
            files = sorted(Path(backup_dir).glob(f"{prefix}_*.zip"), reverse=True)
            for f in files[:30]:
                size = f.stat().st_size
                size_str = f"{size/1024:.1f} KB" if size < 1024*1024 else f"{size/1024/1024:.1f} MB"
                mtime = os.path.getmtime(f)
                backups.append({
                    "name": f.name,
                    "size": size_str,
                    "mtime": datetime.fromtimestamp(mtime).strftime("%Y-%m-%d %H:%M:%S"),
                })
        except Exception as e:
            print(f"read backup history error: {e}")

    return _render("dashboard.html", request, backups=backups, msg=msg)

# -- 配置 --
@app.get("/config", response_class=HTMLResponse)
async def config_page(request: Request, ok: str = ""):
    verify_auth(request)
    config = load_config()
    fields = []
    for f in CONFIG_FIELDS:
        f = dict(f)
        val = config.get(f["key"])
        f["value"] = str(val) if val is not None else f.get("default", "")
        fields.append(f)
    return _render("config.html", request, fields=fields, ok=ok)

@app.post("/save_config")
async def save_config_route(request: Request):
    verify_auth(request)
    form = await request.form()
    config = {}
    for key, value in form.items():
        config[key] = value
    save_config(config)
    return RedirectResponse(url="/config?ok=1", status_code=303)

# -- 备份 --
@app.post("/do_backup")
async def do_backup(request: Request):
    verify_auth(request)
    try:
        result = subprocess.run(
            ["/bin/bash", "-c", f"source {ENV_FILE} && /app/lib/scripts/backup.sh"],
            capture_output=True, text=True, timeout=600
        )
        print(f"backup output:\n{result.stdout[-2000:]}\n{result.stderr[-2000:]}")
    except subprocess.TimeoutExpired:
        print("backup timeout (10min)")
    except Exception as e:
        print(f"backup error: {e}")
    return RedirectResponse(url="/", status_code=303)

# -- 恢复 --
@app.post("/do_restore")
async def do_restore(request: Request, backup_file: str = Form(...)):
    verify_auth(request)
    config = load_config()
    backup_dir = config.get("BACKUP_DIR", "/backup")
    data_dir = config.get("DATA_DIR", "/data")
    zip_password = config.get("ZIP_PASSWORD", "")
    db_type = config.get("DB_TYPE", "sqlite")
    sqlite_db_file = config.get("SQLITE_DB_FILE", "db.sqlite3")
    create_temp = str(config.get("CREATE_TEMP_BACKUP", "true")).lower() == "true"

    backup_path = os.path.join(backup_dir, os.path.basename(backup_file))
    if not os.path.exists(backup_path):
        return HTMLResponse("备份文件不存在", status_code=404)

    temp_backup = os.path.join(data_dir, "_restore_undo")
    try:
        subprocess.run(["docker", "stop", "vaultwarden"], capture_output=True, timeout=30)

        if create_temp and os.path.exists(data_dir):
            if os.path.exists(temp_backup):
                shutil.rmtree(temp_backup)
            os.makedirs(temp_backup, exist_ok=True)
            for item in os.listdir(data_dir):
                ip = os.path.join(data_dir, item)
                if item != "_restore_undo":
                    shutil.move(ip, os.path.join(temp_backup, item))

        unzip_args = ["unzip", "-o", backup_path, "-d", data_dir]
        if zip_password:
            unzip_args = ["unzip", "-P", str(zip_password), "-o", backup_path, "-d", data_dir]
        subprocess.run(unzip_args, check=True, capture_output=True, timeout=120)

        if db_type == "sqlite":
            temp_db = os.path.join(data_dir, "db_dump_temp.db")
            if os.path.exists(temp_db):
                target = os.path.join(data_dir, sqlite_db_file)
                for f in [target + "-wal", target + "-shm"]:
                    if os.path.exists(f): os.remove(f)
                os.replace(temp_db, target)

        subprocess.run(["docker", "start", "vaultwarden"], capture_output=True, timeout=30)

        if os.path.exists(temp_backup):
            shutil.rmtree(temp_backup)
        return RedirectResponse(url="/?msg=恢复成功", status_code=303)

    except Exception as e:
        print(f"restore error: {e}")
        if os.path.exists(temp_backup):
            for item in os.listdir(temp_backup):
                src = os.path.join(temp_backup, item)
                dst = os.path.join(data_dir, item)
                shutil.move(src, dst)
            shutil.rmtree(temp_backup)
        subprocess.run(["docker", "start", "vaultwarden"], capture_output=True, timeout=30)
        return HTMLResponse(f"恢复失败（已回滚）: {e}", status_code=500)

# -- 日志 --
@app.get("/logs")
async def get_logs(request: Request):
    verify_auth(request)
    if os.path.exists(BACKUP_LOG):
        with open(BACKUP_LOG, "r", encoding="utf-8", errors="replace") as f:
            return {"logs": f.read()[-5000:]}
    return {"logs": "暂无日志"}

# -- 健康 --
@app.get("/health")
async def health():
    return {"status": "ok"}

@app.exception_handler(RequiresSetupException)
async def setup_handler(request, exc):
    return RedirectResponse(url="/setup", status_code=303)

@app.exception_handler(RequiresLoginException)
async def login_handler(request, exc):
    return RedirectResponse(url="/login", status_code=303)

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=9876)
