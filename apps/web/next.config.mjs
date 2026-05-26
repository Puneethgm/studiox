import path from 'node:path';
import { fileURLToPath } from 'node:url';

const configDir = path.dirname(fileURLToPath(import.meta.url));

/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,

  // Standalone output: Next.js writes a self-contained server + minimal
  // node_modules to .next/standalone. The Docker image ships only that,
  // not the whole monorepo. Keeps the prod image ~150MB instead of ~1GB.
  output: 'standalone',
  // Tell Next where the workspace root is so it traces deps correctly.
  outputFileTracingRoot: path.join(configDir, '../..'),

  serverExternalPackages: ['officeparser', 'pdfjs-dist'],

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
};

export default nextConfig;
