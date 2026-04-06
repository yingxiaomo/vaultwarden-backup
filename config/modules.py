# 配置模块基类和具体实现

class ConfigModule:
    """配置模块基类"""
    def __init__(self, config_data=None):
        self.config_data = config_data or {}
    
    def to_dict(self):
        """将配置转换为字典"""
        raise NotImplementedError
    
    def from_dict(self, config_data):
        """从字典加载配置"""
        raise NotImplementedError

class CoreConfig(ConfigModule):
    """核心配置模块"""
    def __init__(self, config_data=None):
        super().__init__(config_data)
        self.data_dir = self.config_data.get("DATA_DIR", "/data")
        self.backup_dir = self.config_data.get("BACKUP_DIR", "/backup")
        self.backup_prefix = self.config_data.get("BACKUP_PREFIX", "vaultwarden_backup")
        self.cron_schedule = self.config_data.get("CRON_SCHEDULE", "0 2 * * *")
        self.run_on_startup = self.config_data.get("RUN_ON_STARTUP", "true")
        self.local_backup_keep_days = self.config_data.get("LOCAL_BACKUP_KEEP_DAYS", "15")
        self.create_temp_backup = self.config_data.get("CREATE_TEMP_BACKUP", "true")
        self.timezone = self.config_data.get("TZ", "Asia/Shanghai")
    
    def to_dict(self):
        return {
            "DATA_DIR": self.data_dir,
            "BACKUP_DIR": self.backup_dir,
            "BACKUP_PREFIX": self.backup_prefix,
            "CRON_SCHEDULE": self.cron_schedule,
            "RUN_ON_STARTUP": self.run_on_startup,
            "LOCAL_BACKUP_KEEP_DAYS": self.local_backup_keep_days,
            "CREATE_TEMP_BACKUP": self.create_temp_backup,
            "TZ": self.timezone
        }
    
    def from_dict(self, config_data):
        self.data_dir = config_data.get("DATA_DIR", "/data")
        self.backup_dir = config_data.get("BACKUP_DIR", "/backup")
        self.backup_prefix = config_data.get("BACKUP_PREFIX", "vaultwarden_backup")
        self.cron_schedule = config_data.get("CRON_SCHEDULE", "0 2 * * *")
        self.run_on_startup = config_data.get("RUN_ON_STARTUP", "true")
        self.local_backup_keep_days = config_data.get("LOCAL_BACKUP_KEEP_DAYS", "15")
        self.create_temp_backup = config_data.get("CREATE_TEMP_BACKUP", "true")
        self.timezone = config_data.get("TZ", "Asia/Shanghai")

class DatabaseConfig(ConfigModule):
    """数据库配置模块"""
    def __init__(self, config_data=None):
        super().__init__(config_data)
        self.db_type = self.config_data.get("DB_TYPE", "sqlite")
        self.db_host = self.config_data.get("DB_HOST", "db")
        self.db_port = self.config_data.get("DB_PORT", "3306" if self.db_type == "mysql" else "5432")
        self.db_user = self.config_data.get("DB_USER", "")
        self.db_password = self.config_data.get("DB_PASSWORD", "")
        self.db_name = self.config_data.get("DB_NAME", "")
    
    def to_dict(self):
        return {
            "DB_TYPE": self.db_type,
            "DB_HOST": self.db_host,
            "DB_PORT": self.db_port,
            "DB_USER": self.db_user,
            "DB_PASSWORD": self.db_password,
            "DB_NAME": self.db_name
        }
    
    def from_dict(self, config_data):
        self.db_type = config_data.get("DB_TYPE", "sqlite")
        self.db_host = config_data.get("DB_HOST", "db")
        self.db_port = config_data.get("DB_PORT", "3306" if self.db_type == "mysql" else "5432")
        self.db_user = config_data.get("DB_USER", "")
        self.db_password = config_data.get("DB_PASSWORD", "")
        self.db_name = config_data.get("DB_NAME", "")

class BackupConfig(ConfigModule):
    """备份配置模块"""
    def __init__(self, config_data=None):
        super().__init__(config_data)
        self.zip_password = self.config_data.get("ZIP_PASSWORD", "")
    
    def to_dict(self):
        return {
            "ZIP_PASSWORD": self.zip_password
        }
    
    def from_dict(self, config_data):
        self.zip_password = config_data.get("ZIP_PASSWORD", "")

class NotificationConfig(ConfigModule):
    """通知配置模块"""
    def __init__(self, config_data=None):
        super().__init__(config_data)
        self.apprise_url = self.config_data.get("APPRISE_URL", "")
        self.apprise_api_url = self.config_data.get("APPRISE_API_URL", "")
    
    def to_dict(self):
        return {
            "APPRISE_URL": self.apprise_url,
            "APPRISE_API_URL": self.apprise_api_url
        }
    
    def from_dict(self, config_data):
        self.apprise_url = config_data.get("APPRISE_URL", "")
        self.apprise_api_url = config_data.get("APPRISE_API_URL", "")

class RemoteStorageConfig(ConfigModule):
    """远程存储配置模块"""
    def __init__(self, config_data=None):
        super().__init__(config_data)
        self.rclone_remote = self.config_data.get("RCLONE_REMOTE", "")
        self.rclone_keep_days = self.config_data.get("RCLONE_KEEP_DAYS", "15")
    
    def to_dict(self):
        return {
            "RCLONE_REMOTE": self.rclone_remote,
            "RCLONE_KEEP_DAYS": self.rclone_keep_days
        }
    
    def from_dict(self, config_data):
        self.rclone_remote = config_data.get("RCLONE_REMOTE", "")
        self.rclone_keep_days = config_data.get("RCLONE_KEEP_DAYS", "15")

class WebConfig(ConfigModule):
    """Web面板配置模块"""
    def __init__(self, config_data=None):
        super().__init__(config_data)
        self.web_user = self.config_data.get("WEB_USER", "")
        self.web_pass = self.config_data.get("WEB_PASS", "")
    
    def to_dict(self):
        return {
            "WEB_USER": self.web_user,
            "WEB_PASS": self.web_pass
        }
    
    def from_dict(self, config_data):
        self.web_user = config_data.get("WEB_USER", "")
        self.web_pass = config_data.get("WEB_PASS", "")
