from fastapi import FastAPI, Request, Form, Depends, BackgroundTasks, HTTPException, status
from fastapi.responses import HTMLResponse, RedirectResponse
from fastapi.templating import Jinja2Templates
from fastapi.staticfiles import StaticFiles
import subprocess
import os
import yaml
import datetime
import docker
import hashlib

# 初始化 FastAPI 应用
app = FastAPI()

# 挂载静态文件目录
app.mount("/static", StaticFiles(directory="static"), name="static")

# 初始化 Jinja2 模板
templates = Jinja2Templates(directory="templates")

# 初始化 Docker 客户端
try:
    client = docker.from_env()
except Exception as e:
    print(f"Docker 连接失败: {e}")
    client = None

CONFIG_FILE = "/app/config/config.yaml"
ENV_FILE = "/app/env.sh"

# 定义鉴权失败的拦截异常
class RequiresSetupException(Exception): pass
class RequiresLoginException(Exception): pass

# 拦截器：如果没初始化，跳转到 setup
@app.exception_handler(RequiresSetupException)
async def requires_setup_handler(request: Request, exc: RequiresSetupException):
    return RedirectResponse(url="/setup", status_code=303)

# 拦截器：如果没登录，跳转到 login
@app.exception_handler(RequiresLoginException)
async def requires_login_handler(request: Request, exc: RequiresLoginException):
    return RedirectResponse(url="/login", status_code=303)

# 核心安全守卫 (取代了旧的 HTTPBasic)
def verify_auth(request: Request):
    env_vars = get_env_vars()
    
    # 1. 检查是否是首次运行（未设置账号密码）
    if not env_vars.get("WEB_USER") or not env_vars.get("WEB_PASS"):
        raise RequiresSetupException()
    
    # 2. 检查会话 Cookie 是否合法
    session_token = request.cookies.get("auth_token")
    if not session_token:
        raise RequiresLoginException()
    
    # 3. 验证会话令牌
    expected_token = hashlib.sha256((env_vars.get("WEB_USER") + env_vars.get("WEB_PASS")).encode()).hexdigest()
    if session_token != expected_token:
        raise RequiresLoginException()
    
    return True

# 读取配置：优先读 config.yaml，如果没有则尝试从系统的环境变量中初始化一份默认值
def get_env_vars():
    # 如果有 config.yaml，直接读取
    if os.path.exists(CONFIG_FILE):
        try:
            with open(CONFIG_FILE, "r", encoding="utf-8") as f:
                env_vars = yaml.safe_load(f)
        except Exception as e:
            # YAML 解析失败，创建新的配置文件
            print(f"配置文件解析失败: {e}，创建新的配置文件")
            env_vars = create_default_config()
    else:
        # 如果没有，从 config.yaml.example 中读取带注释的配置
        env_vars = {}
        example_config = "/app/config.yaml.example"
        if os.path.exists(example_config):
            try:
                with open(example_config, "r", encoding="utf-8") as f:
                    # 读取文件内容，保留注释
                    config_content = f.read()
                    # 保存到 config.yaml
                    os.makedirs(os.path.dirname(CONFIG_FILE), exist_ok=True)
                    with open(CONFIG_FILE, "w", encoding="utf-8") as f_out:
                        f_out.write(config_content)
                    # 然后重新读取解析为字典
                    with open(CONFIG_FILE, "r", encoding="utf-8") as f_in:
                        env_vars = yaml.safe_load(f_in)
            except Exception as e:
                # 示例文件也失败，创建默认配置
                print(f"示例配置文件解析失败: {e}，创建默认配置")
                env_vars = create_default_config()
        else:
            # 如果 example 也不存在，创建一个带有完整注释的配置文件
            env_vars = create_default_config()
    
    return env_vars

