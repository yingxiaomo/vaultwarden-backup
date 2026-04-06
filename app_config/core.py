import os
import yaml
import json
import shlex
import subprocess
import re

# 配置文件路径
import os
# 检查是否在 Docker 容器内（通过检查环境变量或文件路径）
def is_docker():
    """检测是否在 Docker 容器内"""
    # 方法 1: 检查环境变量
    if os.environ.get("DOCKER_CONTAINER") == "true":
        return True
    # 方法 2: 检查 /proc/1/cgroup 文件
    try:
        with open("/proc/1/cgroup", "r") as f:
            return "docker" in f.read()
    except:
        pass
    # 方法 3: 检查是否存在 .dockerenv 文件
    if os.path.exists("/.dockerenv"):
        return True
    return False

if is_docker():
    # 在 Docker 容器内
    CONFIG_FILE = "/app/config/config.yaml"
    ENV_FILE = "/app/env.sh"
else:
    # 在本地开发环境
    CONFIG_FILE = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "config", "config.yaml")
    ENV_FILE = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "env.sh")

# 读取配置：优先读 config.yaml，如果没有则返回空配置
def get_env_vars():
    print(f"CONFIG_FILE: {CONFIG_FILE}")
    print(f"CONFIG_FILE exists: {os.path.exists(CONFIG_FILE)}")
    if os.path.exists(CONFIG_FILE):
        try:
            with open(CONFIG_FILE, "r", encoding="utf-8") as f:
                config = yaml.safe_load(f) or {}
                print(f"Config loaded: {config}")
                return config
        except Exception as e:
            print(f"配置文件解析失败: {e}，将返回空配置")
            return {}
    
    # 如果不存在，返回空配置，触发初始化流程
    print("Config file not found, returning empty config")
    return {}

# 保存配置
def save_env_vars(env_vars):
    # 同步更新 env.sh
    with open(ENV_FILE, "w", encoding="utf-8") as f:
        for key, value in env_vars.items():
            if value is not None:
                safe_val = shlex.quote(str(value))
                f.write(f"export {key}={safe_val}\n")
    
    # 强制刷新 Crontab
    cron_schedule = env_vars.get("CRON_SCHEDULE", "0 2 * * *")
    cron_cmd = f"{cron_schedule} . /app/env.sh && /app/lib/scripts/backup.sh > /proc/1/fd/1 2>/proc/1/fd/2\n"
    with open("/tmp/crontab.txt", "w") as f:
        f.write(cron_cmd)
    subprocess.run(["crontab", "/tmp/crontab.txt"])

# 生成默认配置：使用内置硬编码模板并吸收 docker-compose 环境变量
def create_default_config():
    # 1. 唯一事实来源：内置的硬编码模板
    config_content = """# Vaultwarden 备份配置文件
# 请根据实际情况修改以下配置

DB_TYPE: sqlite
BACKUP_PREFIX: vaultwarden_backup
BACKUP_DIR: /backup
DATA_DIR: /data
CRON_SCHEDULE: 0 2 * * *
RUN_ON_STARTUP: 'true'
LOCAL_BACKUP_KEEP_DAYS: '15'
# 恢复前创建临时备份（可能会导致磁盘空间翻倍，建议在磁盘空间充足时启用）
CREATE_TEMP_BACKUP: 'true'
# ZIP_PASSWORD: your_secure_zip_password
# RCLONE_KEEP_DAYS: '15'
# RCLONE_REMOTE: my_onedrive:/vaultwarden_backup
# APPRISE_URL: tgram://bottoken/ChatID
# APPRISE_API_URL: http://apprise:8000  
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
