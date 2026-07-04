#!/bin/bash

# ==========================================
# Vaultwarden Backup Script (Supports SQLite/MySQL/PostgreSQL + Apprise Notifications)
# ==========================================

# Force network proxy settings to ensure Apprise and Rclone can access the internet smoothly
export HTTP_PROXY="${HTTP_PROXY:-}"
export HTTPS_PROXY="${HTTPS_PROXY:-}"
export ALL_PROXY="${ALL_PROXY:-}"

# Basic configuration variables (can be overridden via environment variables)
DB_TYPE="${DB_TYPE:-sqlite}"          # Database type: sqlite, mysql, postgres
DATA_DIR="${DATA_DIR:-/data}"         # Vaultwarden data directory
BACKUP_DIR="${BACKUP_DIR:-/backup}"   # Local temporary backup directory
ZIP_PASSWORD="${ZIP_PASSWORD:-}"      # Zip encryption password (optional)
APPRISE_URL="${APPRISE_URL:-}"        # Apprise notification URL (using command line tool directly)
APPRISE_API_URL="${APPRISE_API_URL%/}"  # Apprise service API address
RCLONE_REMOTE="${RCLONE_REMOTE:-}"    # Rclone remote path, e.g. myremote:/vaultwarden_backup
SQLITE_DB_FILE="${SQLITE_DB_FILE:-db.sqlite3}"  # SQLite database file name (configurable)

# Timestamp for generating unique filenames
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")    # Format: YYYYMMDD_HHMMSS
PREFIX="${BACKUP_PREFIX:-vaultwarden_backup}"
BACKUP_NAME="${PREFIX}_${TIMESTAMP}"
SQL_FILE="${BACKUP_DIR}/${BACKUP_NAME}.sql"
ZIP_FILE="${BACKUP_DIR}/${BACKUP_NAME}.zip"
ZIP_DONE=0

cleanup() {
  rm -f "$SQL_FILE"
  rm -rf "${BACKUP_DIR}/staging_"*
  if [ "$ZIP_DONE" -ne 1 ] && [ -f "$ZIP_FILE" ]; then
      rm -f "$ZIP_FILE"
  fi
}
trap cleanup EXIT

send_notification() {
    local title="$1"
    local body="$2"

    if [ -n "$APPRISE_API_URL" ]; then
        echo "Sending notification using independent Apprise service..."
        if [ -n "$APPRISE_URL" ]; then
            local json=$(jq -n --arg title "$title" --arg body "$body" --arg urls "$APPRISE_URL" '{title: $title, body: $body, urls: $urls}')
            curl -s -X POST "$APPRISE_API_URL/notify" \
                 -H "Content-Type: application/json" \
                 -d "$json"
        else
            curl -s -X POST "$APPRISE_API_URL" \
                 --data-urlencode "title=$title" \
                 --data-urlencode "body=$body"
        fi
        echo ""
    elif [ -n "$APPRISE_URL" ]; then
        echo "Sending notification using local Apprise command line tool..."
        if ! apprise -t "$title" -b "$body" "$APPRISE_URL"; then
            echo "Warning: Apprise notification send failed"
        fi
    else
        echo "APPRISE_URL or APPRISE_API_URL not configured, skipping notification."
    fi
}


if [ -z "$ZIP_PASSWORD" ]; then
    echo "ZIP_PASSWORD not set, will perform unencrypted packaging."
else
    echo "ZIP_PASSWORD set, will perform encrypted packaging."
fi

mkdir -p "$BACKUP_DIR"

if [ ! -w "$BACKUP_DIR" ]; then
    echo "Error: Backup directory $BACKUP_DIR is not writable!"
    send_notification "Vaultwarden Backup Failed ❌" "Backup directory is not writable, please check permissions."
    exit 1
fi

# Check disk space
FREE_SPACE_MB=$(df -Pm "$BACKUP_DIR" | awk '/^\// || /^[0-9]/ {print $(NF-2)}' | sed 's/[^0-9]//g')

if [ -n "$FREE_SPACE_MB" ]; then
    MIN_SPACE_MB=$((5 * 1024))
    if [ "$FREE_SPACE_MB" -lt "$MIN_SPACE_MB" ]; then
        HUMAN_FREE=$(df -h "$BACKUP_DIR" | tail -n 1 | awk '{print $4}')
        echo "Warning: Insufficient disk space in backup directory, remaining: $HUMAN_FREE, less than 5GB!"
        send_notification "Vaultwarden Backup Warning ⚠️" "Insufficient disk space in backup directory, remaining: $HUMAN_FREE, may cause backup failure."
    fi
else
    echo "Warning: Cannot determine disk space, skipping check."
fi

echo "Starting Vaultwarden backup task..."

