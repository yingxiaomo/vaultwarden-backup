# 核心共享模块
from fastapi import Request, Depends
from fastapi.responses import RedirectResponse
from config import get_env_vars
import hashlib

# 定义鉴权失败的拦截异常
class RequiresSetupException(Exception): pass
class RequiresLoginException(Exception): pass

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