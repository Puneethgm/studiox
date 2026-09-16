'use server';

// Server Actions for the CRM Integrations admin page — same pattern as
// apps/web/src/app/admin/studios/[studioId]/settings/actions.ts: runs on the
// server, forwards the auth cookie to the Go API, revalidates this page
// after a mutation, and returns a discriminated {ok:true|false} result
// instead of throwing.

import { cookies } from 'next/headers';
import { revalidatePath } from 'next/cache';
import type { AuthFieldDef, CrmOperation, CrmProvider, LlmProviderName, OperationKey } from './types';

const API_BASE = process.env.API_BASE_URL ?? 'http://localhost:8080';

async function cookieHeader(): Promise<string> {
  const store = await cookies();
  return store.getAll().map((c) => `${c.name}=${c.value}`).join('; ');
}

async function apiFetch<T>(path: string, init: RequestInit = {}): Promise<{ ok: true; data: T } | { ok: false; error: string; details?: Record<string, string> }> {
  const ck = await cookieHeader();
  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...(ck ? { Cookie: ck } : {}),
      ...(init.headers ?? {}),
    },
    cache: 'no-store',
  });
  type ErrBody = { error?: string; details?: Record<string, string> };
  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as ErrBody | null;
    return { ok: false, error: body?.error ?? `HTTP ${res.status}`, details: body?.details };
  }
  return { ok: true, data: (await res.json()) as T };
}

export type ActionResult<T = undefined> =
  | { ok: true; data: T }
  | { ok: false; error: string; details?: Record<string, string> };

// ── CRM providers ──────────────────────────────────────────────

export async function parseProviderDoc(name: string, docText: string): Promise<ActionResult<{ provider: CrmProvider; operations: CrmOperation[] }>> {
  const result = await apiFetch<{ provider: CrmProvider; operations: CrmOperation[] }>('/api/v1/admin/crm-providers/parse', {
    method: 'POST',
    body: JSON.stringify({ name, docText }),
  });
  if (result.ok) revalidatePath('/admin/crm-integrations');
  return result;
}

export async function getProviderOperations(providerId: string): Promise<ActionResult<CrmOperation[]>> {
  const result = await apiFetch<{ operations: CrmOperation[] | null }>(`/api/v1/admin/crm-providers/${providerId}/operations`);
  if (!result.ok) return result;
  return { ok: true, data: result.data.operations ?? [] };
}

export async function updateOperation(
  providerId: string,
  op: {
    operationKey: OperationKey;
    httpMethod: string;
    pathTemplate: string;
    requestMapping: Record<string, unknown>;
    responseMapping: Record<string, unknown>;
    reviewed: boolean;
  },
): Promise<ActionResult<CrmOperation>> {
  const result = await apiFetch<CrmOperation>(`/api/v1/admin/crm-providers/${providerId}/operations/${op.operationKey}`, {
    method: 'PUT',
    body: JSON.stringify(op),
  });
  if (result.ok) revalidatePath('/admin/crm-integrations');
  return result;
}

export async function updateProviderDetails(
  providerId: string,
  input: {
    description: string;
    baseUrl: string;
    authType: 'bearer' | 'api_key' | 'basic' | 'token_exchange';
    authFieldDefs: AuthFieldDef[];
    tokenLoginPath?: string;
    tokenLoginMethod?: string;
    tokenLoginBodyMapping?: Record<string, string>;
    tokenResponsePath?: string;
    tokenExpiryPath?: string;
    tokenExpirySeconds?: number;
  },
): Promise<ActionResult<CrmProvider>> {
  const result = await apiFetch<CrmProvider>(`/api/v1/admin/crm-providers/${providerId}`, {
    method: 'PUT',
    body: JSON.stringify(input),
  });
  if (result.ok) revalidatePath('/admin/crm-integrations');
  return result;
}

export async function activateProvider(providerId: string): Promise<ActionResult<undefined>> {
  const result = await apiFetch<{ ok: boolean }>(`/api/v1/admin/crm-providers/${providerId}/activate`, { method: 'POST' });
  if (result.ok) revalidatePath('/admin/crm-integrations');
  return result.ok ? { ok: true, data: undefined } : result;
}

export async function createProviderManually(input: {
  name: string;
  description: string;
  baseUrl: string;
  authType: 'bearer' | 'api_key' | 'basic';
  authFieldDefs: AuthFieldDef[];
}): Promise<ActionResult<CrmProvider>> {
  const result = await apiFetch<CrmProvider>('/api/v1/admin/crm-providers', {
    method: 'POST',
    body: JSON.stringify(input),
  });
  if (result.ok) revalidatePath('/admin/crm-integrations');
  return result;
}

// ── Studio connections ─────────────────────────────────────────

export async function connectStudioToProvider(
  studioId: string,
  crmProviderId: string,
  credentials: Record<string, string>,
): Promise<ActionResult<undefined>> {
  const result = await apiFetch(`/api/v1/admin/studios/${studioId}/crm-connections`, {
    method: 'POST',
    body: JSON.stringify({ crmProviderId, credentials }),
  });
  if (result.ok) revalidatePath('/admin/crm-integrations');
  return result.ok ? { ok: true, data: undefined } : result;
}

export async function disconnectStudio(studioId: string, providerId: string): Promise<ActionResult<undefined>> {
  const result = await apiFetch(`/api/v1/admin/studios/${studioId}/crm-connections/${providerId}`, { method: 'DELETE' });
  if (result.ok) revalidatePath('/admin/crm-integrations');
  return result.ok ? { ok: true, data: undefined } : result;
}

export async function getStudioConnection(studioId: string): Promise<ActionResult<{ connected: boolean; connection?: { crmProviderId: string } }>> {
  return apiFetch(`/api/v1/admin/studios/${studioId}/crm-connections`);
}

// ── AI task config (which LLM parses CRM docs) ─────────────────

export async function updateAiTaskConfig(
  purpose: string,
  provider: LlmProviderName,
  model: string,
  apiKey: string,
): Promise<ActionResult<undefined>> {
  const result = await apiFetch(`/api/v1/admin/ai-task-configs/${purpose}`, {
    method: 'PUT',
    body: JSON.stringify({ provider, model, apiKey }),
  });
  if (result.ok) revalidatePath('/admin/crm-integrations');
  return result.ok ? { ok: true, data: undefined } : result;
}
