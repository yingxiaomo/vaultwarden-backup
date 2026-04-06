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
import shlex
import json

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
    saved_user = str(env_vars.get("WEB_USER", ""))
    saved_pass = str(env_vars.get("WEB_PASS", ""))
    
    if not saved_user or not saved_pass:
        raise RequiresSetupException()
    
    # 2. 检查会话 Cookie 是否合法
    session_token = request.cookies.get("auth_token")
    if not session_token:
        raise RequiresLoginException()
    
    # 3. 验证会话令牌，使用与登录时相同的生成方式
    expected_token = hashlib.sha256((saved_user + saved_pass).encode()).hexdigest()
    if session_token != expected_token:
        raise RequiresLoginException()
    
    return True

# 读取配置：优先读 config.yaml，如果没有则触发自动初始化
def get_env_vars():
    if os.path.exists(CONFIG_FILE):
        try:
            with open(CONFIG_FILE, "r", encoding="utf-8") as f:
                return yaml.safe_load(f) or {}
        except Exception as e:
            print(f"配置文件解析失败: {e}，将重新初始化配置文件")
            
    # 如果不存在或解析失败，进入全自动初始化流程
    return create_default_config()

# 生成默认配置：使用内置硬编码模板并吸收 docker-compose 环境变量
def create_default_config():
    # 1. 唯一事实来源：内置的硬编码模板
    config_content = """# Vaultwarden 备份配置文件
# 请根据实际情况修改以下配置

DB_TYPE: sqlite
BACKUP_PREFIX: vaultwarden_backup
BACKUP_DIR: /backup
DATA_DIR: /vw_data
CRON_SCHEDULE: 0 2 * * *
RUN_ON_STARTUP: 'true'
LOCAL_BACKUP_KEEP_DAYS: '15'
# ZIP_PASSWORD: your_secure_zip_password
# RCLONE_KEEP_DAYS: '15'
# RCLONE_REMOTE: my_onedrive:/vaultwarden_backup
# APPRISE_URL: tgram://bottoken/ChatID
# APPRISE_API_URL: `http://apprise:8000`
TZ: Asia/Shanghai
WEB_USER: admin
WEB_PASS: admin"""

    # 2. [核心魔法]：动态吸收 docker-compose 传进来的环境变量
    for key, value in os.environ.items():
        if key in ["PATH", "HOSTNAME", "PWD", "HOME", "SHLVL", "TERM"]:
            continue
        if key.isupper():
            # 同样使用 json.dumps，防止外部环境变量里含有特殊的 YAML 字符
            safe_value = json.dumps(value)
            
            pattern = rf'^#?\s*({key}):.*$'
            if re.search(pattern, config_content, flags=re.MULTILINE):
                config_content = re.sub(pattern, lambda _: f'{key}: {safe_value}', config_content, flags=re.MULTILINE)
            else:
                config_content += f'\n{key}: {safe_value}'
    
    # 3. 保存生成的神奇 YAML 到 /app/config 目录
    os.makedirs(os.path.dirname(CONFIG_FILE), exist_ok=True)
    with open(CONFIG_FILE, "w", encoding="utf-8") as f:
        f.write(config_content)
    
    # 4. 读取并返回字典给系统使用
    with open(CONFIG_FILE, "r", encoding="utf-8") as f_in:
        return yaml.safe_load(f_in) or {}



# 后台任务：执行备份
def run_backup_script():
    subprocess.run("source /app/env.sh && /app/backup.sh", shell=True, executable="/bin/bash")

# 容器冷启动初始化事件
@app.on_event("startup")
async def startup_event():
    print("🚀 正在执行冷启动配置同步...")
    env_vars = get_env_vars()
    
    # 强制将读取到的变量同步到底层的 env.sh
    os.makedirs(os.path.dirname(ENV_FILE), exist_ok=True)
    with open(ENV_FILE, "w", encoding="utf-8") as f:
        for key, value in env_vars.items():
            if value is not None:
                # 使用 shlex.quote，不用我们自己再加单引号了，它会自动处理所有特殊符号！
                safe_val = shlex.quote(str(value))
                f.write(f"export {key}={safe_val}\n")
    
    # 强制刷新一次底层 Linux 的 Crontab 定时任务
    cron_schedule = env_vars.get("CRON_SCHEDULE", "0 2 * * *")
    cron_cmd = f"{cron_schedule} . /app/env.sh && /app/backup.sh > /proc/1/fd/1 2>/proc/1/fd/2\n"
    with open("/tmp/crontab.txt", "w") as f:
        f.write(cron_cmd)
    subprocess.run(["crontab", "/tmp/crontab.txt"])
    print("✅ 配置同步完成！底层守护进程已就绪。")

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

