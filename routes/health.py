# 健康检查路由
from fastapi import APIRouter

router = APIRouter()

# 健康检查接口（不需要身份验证）
@router.get("/health")
async def health_check():
    return {"status": "ok"}
