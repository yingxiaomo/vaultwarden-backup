# 配置路由
from fastapi import APIRouter, Request, Form, Depends
from fastapi.responses import HTMLResponse, RedirectResponse
from fastapi.templating import Jinja2Templates
import os
import yaml

from app_config import get_env_vars, save_env_vars
from src.core import verify_auth

# 初始化模板
templates = Jinja2Templates(directory="templates")

router = APIRouter()

# 配置页面 (极简版)
@router.get("/config", response_class=HTMLResponse, dependencies=[Depends(verify_auth)])
async def config(request: Request):
    config_file_path = "/app/config/config.yaml"
    
    # 因为有 startup_event，此时文件百分之百存在，直接读取！
    with open(config_file_path, "r", encoding="utf-8") as f:
        config_content = f.read()
        
    return templates.TemplateResponse(request=request, name="config.html", context={
        "request": request,
        "config_content": config_content
    })

# 保存配置 (升级版：原样保存 YAML，防止注释丢失，并同步生成 env.sh)
@router.post("/save_config", dependencies=[Depends(verify_auth)])
async def save_config(request: Request, yaml_content: str = Form(...)):
    config_file_path = "/app/config/config.yaml"
    
    # 1. 语法检查防线：防止用户把 YAML 写错了导致整个系统崩溃
    try:
        env_vars = yaml.safe_load(yaml_content) or {}
    except Exception as e:
        # 语法错误，直接退回并报错，绝不覆盖原文件
        return templates.TemplateResponse(request=request, name="error.html", context={
            "request": request,
            "error": f"YAML 语法格式错误，保存失败，请检查空格和缩进: {e}"
        })
        
    # 2. 如果语法正确，将带有注释的【原生文本】直接覆盖保存
    os.makedirs(os.path.dirname(config_file_path), exist_ok=True)
    with open(config_file_path, "w", encoding="utf-8") as f:
        f.write(yaml_content)
    
    # 3. 同步更新 env.sh 和 Crontab
    save_env_vars(env_vars)
    
    return RedirectResponse("/", status_code=303)
