# Deploy — single-EC2 Docker

Everything in this folder is the production deployment for Project-X. One
box, Docker Compose, no CI/CD, no image registry. SSH in, run a script, the
app is live.

```
deploy/
  docker-compose.yml      # the stack: postgres + api + web + nginx + one-shots
  api.Dockerfile          # multi-stage Go build → distroless (~30 MB)
  web.Dockerfile          # multi-stage Next.js → standalone (~150 MB)
  nginx/default.conf      # reverse proxy: /api/* → api,  /* → web
  setup-ec2.sh            # one-time: install Docker, swap, dirs
  deploy.sh               # day-to-day: pull → build → migrate → seed → up
  backup.sh               # nightly pg_dump (cron)
  .env.example            # production env template
  .env                    # YOUR real values (gitignored)
  secrets/                # mounted at /secrets in the api container
  backups/                # gzipped pg_dump output
```

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
- `SUPER_ADMIN_PASSWORD` — your login password
- `COOKIE_DOMAIN` and `PUBLIC_FORM_BASE_URL` — your IP (or domain when added)

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

# Schedule nightly at 03:00 UTC
crontab -e
# add this line:
#   0 3 * * * /home/ubuntu/studiox/deploy/backup.sh >> /var/log/projectx-backup.log 2>&1

# Restore
gunzip -c deploy/backups/<file>.sql.gz | \
  docker compose -f deploy/docker-compose.yml exec -T postgres \
    psql -U "$POSTGRES_USER" "$POSTGRES_DB"
```

Backups land in `deploy/backups/` (gitignored), keeping the last 14 days.
For real disaster recovery later: rsync the `backups/` dir to S3 nightly.

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
