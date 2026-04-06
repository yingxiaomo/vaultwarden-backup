#!/bin/bash

# ==========================================
# 上传操作函数库
# ==========================================

# 确保 utils.sh 被加载 (提供 send_notification 等基础函数)
if ! type send_notification >/dev/null 2>&1; then
    source "$(dirname "${BASH_SOURCE[0]}")/utils.sh"
fi

# 测试 Rclone 远程连接
test_rclone_connection() {
    local remote="$1"
    local remote_name="${remote%%:*}"
    
    echo "正在测试 Rclone 远程连接..."
    if ! rclone listremotes | grep -q "^${remote_name}:"; then
        echo "❌ 警告: 找不到 Rclone 配置的远程端: ${remote_name}"
        send_notification "Vaultwarden 备份警告 ⚠️" "找不到 Rclone 配置的远程端: ${remote_name}，将尝试继续上传。"
        return 1
    else
        echo "✅ Rclone 远程连接测试通过。"
        return 0
    fi
}

# 使用 Rclone 上传文件
upload_with_rclone() {
    local file="$1"
    local remote="$2"
    
    echo "正在使用 Rclone 上传备份到 $remote..."
    # 使用 rclone copy 上传文件
    rclone copy "$file" "$remote"
    
    if [ $? -ne 0 ]; then
        echo "错误: Rclone 上传失败！"
        send_notification "Vaultwarden 备份失败 ❌" "Rclone 上传到远程存储失败。"
        return 1
    else
        echo "✅ Rclone 上传成功。"
        return 0
    fi
}

# 清理远端过期备份
cleanup_remote_backups() {
    local remote="$1"
    local prefix="$2"
    local keep_days="$3"
    
    if [ -z "$keep_days" ]; then
        echo "未设置 RCLONE_KEEP_DAYS，跳过远端清理。"
        return 0
    fi
    
    echo "正在清理远端过期备份（保留 $keep_days 天）..."
    # 加入 --include 过滤，绝对防止误删用户网盘里的其他私人文件！
    rclone delete "$remote" --include "${prefix}_*.zip" --min-age "${keep_days}d"
    
    if [ $? -eq 0 ]; then
        echo "✅ 远端过期备份清理成功。"
        return 0
    else
        echo "错误: 远端过期备份清理失败！"
        return 1
    fi
}

# 执行 Rclone 上传和清理
exec_rclone_ops() {
    local zip_file="$1"
    local rclone_remote="$2"
    local prefix="$3"
    local rclone_keep_days="$4"
    
    if [ -n "$rclone_remote" ]; then
        # 测试 Rclone 远程连接
        test_rclone_connection "$rclone_remote"
        
        # 上传文件
        if ! upload_with_rclone "$zip_file" "$rclone_remote"; then
            return 1
        fi
        
        # 清理远端过期备份
        cleanup_remote_backups "$rclone_remote" "$prefix" "$rclone_keep_days"
    else
        echo "未配置 RCLONE_REMOTE，跳过云端上传，备份仅保留在本地。"
    fi
    
    return 0
}
