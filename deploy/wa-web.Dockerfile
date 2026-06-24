#
# Build context: repo root.
#   docker build -f deploy/wa-web.Dockerfile -t projectx-wa-web .
#
# Uses Debian slim so we can install Google Chrome (required by whatsapp-web.js/Puppeteer).
#
FROM node:22-slim

# Install Chrome dependencies + Google Chrome stable
RUN apt-get update && apt-get install -y \
    wget gnupg ca-certificates \
    --no-install-recommends \
  && wget -q -O - https://dl.google.com/linux/linux_signing_key.pub | \
     gpg --dearmor -o /usr/share/keyrings/google-chrome.gpg \
  && echo "deb [arch=amd64 signed-by=/usr/share/keyrings/google-chrome.gpg] \
     http://dl.google.com/linux/chrome/deb/ stable main" \
     > /etc/apt/sources.list.d/google-chrome.list \
  && apt-get update && apt-get install -y \
     google-chrome-stable \
     --no-install-recommends \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Install Node dependencies
COPY apps/wa-web/package.json ./
RUN npm install --omit=dev

# Source
COPY apps/wa-web/src ./src

# Auth sessions are persisted in a Docker volume
ENV AUTH_DIR=/auth
VOLUME ["/auth"]

EXPOSE 3100
CMD ["node", "src/index.js"]
