'use server';

// Server Actions for the centralized AI Provider Settings page (superadmin
// only, /admin/settings) — Groq/Gemini/Claude keys and the shared model
// list every studio's chat-reply waterfall uses. See
// BIFROST_MIGRATION_PLAN.md. Mistral (Knowledge Base OCR) stays per-studio
// — see admin/studios/[studioId]/settings/ai-actions.ts.

import { cookies } from 'next/headers';
import { revalidatePath } from 'next/cache';
import { csrfHeader } from '@/lib/auth';

const API_BASE = process.env.API_BASE_URL ?? 'http://localhost:8080';

export type AIProvider = 'groq' | 'gemini' | 'claude' | 'openai';

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
  keyStatus?: 'active' | 'disabled' | 'failing' | '';
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

export async function getPlatformAIModels(): Promise<GetAIModelsResult> {
  try {
    const res = await fetch(`${API_BASE}/api/v1/admin/ai-providers`, {
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

// testPlatformAIProviderKey both verifies a key works (a live call against
// an already-enabled model, or the platform default) and — only after that
// succeeds — saves it if a new one was typed in. A key is never stored
// unverified.
export async function testPlatformAIProviderKey(
  provider: AIProvider,
  apiKey: string,
): Promise<AIActionResult> {
  const res = await fetch(`${API_BASE}/api/v1/admin/ai-providers/${provider}/test-key`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Cookie: await cookieHeader(), ...(await csrfHeader()) },
    body: JSON.stringify({ apiKey }),
    cache: 'no-store',
  });
  const body = await res.json().catch(() => null);
  if (!res.ok) {
    return { ok: false, error: body?.error ?? `HTTP ${res.status}`, details: body?.details };
  }
  revalidatePath('/admin/settings');
  return { ok: true };
}

export type AddAIModelResult =
  | { ok: true; model: AIModel }
  | { ok: false; error: string; details?: Record<string, string> };

// addAndTestPlatformAIModel both live-tests modelName against the provider
// (using the centrally-configured key) and, on success, persists it
// enabled for every studio — the two are one action per this page's
// "add = test" flow. Also used to promote a virtual default model into a
// real row when its checkbox is checked for the first time.
export async function addAndTestPlatformAIModel(
  provider: AIProvider,
  modelName: string,
): Promise<AddAIModelResult> {
  const res = await fetch(`${API_BASE}/api/v1/admin/ai-providers/${provider}/models`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Cookie: await cookieHeader(), ...(await csrfHeader()) },
    body: JSON.stringify({ modelName }),
    cache: 'no-store',
  });
  const body = await res.json().catch(() => null);
  if (!res.ok) {
    return { ok: false, error: body?.error ?? `HTTP ${res.status}`, details: body?.details };
  }
  revalidatePath('/admin/settings');
  return { ok: true, model: { id: body.id, modelName: body.modelName, enabled: body.enabled } };
}

