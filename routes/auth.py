# 认证路由
from fastapi import APIRouter, Request, Form, Depends
from fastapi.responses import HTMLResponse, RedirectResponse
from fastapi.templating import Jinja2Templates

from app_config import get_env_vars
from src.core import verify_auth, RequiresLoginException, RequiresSetupException
from services.auth import AuthService

# 初始化模板
templates = Jinja2Templates(directory="templates")

router = APIRouter()

# 登录页面
@router.get("/login")
async def login(request: Request, error: str = None):
    # 如果未设置账号密码，跳转到初始化页面
    if not AuthService.is_initialized():
        return RedirectResponse(url="/setup", status_code=303)
    return templates.TemplateResponse(request=request, name="login.html", context={
        "request": request,
        "error": error
    })

# 处理登录表单提交
@router.post("/do_login")
async def do_login(request: Request, username: str = Form(...), password: str = Form(...)):
    # 验证用户名密码
    if AuthService.verify_auth(username, password):
        # 生成会话令牌
        session_token = AuthService.generate_token(username, password)
        
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
