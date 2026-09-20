'use server';

// Server Actions for the AI Assistant settings cards (Groq/Gemini/Claude
// provider keys + per-provider model checklist). Same cookie-forwarding
// pattern as the rest of this folder's actions.ts.

import { cookies } from 'next/headers';
import { revalidatePath } from 'next/cache';

const API_BASE = process.env.API_BASE_URL ?? 'http://localhost:8080';

export type AIProvider = 'groq' | 'gemini' | 'claude';

export interface AIModel {
  id?: string;
  modelName: string;
  enabled: boolean;
  isDefault?: boolean;
  lastTestedAt?: string;
  lastTestOk?: boolean;
  lastTestError?: string;
}

export interface AIProviderConfig {
  provider: AIProvider;
  hasApiKey: boolean;
  keySuffix?: string;
  models: AIModel[];
}

async function cookieHeader() {
  const cookieStore = await cookies();
  return cookieStore
    .getAll()
    .map((c) => `${c.name}=${c.value}`)
    .join('; ');
}

export type GetAIModelsResult =
  | { ok: true; providers: AIProviderConfig[] }
  | { ok: false; error: string };

export async function getAIModels(studioId: string): Promise<GetAIModelsResult> {
  try {
    const res = await fetch(`${API_BASE}/api/v1/studios/${studioId}/ai-models`, {
      method: 'GET',
      headers: { Cookie: await cookieHeader() },
      cache: 'no-store',
    });
    const body = await res.json().catch(() => null);
    if (!res.ok) {
      return { ok: false, error: body?.error ?? `HTTP ${res.status}` };
    }
    return { ok: true, providers: body?.providers ?? [] };
  } catch (err: any) {
    return { ok: false, error: err.message };
  }
}

export type AIActionResult =
  | { ok: true }
  | { ok: false; error: string; details?: Record<string, string> };

// testAIProviderKey both verifies a key works (a live call against an
// already-enabled model, or the platform default) and — only after that
// succeeds — saves it if a new one was typed in. Mirrors the "test = add"
// philosophy used for models: a key is never stored unverified.
export async function testAIProviderKey(
  studioId: string,
  provider: AIProvider,
  apiKey: string,
): Promise<AIActionResult> {
  const res = await fetch(`${API_BASE}/api/v1/studios/${studioId}/ai-models/${provider}/test-key`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Cookie: await cookieHeader() },
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

export type AddAIModelResult =
  | { ok: true; model: AIModel }
  | { ok: false; error: string; details?: Record<string, string> };

// addAndTestAIModel both live-tests modelName against the provider (using
// the studio's saved key) and, on success, persists it enabled — the two
// are one action per the AI Assistant page's "add = test" flow. Also used
// to promote a virtual default model into a real row when its checkbox is
// checked for the first time.
export async function addAndTestAIModel(
  studioId: string,
  provider: AIProvider,
  modelName: string,
): Promise<AddAIModelResult> {
  const res = await fetch(`${API_BASE}/api/v1/studios/${studioId}/ai-models/${provider}/models`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Cookie: await cookieHeader() },
    body: JSON.stringify({ modelName }),
    cache: 'no-store',
  });
  const body = await res.json().catch(() => null);
  if (!res.ok) {
    return { ok: false, error: body?.error ?? `HTTP ${res.status}`, details: body?.details };
  }
  revalidatePath(`/admin/studios/${studioId}/settings`);
  return { ok: true, model: { id: body.id, modelName: body.modelName, enabled: body.enabled } };
}

export async function setAIModelEnabled(
  studioId: string,
  provider: AIProvider,
  modelId: string,
  enabled: boolean,
): Promise<AIActionResult> {
  const res = await fetch(`${API_BASE}/api/v1/studios/${studioId}/ai-models/${provider}/models/${modelId}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json', Cookie: await cookieHeader() },
    body: JSON.stringify({ enabled }),
    cache: 'no-store',
  });
  const body = await res.json().catch(() => null);
  if (!res.ok) {
    return { ok: false, error: body?.error ?? `HTTP ${res.status}` };
  }
  revalidatePath(`/admin/studios/${studioId}/settings`);
  return { ok: true };
}

// reorderAIModels sets which model a provider tries first, second, etc. —
// modelIds[0] is the primary, the rest are fallbacks in order.
export async function reorderAIModels(
  studioId: string,
  provider: AIProvider,
  modelIds: string[],
): Promise<AIActionResult> {
  const res = await fetch(`${API_BASE}/api/v1/studios/${studioId}/ai-models/${provider}/order`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json', Cookie: await cookieHeader() },
    body: JSON.stringify({ modelIds }),
    cache: 'no-store',
  });
  const body = await res.json().catch(() => null);
  if (!res.ok) {
    return { ok: false, error: body?.error ?? `HTTP ${res.status}` };
  }
  revalidatePath(`/admin/studios/${studioId}/settings`);
  return { ok: true };
}

export async function deleteAIModel(
  studioId: string,
  provider: AIProvider,
  modelId: string,
): Promise<AIActionResult> {
  const res = await fetch(`${API_BASE}/api/v1/studios/${studioId}/ai-models/${provider}/models/${modelId}`, {
    method: 'DELETE',
    headers: { Cookie: await cookieHeader() },
    cache: 'no-store',
  });
  const body = await res.json().catch(() => null);
  if (!res.ok) {
    return { ok: false, error: body?.error ?? `HTTP ${res.status}` };
  }
  revalidatePath(`/admin/studios/${studioId}/settings`);
  return { ok: true };
}
