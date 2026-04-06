# 认证服务
import hashlib

from app_config import get_env_vars

class AuthService:
    """认证服务"""
    
    @staticmethod
    def verify_auth(username, password):
        """验证认证信息"""
        env_vars = get_env_vars()
        
        # 强制将读取到的变量转换为字符串，防止纯数字密码对比失败
        saved_user = str(env_vars.get("WEB_USER", ""))
        saved_pass = str(env_vars.get("WEB_PASS", ""))
        
        return username == saved_user and password == saved_pass
    
    @staticmethod
    def generate_token(username, password):
        """生成会话令牌"""
        return hashlib.sha256((username + password).encode()).hexdigest()
    
    @staticmethod
    def verify_token(token):
        """验证会话令牌"""
        env_vars = get_env_vars()
        
        # 强制将读取到的变量转换为字符串，防止纯数字密码对比失败
        saved_user = str(env_vars.get("WEB_USER", ""))
        saved_pass = str(env_vars.get("WEB_PASS", ""))
        
        expected_token = hashlib.sha256((saved_user + saved_pass).encode()).hexdigest()
        return token == expected_token
    
    @staticmethod
    def is_initialized():
        """检查是否已初始化"""
        env_vars = get_env_vars()
        return bool(env_vars.get("WEB_USER") and env_vars.get("WEB_PASS"))
