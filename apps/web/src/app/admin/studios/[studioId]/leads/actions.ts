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
