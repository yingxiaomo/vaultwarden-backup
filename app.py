from fastapi import FastAPI, Request, Form, Depends
from fastapi.responses import HTMLResponse, RedirectResponse
from fastapi.templating import Jinja2Templates
from fastapi.staticfiles import StaticFiles
import subprocess
import os
import json
import datetime
import docker

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

import json

CONFIG_FILE = "/app/config/config.json"
ENV_FILE = "/app/env.sh"

# 读取配置：优先读 config.json，如果没有则尝试从系统的环境变量中初始化一份默认值
def get_env_vars():
    # 如果有 config.json，直接读取
    if os.path.exists(CONFIG_FILE):
        with open(CONFIG_FILE, "r") as f:
            return json.load(f)
            
    # 如果没有，从现有的 env.sh 或者 os.environ 中提取默认值，并创建 config.json
    env_vars = {}
    if os.path.exists(ENV_FILE):
        with open(ENV_FILE, "r") as f:
            for line in f:
                if line.startswith("export "):
                    key, value = line.strip().split("=", 1)
                    key = key[7:]
                    if value.startswith('"') and value.endswith('"') or value.startswith("'") and value.endswith("'"):
                        value = value[1:-1]
                    env_vars[key] = value
    
    # 第一次运行，把读取到的默认值保存为 JSON
    os.makedirs(os.path.dirname(CONFIG_FILE), exist_ok=True)
    with open(CONFIG_FILE, "w") as f:
        json.dump(env_vars, f, indent=4)
        
    return env_vars

# 保存配置：同时保存为 JSON 和 Shell 脚本
def save_env_vars(env_vars):
    # 1. 保存结构化的 config.json (Web 面板使用)
    os.makedirs(os.path.dirname(CONFIG_FILE), exist_ok=True)
    with open(CONFIG_FILE, "w") as f:
        json.dump(env_vars, f, indent=4)
        
    # 2. 同步生成 env.sh (底层 Bash 脚本使用)
    with open(ENV_FILE, "w") as f:
        for key, value in env_vars.items():
            f.write(f"export {key}='{value}'\n")

# 主页面
@app.get("/", response_class=HTMLResponse)
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
    
    return templates.TemplateResponse("index.html", {
        "request": request,
        "disk_space": disk_space,
        "backup_history": backup_history,
        "env_vars": env_vars
    })

# 配置页面
@app.get("/config", response_class=HTMLResponse)
async def config(request: Request):
    env_vars = get_env_vars()
    return templates.TemplateResponse("config.html", {
        "request": request,
        "env_vars": env_vars
    })

# 保存配置
@app.post("/save_config")
async def save_config(request: Request, **form_data):
    env_vars = get_env_vars()
    
    # 更新环境变量
    for key, value in form_data.items():
        if key in env_vars:
            env_vars[key] = value
    
    # 保存环境变量
    save_env_vars(env_vars)
    
    return RedirectResponse("/", status_code=303)

# 恢复页面
@app.get("/restore", response_class=HTMLResponse)
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
    
    return templates.TemplateResponse("restore.html", {
        "request": request,
        "backup_history": backup_history
    })

# 执行恢复
@app.post("/do_restore")
async def do_restore(request: Request, backup_file: str = Form(...)):
    try:
        # 获取最新的环境变量
        env_vars = get_env_vars()
        
        # 停止 Vaultwarden 容器
        if client:
            vaultwarden_containers = client.containers.list(filters={"name": "vaultwarden"})
            for container in vaultwarden_containers:
                container.stop()
                print(f"已停止 Vaultwarden 容器: {container.name}")
        
        # 解压恢复
        backup_dir = env_vars.get("BACKUP_DIR", "/backup")
        backup_path = os.path.join(backup_dir, backup_file)
        data_dir = env_vars.get("DATA_DIR", "/vw_data")
        
        # 解压命令
        if env_vars.get("ZIP_PASSWORD"):
            unzip_cmd = f"unzip -P {env_vars['ZIP_PASSWORD']} -o {backup_path} -d {data_dir}"
        else:
            unzip_cmd = f"unzip -o {backup_path} -d {data_dir}"
        
        subprocess.run(unzip_cmd, shell=True, check=True)
        print(f"已恢复备份: {backup_file}")
        
        # 启动 Vaultwarden 容器
        if client:
            for container in vaultwarden_containers:
                container.start()
                print(f"已启动 Vaultwarden 容器: {container.name}")
        
        return RedirectResponse("/", status_code=303)
    except Exception as e:
        print(f"恢复失败: {e}")
        return templates.TemplateResponse("error.html", {
            "request": request,
            "error": f"恢复失败: {e}"
        })

# 执行备份
@app.post("/do_backup")
async def do_backup(request: Request):
    try:
        # 执行备份脚本
        subprocess.run(["/app/backup.sh"], check=True)
        return RedirectResponse("/", status_code=303)
    except Exception as e:
        print(f"备份失败: {e}")
        return templates.TemplateResponse("error.html", {
            "request": request,
            "error": f"备份失败: {e}"
        })

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=9876)