def create_default_config():
    # 创建一个带有完整注释的配置文件
    config_content = """# Vaultwarden 备份配置文件
# 请根据实际情况修改以下配置

# 数据库类型 (sqlite, mysql, postgres)
DB_TYPE: sqlite

# MySQL 数据库配置（仅当 DB_TYPE=mysql 时使用）
DB_HOST: db
DB_PORT: '3306'
DB_USER: vaultwarden
DB_NAME: vaultwarden

# PostgreSQL 数据库配置（仅当 DB_TYPE=postgres 时使用）
PG_HOST: db
PG_PORT: '5432'
PG_USER: vaultwarden
PG_DB: vaultwarden

# 压缩包 AES 加密密码（可选，如果不设置则进行非加密打包）
# ZIP_PASSWORD: your_secure_zip_password

# 备份文件前缀（默认 vaultwarden_backup）
BACKUP_PREFIX: vaultwarden_backup

# 备份目录
BACKUP_DIR: /backup

# 数据目录
DATA_DIR: /vw_data

# 定时任务执行计划（Cron 表达式）
# 格式：分 时 日 月 周
# 示例：0 2 * * * （每天凌晨 2 点）
CRON_SCHEDULE: 0 2 * * *

# 启动时执行备份（true/false）
RUN_ON_STARTUP: 'true'

# 本地备份保留天数
LOCAL_BACKUP_KEEP_DAYS: '15'

# 远端备份保留天数（可选，如果不设置则不清理）
# RCLONE_KEEP_DAYS: '15'

# Rclone 远程存储配置
# 格式：remote_name:path
# 示例：my_onedrive:/vaultwarden_backup
# RCLONE_REMOTE: my_onedrive:/vaultwarden_backup

# Apprise 通知配置
# 方式 1: 直接配置通知 URL
# APPRISE_URL: tgram://bottoken/ChatID

# 方式 2: 使用独立的 Apprise 服务 API
# APPRISE_API_URL: http://apprise:8000

# 时区配置
TZ: Asia/Shanghai

# HTTP 代理配置（可选）
# HTTP_PROXY: http://192.168.1.100:7890
# HTTPS_PROXY: http://192.168.1.100:7890
# ALL_PROXY: socks5://192.168.1.100:7890

# Web 面板账号密码
WEB_USER: admin
WEB_PASS: admin"""
    
    # 保存到 config.yaml
    os.makedirs(os.path.dirname(CONFIG_FILE), exist_ok=True)
    with open(CONFIG_FILE, "w", encoding="utf-8") as f:
        f.write(config_content)
    
    # 然后重新读取解析为字典
    with open(CONFIG_FILE, "r", encoding="utf-8") as f_in:
        env_vars = yaml.safe_load(f_in)
    
    return env_vars

