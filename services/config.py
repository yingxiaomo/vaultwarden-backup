# 配置服务
import os
import yaml

from config import get_env_vars, save_env_vars

class ConfigService:
    """配置服务"""
    
    @staticmethod
    def get_config_content():
        """获取配置文件内容"""
        config_file_path = "/app/config/config.yaml"
        
        # 因为有 startup_event，此时文件百分之百存在，直接读取！
        with open(config_file_path, "r", encoding="utf-8") as f:
            config_content = f.read()
        
        return config_content
    
    @staticmethod
    def save_config(yaml_content):
        """保存配置"""
        config_file_path = "/app/config/config.yaml"
        
        # 1. 语法检查防线：防止用户把 YAML 写错了导致整个系统崩溃
        try:
            env_vars = yaml.safe_load(yaml_content) or {}
        except Exception as e:
            # 语法错误，直接抛出异常
            raise Exception(f"YAML 语法格式错误，保存失败，请检查空格和缩进: {e}")
            
        # 2. 如果语法正确，将带有注释的【原生文本】直接覆盖保存
        os.makedirs(os.path.dirname(config_file_path), exist_ok=True)
        with open(config_file_path, "w", encoding="utf-8") as f:
            f.write(yaml_content)
        
        # 3. 同步更新 env.sh 和 Crontab
        save_env_vars(env_vars)
        
        return True
