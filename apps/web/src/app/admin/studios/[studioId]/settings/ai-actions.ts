'use server';

// Server Actions for the studio's Mistral OCR key card. Groq/Gemini/Claude
// moved to a centralized, superadmin-only page (/admin/settings, see
// admin/settings/ai-provider-actions.ts) — Mistral stays per-studio since
// it's only used for Knowledge Base image OCR, not the chat-reply
// waterfall. Same cookie-forwarding pattern as the rest of this folder's
// actions.ts.

import { cookies } from 'next/headers';
import { revalidatePath } from 'next/cache';
import { csrfHeader } from '@/lib/auth';

const API_BASE = process.env.API_BASE_URL ?? 'http://localhost:8080';

export interface MistralKeyStatus {
  hasApiKey: boolean;
  keySuffix?: string;
}

async function cookieHeader() {
  const cookieStore = await cookies();
  return cookieStore
    .getAll()
    .map((c) => `${c.name}=${c.value}`)
    .join('; ');
}

export type GetMistralKeyResult =
  | { ok: true; status: MistralKeyStatus }
  | { ok: false; error: string };

export async function getMistralKeyStatus(studioId: string): Promise<GetMistralKeyResult> {
  try {
    const res = await fetch(`${API_BASE}/api/v1/me/studios/${studioId}/mistral-key`, {
      method: 'GET',
      headers: { Cookie: await cookieHeader() },
      cache: 'no-store',
    });
    const body = await res.json().catch(() => null);
    if (!res.ok) {
      return { ok: false, error: body?.error ?? `HTTP ${res.status}` };
    }
    return { ok: true, status: { hasApiKey: !!body?.hasApiKey, keySuffix: body?.keySuffix } };
  } catch (err: any) {
    return { ok: false, error: err.message };
  }
}

export type AIActionResult =
  | { ok: true }
  | { ok: false; error: string; details?: Record<string, string> };

// testMistralKey both verifies a key works (a live call) and — only after
// that succeeds — saves it if a new one was typed in. A key is never
// stored unverified.
export async function testMistralKey(studioId: string, apiKey: string): Promise<AIActionResult> {
  const res = await fetch(`${API_BASE}/api/v1/me/studios/${studioId}/mistral-key/test`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Cookie: await cookieHeader(), ...(await csrfHeader()) },
    body: JSON.stringify({ apiKey }),
    cache: 'no-store',
  });
  const body = await res.json().catch(() => null);
  if (!res.ok) {
    return { ok: false, error: body?.error ?? `HTTP ${res.status}`, details: body?.details };
  }
  revalidatePath(`/admin/studios/${studioId}/settings`);
  return { ok: true };
}
