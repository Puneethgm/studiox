#
# Build context: repo root.
#   docker build -f deploy/wa-web.Dockerfile -t projectx-wa-web .
#
# Baileys talks WhatsApp's protocol directly over a WebSocket — no browser,
# so no Chrome/Puppeteer install is needed here (unlike the old
# whatsapp-web.js-based version of this service).
#
FROM node:22-slim

# git is required at install time by some of Baileys' transitive dependencies
# (git-hosted packages in their dependency tree), not present in the slim base image.
RUN apt-get update && apt-get install -y --no-install-recommends git ca-certificates \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Install Node dependencies
COPY apps/wa-web/package.json ./
RUN npm install --omit=dev

# Source
COPY apps/wa-web/src ./src

# Auth state is persisted in a Docker volume
ENV AUTH_DIR=/auth
VOLUME ["/auth"]

EXPOSE 3100
CMD ["node", "src/index.js"]
