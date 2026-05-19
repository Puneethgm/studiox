'use server';

// Server Action for updating a lead. Runs on the Next.js server, forwards
// the auth cookie to the Go API, then `revalidatePath`s every page that
// shows lead status — so a router.back() (or any subsequent nav) gets
// fresh data without needing a manual browser refresh.

import { cookies } from 'next/headers';
import { revalidatePath } from 'next/cache';

const API_BASE = process.env.API_BASE_URL ?? 'http://localhost:8080';

export type UpdateLeadResult =
  | { ok: true }
  | { ok: false; error: string; details?: Record<string, string> };

export async function updateLeadStatus(
  studioId: string,
  leadId: string,
  data: { status: string; notes: string },
): Promise<UpdateLeadResult> {
  const cookieStore = await cookies();
  const cookieHeader = cookieStore
    .getAll()
    .map((c) => `${c.name}=${c.value}`)
    .join('; ');

  const res = await fetch(
    `${API_BASE}/api/v1/studios/${studioId}/leads/${leadId}`,
    {
      method: 'PATCH',
      headers: {
        'Content-Type': 'application/json',
        ...(cookieHeader ? { Cookie: cookieHeader } : {}),
      },
      body: JSON.stringify(data),
      cache: 'no-store',
    },
  );

  type ErrBody = { error?: string; details?: Record<string, string> };
  const body = (await res.json().catch(() => null)) as ErrBody | null;

  if (!res.ok) {
    return {
      ok: false,
      error: body?.error ?? `HTTP ${res.status}`,
      details: body?.details,
    };
  }

  // Invalidate every page that surfaces lead status — these all re-render
  // from fresh data on the next navigation/visit.
  revalidatePath(`/admin/studios/${studioId}`);
  revalidatePath(`/admin/studios/${studioId}/pipeline`);
  revalidatePath(`/admin/studios/${studioId}/leads`);
  revalidatePath(`/admin/studios/${studioId}/leads/${leadId}`);

  return { ok: true };
}
