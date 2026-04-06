# Docker 工具函数
import os

def get_vaultwarden_containers(client):
    """获取 Vaultwarden 容器列表"""
    if not client:
        return []
    
    # 查找 Vaultwarden 容器
    all_found = client.containers.list(filters={"name": "vaultwarden"})
    # 获取当前容器的短 ID (HOSTNAME)
    current_id = os.environ.get("HOSTNAME", "")
    # 修正：使用包含匹配，支持 Docker Compose 自动生成的容器名称
    vaultwarden_containers = [c for c in all_found if "vaultwarden" in c.name and not c.id.startswith(current_id)]
    
    return vaultwarden_containers
