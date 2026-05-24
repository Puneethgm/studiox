#!/usr/bin/env bash
#
# One-time bootstrap for a fresh Ubuntu EC2 instance.
# Installs Docker + Compose plugin, adds the current user to the docker
# group, and provisions a 2GB swapfile (helps small instances during
# Next.js builds).
#
# Run as a sudo-capable user:
#   bash deploy/setup-ec2.sh
#
# After this finishes, log out + back in so the docker group takes effect.

set -euo pipefail

echo "==> apt update + upgrade"
sudo apt-get update -y
sudo DEBIAN_FRONTEND=noninteractive apt-get upgrade -y

echo "==> Installing Docker Engine + Compose plugin"
if ! command -v docker >/dev/null 2>&1; then
  sudo install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg \
    | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
  sudo chmod a+r /etc/apt/keyrings/docker.gpg
  . /etc/os-release
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu ${VERSION_CODENAME} stable" \
    | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
  sudo apt-get update -y
  sudo apt-get install -y \
    docker-ce docker-ce-cli containerd.io \
    docker-buildx-plugin docker-compose-plugin
else
  echo "    docker already installed: $(docker --version)"
fi

echo "==> Adding $USER to the docker group"
sudo usermod -aG docker "$USER" || true

echo "==> Setting up 2GB swapfile (helps small instances during builds)"
if [ ! -f /swapfile ]; then
  sudo fallocate -l 2G /swapfile
  sudo chmod 600 /swapfile
  sudo mkswap /swapfile
  sudo swapon /swapfile
  echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab > /dev/null
else
  echo "    swapfile already exists"
fi

echo "==> Creating deploy/secrets and deploy/backups dirs"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
mkdir -p "${SCRIPT_DIR}/secrets" "${SCRIPT_DIR}/backups"

cat <<EOF

✓ EC2 setup complete.

Next steps:
  1. Open inbound port 80 in the EC2 security group (22 should already be open;
     add 443 too when you're ready for TLS).
  2. Log out and log back in so the docker group takes effect:
       exit
       ssh -i studiox.pem ubuntu@<your-ip>
  3. Copy and edit the env template:
       cp deploy/.env.example deploy/.env
       nano deploy/.env       # fill in real values
  4. Run the first deploy:
       bash deploy/deploy.sh

EOF
