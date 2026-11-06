import path from 'node:path';
import { fileURLToPath } from 'node:url';

const configDir = path.dirname(fileURLToPath(import.meta.url));

/** @type {import('next).NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  devIndicators: false,

  // Standalone output: Next.js writes a self-contained server + minimal
  // node_modules to .next/standalone. The Docker image ships only that,
  // not the whole monorepo. Keeps the prod image ~150MB instead of ~1GB.
  output: 'standalone',
  // Tell Next where the workspace root is so it traces deps correctly.
  outputFileTracingRoot: path.join(configDir, '../..'),

  serverExternalPackages: ['officeparser', 'pdfjs-dist'],

  // Next.js caps Server Action request bodies at 1MB by default — too small
  // for Knowledge Base file uploads (parseDocument/updateKnowledgeBase in
  // admin/studios/[studioId]/knowledge-base/actions.ts). Raised to cover
  // large documents; local dev only — production's nginx still caps uploads
  // at 50MB (deploy/nginx/nginx/nginx.conf), independent of this.
  experimental: {
    serverActions: {
      bodySizeLimit: '200mb',
    },
  },

  // Disable source maps in production for security
  // (prevent frontend code exposure in DevTools)
  productionBrowserSourceMaps: false,

  // Same-origin API: the browser always calls /api/* on whatever host it's
  // loaded from, and Next.js proxies it to the Go backend. In dev that's
  // localhost:8080; in prod nginx fronts the API at /api/* on the same
  // origin (so this rewrite is unused in prod, but harmless). The browser
  // never sees a hardcoded host.
  async rewrites() {
    const apiBase = process.env.API_BASE_URL ?? 'http://localhost:8080';
    return [
      {
        source: '/api/:path*',
        destination: `${apiBase}/api/:path*`,
      },
      {
        source: '/uploads/:path*',
        destination: `${apiBase}/uploads/:path*`,
      },
    ];
  },

  // Security headers to prevent source map access. Content-Security-
  // Policy moved to src/middleware.ts (platform-security-hardening task
  // 8.1) — it needs a fresh nonce per request, which this static
  // headers() config (evaluated once at build/start, not per-request)
  // cannot generate. Keeping it here too would just mean two
  // Content-Security-Policy headers on the same response, which
  // browsers handle by intersecting them — effectively applying
  // whichever is stricter per-directive — so it's removed here instead
  // of left to silently fight the per-request one.
  async headers() {
    return [
      {
        source: '/:path*',
        headers: [
          {
            key: 'X-Content-Type-Options',
            value: 'nosniff',
          },
          {
            key: 'X-Frame-Options',
            value: 'DENY',
          },
          {
            key: 'X-XSS-Protection',
            value: '1; mode=block',
          },
        ],
      },
    ];
  },
};

export default nextConfig;
