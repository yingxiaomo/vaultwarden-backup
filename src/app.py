from fastapi import FastAPI, Request
from fastapi.responses import RedirectResponse
from fastapi.templating import Jinja2Templates
from fastapi.staticfiles import StaticFiles
from contextlib import asynccontextmanager
import os
import shlex
import sys
import tempfile

# 添加项目根目录到sys.path
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

# 导入配置管理模块
from app_config import get_env_vars, save_env_vars, create_default_config

# 导入核心共享模块
from src.core import RequiresSetupException, RequiresLoginException, verify_auth

# 导入路由模块
from routes import auth_router, backup_router, config_router, restore_router, setup_router, health_router

# 导入工具模块
from utils import run_shell_command

@asynccontextmanager
async def lifespan(app: FastAPI):
    # --- 冷启动配置同步逻辑 ---
    print("🚀 正在执行冷启动配置同步...")
    env_vars = get_env_vars()
    
    # 检查是否已初始化
    saved_user = str(env_vars.get("WEB_USER", ""))
    saved_pass = str(env_vars.get("WEB_PASS", ""))
    
    if saved_user and saved_pass:
        # 已初始化，执行配置同步
        print("✅ 系统已初始化，执行配置同步...")
        
        # 同步 env.sh
        env_file = "/app/env.sh"
        os.makedirs(os.path.dirname(env_file), exist_ok=True)
        with open(env_file, "w", encoding="utf-8") as f:
            for key, value in env_vars.items():
                if value is not None:
                    safe_val = shlex.quote(str(value))
                    f.write(f"export {key}={safe_val}\n")
        
        # 强制刷新 Crontab
        cron_schedule = env_vars.get("CRON_SCHEDULE", "0 2 * * *")
        cron_cmd = f"{cron_schedule} . /app/env.sh && /app/lib/scripts/backup.sh > /proc/1/fd/1 2>/proc/1/fd/2\n"
        with tempfile.NamedTemporaryFile(mode='w', delete=False) as f:
            f.write(cron_cmd)
            temp_file_path = f.name
        try:
            run_shell_command(["crontab", temp_file_path])
        finally:
            os.unlink(temp_file_path)
        print("✅ 配置同步完成！底层守护进程已就绪。")
    else:
        # 未初始化，跳过配置同步
        print("⚠️ 系统未初始化，跳过配置同步，等待用户完成初始化")
    
    yield # 应用运行

# 初始化 FastAPI 应用
app = FastAPI(lifespan=lifespan)

# 挂载静态文件目录
app.mount("/static", StaticFiles(directory="static"), name="static")

# 初始化 Jinja2 模板
templates = Jinja2Templates(directory="templates")

# 注册路由
app.include_router(auth_router)
app.include_router(backup_router)
app.include_router(config_router)
app.include_router(restore_router)
app.include_router(setup_router)
app.include_router(health_router)



# 拦截器：如果没初始化，跳转到 setup
@app.exception_handler(RequiresSetupException)
async def requires_setup_handler(request: Request, exc: RequiresSetupException):
    return RedirectResponse(url="/setup", status_code=303)

# 拦截器：如果没登录，跳转到 login
@app.exception_handler(RequiresLoginException)
async def requires_login_handler(request: Request, exc: RequiresLoginException):
    return RedirectResponse(url="/login", status_code=303)

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=9876)