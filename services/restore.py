# 恢复服务
import os
import subprocess
import shutil

from config import get_env_vars
from utils.docker import get_vaultwarden_containers

class RestoreService:
    """恢复服务"""
    
    @staticmethod
    def restore_backup(backup_file):
        """恢复备份"""
        try:
            # 获取最新的环境变量
            env_vars = get_env_vars()
            
            # 检查数据目录是否存在
            data_dir = env_vars.get("DATA_DIR", "/data")
            if not os.path.exists(data_dir):
                raise Exception(f"数据目录 {data_dir} 不存在，请检查挂载是否正确")
            
            # 停止 Vaultwarden 容器
            vaultwarden_containers = get_vaultwarden_containers()
            for container in vaultwarden_containers:
                container.stop()
                print(f"已停止 Vaultwarden 容器: {container.name}")
            
            # 解压恢复
            backup_dir = env_vars.get("BACKUP_DIR", "/backup")
            backup_file = os.path.basename(backup_file) # 核心安全防线：强制只保留文件名，丢弃所有路径信息
            backup_path = os.path.join(backup_dir, backup_file)
            
            # [核心修复]：在恢复前创建临时备份，以便在恢复失败时能够回滚
            temp_backup_dir = os.path.join(data_dir, "temp_backup")
            create_temp_backup = str(env_vars.get("CREATE_TEMP_BACKUP", "true")).lower() == "true"
            
            if create_temp_backup and os.path.exists(data_dir):
                print("正在创建恢复前的临时备份...")
                # 清理旧的临时备份目录
                if os.path.exists(temp_backup_dir):
                    shutil.rmtree(temp_backup_dir)
                
                # 使用移动操作而不是拷贝操作，避免磁盘空间翻倍
                # 1. 先创建临时备份目录
                os.makedirs(temp_backup_dir, exist_ok=True)
                
                # 2. 移动所有文件到临时备份目录
                for item in os.listdir(data_dir):
                    item_path = os.path.join(data_dir, item)
                    # 核心修复：排除临时备份目录，且如果备份目录在数据目录内，也要排除它
                    if item != "temp_backup" and not backup_path.startswith(item_path):
                        dest_path = os.path.join(temp_backup_dir, item)
                        shutil.move(item_path, dest_path)
                print("✅ 临时备份创建成功")
            else:
                print("⚠️ 跳过临时备份创建")
            
            # 使用列表形式调用，彻底杜绝空格和特殊字符引起的 Bug
            if env_vars.get("ZIP_PASSWORD"):
                # 强制转换为字符串，防止纯数字密码导致 subprocess 崩溃！
                zip_pwd = str(env_vars["ZIP_PASSWORD"])
                unzip_cmd = ["unzip", "-P", zip_pwd, "-o", backup_path, "-d", data_dir]
            else:
                unzip_cmd = ["unzip", "-o", backup_path, "-d", data_dir]
            
            # 移除 shell=True
            subprocess.run(unzip_cmd, check=True)
            print(f"已恢复备份: {backup_file}")
            
            # 处理 MySQL/PostgreSQL 的 SQL 文件导入
            # 初始化变量，避免 UnboundLocalError
            sql_file = None
            
            db_type = env_vars.get("DB_TYPE", "sqlite").lower()
            
            # 处理 SQLite 恢复：将临时备份文件改回 db.sqlite3
            if db_type == "sqlite":
                print("正在处理 SQLite 数据库恢复...")
                # 查找解压后的临时 SQLite 文件
                temp_db_file = os.path.join(data_dir, "db_dump_temp.db")
                if os.path.exists(temp_db_file):
                    print(f"找到 SQLite 备份文件: {temp_db_file}")
                    # 目标数据库文件路径
                    target_db_file = os.path.join(data_dir, "db.sqlite3")
                    
                    # 清理 SQLite WAL 和 SHM 文件，避免数据不一致
                    wal_file = target_db_file + "-wal"
                    shm_file = target_db_file + "-shm"
                    for file in [wal_file, shm_file]:
                        if os.path.exists(file):
                            os.remove(file)
                            print(f"✅ 已清理 SQLite 日志文件: {file}")
                    
                    # 重命名临时文件为正式数据库文件
                    os.replace(temp_db_file, target_db_file)
                    print(f"✅ 已将 SQLite 备份文件重命名为: {target_db_file}")
                else:
                    print("⚠️ 未找到 SQLite 备份文件，跳过数据库恢复")
            elif db_type in ["mysql", "postgres"]:
                print(f"正在处理 {db_type} 数据库恢复...")
                
                # 查找解压后的 SQL 文件（使用固定的临时文件名，避免匹配错误的旧文件）
                sql_file = os.path.join(data_dir, "db_dump_temp.sql")
                if os.path.exists(sql_file):
                    print(f"找到 SQL 文件: {sql_file}")
                    
                    if db_type == "mysql":
                        # MySQL 导入逻辑
                        db_user = env_vars.get("DB_USER")
                        db_password = env_vars.get("DB_PASSWORD")
                        db_name = env_vars.get("DB_NAME")
                        db_host = env_vars.get("DB_HOST", "db")
                        db_port = env_vars.get("DB_PORT", "3306")
                        
                        if db_user and db_password and db_name:
                            print("正在导入 MySQL 数据库...")
                            # 1. 准备环境变量（最安全的方式）
                            env = os.environ.copy()
                            env["MYSQL_PWD"] = db_password
                            # 2. 构建纯列表命令（不使用 shell=True）
                            mysql_cmd = [
                                "mysql",
                                f"--host={db_host}",
                                f"--port={db_port}",
                                f"--user={db_user}",
                                db_name
                            ]
                            # 3. 使用 Python 的文件流处理重定向，避开 Shell 风险
                            with open(sql_file, "r") as f:
                                subprocess.run(mysql_cmd, stdin=f, env=env, check=True) # 直接传入列表，不使用 shell=True
                            print("✅ MySQL 数据库导入成功")
                        else:
                            print("⚠️ MySQL 数据库信息不完整，跳过导入")
                    
                    elif db_type == "postgres":
                        # PostgreSQL 导入逻辑
                        db_user = env_vars.get("DB_USER")
                        db_password = env_vars.get("DB_PASSWORD")
                        db_name = env_vars.get("DB_NAME")
                        db_host = env_vars.get("DB_HOST", "db")
                        db_port = env_vars.get("DB_PORT", "5432")
                        
                        if db_user and db_password and db_name:
                            print("正在导入 PostgreSQL 数据库...")
                            # 准备环境变量（最安全的方式）
                            env = os.environ.copy()
                            env["PGPASSWORD"] = db_password
                            # 使用 psql 命令导入 SQL 文件
                            psql_cmd = [
                                "psql",
                                f"--host={db_host}",
                                f"--port={db_port}",
                                f"--username={db_user}",
                                f"--dbname={db_name}",
                                "-f",
                                sql_file
                            ]
                            subprocess.run(psql_cmd, env=env, check=True)
                            print("✅ PostgreSQL 数据库导入成功")
                        else:
                            print("⚠️ PostgreSQL 数据库信息不完整，跳过导入")
                else:
                    print("⚠️ 未找到 SQL 文件，跳过数据库导入")
            
            # 启动 Vaultwarden 容器
            vaultwarden_containers = get_vaultwarden_containers()
            for container in vaultwarden_containers:
                container.start()
                print(f"已启动 Vaultwarden 容器: {container.name}")
            
            # 清理临时备份
            temp_backup_dir = os.path.join(data_dir, "temp_backup")
            if os.path.exists(temp_backup_dir):
                shutil.rmtree(temp_backup_dir)
                print("✅ 临时备份已清理")
            
            # 清理临时数据库文件
            temp_db_files = ["db_dump_temp.db", "db_dump_temp.sql"]
            for temp_file in temp_db_files:
                temp_file_path = os.path.join(data_dir, temp_file)
                if os.path.exists(temp_file_path):
                    os.remove(temp_file_path)
                    print(f"✅ 临时文件已清理: {temp_file}")
            
            return True
        except Exception as e:
            print(f"恢复失败: {e}")
            # 尝试回滚到临时备份
            env_vars = get_env_vars()
            data_dir = env_vars.get("DATA_DIR", "/data")
            temp_backup_dir = os.path.join(data_dir, "temp_backup")
            create_temp_backup = str(env_vars.get("CREATE_TEMP_BACKUP", "true")).lower() == "true"
            
            if create_temp_backup and os.path.exists(temp_backup_dir):
                print("正在回滚到恢复前的状态...")
                try:
                    # 清理当前数据目录（除了临时备份）
                    for item in os.listdir(data_dir):
                        item_path = os.path.join(data_dir, item)
                        if item != "temp_backup" and os.path.isdir(item_path):
                            shutil.rmtree(item_path)
                        elif item != "temp_backup" and os.path.isfile(item_path):
                            os.remove(item_path)
                    # 从临时备份恢复（使用移动操作，避免磁盘空间不足的问题）
                    for item in os.listdir(temp_backup_dir):
                        item_path = os.path.join(temp_backup_dir, item)
                        dest_path = os.path.join(data_dir, item)
                        # 使用移动操作，同一分区内是原子且不占额外空间的
                        shutil.move(item_path, dest_path)
                    # 清理临时备份
                    shutil.rmtree(temp_backup_dir)
                    print("✅ 回滚成功，系统已恢复到恢复前的状态")
                except Exception as rollback_error:
                    print(f"回滚失败: {rollback_error}")
            else:
                print("⚠️ 未创建临时备份，跳过回滚")
            raise e
