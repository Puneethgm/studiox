#!/usr/bin/env bash
#
# Nightly Postgres backup. Writes a gzipped, AES-256-encrypted pg_dump to
# deploy/backups/ (platform-security-hardening task 6.1 — the gzipped
# dump is encrypted before it ever leaves the host), keeps the last 14
# days, deletes older, then copies the encrypted dump offsite to S3
# unconditionally (task 6.2 — this used to be skipped when S3_BUCKET
# wasn't set; it is now a hard failure instead, since an unconfigured
# offsite copy is a backup gap, not an optional feature) for disaster
# recovery (reuses the same AWS_*/S3_BUCKET creds already used for
# studio logo uploads — see deploy/.env.example).
#
# BACKUP_ENCRYPTION_KEY (a long random passphrase, set once and kept
# safe — losing it means the backups are unrecoverable) must be set in
# .env for this script to run at all.
#
# Prerequisite for the offsite copy: the `aws` CLI must be installed on
# the HOST this script runs on (not inside a container) — it is not
# installed by deploy/setup-ec2.sh today. Install it with:
#   sudo apt-get install -y awscli
# or, to avoid distro packaging lag:
#   https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html
#
# Installed into crontab automatically by deploy/setup-ec2.sh (nightly,
# 03:00 UTC); see deploy/README.md for the exact line and how to check it.
#
# Restore from a backup (see deploy/restore_test.sh for a scripted,
# automated version of exactly this — task 6.3):
#   openssl enc -d -aes-256-cbc -pbkdf2 -iter 100000 \
#       -pass env:BACKUP_ENCRYPTION_KEY \
#       -in deploy/backups/<file>.sql.gz.enc | gunzip -c | \
#     docker compose -f deploy/docker-compose.yml exec -T postgres \
#       psql -U "$POSTGRES_USER" "$POSTGRES_DB"

set -euo pipefail

DEPLOY_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${DEPLOY_DIR}"

# Load AWS_*/S3_BUCKET (and everything else) from .env. Needed because
# cron invokes this script with a minimal environment — docker compose
# itself reads .env automatically, but this script's own shell (for the
# `aws` CLI call below) does not.
if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  . ./.env
  set +a
fi

if [ -z "${BACKUP_ENCRYPTION_KEY:-}" ]; then
  echo "[$(date -u +%FT%TZ)] BACKUP_ENCRYPTION_KEY is not set — refusing to write an unencrypted backup" >&2
  exit 1
fi

# alert_and_exit sends a best-effort failure alert (task 6.2) before
# exiting non-zero. The alert itself is never allowed to be the reason
# this script fails — curl failures here are swallowed so a flaky mail
# API can't mask the real backup failure that triggered this call.
alert_and_exit() {
  local msg="$1"
  echo "[$(date -u +%FT%TZ)] BACKUP FAILED: ${msg}" >&2
  if [ -n "${BACKUP_ALERT_WEBHOOK_URL:-}" ]; then
    curl -fsS -X POST -H 'Content-Type: application/json' \
      -d "{\"text\":\"Nightly backup failed: ${msg}\"}" \
      "${BACKUP_ALERT_WEBHOOK_URL}" >/dev/null 2>&1 || true
  fi
  exit 1
}

mkdir -p backups
TS=$(date -u +%Y%m%d_%H%M%SZ)
PLAIN="backups/${TS}.sql.gz"
OUT="${PLAIN}.enc"

# Run pg_dump *inside* the container so we don't need a host-side psql.
# The container env already has POSTGRES_USER / POSTGRES_DB, so we just
# reference them.
docker compose exec -T postgres sh -c \
  'pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB"' \
  | gzip > "${PLAIN}" || alert_and_exit "pg_dump/gzip failed"

# Encrypt (task 6.1) — AES-256-CBC with a PBKDF2-derived key from
# BACKUP_ENCRYPTION_KEY, matching this script's existing openssl-cli
# style rather than reaching for the Go envelope-encryption helper,
# which isn't callable from a bare shell script. The plaintext gzip is
# removed immediately after — only the .enc file is ever kept on disk
# or copied offsite.
openssl enc -aes-256-cbc -pbkdf2 -iter 100000 -salt \
  -pass env:BACKUP_ENCRYPTION_KEY \
  -in "${PLAIN}" -out "${OUT}" || alert_and_exit "encryption failed"
rm -f "${PLAIN}"

# Retention: 14 days
find backups -type f -name '*.sql.gz.enc' -mtime +14 -delete

echo "[$(date -u +%FT%TZ)] encrypted backup written: ${OUT} ($(stat -c%s "${OUT}") bytes)"

# Offsite copy to S3 — unconditional (task 6.2). A missing S3_BUCKET or
# a failed upload is now a hard failure (with an alert), not a silent
# skip: an unconfigured offsite copy is a backup gap, not an optional
# feature.
if [ -z "${S3_BUCKET:-}" ]; then
  alert_and_exit "S3_BUCKET is not set — offsite copy is required, not optional"
fi
if ! aws s3 cp "${OUT}" "s3://${S3_BUCKET}/backups/db/$(basename "${OUT}")"; then
  alert_and_exit "aws s3 cp to s3://${S3_BUCKET}/backups/db/$(basename "${OUT}") failed"
fi
echo "[$(date -u +%FT%TZ)] backup copied to s3://${S3_BUCKET}/backups/db/$(basename "${OUT}")"
