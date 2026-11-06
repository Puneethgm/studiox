#
# Build context: repo root.
#   docker build -f deploy/web.Dockerfile -t projectx-web .
#
# Uses Next.js standalone output so the runtime image only ships the
# server.js + minimal node_modules — no pnpm, no source, no .next/cache.

# ---------- deps (cached layer) ----------
FROM node:22-alpine AS deps
WORKDIR /repo
RUN corepack enable
ARG API_BASE_URL=http://localhost:8080
ENV API_BASE_URL=${API_BASE_URL}
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/web/package.json apps/web/
RUN pnpm install --frozen-lockfile

# ---------- build ----------
FROM node:22-alpine AS build
WORKDIR /repo
RUN corepack enable
ARG API_BASE_URL=http://localhost:8080
ENV API_BASE_URL=${API_BASE_URL}
COPY --from=deps /repo/node_modules        ./node_modules
COPY --from=deps /repo/apps/web/node_modules ./apps/web/node_modules
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml tsconfig.base.json ./
COPY apps/web ./apps/web
# `mkdir -p public` ensures the runtime COPY succeeds even if no static
# assets are committed (Next.js itself treats public/ as optional).
RUN cd apps/web && pnpm build && mkdir -p public

# ---------- runtime ----------
FROM node:22-alpine AS runtime
WORKDIR /app
ENV NODE_ENV=production
ENV PORT=3000
ENV HOSTNAME=0.0.0.0

# This container only ever runs `node apps/web/server.js` — it never
# invokes npm/corepack/pnpm. node:22-alpine still bundles its own global
# npm CLI (and npm's own internal deps like pacote/sigstore/brace-expansion),
# which periodically picks up HIGH/CRITICAL CVEs in npm's own tooling that
# have nothing to do with our app. Removing it shrinks the actual attack
# surface instead of waiting on an upstream node:22-alpine rebuild.
RUN rm -rf /usr/local/lib/node_modules/npm /usr/local/lib/node_modules/corepack \
    /usr/local/bin/npm /usr/local/bin/npx /usr/local/bin/corepack

# Standalone output: a tiny self-contained server tree.
COPY --from=build /repo/apps/web/.next/standalone ./
COPY --from=build /repo/apps/web/.next/static     ./apps/web/.next/static
COPY --from=build /repo/apps/web/public           ./apps/web/public

EXPOSE 3000
CMD ["node", "apps/web/server.js"]
