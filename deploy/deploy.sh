#!/usr/bin/env bash
#
# Day-to-day deploy: pull latest code, rebuild images, run migrations,
# restart services. Runnable from anywhere — figures out the repo root
# from its own location.
#
#   bash deploy/deploy.sh

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEPLOY_DIR="${REPO_ROOT}/deploy"

cd "${REPO_ROOT}"

if [ ! -f "${DEPLOY_DIR}/.env" ]; then
  echo "✗ ${DEPLOY_DIR}/.env not found. Copy .env.example and fill it in:"
  echo "    cp ${DEPLOY_DIR}/.env.example ${DEPLOY_DIR}/.env"
  echo "    nano ${DEPLOY_DIR}/.env"
  exit 1
fi

# Load variables from .env to read POSTGRES_HOST
if [ -f "${DEPLOY_DIR}/.env" ]; then
  while IFS= read -r line || [ -n "$line" ]; do
    if [[ ! "$line" =~ ^# ]] && [[ ! -z "$line" ]]; then
      export "$line"
    fi
  done < "${DEPLOY_DIR}/.env"
fi

echo "==> git pull"
git pull --ff-only

cd "${DEPLOY_DIR}"

echo "==> Building images"
docker compose build --no-cache

echo "==> Checking Postgres Host"
if [ "${POSTGRES_HOST:-localhost}" = "localhost" ] || [ "${POSTGRES_HOST:-postgres}" = "postgres" ]; then
  echo "==> Bringing up local Postgres container"
  docker compose up -d postgres
else
  echo "==> Using remote Postgres host: ${POSTGRES_HOST}"
fi

echo "==> Applying migrations"
docker compose --profile tools run --rm migrate

echo "==> Seeding super admin (idempotent)"
docker compose --profile tools run --rm seed

echo "==> Bringing up the rest of the stack"
docker compose up -d --remove-orphans

echo "==> Pruning dangling images"
docker image prune -f >/dev/null

echo
echo "✓ Deploy complete."
echo "  Tail logs:    docker compose -f ${DEPLOY_DIR}/docker-compose.yml logs -f"
echo "  Visit:        http://\$(curl -s ifconfig.me)/"
