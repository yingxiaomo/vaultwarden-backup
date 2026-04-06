# 文件工具函数
import os
import datetime

def get_file_size(file_path):
    """获取文件大小（MB）"""
    try:
        size = os.path.getsize(file_path) / (1024 * 1024)  # 转换为 MB
        return f"{size:.2f} MB"
    except Exception as e:
        return f"获取文件大小失败: {e}"

def get_file_mtime(file_path):
    """获取文件修改时间"""
    try:
        stat = os.stat(file_path)
        return datetime.datetime.fromtimestamp(stat.st_mtime).strftime("%Y-%m-%d %H:%M:%S")
    except Exception as e:
        return f"获取文件修改时间失败: {e}"
