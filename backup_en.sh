#!/bin/bash

# ==========================================
# Vaultwarden Backup Script (Supports SQLite/MySQL/PostgreSQL + Apprise Notifications)
# ==========================================

# Force network proxy settings to ensure Apprise and Rclone can access the internet smoothly
# (Leave empty in environment variables if proxy is not needed)
export HTTP_PROXY="${HTTP_PROXY:-}"   # HTTP proxy address, e.g. http://192.168.1.100:7890
export HTTPS_PROXY="${HTTPS_PROXY:-}" # HTTPS proxy address, e.g. http://192.168.1.100:7890
export ALL_PROXY="${ALL_PROXY:-}"     # SOCKS5 proxy address, e.g. socks5://192.168.1.100:7890

# Basic configuration variables (can be overridden via environment variables)
DB_TYPE="${DB_TYPE:-sqlite}"          # Database type: sqlite, mysql, postgres
DATA_DIR="${DATA_DIR:-/data}"      # Vaultwarden data directory
BACKUP_DIR="${BACKUP_DIR:-/backup}"   # Local temporary backup directory
ZIP_PASSWORD="${ZIP_PASSWORD:-}"      # Zip file encryption password (required)
APPRISE_URL="${APPRISE_URL:-}"        # Apprise notification URL (using command line tool directly)
# Read API URL and automatically remove any trailing slashes to accommodate user writing habits
APPRISE_API_URL="${APPRISE_API_URL%/}"  # Apprise service API address (using independent Apprise service)
RCLONE_REMOTE="${RCLONE_REMOTE:-}"    # Rclone remote path, e.g. myremote:/vaultwarden_backup

# Timestamp for generating unique filenames
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")    # Format: YYYYMMDD_HHMMSS
# Backup file prefix, can be customized via environment variable
PREFIX="${BACKUP_PREFIX:-vaultwarden_backup}"
BACkUP_NAME="${PREFIX}_${TIMESTAMP}" # Backup file base name
SQL_FILE="${BACKUP_DIR}/${BACkUP_NAME}.sql"   # Exported SQL file path
ZIP_FILE="${BACKUP_DIR}/${BACkUP_NAME}.zip"   # Final encrypted zip file path

# Zip file generation completion flag
ZIP_DONE=0
# Try to delete temporary files no matter how the script exits
cleanup() {
  rm -f "$SQL_FILE"
  # Clean up any residual staging directories
  rm -rf "${BACKUP_DIR}/staging_"* # Only clean up incomplete zip files if the script exits abnormally before packing is complete
  if [ "$ZIP_DONE" -ne 1 ] && [ -f "$ZIP_FILE" ]; then
      rm -f "$ZIP_FILE"
  fi
}
trap cleanup EXIT

# Helper function for sending notifications
send_notification() {
    local title="$1"  # Notification title
    local body="$2"   # Notification content
    
    # Prioritize using independent Apprise service API
    if [ -n "$APPRISE_API_URL" ]; then
        echo "Sending notification using independent Apprise service..."
        
        # If both API address and specific notification URL are configured (Stateless mode)
        if [ -n "$APPRISE_URL" ]; then
            # Use jq to generate safe JSON
            local json=$(jq -n --arg title "$title" --arg body "$body" --arg urls "$APPRISE_URL" '{title: $title, body: $body, urls: $urls}')
            
            # Use -s to hide curl's progress bar
            curl -s -X POST "$APPRISE_API_URL/notify" \
                 -H "Content-Type: application/json" \
                 -d "$json"
                
        # If configuration path is directly written in the API address (e.g., `http://apprise:8000/notify/mybot`) (Stateful mode)
        else
            # Use --data-urlencode to submit, perfectly preserving newlines and preventing special characters from truncating the form
            curl -s -X POST "$APPRISE_API_URL" \
                 --data-urlencode "title=$title" \
                 --data-urlencode "body=$body"
        fi
        echo "" # Add a newline to avoid log sticking
        
    # Fallback to using local Apprise command line tool
    elif [ -n "$APPRISE_URL" ]; then
        echo "Sending notification using local Apprise command line tool..."
        apprise -t "$title" -b "$body" "$APPRISE_URL"
    else
        echo "APPRISE_URL or APPRISE_API_URL not configured, skipping notification sending."
    fi
}


