'use server';

// Server Actions for identity/studio-users — same pattern as
// ../settings/actions.ts.

import { cookies } from 'next/headers';
import { revalidatePath } from 'next/cache';
import type { StudioUser } from '@/lib/types';

const API_BASE = process.env.API_BASE_URL ?? 'http://localhost:8080';

export type CreateStudioUserResult =
  | { ok: true; user: StudioUser }
  | { ok: false; error: string; details?: Record<string, string> };

export type DeactivateStudioUserResult =
  | { ok: true }
  | { ok: false; error: string; details?: Record<string, string> };

export type SetUserActiveResult =
  | { ok: true; user: StudioUser }
  | { ok: false; error: string };

async function cookieHeader() {
  const cookieStore = await cookies();
  return cookieStore
    .getAll()
    .map((c) => `${c.name}=${c.value}`)
    .join('; ');
}

export async function createStudioUser(
  studioId: string,
  data: { username: string; firstName: string; lastName: string; email: string; roleId: string },
): Promise<CreateStudioUserResult> {
  const res = await fetch(`${API_BASE}/api/v1/studios/${studioId}/users`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Cookie: await cookieHeader() },
    body: JSON.stringify(data),
    cache: 'no-store',
  });

  type ErrBody = { error?: string; details?: Record<string, string> };
  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as ErrBody | null;
    return { ok: false, error: body?.error ?? `HTTP ${res.status}`, details: body?.details };
  }

  revalidatePath(`/admin/studios/${studioId}/users`);
  return { ok: true, user: await res.json() };
}

export async function setUserActive(studioId: string, userId: string, active: boolean): Promise<SetUserActiveResult> {
  const res = await fetch(`${API_BASE}/api/v1/studios/${studioId}/users/${userId}/active`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json', Cookie: await cookieHeader() },
    body: JSON.stringify({ active }),
    cache: 'no-store',
  });
  if (!res.ok) {
    type ErrBody = { error?: string };
    const body = (await res.json().catch(() => null)) as ErrBody | null;
    return { ok: false, error: body?.error ?? `HTTP ${res.status}` };
  }
  revalidatePath(`/admin/studios/${studioId}/users`);
  return { ok: true, user: await res.json() };
}

export async function deactivateStudioUser(studioId: string, userId: string): Promise<DeactivateStudioUserResult> {
  const res = await fetch(`${API_BASE}/api/v1/studios/${studioId}/users/${userId}`, {
    method: 'DELETE',
    headers: { Cookie: await cookieHeader() },
    cache: 'no-store',
  });
  if (!res.ok) {
    type ErrBody = { error?: string };
    const body = (await res.json().catch(() => null)) as ErrBody | null;
    return { ok: false, error: body?.error ?? `HTTP ${res.status}` };
  }
  revalidatePath(`/admin/studios/${studioId}/users`);
  return { ok: true };
}
