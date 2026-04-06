# 路由模块
from .auth import router as auth_router
from .backup import router as backup_router
from .config import router as config_router
from .restore import router as restore_router
from .setup import router as setup_router
from .health import router as health_router

__all__ = [
    'auth_router',
    'backup_router',
    'config_router',
    'restore_router',
    'setup_router',
    'health_router'
]