# 保存配置：同时保存为 YAML 和 Shell 脚本
def save_env_vars(env_vars):
    # 1. 保存结构化的 config.yaml (Web 面板使用) - 保留注释
    os.makedirs(os.path.dirname(CONFIG_FILE), exist_ok=True)
    
    # 如果配置文件存在且包含注释，保留注释
    if os.path.exists(CONFIG_FILE):
        with open(CONFIG_FILE, "r", encoding="utf-8") as f:
            config_content = f.read()
        
        # 如果文件包含注释，使用正则表达式更新值，保留注释
        if "#" in config_content:
            import re
            updated_content = config_content
            for key, value in env_vars.items():
                # 处理值中的引号
                if isinstance(value, str):
                    # 如果值包含空格，需要用引号包围
                    if ' ' in value:
                        # 确保值被正确引用
                        if not (value.startswith('"') and value.endswith('"')) and not (value.startswith("'") and value.endswith("'")):
                            value = f"'{value}'"
                
                # 替换配置值，保留注释
                pattern = f'({key}:).*'
                replacement = f'\1 {value}'
                updated_content = re.sub(pattern, replacement, updated_content)
            
            # 保存更新后的内容
            with open(CONFIG_FILE, "w", encoding="utf-8") as f:
                f.write(updated_content)
        else:
            # 如果文件不包含注释，使用 yaml.dump
            with open(CONFIG_FILE, "w", encoding="utf-8") as f:
                yaml.dump(env_vars, f, default_flow_style=False, allow_unicode=True, indent=2)
    else:
        # 如果文件不存在，创建一个带有注释的配置文件
        config_content = """# Vaultwarden 备份配置文件
# 请根据实际情况修改以下配置

# 数据库类型 (sqlite, mysql, postgres)
DB_TYPE: sqlite

# MySQL 数据库配置（仅当 DB_TYPE=mysql 时使用）
DB_HOST: db
DB_PORT: '3306'
DB_USER: vaultwarden
DB_NAME: vaultwarden

# PostgreSQL 数据库配置（仅当 DB_TYPE=postgres 时使用）
PG_HOST: db
PG_PORT: '5432'
PG_USER: vaultwarden
PG_DB: vaultwarden

# 压缩包 AES 加密密码（可选，如果不设置则进行非加密打包）
# ZIP_PASSWORD: your_secure_zip_password

# 备份文件前缀（默认 vaultwarden_backup）
BACKUP_PREFIX: vaultwarden_backup

# 备份目录
BACKUP_DIR: /backup

# 数据目录
DATA_DIR: /vw_data

# 定时任务执行计划（Cron 表达式）
# 格式：分 时 日 月 周
# 示例：0 2 * * * （每天凌晨 2 点）
CRON_SCHEDULE: 0 2 * * *

# 启动时执行备份（true/false）
RUN_ON_STARTUP: 'true'

# 本地备份保留天数
LOCAL_BACKUP_KEEP_DAYS: '15'

# 远端备份保留天数（可选，如果不设置则不清理）
# RCLONE_KEEP_DAYS: '15'

# Rclone 远程存储配置
# 格式：remote_name:path
# 示例：my_onedrive:/vaultwarden_backup
# RCLONE_REMOTE: my_onedrive:/vaultwarden_backup

# Apprise 通知配置
# 方式 1: 直接配置通知 URL
# APPRISE_URL: tgram://bottoken/ChatID

# 方式 2: 使用独立的 Apprise 服务 API
# APPRISE_API_URL: http://apprise:8000

# 时区配置
TZ: Asia/Shanghai

# HTTP 代理配置（可选）
# HTTP_PROXY: http://192.168.1.100:7890
# HTTPS_PROXY: http://192.168.1.100:7890
# ALL_PROXY: socks5://192.168.1.100:7890

# Web 面板账号密码
WEB_USER: admin
WEB_PASS: admin"""
        
        # 更新配置值
        import re
        updated_content = config_content
        for key, value in env_vars.items():
            # 处理值中的引号
            if isinstance(value, str):
                # 如果值包含空格，需要用引号包围
                if ' ' in value:
                    # 确保值被正确引用
                    if not (value.startswith('"') and value.endswith('"')) and not (value.startswith("'") and value.endswith("'")):
                        value = f"'{value}'"
            
            # 替换配置值，保留注释
            pattern = f'({key}:).*'
            replacement = f'\1 {value}'
            updated_content = re.sub(pattern, replacement, updated_content)
        
        # 保存配置
        with open(CONFIG_FILE, "w", encoding="utf-8") as f:
            f.write(updated_content)
    
    # 2. 同步生成 env.sh (底层 Bash 脚本使用)
    with open(ENV_FILE, "w") as f:
        for key, value in env_vars.items():
            f.write(f"export {key}='{value}'\n")

# 后台任务：执行备份
def run_backup_script():
    subprocess.run("source /app/env.sh && /app/backup.sh", shell=True, executable="/bin/bash")

