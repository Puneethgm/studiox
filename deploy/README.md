# Deploy — single-EC2 Docker

Everything in this folder is the production deployment for Project-X. One
box, Docker Compose, no CI/CD, no image registry. SSH in, run a script, the
app is live.

```
deploy/
  docker-compose.yml      # the stack: postgres (+ replica) + redis + api + web + nginx + one-shots — LIVE / this is the one that runs in production
  docker-compose.prod.yml # STALE duplicate, not used anywhere — scheduled for removal, see note below
  postgres/                # replication init scripts + replica entrypoint, mounted into docker-compose.yml
  api.Dockerfile          # multi-stage Go build → distroless (~30 MB)
  web.Dockerfile          # multi-stage Next.js → standalone (~150 MB)
  nginx/default.conf      # reverse proxy: /api/* → api,  /* → web
  setup-ec2.sh            # one-time: install Docker, swap, dirs, nightly backup cron
  deploy.sh               # day-to-day: pull → build → migrate → seed → up
  backup.sh               # nightly pg_dump + offsite S3 copy (installed into cron by setup-ec2.sh)
  .env.example            # production env template
  .env                    # YOUR real values (gitignored)
  secrets/                # mounted at /secrets in the api container
  backups/                # gzipped pg_dump output
```

> **Note:** `docker-compose.prod.yml` is a stale duplicate of `docker-compose.yml`
> and is not referenced by any script here — `docker-compose.yml` is the only
> one actually deployed. It's left in place for now; removing it is a
> separate cleanup decision, not done as part of this change.

---

## First-time setup (run once on a fresh EC2)

### 1. SSH in and clone

```bash
ssh -i studiox.pem ubuntu@13.250.175.113
git clone https://github.com/mdfawaz1/studiox.git
cd studiox
```

### 2. Bootstrap the box

```bash
bash deploy/setup-ec2.sh
```

This installs Docker + Compose plugin, adds `ubuntu` to the `docker` group,
and provisions a 2GB swapfile. **Log out + back in afterward** so the group
membership takes effect.

### 3. Open port 80 in the EC2 security group

In the AWS console: EC2 → Instances → your instance → Security → click the
security group → Inbound rules → Edit → add `HTTP (80)` from `0.0.0.0/0`.
(Add `443` too for when you set up TLS.)

### 4. Configure environment

```bash
cp deploy/.env.example deploy/.env
nano deploy/.env       # fill in real values; see notes inside the file
```

Key values you must change:
- `POSTGRES_PASSWORD` — long random
- `JWT_SECRET` — 32+ random chars
- `REDIS_PASSWORD` — long random (stack refuses to start without it — see below)
- `SUPER_ADMIN_PASSWORD` — your login password
- `COOKIE_DOMAIN` and `PUBLIC_FORM_BASE_URL` — your IP (or domain when added)

### Redis (AI answer cache)

The `redis` service backs a per-studio cache of AI-generated answers (chat
replies, conversation summaries, style-profile rebuilds, AI-generated
templates) — an identical prompt for the same studio is served from Redis
instead of paying for and waiting on another Groq/Gemini/Claude call. It's
backend-network-only (never exposed on a host port) and requires
`REDIS_PASSWORD` to start at all — the stack fails fast with a clear message
if it's unset, same as `JWT_SECRET`/`TOKEN_ENCRYPTION_KEY`.

It holds no data worth keeping: no volume, safe to lose on restart or
redeploy — the api container transparently falls back to calling the model
directly if Redis is ever unreachable.

### 5. (Optional) Google Sheets

If you're enabling Sheets exports, drop the service-account JSON at
`deploy/secrets/google-credentials.json` and set `GOOGLE_SHEETS_ID` in
`.env`. See [`docs/SETUP_GOOGLE_SHEETS.md`](../docs/SETUP_GOOGLE_SHEETS.md).

### 6. First deploy

```bash
bash deploy/deploy.sh
```

