# Vaultwarden Backup

[![Docker Pulls](https://img.shields.io/docker/pulls/yingxiaomo/vaultwarden-backup.svg)](https://hub.docker.com/r/yingxiaomo/vaultwarden-backup)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

There are many existing backup projects out there, but they are often overly complex. I needed a simple backup solution for Vaultwarden, so I created this. It supports SQLite, MySQL, and PostgreSQL databases, and integrates Apprise notification and Rclone cloud synchronization features.

## ✨ Core Features

- **Multi-database support**: Perfectly compatible with SQLite (default), MySQL/MariaDB, and PostgreSQL.
- **Flexible packaging options**: Supports AES encrypted packaging (password required) and unencrypted packaging to meet different security needs.
- **Compression integrity verification**: Automatically verifies if the compressed package is corrupted before upload, ensuring the reliability of backup files.
- **Rclone cloud synchronization**: Seamlessly integrates with Rclone, supporting automatic backup upload to dozens of cloud storage services such as OneDrive, Google Drive, Alibaba Cloud Disk, WebDAV, etc.
- **Remote backup automatic cleanup**: Supports automatic cleanup of expired cloud backups to prevent cloud storage space from being filled up.
- **Apprise notification system**: Supports almost all mainstream notification platforms including Telegram, DingTalk, WeCom, ServerChan, Discord, Email, etc. Real-time updates on backup success or failure.  
- **Support for independent Apprise service**: Can use the built-in Apprise command line tool or connect to an independently deployed Apprise service, facilitating notification configuration sharing across multiple containers. 
- **Flexible scheduled tasks**: Based on Cron expressions, customizable backup frequency.
- **Backup on startup**: Supports automatic execution of a backup when the container starts, convenient for configuration verification.
- **Local backup automatic cleanup**: Automatically cleans up expired local backup files to prevent disk space exhaustion.
- **Custom backup file prefix**: Supports customizing backup file prefixes through environment variables, facilitating multi-instance management.
- **Database backup parameter optimization**: Adds professional parameters for MySQL and PostgreSQL to improve backup consistency and reduce impact on the main program.
- **Docker health check**: Built-in Healthcheck to monitor the status of scheduled task processes in real-time.

## 🚀 Quick Start

### 1. Preparation

Ensure you have Docker and Docker Compose installed. If you need to use Rclone to upload to the cloud, prepare your `rclone.conf` configuration file first.

### 2. Create `docker-compose.en.yml`

Please directly use the [docker-compose.en.yml](docker-compose.en.yml) file in the project root directory as the configuration template, and modify the corresponding environment variables according to your actual needs.

### 3. Start the Service

Run in the directory where `docker-compose.en.yml` is located:

```bash
docker-compose -f docker-compose.en.yml up -d
```

## ⚙ Environment Variables

| Environment Variable       | Description                                        | Default                  | Required |
| :----------------------- | :---------------------------------------- | :------------------- | :--- |
| `DB_TYPE`                | Database type (`sqlite`, `mysql`, `postgres`)     | `sqlite`             | No   |
| `ZIP_PASSWORD`           | Zip file AES encryption password (optional, if not set, unencrypted packaging will be performed)           | None                    | No   |     
| `CRON_SCHEDULE`          | Cron expression for scheduled backup                            | `0 2 * * *` (2 AM daily) | No   |
| `RUN_ON_STARTUP`         | Whether to execute a backup immediately when the container starts (`true`/`false`)          | `true`               | No   |
| `LOCAL_BACKUP_KEEP_DAYS` | Local backup retention days                                  | `15`                 | No   |
| `RCLONE_KEEP_DAYS`       | Remote backup retention days (optional, if not set, no cleanup)                   | None                    | No   |        
| `BACKUP_PREFIX`          | Backup file prefix                                    | `vaultwarden_backup` | No   |
| `APPRISE_URL`            | Apprise notification URL (using command line tool directly)                | None                    | No   |
| `APPRISE_API_URL`        | Apprise service API address (using independent Apprise service)       | None                    | No   |
| `RCLONE_REMOTE`          | Rclone remote path (e.g., `myremote:/backup`)       | None                    | No   |
| `DATA_DIR`               | Vaultwarden data directory path                        | `/data`           | No   |
| `BACKUP_DIR`             | Local temporary backup directory path                                | `/backup`            | No   |
| `DB_HOST`                | Database host address (MySQL/PostgreSQL)                | `db`                 | No   |
| `DB_PORT`                | Database port (MySQL: `3306`, PostgreSQL: `5432`) | Corresponding default port               | No   |
| `DB_USER`                | Database username (MySQL/PostgreSQL)                 | None                    | No   |
| `DB_PASSWORD`            | Database password (MySQL/PostgreSQL)                  | None                    | No   |
| `DB_NAME`                | Database name (MySQL/PostgreSQL)                  | None                    | No   |
| `HTTP_PROXY`             | HTTP proxy address                                 | None                    | No   |
| `HTTPS_PROXY`            | HTTPS proxy address                                | None                    | No   |
| `ALL_PROXY`              | SOCKS5 proxy address                               | None                    | No   |
| `TZ`                     | Timezone setting                                      | `Asia/Shanghai`      | No   |
| `LANGUAGE`               | Language setting (`zh` for Chinese, `en` for English) | `zh`                 | No   |

## 📢 Apprise Notification Configuration Examples

Apprise supports many platforms. Here are some common platform configuration formats:

- **Telegram**: `tgram://bottoken/ChatID`
- **DingTalk**: `dingtalk://Token`
- **WeCom**: `wxtx://CorpID/CorpSecret/AgentID`
- **ServerChan**: `schan://SendKey`
- **Discord**: `discord://webhook_id/webhook_token`
- **Email (SMTP)**: `mailto://user:pass@domain.com`

For more platform support and detailed configuration methods, please refer to the [Apprise official documentation](https://github.com/caronc/apprise).

## 📄 License

This project is licensed under the [MIT License](LICENSE).
