import { cookies, headers as nextHeaders } from 'next/headers';
import { redirect } from 'next/navigation';
import type { Me } from './types';
import { sanitizeNextPath } from './sessionGuard';

// Server-only address of the Go API. Never exposed to the browser.
const BASE = process.env.API_BASE_URL ?? 'http://localhost:8080';

async function cookieHeader(): Promise<string> {
  const store = await cookies();
  return store.getAll().map((c) => `${c.name}=${c.value}`).join('; ');
}

// /login?next=<path>, built from the x-pathname header middleware.ts stamps
// on every /admin/* request — so a dead-session redirect sends the user
// back to the page they were on, not always the role default.
async function loginRedirectPath(): Promise<string> {
  const h = await nextHeaders();
  const next = sanitizeNextPath(h.get('x-pathname'));
  return next ? `/login?next=${encodeURIComponent(next)}` : '/login';
}

// requireSession runs in server components / layouts and redirects to /login
// when the cookie is missing or the API rejects it. Returns the current user.
export async function requireSession(): Promise<Me> {
  const ck = await cookieHeader();
  const res = await fetch(`${BASE}/api/v1/auth/me`, {
    headers: ck ? { Cookie: ck } : {},
    cache: 'no-store',
  });
  if (!res.ok) redirect(await loginRedirectPath());
  return (await res.json()) as Me;
}

// serverFetch is for RSC data loading: forwards the auth cookie to the API.
export async function serverFetch<T>(path: string, init: RequestInit = {}): Promise<T> {
  const ck = await cookieHeader();
  const res = await fetch(`${BASE}${path}`, {
    ...init,
    headers: {
      ...(init.headers ?? {}),
      ...(ck ? { Cookie: ck } : {}),
    },
    cache: 'no-store',
  });
  if (res.status === 401) {
    const body = await res.json().catch(() => null) as { code?: string } | null;
    // Expired JWT, idle session timeout, or revoked session (logout,
    // password change) — send them back to log in instead of surfacing a
    // generic "Something went wrong" error boundary for what's really just
    // an expired session. serverFetch is only ever used for already-
    // authenticated data loading (never the login attempt itself), so any
    // "unauthorized" here genuinely means the session is dead — but keep
    // the code check anyway, matching the client-side api() helper, in
    // case that ever changes.
    if (body?.code === 'unauthorized') redirect(await loginRedirectPath());
    throw new Error(`API ${path}: ${res.status}`);
  }
  if (!res.ok) {
    throw new Error(`API ${path}: ${res.status}`);
  }
  return (await res.json()) as T;
}