export async function setPlatformAIModelEnabled(modelId: string, enabled: boolean): Promise<AIActionResult> {
  const res = await fetch(`${API_BASE}/api/v1/admin/ai-providers/models/${modelId}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json', Cookie: await cookieHeader(), ...(await csrfHeader()) },
    body: JSON.stringify({ enabled }),
    cache: 'no-store',
  });
  const body = await res.json().catch(() => null);
  if (!res.ok) {
    return { ok: false, error: body?.error ?? `HTTP ${res.status}` };
  }
  revalidatePath('/admin/settings');
  return { ok: true };
}

// reorderPlatformAIModels sets which model a provider tries first, second,
// etc. — modelIds[0] is the primary, the rest are fallbacks in order.
export async function reorderPlatformAIModels(provider: AIProvider, modelIds: string[]): Promise<AIActionResult> {
  const res = await fetch(`${API_BASE}/api/v1/admin/ai-providers/${provider}/order`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json', Cookie: await cookieHeader(), ...(await csrfHeader()) },
    body: JSON.stringify({ modelIds }),
    cache: 'no-store',
  });
  const body = await res.json().catch(() => null);
  if (!res.ok) {
    return { ok: false, error: body?.error ?? `HTTP ${res.status}` };
  }
  revalidatePath('/admin/settings');
  return { ok: true };
}

// rotatePlatformAIProviderKey replaces the stored key with a freshly
// re-encrypted envelope and records the change as a "rotated" audit
// action — used when a key is already saved and the superadmin wants to
// swap it for a new one (not the first-time save flow, which goes through
// testPlatformAIProviderKey).
export async function rotatePlatformAIProviderKey(provider: AIProvider, apiKey: string): Promise<AIActionResult> {
  const res = await fetch(`${API_BASE}/api/v1/admin/ai-providers/${provider}/key/rotate`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Cookie: await cookieHeader(), ...(await csrfHeader()) },
    body: JSON.stringify({ apiKey }),
    cache: 'no-store',
  });
  const body = await res.json().catch(() => null);
  if (!res.ok) {
    return { ok: false, error: body?.error ?? `HTTP ${res.status}`, details: body?.details };
  }
  revalidatePath('/admin/settings');
  return { ok: true };
}

// setPlatformAIProviderKeyEnabled disables (or re-enables) a provider's
// saved key without deleting it — a disabled key is treated as "not
// configured" by every AI feature until re-enabled.
export async function setPlatformAIProviderKeyEnabled(provider: AIProvider, enabled: boolean): Promise<AIActionResult> {
  const res = await fetch(`${API_BASE}/api/v1/admin/ai-providers/${provider}/key/status`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json', Cookie: await cookieHeader(), ...(await csrfHeader()) },
    body: JSON.stringify({ status: enabled ? 'active' : 'disabled' }),
    cache: 'no-store',
  });
  const body = await res.json().catch(() => null);
  if (!res.ok) {
    return { ok: false, error: body?.error ?? `HTTP ${res.status}` };
  }
  revalidatePath('/admin/settings');
  return { ok: true };
}

export interface AIAlertThresholds {
  failureRatePct: number;
  failureRateWindowMinutes: number;
  dailySpendBudgetUsd: number;
}

export type GetAIAlertThresholdsResult =
  | { ok: true; thresholds: AIAlertThresholds }
  | { ok: false; error: string };

export async function getAIAlertThresholds(): Promise<GetAIAlertThresholdsResult> {
  try {
    const res = await fetch(`${API_BASE}/api/v1/admin/ai-providers/alert-thresholds`, {
      method: 'GET',
      headers: { Cookie: await cookieHeader() },
      cache: 'no-store',
    });
    const body = await res.json().catch(() => null);
    if (!res.ok) {
      return { ok: false, error: body?.error ?? `HTTP ${res.status}` };
    }
    return { ok: true, thresholds: body };
  } catch (err: any) {
    return { ok: false, error: err.message };
  }
}

export async function setAIAlertThresholds(thresholds: AIAlertThresholds): Promise<AIActionResult> {
  const res = await fetch(`${API_BASE}/api/v1/admin/ai-providers/alert-thresholds`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', Cookie: await cookieHeader(), ...(await csrfHeader()) },
    body: JSON.stringify(thresholds),
    cache: 'no-store',
  });
  const body = await res.json().catch(() => null);
  if (!res.ok) {
    return { ok: false, error: body?.details?.thresholds ?? body?.error ?? `HTTP ${res.status}` };
  }
  revalidatePath('/admin/settings');
  return { ok: true };
}

export interface PlatformAlert {
  id: string;
  kind: string;
  detail: Record<string, unknown>;
  studioId?: string;
  createdAt: string;
  resolvedAt?: string;
}

export type GetPlatformAlertsResult =
  | { ok: true; alerts: PlatformAlert[] }
  | { ok: false; error: string };

export async function getPlatformAlerts(): Promise<GetPlatformAlertsResult> {
  try {
    const res = await fetch(`${API_BASE}/api/v1/admin/ai-providers/alerts`, {
      method: 'GET',
      headers: { Cookie: await cookieHeader() },
      cache: 'no-store',
    });
    const body = await res.json().catch(() => null);
    if (!res.ok) {
      return { ok: false, error: body?.error ?? `HTTP ${res.status}` };
    }
    return { ok: true, alerts: body?.alerts ?? [] };
  } catch (err: any) {
    return { ok: false, error: err.message };
  }
}

export async function deletePlatformAIModel(modelId: string): Promise<AIActionResult> {
  const res = await fetch(`${API_BASE}/api/v1/admin/ai-providers/models/${modelId}`, {
    method: 'DELETE',
    headers: { Cookie: await cookieHeader(), ...(await csrfHeader()) },
    cache: 'no-store',
  });
  const body = await res.json().catch(() => null);
  if (!res.ok) {
    return { ok: false, error: body?.error ?? `HTTP ${res.status}` };
  }
  revalidatePath('/admin/settings');
  return { ok: true };
}
