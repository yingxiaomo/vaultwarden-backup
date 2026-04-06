# 备份服务
import os
import datetime
import subprocess

from config import get_env_vars

class BackupService:
    """备份服务"""
    
    @staticmethod
    def get_backup_history():
        """获取备份历史"""
        backup_history = []
        try:
            # 使用 env_vars.get 确保与配置同步
            env_vars = get_env_vars()
            backup_dir = env_vars.get("BACKUP_DIR", "/backup")
            if os.path.exists(backup_dir):
                files = os.listdir(backup_dir)
                for file in files:
                    if file.endswith(".zip"):
                        file_path = os.path.join(backup_dir, file)
                        stat = os.stat(file_path)
                        size = os.path.getsize(file_path) / (1024 * 1024)  # 转换为 MB
                        backup_history.append({
                            "name": file,
                            "size": f"{size:.2f} MB",
                            "mtime": datetime.datetime.fromtimestamp(stat.st_mtime).strftime("%Y-%m-%d %H:%M:%S")
                        })
                backup_history.sort(key=lambda x: x["mtime"], reverse=True)
        except Exception as e:
            backup_history = [f"获取备份历史失败: {e}"]
        return backup_history
    
    @staticmethod
    def run_backup_script():
        """执行备份脚本"""
        try:
            # 执行备份脚本
            subprocess.run("source /app/env.sh && /app/lib/scripts/backup.sh", shell=True, executable="/bin/bash")
            return True
        except Exception as e:
            print(f"备份失败: {e}")
            return False
    
    @staticmethod
    def get_backup_logs():
        """获取备份日志"""
        log_file = "/app/config/backup.log"
        try:
            if os.path.exists(log_file):
                with open(log_file, "r", encoding="utf-8") as f:
                    # 读取最后50行日志
                    lines = f.readlines()
                    last_lines = lines[-50:]
                    log_content = "".join(last_lines)
                    return {"logs": log_content}
            else:
                return {"logs": "日志文件不存在，尚未执行备份任务"}
        except Exception as e:
            return {"logs": f"读取日志失败: {e}"}
