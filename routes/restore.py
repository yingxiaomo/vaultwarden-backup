# 恢复路由
from fastapi import APIRouter, Request, Form, Depends
from fastapi.responses import HTMLResponse, RedirectResponse
from fastapi.templating import Jinja2Templates
import os
import datetime

from config import get_env_vars
from src.core import verify_auth
from services.backup import BackupService
from services.restore import RestoreService

# 初始化模板
templates = Jinja2Templates(directory="templates")

router = APIRouter()

# 恢复页面
@router.get("/restore", response_class=HTMLResponse, dependencies=[Depends(verify_auth)])
async def restore(request: Request):
    # 获取备份历史
    backup_history = BackupService.get_backup_history()
    
    return templates.TemplateResponse(request=request, name="restore.html", context={
        "request": request,
        "backup_history": backup_history
    })

# 执行恢复
@router.post("/do_restore", dependencies=[Depends(verify_auth)])
async def do_restore(request: Request, backup_file: str = Form(...)):
    try:
        # 执行恢复
        RestoreService.restore_backup(backup_file)
        return RedirectResponse("/", status_code=303)
    except Exception as e:
        print(f"恢复失败: {e}")
        return templates.TemplateResponse(request=request, name="error.html", context={
            "request": request,
            "error": f"恢复失败: {e}"
        })
