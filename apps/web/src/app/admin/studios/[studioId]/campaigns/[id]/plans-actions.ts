'use server';

import { cookies } from 'next/headers';
import { revalidatePath } from 'next/cache';

const API_BASE = process.env.API_BASE_URL ?? 'http://localhost:8080';

export type UpdateCampaignPlansResult =
  | { ok: true }
  | { ok: false; error: string; details?: Record<string, string> };

export async function updateCampaignFitnessPlans(data: {
  studioId: string;
  campaignId: string;
  studioSlug: string;
  campaignSlug: string;
  fitnessPlans: string[];
}): Promise<UpdateCampaignPlansResult> {
  const cookieStore = await cookies();
  const cookieHeader = cookieStore
    .getAll()
    .map((c) => `${c.name}=${c.value}`)
    .join('; ');

  const res = await fetch(`${API_BASE}/api/v1/studios/${data.studioId}/campaigns/${data.campaignId}`, {
    method: 'PATCH',
    headers: {
      'Content-Type': 'application/json',
      ...(cookieHeader ? { Cookie: cookieHeader } : {}),
    },
    body: JSON.stringify({ fitnessPlans: data.fitnessPlans }),
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

  revalidatePath(`/admin/studios/${data.studioId}`);
  revalidatePath(`/admin/studios/${data.studioId}/campaigns`);
  revalidatePath(`/admin/studios/${data.studioId}/campaigns/${data.campaignId}`);
  revalidatePath(`/l/${data.studioSlug}/${data.campaignSlug}`);

  return { ok: true };
}