# 1. Database backup
echo "Backing up database (type: $DB_TYPE)..."

case "$DB_TYPE" in
    sqlite)
        if ! command -v sqlite3 > /dev/null 2>&1; then
            echo "Error: sqlite3 command not found."
            send_notification "Vaultwarden Backup Failed ❌" "sqlite3 command not found."
            exit 1
        fi
        SQLITE_DB="${DATA_DIR}/${SQLITE_DB_FILE}"
        if [ -f "$SQLITE_DB" ]; then
            echo "Backing up SQLite database using .backup command..."
            if sqlite3 "$SQLITE_DB" ".backup '$SQL_FILE'"; then
                echo "✅ SQLite database backup successful."
            else
                echo "Error: SQLite database backup failed!"
                send_notification "Vaultwarden Backup Failed ❌" "SQLite database backup failed, please check file permissions."
                exit 1
            fi
        else
            echo "Error: SQLite database file $SQLITE_DB not found"
            echo "Hint: Use SQLITE_DB_FILE env var to specify the file name (default: db.sqlite3)"
            send_notification "Vaultwarden Backup Failed ❌" "SQLite database file not found. Check DATA_DIR and SQLITE_DB_FILE configuration."
            exit 1
        fi
        ;;
    mysql)
        if ! command -v mysqldump > /dev/null 2>&1; then
            echo "Error: mysqldump command not found."
            send_notification "Vaultwarden Backup Failed ❌" "mysqldump command not found."
            exit 1
        fi
        if [ -z "$DB_USER" ] || [ -z "$DB_PASSWORD" ] || [ -z "$DB_NAME" ]; then
            echo "Error: MySQL backup requires DB_USER, DB_PASSWORD and DB_NAME environment variables."
            send_notification "Vaultwarden Backup Failed ❌" "MySQL backup requires DB_USER, DB_PASSWORD and DB_NAME environment variables."
            exit 1
        fi
        echo "Backing up MySQL database using mysqldump..."
        if mysqldump --single-transaction --quick --opt \
            -h "${DB_HOST:-db}" -P "${DB_PORT:-3306}" \
            -u "${DB_USER}" --password="${DB_PASSWORD}" \
            "${DB_NAME}" > "$SQL_FILE"; then
            echo "✅ MySQL database backup successful."
        else
            echo "Error: MySQL database backup failed!"
            send_notification "Vaultwarden Backup Failed ❌" "MySQL database backup failed, please check connection and permissions."
            exit 1
        fi
        ;;
    postgres)
        if ! command -v pg_dump > /dev/null 2>&1; then
            echo "Error: pg_dump command not found."
            send_notification "Vaultwarden Backup Failed ❌" "pg_dump command not found."
            exit 1
        fi
        if [ -z "$DB_USER" ] || [ -z "$DB_PASSWORD" ] || [ -z "$DB_NAME" ]; then
            echo "Error: PostgreSQL backup requires DB_USER, DB_PASSWORD and DB_NAME environment variables."
            send_notification "Vaultwarden Backup Failed ❌" "PostgreSQL backup requires DB_USER, DB_PASSWORD and DB_NAME environment variables."
            exit 1
        fi
        export PGPASSWORD="${DB_PASSWORD}"
        echo "Backing up PostgreSQL database using pg_dump..."
        if pg_dump --no-owner --no-privileges --clean \
            -h "${DB_HOST:-db}" -p "${DB_PORT:-5432}" \
            -U "${DB_USER}" -d "${DB_NAME}" -F p > "$SQL_FILE"; then
            echo "✅ PostgreSQL database backup successful."
        else
            echo "Error: PostgreSQL database backup failed!"
            send_notification "Vaultwarden Backup Failed ❌" "PostgreSQL database backup failed, please check connection and permissions."
            exit 1
        fi
        unset PGPASSWORD
        ;;
    *)
        echo "Error: Unsupported database type $DB_TYPE"
        send_notification "Vaultwarden Backup Failed ❌" "Unsupported database type: $DB_TYPE"
        exit 1
        ;;
esac

# 2. Packaging
echo "Assembling files and packaging..."

STAGING_DIR="${BACKUP_DIR}/staging_${TIMESTAMP}"
mkdir -p "${STAGING_DIR}/data"

mv "$SQL_FILE" "${STAGING_DIR}/"

