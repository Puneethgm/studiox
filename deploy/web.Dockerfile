# syntax=docker/dockerfile:1.6
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
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/web/package.json apps/web/
RUN pnpm install --frozen-lockfile

# ---------- build ----------
FROM node:22-alpine AS build
WORKDIR /repo
RUN corepack enable
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

# Standalone output: a tiny self-contained server tree.
COPY --from=build /repo/apps/web/.next/standalone ./
COPY --from=build /repo/apps/web/.next/static     ./apps/web/.next/static
COPY --from=build /repo/apps/web/public           ./apps/web/public

EXPOSE 3000
CMD ["node", "apps/web/server.js"]
