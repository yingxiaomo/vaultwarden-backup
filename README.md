# Vaultwarden Backup

[![Docker Pulls](https://img.shields.io/docker/pulls/yingxiaomo/vaultwarden-backup.svg)](https://hub.docker.com/r/yingxiaomo/vaultwarden-backup)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

市面上已经有很多现成的备份项目了，但是他们都是大而全，搞的太复杂了，我需要的只是一个简单的 Vaultwarden 备份解决方案，于是就有了这个东西，它支持 SQLite、MySQL 和 PostgreSQL 数据库，并集成了 Apprise 消息通知和 Rclone 云端同步功能。

## ✨ 核心特性 (Features)

- **多数据库支持**：完美兼容 SQLite (默认)、MySQL/MariaDB 和 PostgreSQL。
- **灵活的打包选项**：支持 AES 加密打包（需设置密码）和非加密打包，满足不同安全需求。
- **压缩包完整性校验**：在上传前自动校验压缩包是否损坏，确保备份文件的可靠性。
- **Rclone 云端同步**：无缝集成 Rclone，支持将备份自动上传至 OneDrive、Google Drive、阿里云盘、WebDAV 等数十种云存储。
- **远端备份自动清理**：支持自动清理云端过期备份，防止云存储空间被占满。
- **Apprise 消息通知**：支持 Telegram、钉钉、企业微信、Server酱、Discord、Email 等几乎所有主流通知平台，备份成功或失败实时掌握。
- **支持独立 Apprise 服务**：可以使用容器内置的 Apprise 命令行工具，也可以连接到独立部署的 Apprise 服务，方便多容器共享通知配置。
- **灵活的定时任务**：基于 Cron 表达式，可自定义备份频率。
- **启动即备份**：支持容器启动时自动执行一次备份，方便验证配置。
- **本地备份自动清理**：自动清理过期的本地备份文件，防止磁盘空间耗尽。
- **自定义备份文件前缀**：支持通过环境变量自定义备份文件前缀，方便多实例管理。
- **数据库备份参数优化**：针对 MySQL 和 PostgreSQL 增加专业参数，提高备份一致性并减少对主程序的影响。
- **Docker 健康检查**：内置 Healthcheck，实时监控定时任务进程状态。
- **Web 面板管理**：提供可视化的 Web 界面，支持配置管理、备份历史查看和一键恢复操作。
- **开箱即用**：支持通过 Web 面板进行全配置，无需修改 docker-compose.yml 文件。

## 🚀 快速开始 (Quick Start)

### 1. 准备工作

确保你已经安装了 Docker 和 Docker Compose。如果你需要使用 Rclone 上传到云端，请先准备好 `rclone.conf` 配置文件。

### 2. 选择配置方式

本项目提供两种配置方式，你可以根据自己的需求选择：

#### 2.1 传统方式：通过环境变量配置

适用于习惯通过 docker-compose.yml 直接配置的用户：

```yaml
version: '3.8'

services:
  vaultwarden-backup:
    image: yingxiaomo/vaultwarden-backup:latest
    container_name: vaultwarden_backup
    restart: on-failure
    environment:
      - DB_TYPE=sqlite
      # 压缩包 AES 加密密码 (可选，如果不设置则进行非加密打包)
      # - ZIP_PASSWORD=your_secure_zip_password
      - CRON_SCHEDULE=0 2 * * *
      - RUN_ON_STARTUP=true
      - LOCAL_BACKUP_KEEP_DAYS=15
      # 远端备份保留天数 (可选，如果不设置则不清理)
      # - RCLONE_KEEP_DAYS=15
      # 备份文件前缀 (默认 vaultwarden_backup)
      # - BACKUP_PREFIX=vaultwarden_backup
      # - RCLONE_REMOTE=my_onedrive:/vaultwarden_backup
      # - APPRISE_URL=tgram://bottoken/ChatID
      # 方式 2: 使用独立的 Apprise 服务 API
      # - APPRISE_API_URL=http://apprise:8000
      # 时区配置 (默认 Asia/Shanghai)
      - TZ=Asia/Shanghai
    volumes:
      - /path/to/your/vw_data:/vw_data:ro
      - ./backups:/backup
      # - ./rclone_config:/config/rclone/
      # 挂载配置目录，用于持久化 Web 面板配置
      - ./config:/app/config
      # 挂载 Docker 套接字，用于控制其他容器
      - /var/run/docker.sock:/var/run/docker.sock
    ports:
      - "9876:9876"  # Web 面板端口
```

#### 2.2 推荐方式：通过 Web 面板配置（极简版）

适用于喜欢通过可视化界面配置的用户，只需保留必要的挂载：

```yaml
version: '3.8'

services:
  vaultwarden-backup-web:
    image: yingxiaomo/vaultwarden-backup:dev
    container_name: vaultwarden_backup_dev
    restart: on-failure
    
    ports:
      - "9876:9876"  # Web 访问端口
    
    volumes:
      # 物理文件的挂载必须保留
      - ./vw_data:/vw_data:ro
      - ./backups:/backup
      - ./rclone_config:/config/rclone/
      - ./config:/app/config  # 用于持久化 Web 面板配置（config.yaml）
      - /var/run/docker.sock:/var/run/docker.sock
```

### 3. 启动服务

在 `docker-compose.yml` 所在目录下运行：

```bash
docker-compose up -d
```

### 4. 使用 Web 面板管理（推荐）

从 v2.0 版本开始，我们提供了可视化的 Web 面板，支持完全通过网页进行配置管理。

#### 4.1 访问 Web 面板

启动容器后，在浏览器中访问：

```
http://localhost:9876
```

#### 4.2 配置说明

1. **首次访问**：容器启动时会自动在 `./config` 目录中初始化 `config.yaml` 文件，Web 面板会显示默认配置。
2. **修改配置**：在 Web 面板中修改配置后，点击 "保存配置" 按钮，配置会自动保存到 `./config/config.yaml` 文件中。
3. **持久化**：由于配置文件被映射到宿主机的 `./config` 目录，即使容器重启，配置也会永久保存。
4. **配置迁移**：只需拷贝 `./config` 目录，即可在不同机器间快速迁移配置。
5. **一键操作**：Web 面板支持一键执行备份和恢复操作，方便快捷。

#### 4.3 config.yaml 示例

完整的配置示例请参考项目根目录下的 `config.yaml.example` 文件，该文件包含所有可用的配置项和详细说明。

## ⚙️ 环境变量说明 (Environment Variables)

| 环境变量                     | 说明                                        | 默认值                  | 必填 |
| :----------------------- | :---------------------------------------- | :------------------- | :- |
| `DB_TYPE`                | 数据库类型 (`sqlite`, `mysql`, `postgres`)     | `sqlite`             | 否  |
| `ZIP_PASSWORD`           | 压缩包 AES 加密密码 (可选，如果不设置则进行非加密打包)           | 无                    | 否  |
| `CRON_SCHEDULE`          | 定时备份的 Cron 表达式                            | `0 2 * * *` (每天凌晨2点) | 否  |
| `RUN_ON_STARTUP`         | 容器启动时是否立即执行一次备份 (`true`/`false`)          | `true`               | 否  |
| `LOCAL_BACKUP_KEEP_DAYS` | 本地备份保留天数                                  | `15`                 | 否  |
| `RCLONE_KEEP_DAYS`       | 远端备份保留天数 (可选，如果不设置则不清理)                   | 无                    | 否  |
| `BACKUP_PREFIX`          | 备份文件前缀                                    | `vaultwarden_backup` | 否  |
| `APPRISE_URL`            | Apprise 通知 URL (直接使用命令行工具)                | 无                    | 否  |
| `APPRISE_API_URL`        | Apprise 服务 API 地址 (使用独立 Apprise 服务)       | 无                    | 否  |
| `RCLONE_REMOTE`          | Rclone 远程路径 (例如 `myremote:/backup`)       | 无                    | 否  |
| `DATA_DIR`               | Vaultwarden 数据目录路径                        | `/vw_data`           | 否  |
| `BACKUP_DIR`             | 本地临时备份目录路径                                | `/backup`            | 否  |
| `DB_HOST`                | 数据库主机地址 (MySQL/PostgreSQL)                | `db`                 | 否  |
| `DB_PORT`                | 数据库端口 (MySQL: `3306`, PostgreSQL: `5432`) | 对应默认端口               | 否  |
| `DB_USER`                | 数据库用户名 (MySQL/PostgreSQL)                 | 无                    | 否  |
| `DB_PASSWORD`            | 数据库密码 (MySQL/PostgreSQL)                  | 无                    | 否  |
| `DB_NAME`                | 数据库名称 (MySQL/PostgreSQL)                  | 无                    | 否  |
| `HTTP_PROXY`             | HTTP 代理地址                                 | 无                    | 否  |
| `HTTPS_PROXY`            | HTTPS 代理地址                                | 无                    | 否  |
| `ALL_PROXY`              | SOCKS5 代理地址                               | 无                    | 否  |
| `TZ`                     | 时区设置                                      | `Asia/Shanghai`      | 否  |

## 📢 Apprise 通知配置示例

Apprise 支持非常多的平台，以下是一些常见平台的配置格式：

- **Telegram**: `tgram://bottoken/ChatID`
- **钉钉 (DingTalk)**: `dingtalk://Token`
- **企业微信**: `wxtx://CorpID/CorpSecret/AgentID`
- **Server酱**: `schan://SendKey`
- **Discord**: `discord://webhook_id/webhook_token`
- **Email (SMTP)**: `mailto://user:pass@domain.com`

更多平台支持和详细配置方法，请参考 [Apprise 官方文档](https://github.com/caronc/apprise)。

## 📄 许可证 (License)

本项目采用 [MIT License](LICENSE) 开源许可证。
