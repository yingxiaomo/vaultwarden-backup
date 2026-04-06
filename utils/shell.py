# Shell 工具函数
import subprocess

def run_shell_command(command, shell=True, executable=None):
    """运行 shell 命令"""
    try:
        result = subprocess.run(command, shell=shell, executable=executable, capture_output=True, text=True)
        return result
    except Exception as e:
        print(f"运行命令失败: {e}")
        return None
