#!/usr/bin/env bash
#
# Nightly Postgres backup. Writes a gzipped pg_dump to deploy/backups/,
# keeps the last 14 days, deletes older.
#
# Add to crontab on the EC2 box (run `crontab -e`):
#   0 3 * * * /home/ubuntu/studiox/deploy/backup.sh >> /var/log/projectx-backup.log 2>&1
#
# Restore from a backup:
#   gunzip -c deploy/backups/<file>.sql.gz | \
#     docker compose -f deploy/docker-compose.yml exec -T postgres \
#       psql -U "$POSTGRES_USER" "$POSTGRES_DB"

set -euo pipefail

DEPLOY_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${DEPLOY_DIR}"

mkdir -p backups
TS=$(date -u +%Y%m%d_%H%M%SZ)
OUT="backups/${TS}.sql.gz"

# Run pg_dump *inside* the container so we don't need a host-side psql.
# The container env already has POSTGRES_USER / POSTGRES_DB, so we just
# reference them.
docker compose exec -T postgres sh -c \
  'pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB"' \
  | gzip > "${OUT}"

# Retention: 14 days
find backups -type f -name '*.sql.gz' -mtime +14 -delete

echo "[$(date -u +%FT%TZ)] backup written: ${OUT} ($(stat -c%s "${OUT}") bytes)"
