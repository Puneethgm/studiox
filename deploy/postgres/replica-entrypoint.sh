#!/bin/sh
#
# Custom entrypoint for the postgres_replica service.
#
# The container restarts on every `deploy/deploy.sh` run, so this must NOT
# re-clone the primary every time — only on a genuinely empty data
# directory (first boot / fresh volume). On every later boot, PGDATA
# already contains PG_VERSION plus the standby.signal + primary_conninfo
# that `pg_basebackup -R` wrote on the first run, so we skip straight to
# starting postgres normally and it comes up in standby mode on its own.
set -e

PGDATA="${PGDATA:-/var/lib/postgresql/data}"

if [ -f "${PGDATA}/PG_VERSION" ]; then
  echo "[replica-entrypoint] ${PGDATA} already initialized (PG_VERSION present) — starting postgres normally."
else
  echo "[replica-entrypoint] ${PGDATA} is empty — cloning primary (${POSTGRES_HOST:-postgres}) with pg_basebackup..."

  if [ -z "${POSTGRES_REPLICATION_PASSWORD:-}" ]; then
    echo "[replica-entrypoint] FATAL: POSTGRES_REPLICATION_PASSWORD is not set — cannot authenticate as replicator." >&2
    exit 1
  fi

  export PGPASSWORD="${POSTGRES_REPLICATION_PASSWORD}"
  pg_basebackup \
    -h "${POSTGRES_HOST:-postgres}" \
    -U replicator \
    -D "${PGDATA}" \
    -P \
    --wal-method=stream \
    -R
  unset PGPASSWORD

  echo "[replica-entrypoint] base backup complete; standby.signal + primary_conninfo written by -R."
fi

exec docker-entrypoint.sh postgres
