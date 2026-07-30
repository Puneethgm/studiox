#
# Build context: repo root.
#   docker build -f deploy/tg-web.Dockerfile -t projectx-tg-web .
#
# teleproto (MTProto client, GramJS's maintained fork) is pure JS — no
# native build step needed, unlike some other MTProto/media libraries.
#
FROM node:22-slim

WORKDIR /app

# Install Node dependencies
COPY apps/tg-web/package.json ./
RUN npm install --omit=dev

# Source
COPY apps/tg-web/src ./src

EXPOSE 3101
CMD ["node", "src/index.js"]