# 主页面
@app.get("/", response_class=HTMLResponse, dependencies=[Depends(verify_auth)])
async def root(request: Request):
    # 获取备份历史
    backup_history = []
    try:
        backup_dir = os.environ.get("BACKUP_DIR", "/backup")
        if os.path.exists(backup_dir):
            files = os.listdir(backup_dir)
            for file in files:
                if file.endswith(".zip"):
                    file_path = os.path.join(backup_dir, file)
                    stat = os.stat(file_path)
                    size = os.path.getsize(file_path) / (1024 * 1024)  # 转换为 MB
                    backup_history.append({
                        "name": file,
                        "size": f"{size:.2f} MB",
                        "mtime": datetime.datetime.fromtimestamp(stat.st_mtime).strftime("%Y-%m-%d %H:%M:%S")
                    })
            backup_history.sort(key=lambda x: x["mtime"], reverse=True)
    except Exception as e:
        backup_history = [f"获取备份历史失败: {e}"]
    
    # 获取环境变量
    env_vars = get_env_vars()
    
    return templates.TemplateResponse(request=request, name="index.html", context={
        "request": request,
        "backup_history": backup_history,
        "env_vars": env_vars
    })

# 配置页面 (升级版：直接返回纯文本，保留注释)
@app.get("/config", response_class=HTMLResponse, dependencies=[Depends(verify_auth)])
async def config(request: Request):
    config_file_path = "/app/config/config.yaml"
    config_content = ""
    
    if os.path.exists(config_file_path):
        with open(config_file_path, "r", encoding="utf-8") as f:
            config_content = f.read()
        
        # 检查配置文件是否包含注释
        if "#" not in config_content:
            # 如果没有注释，重新初始化配置文件
            get_env_vars()
            with open(config_file_path, "r", encoding="utf-8") as f:
                config_content = f.read()
    else:
        # 如果文件还不存在，触发一下初始化再读取
        get_env_vars()
        with open(config_file_path, "r", encoding="utf-8") as f:
            config_content = f.read()
            
    return templates.TemplateResponse(request=request, name="config.html", context={
        "request": request,
        "config_content": config_content
    })

# 保存配置 (升级版：原样保存 YAML，防止注释丢失，并同步生成 env.sh)
@app.post("/save_config", dependencies=[Depends(verify_auth)])
async def save_config(request: Request, yaml_content: str = Form(...)):
    config_file_path = "/app/config/config.yaml"
    
    # 1. 语法检查防线：防止用户把 YAML 写错了导致整个系统崩溃
    try:
        env_vars = yaml.safe_load(yaml_content) or {}
    except Exception as e:
        # 语法错误，直接退回并报错，绝不覆盖原文件
        return templates.TemplateResponse(request=request, name="error.html", context={
            "request": request,
            "error": f"YAML 语法格式错误，保存失败，请检查空格和缩进: {e}"
        })
        
    # 2. 如果语法正确，将带有注释的【原生文本】直接覆盖保存
    os.makedirs(os.path.dirname(config_file_path), exist_ok=True)
    with open(config_file_path, "w", encoding="utf-8") as f:
        f.write(yaml_content)
    
    # 3. 翻译并同步生成底层的 env.sh
    env_file_path = "/app/env.sh"
    with open(env_file_path, "w", encoding="utf-8") as f:
        for key, value in env_vars.items():
            if value is not None:  # 防止空值报错
                f.write(f"export {key}='{value}'\n")
    
    # 4. 实时刷新 Linux 系统的定时任务 (Crontab)
    cron_schedule = env_vars.get("CRON_SCHEDULE", "0 2 * * *")
    cron_cmd = f"{cron_schedule} . /app/env.sh && /app/backup.sh > /proc/1/fd/1 2>/proc/1/fd/2\n"
    with open("/tmp/crontab.txt", "w") as f:
        f.write(cron_cmd)
    subprocess.run(["crontab", "/tmp/crontab.txt"])
    
    return RedirectResponse("/", status_code=303)