Builds images (~3 min on first run, faster after thanks to layer cache),
brings up Postgres, runs migrations, seeds the super admin, starts the
stack. When it finishes, browse to `http://<your-ip>/` and sign in.

---

## Day-to-day

After local code changes, push to GitHub, then on the box:

```bash
ssh -i studiox.pem ubuntu@13.250.175.113
cd studiox
bash deploy/deploy.sh
```

That's it. Migrations run before the new api container takes traffic, so
schema changes are safe.

---

## Common operations

```bash
# Tail live logs from everything
docker compose -f deploy/docker-compose.yml logs -f

# Tail one service
docker compose -f deploy/docker-compose.yml logs -f api
docker compose -f deploy/docker-compose.yml logs -f web

# Status of all services
docker compose -f deploy/docker-compose.yml ps

# Restart a service
docker compose -f deploy/docker-compose.yml restart api

# Open a psql shell
docker compose -f deploy/docker-compose.yml exec postgres \
  psql -U "$POSTGRES_USER" "$POSTGRES_DB"

# Shell into a container
docker compose -f deploy/docker-compose.yml exec api sh   # (won't work — distroless has no shell)
docker compose -f deploy/docker-compose.yml exec web sh

# Re-seed the super admin (e.g. after changing the password in .env)
docker compose -f deploy/docker-compose.yml --profile tools run --rm seed
```

---

## Backups

```bash
# Manual on-demand backup
bash deploy/backup.sh

# Nightly cron is installed automatically by setup-ec2.sh (idempotent —
# safe to re-run setup-ec2.sh after an upgrade, it won't double-install):
#   0 3 * * * /home/ubuntu/studiox/deploy/backup.sh >> /var/log/projectx-backup.log 2>&1
# Check it's there:
crontab -l | grep backup.sh

# Restore
gunzip -c deploy/backups/<file>.sql.gz | \
  docker compose -f deploy/docker-compose.yml exec -T postgres \
    psql -U "$POSTGRES_USER" "$POSTGRES_DB"
```

Backups land in `deploy/backups/` (gitignored), keeping the last 14 days
locally. Each run also copies the dump to
`s3://${S3_BUCKET}/backups/db/<file>.sql.gz` (same `AWS_*`/`S3_BUCKET` creds
already used for studio logo uploads) for offsite disaster recovery —
requires the `aws` CLI on the host and `S3_BUCKET` set in `.env`; if
`S3_BUCKET` is left blank, the offsite copy is skipped with a logged
message and the local backup still succeeds.

### Restore drill

Backups are only as good as your last successful restore. Periodically
(e.g. monthly), confirm the latest backup is actually valid:

1. Pick the newest file in `deploy/backups/` (or pull the latest from
   `s3://${S3_BUCKET}/backups/db/`).
2. Spin up a scratch Postgres container — **not** the production
   `postgres` service — and restore into that:
   ```bash
   docker run --rm -d --name px-restore-drill \
     -e POSTGRES_USER="$POSTGRES_USER" \
     -e POSTGRES_PASSWORD="$POSTGRES_PASSWORD" \
     -e POSTGRES_DB="$POSTGRES_DB" \
     pgvector/pgvector:pg16
   sleep 5   # wait for it to accept connections
   gunzip -c deploy/backups/<file>.sql.gz | \
     docker exec -i px-restore-drill psql -U "$POSTGRES_USER" "$POSTGRES_DB"
   docker exec px-restore-drill psql -U "$POSTGRES_USER" "$POSTGRES_DB" -c '\dt' # sanity-check tables came back
   docker rm -f px-restore-drill
   ```
3. If the restore errors or the row/table counts look wrong, the backup
   (or the backup process) is broken — fix it before you need it for real.

This is a manual runbook step, not an automated job — an automated drill
would mean something periodically restoring over a real target, which is
its own risk and is explicitly out of scope here.

## Read replica

