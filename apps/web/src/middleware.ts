import { NextRequest, NextResponse } from 'next/server';

// Per-request CSP nonce (platform-security-hardening task 8.1) — lets
// script-src drop 'unsafe-inline' in production while this app's own
// few known inline scripts (layout.tsx's theme-detector, the two GTM
// snippets) still execute via an explicit nonce attribute instead.
// Dev keeps 'unsafe-inline'/'unsafe-eval' on top of the nonce: Next's
// hot-reload/react-refresh runtime uses eval() and isn't nonce-aware,
// and breaking it would make local dev unusable (see the removed CSP
// comment this replaced in next.config.mjs, which documented the exact
// same dev-mode constraint for the previous, non-nonce policy).
function buildCSP(nonce: string, isDev: boolean) {
  return [
    "default-src 'self'",
    `script-src 'self' 'nonce-${nonce}'${isDev ? " 'unsafe-inline' 'unsafe-eval'" : ''} https://www.googletagmanager.com https://js.stripe.com`,
    "style-src 'self' 'unsafe-inline' https://fonts.googleapis.com",
    "img-src 'self' data: blob: https:",
    "font-src 'self' data: https://fonts.gstatic.com",
    "connect-src 'self' https://*.stripe.com https://www.google-analytics.com https://www.googletagmanager.com",
    "frame-src 'self' https://js.stripe.com https://hooks.stripe.com https://*.stripe.com",
    "object-src 'none'",
    "base-uri 'self'",
    "form-action 'self'",
    "report-uri /api/csp-report",
  ].join('; ');
}

// requireSession / serverFetch (lib/auth.ts) redirect to /login on a dead
// session, but a Server Component has no built-in way to read the path it's
// currently rendering. This stamps it onto a request header so those
// redirects can carry it as /login?next=<path>, letting the login page send
// the user back to the page they were on instead of always landing on the
// role default. Scoped to /admin/* — the only auth-gated tree.
export function middleware(request: NextRequest) {
  const headers = new Headers(request.headers);
  headers.set('x-pathname', request.nextUrl.pathname + request.nextUrl.search);

  const nonce = crypto.randomUUID().replace(/-/g, '');
  headers.set('x-nonce', nonce);

  const response = NextResponse.next({ request: { headers } });
  response.headers.set(
    'Content-Security-Policy',
    buildCSP(nonce, process.env.NODE_ENV !== 'production'),
  );
  return response;
}

export const config = {
  // Broadened from '/admin/:path*' to every page route (task 8.1 needs a
  // per-request nonce everywhere a page can render an inline script, not
  // just /admin) — still excludes _next assets and common static file
  // extensions so this middleware doesn't run on every JS/CSS/image
  // request.
  matcher: ['/((?!_next/static|_next/image|favicon.ico|.*\\.(?:svg|png|jpg|jpeg|gif|webp|ico)$).*)'],
};
