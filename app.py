from fastapi import FastAPI, Request, Form, Depends, BackgroundTasks, HTTPException, status
from fastapi.responses import HTMLResponse, RedirectResponse
from fastapi.templating import Jinja2Templates
from fastapi.staticfiles import StaticFiles
from fastapi.security import HTTPBasic, HTTPBasicCredentials
import subprocess
import os
import yaml
import datetime
import docker
import secrets

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

# 身份验证配置
security = HTTPBasic()

# 验证凭据
def verify_credentials(credentials: HTTPBasicCredentials = Depends(security)):
    # 从配置中获取用户名和密码
    env_vars = get_env_vars()
    correct_username = secrets.compare_digest(credentials.username, env_vars.get("WEB_USER", "admin"))
    correct_password = secrets.compare_digest(credentials.password, env_vars.get("WEB_PASS", "admin"))
    if not (correct_username and correct_password):
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Incorrect username or password",
            headers={"WWW-Authenticate": "Basic"},
        )
    return credentials

import yaml

CONFIG_FILE = "/app/config/config.yaml"
ENV_FILE = "/app/env.sh"

# 读取配置：优先读 config.yaml，如果没有则尝试从系统的环境变量中初始化一份默认值
def get_env_vars():
    # 如果有 config.yaml，直接读取
    if os.path.exists(CONFIG_FILE):
        with open(CONFIG_FILE, "r", encoding="utf-8") as f:
            env_vars = yaml.safe_load(f)
    else:
        # 如果没有，从 config.yaml.example 中读取带注释的配置
        env_vars = {}
        example_config = "/app/config.yaml.example"
        if os.path.exists(example_config):
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
        else:
            # 如果 example 也不存在，从 env.sh 中提取
            if os.path.exists(ENV_FILE):
                with open(ENV_FILE, "r") as f:
                    for line in f:
                        if line.startswith("export "):
                            key, value = line.strip().split("=", 1)
                            key = key[7:]
                            if value.startswith('"') and value.endswith('"') or value.startswith("'") and value.endswith("'"):
                                value = value[1:-1]
                            env_vars[key] = value
            
            # 第一次运行，把读取到的默认值保存为 YAML
            os.makedirs(os.path.dirname(CONFIG_FILE), exist_ok=True)
            with open(CONFIG_FILE, "w", encoding="utf-8") as f:
                yaml.dump(env_vars, f, default_flow_style=False, allow_unicode=True, indent=2)
    
    # 确保 Web 面板的账号密码始终在字典里，以便前端渲染
    if "WEB_USER" not in env_vars:
        env_vars["WEB_USER"] = "admin"
    if "WEB_PASS" not in env_vars:
        env_vars["WEB_PASS"] = "admin"
        
    return env_vars

# 保存配置：同时保存为 YAML 和 Shell 脚本
def save_env_vars(env_vars):
    # 1. 保存结构化的 config.yaml (Web 面板使用)
    os.makedirs(os.path.dirname(CONFIG_FILE), exist_ok=True)
    with open(CONFIG_FILE, "w", encoding="utf-8") as f:
        yaml.dump(env_vars, f, default_flow_style=False, allow_unicode=True, indent=2)
        
    # 2. 同步生成 env.sh (底层 Bash 脚本使用)
    with open(ENV_FILE, "w") as f:
        for key, value in env_vars.items():
            f.write(f"export {key}='{value}'\n")

# 后台任务：执行备份
def run_backup_script():
    subprocess.run("source /app/env.sh && /app/backup.sh", shell=True, executable="/bin/bash")

# 主页面
@app.get("/", response_class=HTMLResponse, dependencies=[Depends(verify_credentials)])
async def root(request: Request):
    # 获取磁盘空间
    try:
        # 使用 df -h 获取人类可读的磁盘空间信息（用于前端显示）
        df_output = subprocess.check_output(["df", "-h"], universal_newlines=True)
        disk_space = df_output.splitlines()
        
        # 使用 df -m 获取以 MB 为单位的磁盘空间信息（用于后续可能的空间不足提示逻辑）
        df_m_output = subprocess.check_output(["df", "-m"], universal_newlines=True)
        disk_space_mb = df_m_output.splitlines()
        
        # 提取根目录的剩余空间（用于示例）
        root_free_space = None
        for line in disk_space_mb:
            if line.startswith("/"):
                parts = line.split()
                if len(parts) >= 4:
                    root_free_space = parts[3]
                    break
    except Exception as e:
        disk_space = [f"获取磁盘空间失败: {e}"]
        disk_space_mb = []
        root_free_space = None
    
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
        "disk_space": disk_space,
        "backup_history": backup_history,
        "env_vars": env_vars
    })

# 配置页面 (升级版：直接返回纯文本，保留注释)
@app.get("/config", response_class=HTMLResponse, dependencies=[Depends(verify_credentials)])
async def config(request: Request):
    config_file_path = "/app/config/config.yaml"
    config_content = ""
    
    if os.path.exists(config_file_path):
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
@app.post("/save_config", dependencies=[Depends(verify_credentials)])
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
@app.get("/restore", response_class=HTMLResponse, dependencies=[Depends(verify_credentials)])
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
@app.post("/do_restore", dependencies=[Depends(verify_credentials)])
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
@app.post("/do_backup", dependencies=[Depends(verify_credentials)])
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
@app.get("/logs", dependencies=[Depends(verify_credentials)])
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

# 健康检查接口（不需要身份验证）
@app.get("/health")
async def health_check():
    return {"status": "ok"}

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=9876)