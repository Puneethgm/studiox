'use server';

// Server Action for updating a studio's settings. Same pattern as the lead
// action — runs on the server, forwards auth cookie, revalidates the pages
// that display studio identity (overview, super-admin studios list, the
// public form for the studio's slug, and the AppShell which reads /me).

import { cookies } from 'next/headers';
import { revalidatePath } from 'next/cache';

const API_BASE = process.env.API_BASE_URL ?? 'http://localhost:8080';

export type UpdateStudioResult =
  | { ok: true }
  | { ok: false; error: string; details?: Record<string, string> };

export type ChangePasswordResult =
  | { ok: true }
  | { ok: false; error: string; details?: Record<string, string> };

export async function updateStudioSettings(
  studioId: string,
  studioSlug: string,
  data: {
    name: string;
    brandColor: string;
    logoUrl: string;
    contactEmail: string;
    active: boolean;
    availabilitySlots?: any;
    availabilityTimezone?: string;
    geminiApiKey?: string;
    metaAppId?: string;
    metaAppSecret?: string;
    googleClientId?: string;
    googleClientSecret?: string;
    googleDeveloperToken?: string;
    knowledgeBase?: string;
  },
): Promise<UpdateStudioResult> {
  const cookieStore = await cookies();
  const cookieHeader = cookieStore
    .getAll()
    .map((c) => `${c.name}=${c.value}`)
    .join('; ');

  const res = await fetch(`${API_BASE}/api/v1/me/studios/${studioId}`, {
    method: 'PATCH',
    headers: {
      'Content-Type': 'application/json',
      ...(cookieHeader ? { Cookie: cookieHeader } : {}),
    },
    body: JSON.stringify(data),
    cache: 'no-store',
  });

  type ErrBody = { error?: string; details?: Record<string, string> };
  const body = (await res.json().catch(() => null)) as ErrBody | null;

  if (!res.ok) {
    return {
      ok: false,
      error: body?.error ?? `HTTP ${res.status}`,
      details: body?.details,
    };
  }

  // Studio identity surfaces in many places — invalidate them all.
  revalidatePath(`/admin/studios/${studioId}`);
  revalidatePath(`/admin/studios/${studioId}/settings`);
  revalidatePath(`/admin/studios`); // super-admin list shows logo/colour
  revalidatePath(`/admin`);
  revalidatePath(`/l/${studioSlug}`, 'layout'); // public form for this studio

  return { ok: true };
}

export async function changeMyPassword(data: {
  currentPassword: string;
  newPassword: string;
  confirmPassword: string;
}): Promise<ChangePasswordResult> {
  const cookieStore = await cookies();
  const cookieHeader = cookieStore
    .getAll()
    .map((c) => `${c.name}=${c.value}`)
    .join('; ');

  const res = await fetch(`${API_BASE}/api/v1/auth/password`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(cookieHeader ? { Cookie: cookieHeader } : {}),
    },
    body: JSON.stringify(data),
    cache: 'no-store',
  });

  if (res.ok) {
    revalidatePath('/admin');
    return { ok: true };
  }

  type ErrBody = { error?: string; details?: Record<string, string> };
  const body = (await res.json().catch(() => null)) as ErrBody | null;

  return {
    ok: false,
    error: body?.error ?? `HTTP ${res.status}`,
    details: body?.details,
  };
}

export interface SheetsSettingsResult {
  ok: boolean;
  error?: string;
  data?: {
    spreadsheetId: string;
    tabName: string;
    active: boolean;
  };
}

export async function getSheetsSettings(studioId: string): Promise<SheetsSettingsResult> {
  const cookieStore = await cookies();
  const cookieHeader = cookieStore
    .getAll()
    .map((c) => `${c.name}=${c.value}`)
    .join('; ');

  try {
    const res = await fetch(`${API_BASE}/api/v1/studios/${studioId}/leads/sheets-settings`, {
      method: 'GET',
      headers: {
        ...(cookieHeader ? { Cookie: cookieHeader } : {}),
      },
      cache: 'no-store',
    });
    if (!res.ok) {
      return { ok: false, error: `HTTP ${res.status}` };
    }
    const data = await res.json();
    return { ok: true, data };
  } catch (err: any) {
    return { ok: false, error: err.message };
  }
}

export async function saveSheetsSettings(
  studioId: string,
  data: {
    spreadsheetId: string;
    tabName: string;
    active: boolean;
  }
): Promise<UpdateStudioResult> {
  const cookieStore = await cookies();
  const cookieHeader = cookieStore
    .getAll()
    .map((c) => `${c.name}=${c.value}`)
    .join('; ');

  const res = await fetch(`${API_BASE}/api/v1/studios/${studioId}/leads/sheets-settings`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(cookieHeader ? { Cookie: cookieHeader } : {}),
    },
    body: JSON.stringify(data),
    cache: 'no-store',
  });

  type ErrBody = { error?: string; details?: Record<string, string> };
  const body = (await res.json().catch(() => null)) as ErrBody | null;

  if (!res.ok) {
    return {
      ok: false,
      error: body?.error ?? `HTTP ${res.status}`,
      details: body?.details,
    };
  }

  revalidatePath(`/admin/studios/${studioId}/settings`);
  return { ok: true };
}
