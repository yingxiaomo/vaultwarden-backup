# 备份路由
from fastapi import APIRouter, Request, BackgroundTasks, Depends
from fastapi.responses import HTMLResponse, RedirectResponse
from fastapi.templating import Jinja2Templates
import os
import datetime

from config import get_env_vars
from src.core import verify_auth
from services.backup import BackupService

# 初始化模板
templates = Jinja2Templates(directory="templates")

router = APIRouter()

# 主页面
@router.get("/", response_class=HTMLResponse, dependencies=[Depends(verify_auth)])
async def root(request: Request):
    # 获取环境变量
    env_vars = get_env_vars()
    
    # 获取备份历史
    backup_history = BackupService.get_backup_history()
    
    return templates.TemplateResponse(request=request, name="index.html", context={
        "request": request,
        "backup_history": backup_history,
        "env_vars": env_vars
    })

# 执行备份
@router.post("/do_backup", dependencies=[Depends(verify_auth)])
async def do_backup(request: Request, background_tasks: BackgroundTasks):
    try:
        # 添加后台任务
        background_tasks.add_task(BackupService.run_backup_script)
        # 立即返回，避免网页卡顿
        return RedirectResponse("/", status_code=303)
    except Exception as e:
        print(f"备份失败: {e}")
        return templates.TemplateResponse(request=request, name="error.html", context={
            "request": request,
            "error": f"备份失败: {e}"
        })

# 获取备份日志
@router.get("/logs", dependencies=[Depends(verify_auth)])
async def get_backup_logs():
    return BackupService.get_backup_logs()