shopt -s nullglob
cp -a "$DATA_DIR"/* "${STAGING_DIR}/data/" 2>/dev/null || true
shopt -u nullglob

rm -f "${STAGING_DIR}/data/"*.sqlite3* 2>/dev/null || true

cd "${STAGING_DIR}" || exit 1

if [ -z "$ZIP_PASSWORD" ]; then
    echo "Packing without encryption..."
    zip -r -q "$ZIP_FILE" . -x ".*"
else
    echo "Packing with encryption..."
    zip -r -P "$ZIP_PASSWORD" -q "$ZIP_FILE" . -x ".*"
fi

ZIP_EXIT_CODE=$?
if [ $ZIP_EXIT_CODE -ne 0 ]; then
    echo "Error: Packing failed!"
    send_notification "Vaultwarden Backup Failed ❌" "Error occurred during zip packing, please check."
    exit 1
fi

if [ ! -f "$ZIP_FILE" ]; then
    echo "Error: Zip file $ZIP_FILE does not exist!"
    send_notification "Vaultwarden Backup Failed ❌" "Zip file does not exist, packing may have failed."
    exit 1
fi

ZIP_DONE=1

cd "$BACKUP_DIR"
rm -rf "${STAGING_DIR}"

echo "Verifying zip file integrity..."
if [ -z "$ZIP_PASSWORD" ]; then
    if unzip -tq "$ZIP_FILE" > /dev/null 2>&1; then
        echo "✅ Zip file integrity verification passed."
    else
        echo "❌ Zip file corrupted!"
        send_notification "Vaultwarden Backup Failed ❌" "Generated zip file verification failed, please check disk space."
        exit 1
    fi
else
    if unzip -tq -P "$ZIP_PASSWORD" "$ZIP_FILE" > /dev/null 2>&1; then
        echo "✅ Zip file integrity verification passed."
    else
        echo "❌ Zip file corrupted!"
        send_notification "Vaultwarden Backup Failed ❌" "Generated zip file verification failed, please check."
        exit 1
    fi
fi

FILE_SIZE=$(du -h "$ZIP_FILE" | cut -f1)

# 3. Rclone upload (if configured)
if [ -n "$RCLONE_REMOTE" ]; then
    echo "Checking Rclone remote configuration..."
    if ! rclone listremotes | grep -q "^${RCLONE_REMOTE%%:*}:"; then
        echo "❌ Error: Rclone remote not found: ${RCLONE_REMOTE%%:*}"
        send_notification "Vaultwarden Backup Failed ❌" "Rclone remote not found: ${RCLONE_REMOTE%%:*}, please check Rclone config."
        exit 1
    else
        echo "✅ Rclone remote configuration check passed."
    fi

    echo "Uploading backup to $RCLONE_REMOTE using Rclone..."
    UPLOAD_SUCCESS=0
    MAX_RETRIES=3
    RETRY_DELAY=5

    for i in $(seq 1 $MAX_RETRIES); do
        echo "Upload attempt $i/$MAX_RETRIES..."
        if rclone copy "$ZIP_FILE" "$RCLONE_REMOTE"; then
            UPLOAD_SUCCESS=1
            break
        fi
        if [ $i -lt $MAX_RETRIES ]; then
            echo "Upload failed, retrying in ${RETRY_DELAY}s..."
            sleep $RETRY_DELAY
            RETRY_DELAY=$((RETRY_DELAY * 2))
        fi
    done

    if [ $UPLOAD_SUCCESS -ne 1 ]; then
        echo "Error: Rclone upload failed (retried $MAX_RETRIES times)!"
        send_notification "Vaultwarden Backup Failed ❌" "Rclone upload to remote storage failed (retried $MAX_RETRIES times)."
        exit 1
    fi

    # 4. Remote backup cleanup
    if [ -z "$RCLONE_KEEP_DAYS" ]; then
        echo "RCLONE_KEEP_DAYS not set, skipping remote cleanup."
    else
        echo "Cleaning up remote expired backups (keeping $RCLONE_KEEP_DAYS days)..."
        rclone delete "$RCLONE_REMOTE" \
            --filter "+ ${PREFIX}_*.zip" \
            --filter "- *" \
            --min-age "${RCLONE_KEEP_DAYS}d" \
            --verbose 2>&1 | head -20
        echo "Remote cleanup completed."
    fi
else
    echo "RCLONE_REMOTE not configured, skipping cloud upload."
fi

# 5. Cleanup
echo "Cleaning up temporary files..."
rm -f "$SQL_FILE"

echo "Cleaning up expired backups..."
KEEP_DAYS=${LOCAL_BACKUP_KEEP_DAYS:-15}
find "$BACKUP_DIR" -name "${PREFIX}_*.zip" -mtime +$KEEP_DAYS -delete 2>/dev/null || \
    find "$BACKUP_DIR" -name "${PREFIX}_*.zip" -mtime +$KEEP_DAYS -exec rm -f {} \;
echo "Cleaned up local backups older than $KEEP_DAYS days."

# 6. Send success notification
echo "Backup task completed!"
send_notification "Vaultwarden Backup Success ✅" "Type: $DB_TYPE
Size: $FILE_SIZE
File: $BACKUP_NAME.zip"

exit 0