# 认证路由
from fastapi import APIRouter, Request, Form, Depends
from fastapi.responses import HTMLResponse, RedirectResponse
from fastapi.templating import Jinja2Templates
import hashlib

from config import get_env_vars
from src.core import verify_auth, RequiresLoginException, RequiresSetupException

# 初始化模板
templates = Jinja2Templates(directory="templates")

router = APIRouter()

# 登录页面
@router.get("/login")
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
@router.post("/do_login")
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
@router.get("/logout")
async def logout(request: Request):
    response = RedirectResponse(url="/login", status_code=303)
    response.delete_cookie("auth_token")
    return response
