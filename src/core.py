# 核心共享模块
from fastapi import Request, Depends
from fastapi.responses import RedirectResponse
from services.auth import AuthService

# 定义鉴权失败的拦截异常
class RequiresSetupException(Exception): pass
class RequiresLoginException(Exception): pass

# 核心安全守卫 (取代了旧的 HTTPBasic)
def verify_auth(request: Request):
    # 1. 检查是否是首次运行（未设置账号密码）
    if not AuthService.is_initialized():
        raise RequiresSetupException()
    
    # 2. 检查会话 Cookie 是否合法
    session_token = request.cookies.get("auth_token")
    if not session_token:
        raise RequiresLoginException()
    
    # 3. 验证会话令牌
    if not AuthService.verify_token(session_token):
        raise RequiresLoginException()
    
    return True