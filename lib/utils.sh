#!/bin/bash

# ==========================================
# 通用工具函数库
# ==========================================

# 发送通知的辅助函数
send_notification() {
    local title="$1"  # 通知标题
    local body="$2"   # 通知内容
    
    # 优先使用独立的 Apprise 服务 API
    if [ -n "$APPRISE_API_URL" ]; then
        echo "使用独立 Apprise 服务发送通知..."
        
        # 如果同时配置了 API 地址和具体的通知 URL (Stateless 模式)
        if [ -n "$APPRISE_URL" ]; then
            # 解决 JSON 换行符报错问题：将实际换行符替换为文本 "\n"
            local safe_body="${body//$'\n'/\\n}"
            # 防止标题或内容中包含双引号导致 JSON 结构破坏
            safe_body="${safe_body//\"/\\\"}"
            local safe_title="${title//\"/\\\"}"
            
            # 使用 -s 隐藏 curl 的进度条
            curl -s -X POST "$APPRISE_API_URL/notify" \
                 -H "Content-Type: application/json" \
                 -d "{\"title\": \"$safe_title\", \"body\": \"$safe_body\", \"urls\": \"$APPRISE_URL\"}"
                
        # 在 API 地址里直接写了配置路径 (如 `http://apprise:8000/notify/mybot)`  (Stateful 模式)
        else
            # 使用 --data-urlencode 提交，完美保留换行符并防止特殊字符截断表单
            curl -s -X POST "$APPRISE_API_URL" \
                 --data-urlencode "title=$title" \
                 --data-urlencode "body=$body"
        fi
        echo "" # 补一个换行避免日志粘连
        
    # 回退使用本地 Apprise 命令行工具
    elif [ -n "$APPRISE_URL" ]; then
        echo "使用本地 Apprise 命令行工具发送通知..."
        apprise -t "$title" -b "$body" "$APPRISE_URL"
    else
        echo "未配置 APPRISE_URL 或 APPRISE_API_URL，跳过通知发送。"
    fi
}

# 检查磁盘空间的函数
check_disk_space() {
    local dir="$1"
    local min_space_mb="$2"
    
    # 使用 df -Pm 以 MB 为单位输出，-P 参数确保 POSIX 标准格式，避免路径太长导致换行
    # 检查备份目录所在分区的可用空间
    local free_space_mb=$(df -Pm "$dir" | awk 'END{print $(NF-2)}' | sed 's/[^0-9]//g')
    
    # 确保获取到的是有效数字，防止 df 失败导致 bash 报错
    if ! [[ "$free_space_mb" =~ ^[0-9]+$ ]]; then
        echo "警告: 无法获取目录 $dir 的磁盘空间信息，跳过空间检查。"
        return 0
    fi
    
    if [ "$free_space_mb" -lt "$min_space_mb" ]; then
        # 获取剩余空间
        local human_free=$(df -h "$dir" | tail -n 1 | awk '{print $4}')
        echo "警告: 备份目录所在磁盘空间不足，剩余空间为 $human_free，小于 ${min_space_mb}MB！"
        send_notification "Vaultwarden 备份警告 ⚠️" "备份目录所在磁盘空间不足，剩余空间为 $human_free，小于 ${min_space_mb}MB，可能导致备份失败。"
        return 1
    else
        return 0
    fi
}

# 清理过期文件的函数
cleanup_old_files() {
    local dir="$1"
    local pattern="$2"
    local keep_days="$3"
    
    echo "正在清理 $keep_days 天前的旧文件..."
    find "$dir" -name "$pattern" -mtime +$keep_days -exec rm {} \;
    echo "已清理 $keep_days 天前的旧文件。"
}
