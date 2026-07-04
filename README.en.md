# Vaultwarden Backup

[![GHCR](https://img.shields.io/badge/GitHub%20Container%20Registry-ghcr.io-blue?logo=github)](https://github.com/yingxiaomo/vaultwarden-backup/pkgs/container/vaultwarden-backup)
[![Build](https://github.com/yingxiaomo/vaultwarden-backup/actions/workflows/docker-publish.yml/badge.svg)](https://github.com/yingxiaomo/vaultwarden-backup/actions)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## Configuration

All settings can be configured through the Web Panel Config page, no manual editing of environment variables needed.

### First Time Setup

1. Start the container and visit `http://your-server-ip:9876`
2. Follow the setup wizard to create admin account and basic settings
3. Log in and adjust backup schedules, notifications, cloud storage on the Config page

### Web Panel Features

| Feature | Description |
|---------|-------------|
| **Dashboard** | View backup history, trigger manual backup, restore backups |
| **Config** | Modify database, schedule, cloud storage, notification settings |

Configuration is persisted in the `./config:/app/config` volume mount and survives container restarts.


## 📢 Apprise Notification Configuration Examples

Apprise supports many platforms. Here are some common platform configuration formats:

- **Telegram**: `tgram://bottoken/ChatID`
- **DingTalk**: `dingtalk://Token`
- **WeCom**: `wxtx://CorpID/CorpSecret/AgentID`
- **ServerChan**: `schan://SendKey`
- **Discord**: `discord://webhook_id/webhook_token`
- **Email (SMTP)**: `mailto://user:pass@domain.com`

For more platform support and detailed configuration methods, please refer to the [Apprise official documentation](https://github.com/caronc/apprise).

## 📁 Rclone Configuration Reference

Rclone supports multiple cloud storage services. For detailed configuration methods, please refer to the official documentation:

- [Rclone Remote Setup Official Documentation](https://rclone.org/remote_setup/)
- [Rclone S3 Configuration Official Documentation](https://rclone.org/s3/)
- [Rclone FTP Configuration Official Documentation](https://rclone.org/ftp/)

## 📄 License

This project is licensed under the [MIT License](LICENSE).