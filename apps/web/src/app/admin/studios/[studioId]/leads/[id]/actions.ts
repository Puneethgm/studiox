'use server';

// Server Action for updating a lead. Runs on the Next.js server, forwards
// the auth cookie to the Go API, then `revalidatePath`s every page that
// shows lead status — so a router.back() (or any subsequent nav) gets
// fresh data without needing a manual browser refresh.

import { cookies } from 'next/headers';
import { revalidatePath } from 'next/cache';
import { csrfHeader } from '@/lib/auth';

const API_BASE = process.env.API_BASE_URL ?? 'http://localhost:8080';

export type UpdateLeadResult =
  | { ok: true }
  | { ok: false; error: string; details?: Record<string, string> };

export async function updateLeadStatus(
  studioId: string,
  leadId: string,
  data: {
    status: string;
    notes: string;
    firstName?: string;
    lastName?: string;
    fitnessPlan?: string;
    contactMade?: boolean;
    hotLead?: boolean;
    trialPurchased?: boolean;
    assignedTo?: string;
    trialAttended?: boolean;
    memberSold?: boolean;
    monthlyFee?: number;
    currency?: string;
    offer?: string;
    furtherNotes?: string;
    tags?: string[];
    // "" clears the reminder — see patchLeadReq's doc comment in
    // apps/api/internal/leads/http.go for why this has to be a string,
    // not an optional ISO date, to actually be able to clear it.
    reminderAt?: string;
    reminderNote?: string;
  },
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
        ...(await csrfHeader()),
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

// Minimal partial update for the Contact details card's click-to-edit
// Email/Phone fields — same true-partial-update endpoint as updateLeadTags,
// so sending just one of these never touches the other unsaved form state.
export async function updateLeadContact(
  studioId: string,
  leadId: string,
  fields: { email?: string; phone?: string },
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
        ...(await csrfHeader()),
      },
      body: JSON.stringify(fields),
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

  revalidatePath(`/admin/studios/${studioId}`);
  revalidatePath(`/admin/studios/${studioId}/pipeline`);
  revalidatePath(`/admin/studios/${studioId}/leads`);
  revalidatePath(`/admin/studios/${studioId}/leads/${leadId}`);

  return { ok: true };
}

// Tags auto-save on every add/remove (no "Save changes" click needed) — a
// separate, minimal PATCH so adding a tag can't accidentally carry along
// whatever's currently typed into the rest of the (unsaved) form. The API
// is a true partial update (see patchLeadReq in apps/api/internal/leads/
// http.go), so sending only `tags` leaves every other field untouched.
export async function updateLeadTags(
  studioId: string,
  leadId: string,
  tags: string[],
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
        ...(await csrfHeader()),
      },
      body: JSON.stringify({ tags }),
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

  revalidatePath(`/admin/studios/${studioId}`);
  revalidatePath(`/admin/studios/${studioId}/pipeline`);
  revalidatePath(`/admin/studios/${studioId}/leads`);
  revalidatePath(`/admin/studios/${studioId}/leads/${leadId}`);

  return { ok: true };
}