if [ -z "$ZIP_PASSWORD" ]; then
    echo "ZIP_PASSWORD environment variable not set, will perform unencrypted packaging."
else
    echo "ZIP_PASSWORD environment variable set, will perform encrypted packaging."
fi

# Ensure backup directory exists
mkdir -p "$BACKUP_DIR"

# Check backup directory permissions
if [ ! -w "$BACKUP_DIR" ]; then
    echo "Error: Backup directory $BACKUP_DIR does not have write permission!"
    send_notification "Vaultwarden Backup Failed ❌" "Backup directory does not have write permission, please check permission settings."
    exit 1
fi

# Check disk space
# Use df -Pm to output in MB to avoid integer overflow on 32-bit systems
# Logic: First lock the line containing the mount point, extract the third column from the end, and forcefully remove all non-numeric characters
FREE_SPACE_MB=$(df -Pm "$BACKUP_DIR" | awk '/^\// || /^[0-9]/ {print $(NF-2)}' | sed 's/[^0-9]//g')
# 5GB converted to MB
MIN_SPACE_MB=$((5 * 1024))

if [ "$FREE_SPACE_MB" -lt "$MIN_SPACE_MB" ]; then
    # Get remaining space
    HUMAN_FREE=$(df -h "$BACKUP_DIR" | tail -n 1 | awk '{print $4}')
    echo "Warning: Insufficient disk space in the backup directory, remaining space is $HUMAN_FREE, less than 5GB!"
    send_notification "Vaultwarden Backup Warning ⚠️" "Insufficient disk space in the backup directory, remaining space is $HUMAN_FREE, less than 5GB, which may cause backup failure."
fi

echo "Starting Vaultwarden backup task..."

# 1. Database backup logic
echo "Backing up database (type: $DB_TYPE)..."

# Check if database backup tools exist
case "$DB_TYPE" in
    sqlite)
        # Check if sqlite3 command exists
        if ! command -v sqlite3 > /dev/null 2>&1; then
            echo "Error: sqlite3 command not found, please make sure SQLite tools are installed."
            send_notification "Vaultwarden Backup Failed ❌" "sqlite3 command not found, please make sure SQLite tools are installed."
            exit 1
        fi
        # SQLite backup logic: Use .backup command for safe export
        SQLITE_DB="${DATA_DIR}/db.sqlite3" # Default SQLite database path
        if [ -f "$SQLITE_DB" ]; then
            echo "Backing up SQLite database using .backup command..."
            if sqlite3 "$SQLITE_DB" ".backup '$SQL_FILE'"; then
                echo "✅ SQLite database backup successful."
            else
                echo "Error: SQLite database backup failed!"
                send_notification "Vaultwarden Backup Failed ❌" "SQLite database backup failed, please check database file permissions."
                exit 1
            fi
        else
            echo "Error: SQLite database file $SQLITE_DB not found"
            send_notification "Vaultwarden Backup Failed ❌" "SQLite database file not found."
            exit 1
        fi
        ;;
    mysql)
        # Check if mysqldump command exists
        if ! command -v mysqldump > /dev/null 2>&1; then
            echo "Error: mysqldump command not found, please make sure MySQL client tools are installed."
            send_notification "Vaultwarden Backup Failed ❌" "mysqldump command not found, please make sure MySQL client tools are installed."
            exit 1
        fi
        # Check necessary environment variables
        if [ -z "$DB_USER" ] || [ -z "$DB_PASSWORD" ] || [ -z "$DB_NAME" ]; then
            echo "Error: MySQL backup requires DB_USER, DB_PASSWORD and DB_NAME environment variables."
            send_notification "Vaultwarden Backup Failed ❌" "MySQL backup requires DB_USER, DB_PASSWORD and DB_NAME environment variables."
            exit 1
        fi
        # MySQL/MariaDB backup logic: Use mysqldump to export
        export MYSQL_PWD="${DB_PASSWORD}" 
        echo "Backing up MySQL database using mysqldump..."
        if mysqldump --single-transaction --quick --opt -h "${DB_HOST:-db}" -P "${DB_PORT:-3306}" -u "${DB_USER}" "${DB_NAME}" > "$SQL_FILE"; then
            echo "✅ MySQL database backup successful."
        else
            echo "Error: MySQL database backup failed!"
            send_notification "Vaultwarden Backup Failed ❌" "MySQL database backup failed, please check database connection and permissions."
            exit 1
        fi
        ;;
    postgres)
        # Check if pg_dump command exists
        if ! command -v pg_dump > /dev/null 2>&1; then
            echo "Error: pg_dump command not found, please make sure PostgreSQL client tools are installed."
            send_notification "Vaultwarden Backup Failed ❌" "pg_dump command not found, please make sure PostgreSQL client tools are installed."
            exit 1
        fi
        # Check necessary environment variables
        if [ -z "$DB_USER" ] || [ -z "$DB_PASSWORD" ] || [ -z "$DB_NAME" ]; then
            echo "Error: PostgreSQL backup requires DB_USER, DB_PASSWORD and DB_NAME environment variables."
            send_notification "Vaultwarden Backup Failed ❌" "PostgreSQL backup requires DB_USER, DB_PASSWORD and DB_NAME environment variables."
            exit 1
        fi
        # PostgreSQL backup logic: Use pg_dump to export
        export PGPASSWORD="${DB_PASSWORD}" 
        echo "Backing up PostgreSQL database using pg_dump..."
        if pg_dump --no-owner --no-privileges --clean -h "${DB_HOST:-db}" -p "${DB_PORT:-5432}" -U "${DB_USER}" -d "${DB_NAME}" -F p > "$SQL_FILE"; then
            echo "✅ PostgreSQL database backup successful."
        else
            echo "Error: PostgreSQL database backup failed!"
            send_notification "Vaultwarden Backup Failed ❌" "PostgreSQL database backup failed, please check database connection and permissions."
            exit 1
        fi
        ;;
    *)
        echo "Error: Unsupported database type $DB_TYPE"
        send_notification "Vaultwarden Backup Failed ❌" "Unsupported database type: $DB_TYPE"
        exit 1
        ;;
