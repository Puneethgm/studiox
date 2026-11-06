#!/usr/bin/env bash
#
# Automated quarterly restore-test job (platform-security-hardening
# task 6.3). Decrypts the most recent backup, restores it into a
# scratch database (never the real one), and confirms row counts match
# the source database at restore time for a given table list. Intended
# to be scheduled quarterly (cron entry not wired up here — see this
# task's own note on why: no real production backup/cron infra exists
# in this sandbox to point it at).
#
# Usage: ./restore_test.sh <backup-file.sql.gz.enc>

set -euo pipefail

DEPLOY_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${DEPLOY_DIR}"

if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  . ./.env
  set +a
fi

BACKUP_FILE="${1:-}"
if [ -z "${BACKUP_FILE}" ]; then
  BACKUP_FILE=$(ls -t backups/*.sql.gz.enc 2>/dev/null | head -1)
fi
if [ -z "${BACKUP_FILE}" ] || [ ! -f "${BACKUP_FILE}" ]; then
  echo "No backup file found/given — nothing to restore-test" >&2
  exit 1
fi
if [ -z "${BACKUP_ENCRYPTION_KEY:-}" ]; then
  echo "BACKUP_ENCRYPTION_KEY is not set — cannot decrypt ${BACKUP_FILE}" >&2
  exit 1
fi

SCRATCH_DB="restore_test_scratch"
TMP_SQL="$(mktemp)"
trap 'rm -f "${TMP_SQL}"' EXIT

echo "[$(date -u +%FT%TZ)] decrypting ${BACKUP_FILE}"
openssl enc -d -aes-256-cbc -pbkdf2 -iter 100000 \
  -pass env:BACKUP_ENCRYPTION_KEY \
  -in "${BACKUP_FILE}" | gunzip -c > "${TMP_SQL}"

echo "[$(date -u +%FT%TZ)] (re)creating scratch database ${SCRATCH_DB}"
docker compose exec -T postgres sh -c \
  "psql -U \"\$POSTGRES_USER\" -d postgres -c \"DROP DATABASE IF EXISTS ${SCRATCH_DB}\" && \
   psql -U \"\$POSTGRES_USER\" -d postgres -c \"CREATE DATABASE ${SCRATCH_DB}\""

echo "[$(date -u +%FT%TZ)] restoring into ${SCRATCH_DB}"
docker compose exec -T postgres sh -c \
  "psql -U \"\$POSTGRES_USER\" -d ${SCRATCH_DB}" < "${TMP_SQL}" > /dev/null

# Row-count comparison: a small, representative table list rather than
# every table, matching the task's "confirms row counts match the
# source at backup time" — studios and users are the two tables every
# other table in this schema ultimately hangs off of, so a mismatch
# there is the strongest signal something restored wrong.
FAILED=0
for TABLE in studios users; do
  SOURCE_COUNT=$(docker compose exec -T postgres sh -c \
    "psql -U \"\$POSTGRES_USER\" -d \"\$POSTGRES_DB\" -t -c \"SELECT count(*) FROM ${TABLE}\"" | tr -d '[:space:]')
  SCRATCH_COUNT=$(docker compose exec -T postgres sh -c \
    "psql -U \"\$POSTGRES_USER\" -d ${SCRATCH_DB} -t -c \"SELECT count(*) FROM ${TABLE}\"" | tr -d '[:space:]')
  if [ "${SOURCE_COUNT}" != "${SCRATCH_COUNT}" ]; then
    echo "MISMATCH: ${TABLE} source=${SOURCE_COUNT} restored=${SCRATCH_COUNT}" >&2
    FAILED=1
  else
    echo "OK: ${TABLE} row count matches (${SOURCE_COUNT})"
  fi
done

echo "[$(date -u +%FT%TZ)] cleaning up scratch database"
docker compose exec -T postgres sh -c \
  "psql -U \"\$POSTGRES_USER\" -d postgres -c \"DROP DATABASE IF EXISTS ${SCRATCH_DB}\""

exit "${FAILED}"