# 恢复页面
@app.get("/restore", response_class=HTMLResponse, dependencies=[Depends(verify_auth)])
async def restore(request: Request):
    # 获取备份历史
    backup_history = []
    try:
        backup_dir = os.environ.get("BACKUP_DIR", "/backup")
        if os.path.exists(backup_dir):
            files = os.listdir(backup_dir)
            for file in files:
                if file.endswith(".zip"):
                    file_path = os.path.join(backup_dir, file)
                    stat = os.stat(file_path)
                    size = os.path.getsize(file_path) / (1024 * 1024)  # 转换为 MB
                    backup_history.append({
                        "name": file,
                        "size": f"{size:.2f} MB",
                        "mtime": datetime.datetime.fromtimestamp(stat.st_mtime).strftime("%Y-%m-%d %H:%M:%S")
                    })
            backup_history.sort(key=lambda x: x["mtime"], reverse=True)
    except Exception as e:
        backup_history = [f"获取备份历史失败: {e}"]
    
    return templates.TemplateResponse(request=request, name="restore.html", context={
        "request": request,
        "backup_history": backup_history
    })

# 执行恢复
@app.post("/do_restore", dependencies=[Depends(verify_auth)])
async def do_restore(request: Request, backup_file: str = Form(...)):
    try:
        # 获取最新的环境变量
        env_vars = get_env_vars()
        
        # 检查数据目录是否存在
        data_dir = env_vars.get("DATA_DIR", "/vw_data")
        if not os.path.exists(data_dir):
            raise Exception(f"数据目录 {data_dir} 不存在，请检查挂载是否正确")
        
        # 停止 Vaultwarden 容器
        if client:
            vaultwarden_containers = client.containers.list(filters={"name": "vaultwarden"})
            for container in vaultwarden_containers:
                container.stop()
                print(f"已停止 Vaultwarden 容器: {container.name}")
        
        # 解压恢复
        backup_dir = env_vars.get("BACKUP_DIR", "/backup")
        backup_path = os.path.join(backup_dir, backup_file)
        
        # 使用列表形式调用，彻底杜绝空格和特殊字符引起的 Bug
        if env_vars.get("ZIP_PASSWORD"):
            unzip_cmd = ["unzip", "-P", env_vars["ZIP_PASSWORD"], "-o", backup_path, "-d", data_dir]
        else:
            unzip_cmd = ["unzip", "-o", backup_path, "-d", data_dir]
        
        # 移除 shell=True
        subprocess.run(unzip_cmd, check=True)
        print(f"已恢复备份: {backup_file}")
        
        # 启动 Vaultwarden 容器
        if client:
            for container in vaultwarden_containers:
                container.start()
                print(f"已启动 Vaultwarden 容器: {container.name}")
        
        return RedirectResponse("/", status_code=303)
    except Exception as e:
        print(f"恢复失败: {e}")
        return templates.TemplateResponse(request=request, name="error.html", context={
            "request": request,
            "error": f"恢复失败: {e}"
        })

# 执行备份
@app.post("/do_backup", dependencies=[Depends(verify_auth)])
async def do_backup(request: Request, background_tasks: BackgroundTasks):
    try:
        # 添加后台任务
        background_tasks.add_task(run_backup_script)
        # 立即返回，避免网页卡顿
        return RedirectResponse("/", status_code=303)
    except Exception as e:
        print(f"备份失败: {e}")
        return templates.TemplateResponse(request=request, name="error.html", context={
            "request": request,
            "error": f"备份失败: {e}"
        })

# 获取备份日志
@app.get("/logs", dependencies=[Depends(verify_auth)])
async def get_backup_logs():
    log_file = "/app/config/backup.log"
    try:
        if os.path.exists(log_file):
            with open(log_file, "r", encoding="utf-8") as f:
                # 读取最后50行日志
                lines = f.readlines()
                last_lines = lines[-50:]
                log_content = "".join(last_lines)
                return {"logs": log_content}
        else:
            return {"logs": "日志文件不存在，尚未执行备份任务"}
    except Exception as e:
        return {"logs": f"读取日志失败: {e}"}

# 初始化向导页面
@app.get("/setup")
async def setup(request: Request):
    env_vars = get_env_vars()
    # 如果已经设置了账号密码，直接跳转到登录页
    if env_vars.get("WEB_USER") and env_vars.get("WEB_PASS"):
        return RedirectResponse(url="/login", status_code=303)
    return templates.TemplateResponse(request=request, name="setup.html", context={
        "request": request
    })