esac

# Check if database backup was successful
if [ $? -ne 0 ]; then
    echo "Error: Database backup failed!"
    send_notification "Vaultwarden Backup Failed ❌" "Database ($DB_TYPE) export failed."
    exit 1
fi

# 2. Packaging logic
echo "Assembling files and packaging..."

# Create a temporary staging directory to ensure an extremely clean structure after extraction
STAGING_DIR="${BACKUP_DIR}/staging_${TIMESTAMP}"
mkdir -p "${STAGING_DIR}/data"

# [Step 1]: Move the exported SQL file to the root of the staging directory
mv "$SQL_FILE" "${STAGING_DIR}/"

# [Step 2]: Copy the entire contents of the /data directory to the data folder in the staging directory
# This is done to ensure that when extracted, you get a folder named data for easy restoration
cp -a "$DATA_DIR"/* "${STAGING_DIR}/data/" 2>/dev/null || true

# [Key detail]: Delete the physically copied sqlite database files.
# Because we have already exported SQL scripts using dedicated command logic earlier, if we don't delete them here,
# the zip file size will double, and database file conflicts may occur during restoration.
rm -f "${STAGING_DIR}/data/"*.sqlite3* 2>/dev/null || true

# [Step 3]: Switch to the staging directory to execute packaging
cd "${STAGING_DIR}" || exit 1

if [ -z "$ZIP_PASSWORD" ]; then
    echo "Packing without encryption..."
    zip -r -q "$ZIP_FILE" .
else
    echo "Packing with encryption..."
    zip -r -P "$ZIP_PASSWORD" -q "$ZIP_FILE" .
fi

# Check if packaging was successful
if [ $? -ne 0 ]; then
    echo "Error: Packing failed!"
    send_notification "Vaultwarden Backup Failed ❌" "Error occurred during zip packing, please check."
    exit 1
fi

# Check if zip file exists
if [ ! -f "$ZIP_FILE" ]; then
    echo "Error: Zip file $ZIP_FILE does not exist!"
    send_notification "Vaultwarden Backup Failed ❌" "Zip file does not exist, packing may have failed."
    exit 1
fi

# Mark packaging as successful, so even if upload fails, the local zip file won't be deleted by trap
ZIP_DONE=1

# [Step 4]: After packaging is complete, clean up the staging directory
cd "$BACKUP_DIR"
rm -rf "${STAGING_DIR}"

# Check zip file integrity
echo "Verifying zip file integrity..."
if [ -z "$ZIP_PASSWORD" ]; then
    # Verification for unencrypted packaging: Use unzip -tq for silent testing
    if unzip -tq "$ZIP_FILE" > /dev/null 2>&1; then
        echo "✅ Zip file integrity verification passed."
    else
        echo "❌ Zip file corrupted!"
        send_notification "Vaultwarden Backup Failed ❌" "Generated zip file verification failed, please check disk space."
        exit 1
    fi
else
    # Verification for encrypted packaging: Explicitly pass -P password for testing
    if unzip -tq -P "$ZIP_PASSWORD" "$ZIP_FILE" > /dev/null 2>&1; then
        echo "✅ Zip file integrity verification passed."
    else
        echo "❌ Zip file corrupted!"
        send_notification "Vaultwarden Backup Failed ❌" "Generated zip file password verification failed, please check."
        exit 1
    fi
fi

# Get zip file size
FILE_SIZE=$(du -h "$ZIP_FILE" | cut -f1)

# 3. Rclone upload logic (if configured)
if [ -n "$RCLONE_REMOTE" ]; then
    # Test Rclone remote connection (global variables are already effective, no need to specify --config)
    echo "Testing Rclone remote connection..."
    if ! rclone listremotes | grep -q "^${RCLONE_REMOTE%%:*}:"; then
        echo "❌ Warning: Rclone remote not found: ${RCLONE_REMOTE%%:*}"
        send_notification "Vaultwarden Backup Warning ⚠️" "Rclone remote not found: ${RCLONE_REMOTE%%:*}, will attempt to continue uploading."
    else
        echo "✅ Rclone remote connection test passed."
    fi
    
    echo "Uploading backup to $RCLONE_REMOTE using Rclone..."
    # Use rclone copy to upload file
    rclone copy "$ZIP_FILE" "$RCLONE_REMOTE"
    
    if [ $? -ne 0 ]; then
        echo "Error: Rclone upload failed!"
        send_notification "Vaultwarden Backup Failed ❌" "Rclone upload to remote storage failed."
        exit 1
    fi
    
    # 4. Remote backup automatic cleanup
    if [ -z "$RCLONE_KEEP_DAYS" ]; then
        echo "RCLONE_KEEP_DAYS not set, skipping remote cleanup."
    else
        echo "Cleaning up remote expired backups (keeping $RCLONE_KEEP_DAYS days)..."
        # Add --include filter to absolutely prevent accidental deletion of other private files in user's网盘!
        rclone delete "$RCLONE_REMOTE" --include "${PREFIX}_*.zip" --min-age "${RCLONE_KEEP_DAYS}d"
    fi
else
    echo "RCLONE_REMOTE not configured, skipping cloud upload, backup only kept locally."
fi

# 4. Clean up temporary files
echo "Cleaning up temporary files..."
rm -f "$SQL_FILE" # Delete unencrypted SQL file
# If upload is successful, you can choose to delete local ZIP package, here we keep it just in case, or clean up old backups according to strategy
# rm -f "$ZIP_FILE" 

# 5. Automatic cleanup of expired backups
echo "Cleaning up expired backups..."
# Automatically clean up expired local backup files to prevent disk from being full
KEEP_DAYS=${LOCAL_BACKUP_KEEP_DAYS:-15}
find "$BACKUP_DIR" -name "${PREFIX}_*.zip" -mtime +$KEEP_DAYS -exec rm {} \;
echo "Cleaned up old local backups from $KEEP_DAYS days ago."

# 6. Send success notification
echo "Backup task completed!"
send_notification "Vaultwarden Backup Success ✅" "Type: $DB_TYPE\nSize: $FILE_SIZE\nFile: $BACkUP_NAME.zip"

exit 0