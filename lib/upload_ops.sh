#!/bin/bash

# ==========================================
# 上传操作函数库
# ==========================================

# 确保 utils.sh 被加载（提供 send_notification 等基础函数）
if ! type send_notification >/dev/null 2>&1; then
    source "$(dirname "${BASH_SOURCE[0]}")/utils.sh"
fi

# 测试 Rclone 远程连接
test_rclone_connection() {
    local remote="$1"
    local remote_name="${remote%%:*}"
    
    echo "正在测试 Rclone 远程连接..."
    if ! rclone listremotes | grep -q "^${remote_name}:"; then
        echo "错误: 找不到 Rclone 配置的远程端: ${remote_name}"
        send_notification "Vaultwarden 备份失败" "找不到 Rclone 远程配置: ${remote_name}，请检查 Rclone 配置文件。"
        return 1
    else
        echo "Rclone 远程连接测试通过。"
        return 0
    fi
}

# 使用 Rclone 上传文件（带重试机制）
upload_with_rclone() {
    local file="$1"
    local remote="$2"
    
    echo "正在使用 Rclone 上传备份到 $remote..."
    
    # 带重试的上传逻辑（最多重试 3 次，应对网络波动）
    local upload_success=0
    local max_retries=3
    local retry_delay=5
    
    for i in $(seq 1 $max_retries); do
        echo "上传尝试 $i/$max_retries..."
        if rclone copy "$file" "$remote"; then
            upload_success=1
            break
        fi
        if [ $i -lt $max_retries ]; then
            echo "上传失败，${retry_delay} 秒后重试..."
            sleep $retry_delay
            retry_delay=$((retry_delay * 2))
        fi
    done
    
    if [ $upload_success -ne 1 ]; then
        echo "错误: Rclone 上传失败（已重试 $max_retries 次）！"
        send_notification "Vaultwarden 备份失败" "Rclone 上传到远程存储失败（已重试 $max_retries 次）。"
        return 1
    else
        echo "Rclone 上传成功。"
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
    # 使用 filter 精确控制：只匹配备份文件前缀，排除其他文件
    # 注意：rclone filter 规则按顺序执行，最后匹配的规则决定文件命运
    rclone delete "$remote" \
        --filter "+ ${prefix}_*.zip" \
        --filter "- *" \
        --min-age "${keep_days}d" \
        --verbose 2>&1 | head -20
    
    echo "远端清理完成。"
    return 0
}

# 执行 Rclone 上传和清理
exec_rclone_ops() {
    local zip_file="$1"
    local rclone_remote="$2"
    local prefix="$3"
    local rclone_keep_days="$4"
    
    if [ -n "$rclone_remote" ]; then
        # 测试 Rclone 远程连接
        test_rclone_connection "$rclone_remote" || return 1
        
        # 上传文件
        upload_with_rclone "$zip_file" "$rclone_remote" || return 1
        
        # 清理远端过期备份
        cleanup_remote_backups "$rclone_remote" "$prefix" "$rclone_keep_days"
    else
        echo "未配置 RCLONE_REMOTE，跳过云端上传，备份仅保留在本地。"
    fi
    
    return 0
}
