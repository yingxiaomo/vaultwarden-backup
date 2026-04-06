# 初始化路由
from fastapi import APIRouter, Request, Form
from fastapi.responses import HTMLResponse, RedirectResponse
from fastapi.templating import Jinja2Templates
import os
import json
import shlex
import hashlib

from app_config import get_env_vars
from utils import run_shell_command

# 初始化模板
templates = Jinja2Templates(directory="templates")

router = APIRouter()

# 初始化向导页面
@router.get("/setup")
async def setup(request: Request):
    env_vars = get_env_vars()
    # 如果已经设置了账号密码，直接跳转到登录页
    if env_vars.get("WEB_USER") and env_vars.get("WEB_PASS"):
        return RedirectResponse(url="/login", status_code=303)
    return templates.TemplateResponse(request=request, name="setup.html", context={
        "request": request
    })

# 处理初始化表单提交
@router.post("/do_setup")
async def do_setup(request: Request, 
                  username: str = Form(...), 
                  password: str = Form(...),
                  db_type: str = Form(...),
                  db_host: str = Form(None),
                  db_port: str = Form(None),
                  db_user: str = Form(None),
                  db_password: str = Form(None),
                  db_name: str = Form(None),
                  data_dir: str = Form("/data"),
                  backup_dir: str = Form("/backup"),
                  backup_prefix: str = Form("vaultwarden_backup"),
                  cron_schedule: str = Form("0 2 * * *")):
    config_file_path = "/app/config/config.yaml"
    
    # 创建完整的配置文件内容
    config_content = f"""# Vaultwarden 备份配置文件
# 请根据实际情况修改以下配置

# 数据库类型: sqlite, mysql, postgres
DB_TYPE: {json.dumps(db_type)}

# 数据目录
# Vaultwarden 数据存储目录，通常为 /data
DATA_DIR: {json.dumps(data_dir)}

# 备份目录
# 本地备份存储目录
BACKUP_DIR: {json.dumps(backup_dir)}

# 备份前缀
# 备份文件的前缀名称
BACKUP_PREFIX: {json.dumps(backup_prefix)}

# 定时备份计划
# 格式: 分钟 小时 日 月 周
# 例如: 0 2 * * * (每天凌晨2点)
CRON_SCHEDULE: {json.dumps(cron_schedule)}

# 启动时执行备份
RUN_ON_STARTUP: 'true'

# 本地备份保留天数
LOCAL_BACKUP_KEEP_DAYS: '15'

# 恢复前创建临时备份（可能会导致磁盘空间翻倍，建议在磁盘空间充足时启用）
CREATE_TEMP_BACKUP: 'true'

# 压缩包密码（可选）
# ZIP_PASSWORD: your_secure_zip_password

# Rclone 配置（可选）
# RCLONE_KEEP_DAYS: '15'
# RCLONE_REMOTE: my_onedrive:/vaultwarden_backup

# Apprise 通知配置（可选）
# APPRISE_URL: tgram://bottoken/ChatID
# APPRISE_API_URL: http://apprise:8000  

# 时区设置
TZ: Asia/Shanghai

# Web 面板账号密码
WEB_USER: {json.dumps(username)}
WEB_PASS: {json.dumps(password)}
"""
    
    # 如果是 MySQL 或 PostgreSQL，添加数据库连接信息
    if db_type in ["mysql", "postgres"]:
        config_content += f"""
# 数据库连接信息
DB_HOST: {json.dumps(db_host or "db")}
DB_PORT: {json.dumps(db_port or ("3306" if db_type == "mysql" else "5432"))}
DB_USER: {json.dumps(db_user or "")}
DB_PASSWORD: {json.dumps(db_password or "")}
DB_NAME: {json.dumps(db_name or "")}
"""
    
    # 保存配置
    os.makedirs(os.path.dirname(config_file_path), exist_ok=True)
    with open(config_file_path, "w", encoding="utf-8") as f:
        f.write(config_content)
    
    # 同步更新 env.sh
    env_vars = get_env_vars()
    env_file_path = "/app/env.sh"
    with open(env_file_path, "w", encoding="utf-8") as f:
        for key, value in env_vars.items():
            if value is not None:
                # 使用 shlex.quote，不用我们自己再加单引号了，它会自动处理所有特殊符号！
                safe_val = shlex.quote(str(value))
                f.write(f"export {key}={safe_val}\n")
    
    # 强制刷新 Crontab
    import tempfile
    cron_schedule = env_vars.get("CRON_SCHEDULE", "0 2 * * *")
    cron_cmd = f"{cron_schedule} . /app/env.sh && /app/lib/scripts/backup.sh > /proc/1/fd/1 2>/proc/1/fd/2\n"
    with tempfile.NamedTemporaryFile(mode='w', delete=False) as f:
        f.write(cron_cmd)
        temp_file_path = f.name
    try:
        run_shell_command(["crontab", temp_file_path])
    finally:
        import os
        os.unlink(temp_file_path)
    
    # 生成会话令牌
    session_token = hashlib.sha256((username + password).encode()).hexdigest()
    
    # 重定向到主页，并设置会话 Cookie
    response = RedirectResponse(url="/", status_code=303)
    response.set_cookie("auth_token", session_token, httponly=True, max_age=86400)
    return response