# 处理初始化表单提交
@app.post("/do_setup")
async def do_setup(request: Request, username: str = Form(...), password: str = Form(...)):
    config_file_path = "/app/config/config.yaml"
    example_config = "/app/config.yaml.example"
    
    # 读取配置文件内容，保留注释
    if os.path.exists(config_file_path):
        with open(config_file_path, "r", encoding="utf-8") as f:
            config_content = f.read()
    elif os.path.exists(example_config):
        with open(example_config, "r", encoding="utf-8") as f:
            config_content = f.read()
    else:
        # 如果都不存在，创建一个基本配置
        config_content = "# Vaultwarden 备份配置文件\n# 请根据实际情况修改以下配置\n\nDB_TYPE: sqlite\nCRON_SCHEDULE: 0 2 * * *\nRUN_ON_STARTUP: 'true'\nLOCAL_BACKUP_KEEP_DAYS: '15'\nBACKUP_PREFIX: vaultwarden_backup\nBACKUP_DIR: /backup\nDATA_DIR: /vw_data\nTZ: Asia/Shanghai\n"
    
    # 更新账号密码
    # 查找并替换 WEB_USER 和 WEB_PASS
    import re
    config_content = re.sub(r'(WEB_USER:).*', f'\1 {username}', config_content)
    config_content = re.sub(r'(WEB_PASS:).*', f'\1 {password}', config_content)
    
    # 如果没有 WEB_USER 和 WEB_PASS，添加到文件末尾
    if 'WEB_USER:' not in config_content:
        config_content += f'\n# Web 面板账号密码\nWEB_USER: {username}\nWEB_PASS: {password}\n'
    
    # 保存配置
    os.makedirs(os.path.dirname(config_file_path), exist_ok=True)
    with open(config_file_path, "w", encoding="utf-8") as f:
        f.write(config_content)
    
    # 读取更新后的配置
    env_vars = get_env_vars()
    
    # 生成 env.sh
    env_file_path = "/app/env.sh"
    with open(env_file_path, "w", encoding="utf-8") as f:
        for key, value in env_vars.items():
            if value is not None:
                f.write(f"export {key}='{value}'\n")
    
    # 生成会话令牌
    session_token = hashlib.sha256((username + password).encode()).hexdigest()
    
    # 重定向到主页，并设置会话 Cookie
    response = RedirectResponse(url="/", status_code=303)
    response.set_cookie("auth_token", session_token, httponly=True, max_age=86400)  # 24小时
    return response

# 登录页面
@app.get("/login")
async def login(request: Request, error: str = None):
    env_vars = get_env_vars()
    # 如果未设置账号密码，跳转到初始化页面
    if not env_vars.get("WEB_USER") or not env_vars.get("WEB_PASS"):
        return RedirectResponse(url="/setup", status_code=303)
    return templates.TemplateResponse(request=request, name="login.html", context={
        "request": request,
        "error": error
    })

# 处理登录表单提交
@app.post("/do_login")
async def do_login(request: Request, username: str = Form(...), password: str = Form(...)):
    env_vars = get_env_vars()
    
    # 验证用户名密码
    if username == env_vars.get("WEB_USER") and password == env_vars.get("WEB_PASS"):
        # 生成会话令牌
        session_token = hashlib.sha256((username + password).encode()).hexdigest()
        
        # 重定向到主页，并设置会话 Cookie
        response = RedirectResponse(url="/", status_code=303)
        response.set_cookie("auth_token", session_token, httponly=True, max_age=86400)  # 24小时
        return response
    else:
        # 登录失败，返回错误信息
        return RedirectResponse(url="/login?error=用户名或密码错误", status_code=303)

# 登出
@app.get("/logout")
async def logout(request: Request):
    response = RedirectResponse(url="/login", status_code=303)
    response.delete_cookie("auth_token")
    return response

# 健康检查接口（不需要身份验证）
@app.get("/health")
async def health_check():
    return {"status": "ok"}

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=9876)