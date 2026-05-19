import { cookies } from 'next/headers';
import { redirect } from 'next/navigation';
import type { Me } from './types';

// Server-only address of the Go API. Never exposed to the browser.
const BASE = process.env.API_BASE_URL ?? 'http://localhost:8080';

async function cookieHeader(): Promise<string> {
  const store = await cookies();
  return store.getAll().map((c) => `${c.name}=${c.value}`).join('; ');
}

// requireSession runs in server components / layouts and redirects to /login
// when the cookie is missing or the API rejects it. Returns the current user.
export async function requireSession(): Promise<Me> {
  const ck = await cookieHeader();
  const res = await fetch(`${BASE}/api/v1/auth/me`, {
    headers: ck ? { Cookie: ck } : {},
    cache: 'no-store',
  });
  if (!res.ok) redirect('/login');
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
  if (!res.ok) {
    throw new Error(`API ${path}: ${res.status}`);
  }
  return (await res.json()) as T;
}
