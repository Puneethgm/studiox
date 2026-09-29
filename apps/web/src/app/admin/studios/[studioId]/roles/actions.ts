'use server';

// Server Actions for identity/studio-roles — same pattern as
// ../settings/actions.ts: run on the server, forward the auth cookie,
// revalidate every page whose data depends on roles (the roles list itself,
// and the users list, since it shows each user's role name).

import { cookies } from 'next/headers';
import { revalidatePath } from 'next/cache';
import type { StudioRole } from '@/lib/types';

const API_BASE = process.env.API_BASE_URL ?? 'http://localhost:8080';

export type RoleResult =
  | { ok: true; role: StudioRole }
  | { ok: false; error: string; details?: Record<string, string> };

export type DeleteRoleResult =
  | { ok: true }
  | { ok: false; error: string; details?: Record<string, string> };

async function cookieHeader() {
  const cookieStore = await cookies();
  return cookieStore
    .getAll()
    .map((c) => `${c.name}=${c.value}`)
    .join('; ');
}

async function parseErrorBody(res: Response) {
  type ErrBody = { error?: string; details?: Record<string, string> };
  const body = (await res.json().catch(() => null)) as ErrBody | null;
  return { error: body?.error ?? `HTTP ${res.status}`, details: body?.details };
}

export async function createRole(
  studioId: string,
  data: { name: string; description: string; permissionKeys: string[] },
): Promise<RoleResult> {
  const res = await fetch(`${API_BASE}/api/v1/studios/${studioId}/roles`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Cookie: await cookieHeader() },
    body: JSON.stringify(data),
    cache: 'no-store',
  });
  if (!res.ok) {
    const err = await parseErrorBody(res);
    return { ok: false, ...err };
  }
  revalidatePath(`/admin/studios/${studioId}/roles`);
  return { ok: true, role: await res.json() };
}

export async function updateRole(
  studioId: string,
  roleId: string,
  data: { name: string; description: string; permissionKeys: string[] },
): Promise<RoleResult> {
  const res = await fetch(`${API_BASE}/api/v1/studios/${studioId}/roles/${roleId}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json', Cookie: await cookieHeader() },
    body: JSON.stringify(data),
    cache: 'no-store',
  });
  if (!res.ok) {
    const err = await parseErrorBody(res);
    return { ok: false, ...err };
  }
  revalidatePath(`/admin/studios/${studioId}/roles`);
  revalidatePath(`/admin/studios/${studioId}/users`);
  return { ok: true, role: await res.json() };
}

export async function setRoleActive(studioId: string, roleId: string, active: boolean): Promise<RoleResult> {
  const res = await fetch(`${API_BASE}/api/v1/studios/${studioId}/roles/${roleId}/active`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json', Cookie: await cookieHeader() },
    body: JSON.stringify({ active }),
    cache: 'no-store',
  });
  if (!res.ok) {
    const err = await parseErrorBody(res);
    return { ok: false, ...err };
  }
  revalidatePath(`/admin/studios/${studioId}/roles`);
  revalidatePath(`/admin/studios/${studioId}/users`);
  return { ok: true, role: await res.json() };
}

export async function deleteRole(studioId: string, roleId: string): Promise<DeleteRoleResult> {
  const res = await fetch(`${API_BASE}/api/v1/studios/${studioId}/roles/${roleId}`, {
    method: 'DELETE',
    headers: { Cookie: await cookieHeader() },
    cache: 'no-store',
  });
  if (!res.ok) {
    const err = await parseErrorBody(res);
    return { ok: false, ...err };
  }
  revalidatePath(`/admin/studios/${studioId}/roles`);
  return { ok: true };
}
