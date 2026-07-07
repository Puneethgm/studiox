'use server';

import { cookies } from 'next/headers';
import { revalidatePath } from 'next/cache';

const API_BASE = process.env.API_BASE_URL ?? 'http://localhost:8080';

export interface ImportResult {
  ok: boolean;
  error?: string;
  imported?: number;
  message?: string;
}

export async function importLeadsAction(studioId: string, formData: FormData): Promise<ImportResult> {
  const cookieStore = await cookies();
  const cookieHeader = cookieStore
    .getAll()
    .map((c) => `${c.name}=${c.value}`)
    .join('; ');

  try {
    const res = await fetch(`${API_BASE}/api/v1/studios/${studioId}/leads/import`, {
      method: 'POST',
      headers: {
        ...(cookieHeader ? { Cookie: cookieHeader } : {}),
      },
      body: formData,
      cache: 'no-store',
    });

    const data = await res.json();
    if (!res.ok) {
      return { ok: false, error: data.error || `HTTP ${res.status}` };
    }

    revalidatePath(`/admin/studios/${studioId}/leads`);
    return { ok: true, imported: data.imported, message: data.message };
  } catch (err: any) {
    return { ok: false, error: err.message || 'An unknown error occurred.' };
  }
}

export type UpdatePipelineStatusResult = { ok: true } | { ok: false; error: string };

// Minimal status-only PATCH for the pipeline board's drag-and-drop — every
// field on the backend is optional, so this only ever sends `status`,
// leaving notes/contactMade/etc. untouched.
export async function updatePipelineStatus(
  studioId: string,
  leadId: string,
  status: string,
): Promise<UpdatePipelineStatusResult> {
  const cookieStore = await cookies();
  const cookieHeader = cookieStore
    .getAll()
    .map((c) => `${c.name}=${c.value}`)
    .join('; ');

  try {
    const res = await fetch(`${API_BASE}/api/v1/studios/${studioId}/leads/${leadId}`, {
      method: 'PATCH',
      headers: {
        'Content-Type': 'application/json',
        ...(cookieHeader ? { Cookie: cookieHeader } : {}),
      },
      body: JSON.stringify({ status }),
      cache: 'no-store',
    });

    if (!res.ok) {
      const data = await res.json().catch(() => null);
      return { ok: false, error: data?.error || `HTTP ${res.status}` };
    }

    revalidatePath(`/admin/studios/${studioId}`);
    revalidatePath(`/admin/studios/${studioId}/pipeline`);
    revalidatePath(`/admin/studios/${studioId}/leads`);
    revalidatePath(`/admin/studios/${studioId}/leads/${leadId}`);
    return { ok: true };
  } catch (err: any) {
    return { ok: false, error: err.message || 'An unknown error occurred.' };
  }
}
