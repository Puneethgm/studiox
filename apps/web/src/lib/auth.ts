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

// csrfHeader reads the px_csrf cookie (set alongside the session cookie by
// the API — see identity/http.go) and returns it as a header object ready
// to spread into a fetch call: `headers: { ...someHeaders, ...(await
// csrfHeader()) }`. A Server Action running on the Next.js server has no
// document.cookie, so it has to read this the same way it reads the auth
// cookie — via next/headers' cookies() — and echo it back itself, the same
// contract the client-side api() helper (lib/api.ts) fulfills in the
// browser. Every Server Action in this app that does a mutating fetch
// (POST/PATCH/PUT/DELETE) needs this spread into its headers or the API's
// CSRF check (RequireAuth) rejects it with 403 csrf_mismatch.
export async function csrfHeader(): Promise<Record<string, string>> {
  const token = (await cookies()).get('px_csrf')?.value;
  return token ? { 'X-CSRF-Token': token } : {};
}

// serverMutate is serverFetch's counterpart for Server Actions that write
// (POST/PATCH/PUT/DELETE) — forwards the auth cookie same as serverFetch,
// plus the CSRF token. The px_csrf cookie (set alongside the session cookie
// by the API — see identity/http.go) is deliberately NOT HttpOnly for
// exactly this: a Server Action runs on the Next.js server, which has no
// access to document.cookie, so it has to read it the same way it reads the
// auth cookie — via next/headers' cookies() — and echo it back as the
// X-CSRF-Token header itself, the same contract the client-side api()
// helper (lib/api.ts) fulfills in the browser. Does NOT redirect on 401
// (unlike serverFetch) — a Server Action's caller is a client component
// expecting a normal return value to show an inline error, not a server
// redirect out from under it.
export async function serverMutate<T>(path: string, init: RequestInit = {}): Promise<T> {
  const store = await cookies();
  const ck = store.getAll().map((c) => `${c.name}=${c.value}`).join('; ');
  const csrfToken = store.get('px_csrf')?.value;
  const res = await fetch(`${BASE}${path}`, {
    ...init,
    headers: {
      ...(init.headers ?? {}),
      ...(ck ? { Cookie: ck } : {}),
      ...(csrfToken ? { 'X-CSRF-Token': csrfToken } : {}),
    },
    cache: 'no-store',
  });
  if (!res.ok) {
    const body = await res.json().catch(() => null) as { error?: string } | null;
    throw new Error(body?.error ?? `API ${path}: ${res.status}`);
  }
  if (res.status === 204) return undefined as T;
  const text = await res.text();
  return (text ? JSON.parse(text) : undefined) as T;
}