`docker-compose.yml` runs a streaming (physical) read replica of
`postgres`, called `postgres_replica`, cloned via `pg_basebackup` on first
boot (see `deploy/postgres/replica-entrypoint.sh` and
`deploy/postgres/init-replication.sql`/`init-replication-hba.sh`). It's for
read-scaling and failover **visibility** only — there is no automatic
promotion/failover wired up. If the primary goes down, the replica keeps
serving stale reads (or stops receiving new WAL) but nothing fails over to
it automatically; that would be a separate, deliberately out-of-scope
change.

Requires `POSTGRES_REPLICATION_PASSWORD` in `.env` (both `postgres` and
`postgres_replica` refuse to start without it). The API can optionally
point read-only queries at `POSTGRES_READ_HOST` (defaults to
`postgres_replica` in `.env.example`) once the Go side supports it.

Check replication is actually streaming:

```bash
docker compose -f deploy/docker-compose.yml exec postgres \
  psql -U "$POSTGRES_USER" -c "SELECT * FROM pg_stat_replication;"
```

A populated row (one per connected replica) with `state = streaming` means
it's working. An empty result means the replica hasn't connected — check
`docker compose logs postgres_replica` for the `pg_basebackup` output.

---

## Observability (logs)

`docker-compose.yml` runs Loki (log storage), Promtail (ships every
container's stdout to Loki — auto-discovers containers via the Docker
socket, no per-service config needed) and Grafana (browse/search them).
Configs live in `deploy/loki/`, `deploy/promtail/`, `deploy/grafana/`.

This is logs only — no metrics (Prometheus) or alerting yet; that's
later, separate work.

Requires `GRAFANA_ADMIN_PASSWORD` in `.env` (the `grafana` service refuses
to start without it).

Grafana is bound to `127.0.0.1:3001` on the box only — it is **never**
exposed publicly and is not proxied through nginx. Reach it over an SSH
tunnel:

```bash
ssh -L 3001:localhost:3001 ubuntu@your-ec2-host
# then open http://localhost:3001 on your own machine, log in as
# admin / $GRAFANA_ADMIN_PASSWORD
```

In Grafana, use **Explore** with the Loki datasource (already
provisioned). Useful starting queries:

```logql
{service="api"}
{service="api", level="ERROR"}
{service="wa-web"} |= "error"
```

Note: Go services (`api`) log structured JSON via `slog` with text levels
("INFO"/"ERROR"/"WARN"); Node services (`wa-web`, `tg-web`) log via `pino`
with numeric levels (30/40/50/60). Filtering by `level` works within one
ecosystem but the values don't line up across the two — a known,
accepted limitation for now (see `deploy/promtail/promtail-config.yml`).

---

## Adding a domain + HTTPS later

Once you have DNS pointing at the box:

1. In `deploy/.env`, change `COOKIE_DOMAIN` and `PUBLIC_FORM_BASE_URL` to
   the domain. Set `COOKIE_SECURE=true`.
2. Add a certbot sidecar to `docker-compose.yml` (or run certbot directly
   on the host) and uncomment the `443:443` port + the `certs` volume in
   the `nginx` service.
3. Update `nginx/default.conf` with a `listen 443 ssl;` block and a 301
   redirect from `:80`.
4. `bash deploy/deploy.sh`.

I'll write that section out in detail when you're ready — for now the
HTTP-only setup is the right shape.

---

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| `docker: command not found` after `setup-ec2.sh` | You haven't logged out + back in yet |
| `permission denied` on `/var/run/docker.sock` | Same — log out, log back in |
| `bind: address already in use :80` | Something else is on port 80 — `sudo lsof -nP -iTCP:80` |
| Web compiles but pages 500 | Check `docker compose logs web`. Common: API_BASE_URL not reaching `api:8080`. |
| Login works, then logout immediately | `COOKIE_DOMAIN` mismatch with the host you're hitting |
| Sheets worker logs `disabled — no credentials` | Either deliberate (no creds) or the file isn't at `deploy/secrets/google-credentials.json` |
