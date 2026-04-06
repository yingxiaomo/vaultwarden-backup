#!/usr/bin/env python3
# 测试配置管理模块

import sys
import os

# 添加项目根目录到 Python 路径
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from config import get_env_vars, save_env_vars, create_default_config
from config.modules import CoreConfig, DatabaseConfig, BackupConfig, NotificationConfig, RemoteStorageConfig, WebConfig

print("=== 测试配置管理模块 ===")

# 测试 1: 获取当前配置
print("\n1. 测试获取当前配置:")
env_vars = get_env_vars()
print(f"当前配置: {env_vars}")

# 测试 2: 测试配置模块
print("\n2. 测试配置模块:")

# 测试核心配置
core_config = CoreConfig(env_vars)
print(f"核心配置: {core_config.to_dict()}")

# 测试数据库配置
db_config = DatabaseConfig(env_vars)
print(f"数据库配置: {db_config.to_dict()}")

# 测试备份配置
backup_config = BackupConfig(env_vars)
print(f"备份配置: {backup_config.to_dict()}")

# 测试通知配置
notification_config = NotificationConfig(env_vars)
print(f"通知配置: {notification_config.to_dict()}")

# 测试远程存储配置
remote_config = RemoteStorageConfig(env_vars)
print(f"远程存储配置: {remote_config.to_dict()}")

# 测试 Web 配置
web_config = WebConfig(env_vars)
print(f"Web 配置: {web_config.to_dict()}")

# 测试 3: 测试配置保存
print("\n3. 测试配置保存:")
test_config = {
    "DATA_DIR": "/test/data",
    "BACKUP_DIR": "/test/backup",
    "DB_TYPE": "mysql",
    "DB_HOST": "localhost",
    "DB_PORT": "3306",
    "DB_USER": "test",
    "DB_PASSWORD": "test123",
    "DB_NAME": "vaultwarden",
    "WEB_USER": "test_admin",
    "WEB_PASS": "test_pass"
}

try:
    save_env_vars(test_config)
    print("配置保存成功！")
    
    # 验证保存结果
    new_env_vars = get_env_vars()
    print(f"保存后的配置: {new_env_vars}")
except Exception as e:
    print(f"配置保存失败: {e}")

print("\n=== 测试完成 ===")