# 配置页面 (极简版)
@app.get("/config", response_class=HTMLResponse, dependencies=[Depends(verify_auth)])
async def config(request: Request):
    config_file_path = "/app/config/config.yaml"
    
    # 因为有 startup_event，此时文件百分之百存在，直接读取！
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
    
    # 生成 env.sh
    env_file_path = "/app/env.sh"
    with open(env_file_path, "w", encoding="utf-8") as f:
        for key, value in env_vars.items():
            if value is not None:
                # 使用 shlex.quote，不用我们自己再加单引号了，它会自动处理所有特殊符号！
                safe_val = shlex.quote(str(value))
                f.write(f"export {key}={safe_val}\n")
    
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
            # 这里的过滤器可能会抓到包含 vaultwarden 名字的所有容器（包括 backup 自己）
            all_found = client.containers.list(filters={"name": "vaultwarden"})
            # 过滤出真正的目标（精准匹配，且排除掉自己）
            vaultwarden_containers = [c for c in all_found if c.name == "vaultwarden" and c.id != os.environ.get("HOSTNAME")]
            
            for container in vaultwarden_containers:
                container.stop()
                print(f"已停止 Vaultwarden 容器: {container.name}")
        
        # 解压恢复
        backup_dir = env_vars.get("BACKUP_DIR", "/backup")
        backup_file = os.path.basename(backup_file) # 核心安全防线：强制只保留文件名，丢弃所有路径信息
        backup_path = os.path.join(backup_dir, backup_file)
        
        # 使用列表形式调用，彻底杜绝空格和特殊字符引起的 Bug
        if env_vars.get("ZIP_PASSWORD"):
            # 强制转换为字符串，防止纯数字密码导致 subprocess 崩溃！
            zip_pwd = str(env_vars["ZIP_PASSWORD"])
            unzip_cmd = ["unzip", "-P", zip_pwd, "-o", backup_path, "-d", data_dir]
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
    
    # 此时 config.yaml 百分之百存在（已被 startup_event 创建）
    with open(config_file_path, "r", encoding="utf-8") as f:
        config_content = f.read()
    
    # 查找并替换 WEB_USER 和 WEB_PASS
    
    # 用 json.dumps 完美包裹字符串，变成 "Abcd@123#"，彻底杜绝 YAML 解析错误
    safe_user = json.dumps(username)
    safe_pass = json.dumps(password)
    
    config_content = re.sub(r'^#?\s*WEB_USER:.*$', lambda _: f'WEB_USER: {safe_user}', config_content, flags=re.MULTILINE)
    config_content = re.sub(r'^#?\s*WEB_PASS:.*$', lambda _: f'WEB_PASS: {safe_pass}', config_content, flags=re.MULTILINE)
    
    if 'WEB_USER:' not in config_content:
        config_content += f'\n# Web 面板账号密码\nWEB_USER: {safe_user}\nWEB_PASS: {safe_pass}\n'
    
    # 保存配置
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
    
    # 生成会话令牌
    session_token = hashlib.sha256((username + password).encode()).hexdigest()
    
    # 重定向到主页，并设置会话 Cookie
    response = RedirectResponse(url="/", status_code=303)
    response.set_cookie("auth_token", session_token, httponly=True, max_age=86400)
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
    
    # 强制将读取到的变量转换为字符串，防止纯数字密码对比失败
    saved_user = str(env_vars.get("WEB_USER", ""))
    saved_pass = str(env_vars.get("WEB_PASS", ""))
    
    # 验证用户名密码
    if username == saved_user and password == saved_pass:
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