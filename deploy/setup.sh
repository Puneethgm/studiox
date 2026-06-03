#!/bin/bash
# =============================================================
#  1herosocial.ai — EC2 Production Setup Script
#  Run this once on a fresh Ubuntu EC2 instance
#  Usage: chmod +x setup.sh && sudo ./setup.sh
# =============================================================

set -e  # Exit immediately on error

DOMAIN="1herosocial.ai"
REPO_URL=""  # <-- FILL THIS IN: your git repo URL
APP_DIR="/home/ubuntu/studiox"
EMAIL="admin@1herosocial.ai"  # <-- FILL THIS IN: your email for SSL cert

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

log()  { echo -e "${GREEN}[✓] $1${NC}"; }
warn() { echo -e "${YELLOW}[!] $1${NC}"; }
err()  { echo -e "${RED}[✗] $1${NC}"; exit 1; }

# ── 1. System update ──────────────────────────────────────────
log "Updating system packages..."
apt-get update -qq && apt-get upgrade -y -qq

# ── 2. Install dependencies ───────────────────────────────────
log "Installing Docker, Certbot, Git..."
apt-get install -y -qq \
    docker.io \
    docker-compose-v2 \
    git \
    certbot \
    curl \
    ufw

# ── 3. Start & enable Docker ──────────────────────────────────
log "Enabling Docker..."
systemctl enable docker
systemctl start docker
usermod -aG docker ubuntu

# ── 4. Configure firewall ─────────────────────────────────────
log "Configuring UFW firewall..."
ufw allow 22/tcp   # SSH
ufw allow 80/tcp   # HTTP
ufw allow 443/tcp  # HTTPS
ufw --force enable

# ── 5. Clone repository ───────────────────────────────────────
if [ -z "$REPO_URL" ]; then
    warn "REPO_URL is not set! Please edit this script and set REPO_URL."
    warn "Skipping git clone. Make sure code is at: $APP_DIR"
else
    log "Cloning repository..."
    if [ -d "$APP_DIR" ]; then
        warn "Directory $APP_DIR already exists, pulling latest..."
        cd "$APP_DIR" && git pull
    else
        git clone "$REPO_URL" "$APP_DIR"
    fi
fi

cd "$APP_DIR"

# ── 6. Create deploy/.env if not exists ─────────────────────
if [ ! -f "$APP_DIR/deploy/.env" ]; then
    log "Creating deploy/.env from example..."
    cp "$APP_DIR/deploy/.env.example" "$APP_DIR/deploy/.env"
    warn "IMPORTANT: Edit deploy/.env with your real secrets before continuing!"
    warn "  nano $APP_DIR/deploy/.env"
    warn "Press ENTER after editing deploy/.env to continue..."
    read -r
fi

# ── 7. Issue SSL certificate via Let's Encrypt ────────────────
log "Obtaining SSL certificate for $DOMAIN..."
if [ -d "/etc/letsencrypt/live/$DOMAIN" ]; then
    warn "SSL certificate already exists for $DOMAIN — skipping."
else
    # Port 80 must be free — no nginx running yet
    certbot certonly \
        --standalone \
        --non-interactive \
        --agree-tos \
        --email "$EMAIL" \
        -d "$DOMAIN" \
        -d "www.$DOMAIN"
    log "SSL certificate issued successfully!"
fi

# ── 8. Set up SSL auto-renewal cron ──────────────────────────
log "Setting up SSL auto-renewal cron job..."
(crontab -l 2>/dev/null | grep -v "certbot renew"; echo "0 3 * * * certbot renew --quiet && docker exec projectx-nginx nginx -s reload") | crontab -
log "Auto-renewal cron job added (runs daily at 3am)."

# ── 9. Build & start Docker containers ───────────────────────
log "Building Docker images (this may take a few minutes)..."
docker compose -f "$APP_DIR/deploy/docker-compose.prod.yml" --env-file "$APP_DIR/deploy/.env" build

log "Starting all services..."
docker compose -f "$APP_DIR/deploy/docker-compose.prod.yml" --env-file "$APP_DIR/deploy/.env" up -d

# ── 10. Wait for services to be healthy ───────────────────────
log "Waiting for services to start..."
sleep 10

# ── 11. Check status ──────────────────────────────────────────
log "Checking container status..."
docker compose -f "$APP_DIR/deploy/docker-compose.prod.yml" ps

# ── Done! ─────────────────────────────────────────────────────
echo ""
echo -e "${GREEN}============================================${NC}"
echo -e "${GREEN}  Deployment complete!${NC}"
echo -e "${GREEN}============================================${NC}"
echo -e "  🌐  https://$DOMAIN"
echo -e "  📋  Logs: docker compose -f docker-compose.prod.yml logs -f"
echo -e "  🔄  Update: git pull && docker compose -f docker-compose.prod.yml up -d --build"
echo ""
