// Browser-side API helper.
//
// All requests use *relative* paths (e.g. `/api/v1/auth/login`). The browser
// resolves them against `window.location.origin` automatically, and Next.js's
// rewrite rule (see next.config.mjs) proxies `/api/*` to the Go backend in
// dev — same-origin in prod via the deploy's ingress. There is intentionally
// no hardcoded API host in client code.

import { notifySessionExpiring } from './sessionGuard';

// The CSRF token lives in a deliberately non-HttpOnly cookie (px_csrf, set
// alongside the session cookie on login/every refresh — see
// identity/http.go's issueSessionCookie/RequireAuth) specifically so this
// can read it and echo it back as a header. The security property isn't
// "the cookie is secret" — it's that a cross-site attacker page can trigger
// the cookie to be sent automatically but can never read its value (browser
// same-origin policy) to also forge a matching header, which the API
// compares against the session's own sealed CSRFToken claim.
export function readCSRFCookie(): string | undefined {
  if (typeof document === 'undefined') return undefined;
  const match = document.cookie.match(/(?:^|; )px_csrf=([^;]*)/);
  const raw = match?.[1];
  return raw !== undefined ? decodeURIComponent(raw) : undefined;
}

const MUTATING_METHODS = new Set(['POST', 'PATCH', 'PUT', 'DELETE']);

export class ApiError extends Error {
  status: number;
  code?: string;
  details?: Record<string, string>;

  constructor(message: string, status: number, code?: string, details?: Record<string, string>) {
    super(message);
    this.status = status;
    this.code = code;
    this.details = details;
  }
}

interface ApiOptions extends Omit<RequestInit, 'body'> {
  json?: unknown;
}

export async function api<T>(path: string, opts: ApiOptions = {}): Promise<T> {
  const { json, headers, ...rest } = opts;
  const method = (rest.method ?? 'GET').toUpperCase();
  const csrfHeaders: Record<string, string> = {};
  if (MUTATING_METHODS.has(method)) {
    const token = readCSRFCookie();
    if (token) csrfHeaders['X-CSRF-Token'] = token;
  }
  const res = await fetch(path, {
    ...rest,
    credentials: 'include',
    headers: {
      ...(json ? { 'Content-Type': 'application/json' } : {}),
      ...csrfHeaders,
      ...headers,
    },
    body: json ? JSON.stringify(json) : (rest as RequestInit).body,
  });

  if (res.status === 204) return undefined as T;

  const text = await res.text();
  const body = text ? JSON.parse(text) : undefined;

  if (res.status === 401 && body?.code === 'unauthorized' && typeof window !== 'undefined') {
    // Expired JWT, idle session timeout, or revoked session — a full
    // navigation to /login rather than a client-side router push, so any
    // stale in-memory state from the dead session doesn't linger. Checked
    // by code, not just status: a failed *login attempt* is also a 401 but
    // uses invalid_credentials/account_inactive/role_inactive — those must
    // stay on the login form as an inline error, not bounce through here.
    //
    // notifySessionExpiring() fires first and synchronously, so any mounted
    // component with not-yet-saved state (see lib/sessionGuard.ts) gets one
    // last chance to stash it to localStorage before this navigation tears
    // the page down. The `next` param sends the user back to the same page
    // once they log back in instead of the role default.
    notifySessionExpiring();
    const next = window.location.pathname + window.location.search;
    window.location.href = `/login?next=${encodeURIComponent(next)}`;
  }

  if (!res.ok) {
    throw new ApiError(
      body?.error ?? `HTTP ${res.status}`,
      res.status,
      body?.code,
      body?.details,
    );
  }
  return body as T;
}
