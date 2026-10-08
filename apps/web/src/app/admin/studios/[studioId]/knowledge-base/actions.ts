'use server';

import { cookies } from 'next/headers';
import { revalidatePath } from 'next/cache';
import { parseOffice } from 'officeparser';

const API_BASE = process.env.API_BASE_URL ?? 'http://localhost:8080';

export interface ParseResult {
  ok: boolean;
  error?: string;
  data?: {
    name: string;
    text: string;
  };
}

export async function parseDocument(formData: FormData): Promise<ParseResult> {
  try {
    const file = formData.get('file') as File | null;
    if (!file) {
      return { ok: false, error: 'No file uploaded' };
    }

    const buffer = Buffer.from(await file.arrayBuffer());
    let text = '';

    const name = file.name.toLowerCase();
    if (name.endsWith('.txt') || name.endsWith('.csv') || name.endsWith('.json') || name.endsWith('.md')) {
      text = buffer.toString('utf-8');
    } else {
      const ext = name.split('.').pop();
      // officeparser parses docx, pptx, xlsx, pdf
      const ast = await parseOffice(buffer, { fileType: ext as any });
      text = ast.toText();
    }

    return {
      ok: true,
      data: {
        name: file.name,
        text: text,
      },
    };
  } catch (err: any) {
    console.error('Error parsing document:', err);
    return { ok: false, error: err.message || 'Failed to parse document' };
  }
}

export interface OcrResult {
  ok: boolean;
  error?: string;
  data?: { text: string };
}

// ocrImage forwards an image to the backend's Mistral OCR endpoint — unlike
// parseDocument (parsed entirely here, client-independent), this needs the
// studio's Mistral API key server-side, so it's a thin proxy to the API
// rather than doing the work in this action itself.
export async function ocrImage(studioId: string, formData: FormData): Promise<OcrResult> {
  const cookieStore = await cookies();
  const cookieHeader = cookieStore
    .getAll()
    .map((c) => `${c.name}=${c.value}`)
    .join('; ');

  try {
    const res = await fetch(`${API_BASE}/api/v1/studios/${studioId}/knowledge-base/ocr`, {
      method: 'POST',
      headers: {
        ...(cookieHeader ? { Cookie: cookieHeader } : {}),
      },
      body: formData,
      cache: 'no-store',
    });

    if (!res.ok) {
      const body = await res.json().catch(() => null);
      return { ok: false, error: body?.error ?? `HTTP ${res.status}` };
    }

    const data = await res.json() as { text: string };
    return { ok: true, data };
  } catch (err: any) {
    return { ok: false, error: err.message || 'Failed to extract text from image' };
  }
}

export async function updateKnowledgeBase(
  studioId: string,
  studioSlug: string,
  knowledgeBase: string,
  _textPlatform: string,
  knowledgeBaseFiles: { name: string; url: string; text: string; platform?: string }[],
  greetingMessage: string
) {
  const cookieStore = await cookies();
  const cookieHeader = cookieStore
    .getAll()
    .map((c) => `${c.name}=${c.value}`)
    .join('; ');

  // Fetch existing studio first so we preserve other fields
  const getRes = await fetch(`${API_BASE}/api/v1/me/studios/${studioId}`, {
    method: 'GET',
    headers: {
      ...(cookieHeader ? { Cookie: cookieHeader } : {}),
    },
    cache: 'no-store',
  });
  if (!getRes.ok) {
    return { ok: false, error: `Failed to fetch studio details: HTTP ${getRes.status}` };
  }

  const res = await fetch(`${API_BASE}/api/v1/me/studios/${studioId}`, {
    method: 'PATCH',
    headers: {
      'Content-Type': 'application/json',
      ...(cookieHeader ? { Cookie: cookieHeader } : {}),
    },
    body: JSON.stringify({
      knowledgeBase,
      knowledgeBaseFiles,
      greetingMessage,
    }),
    cache: 'no-store',
  });

  if (!res.ok) {
    const body = await res.json().catch(() => null);
    return {
      ok: false,
      error: body?.error ?? `HTTP ${res.status}`,
    };
  }

  revalidatePath(`/admin/studios/${studioId}/knowledge-base`);
  revalidatePath(`/admin/studios/${studioId}`);
  revalidatePath(`/l/${studioSlug}`, 'layout');

  return { ok: true };
}

export async function updateCommunicationStyle(studioId: string, communicationStyleProfile: string) {
  const cookieStore = await cookies();
  const cookieHeader = cookieStore
    .getAll()
    .map((c) => `${c.name}=${c.value}`)
    .join('; ');

  const res = await fetch(`${API_BASE}/api/v1/studios/${studioId}/communication-style`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      ...(cookieHeader ? { Cookie: cookieHeader } : {}),
    },
    body: JSON.stringify({ communicationStyleProfile }),
    cache: 'no-store',
  });

  if (!res.ok) {
    const body = await res.json().catch(() => null);
    return {
      ok: false,
      error: body?.error ?? `HTTP ${res.status}`,
    };
  }

  revalidatePath(`/admin/studios/${studioId}/knowledge-base`);
  return { ok: true };
}

export async function updateStyleRefreshInterval(studioId: string, styleRefreshIntervalMinutes: number) {
  const cookieStore = await cookies();
  const cookieHeader = cookieStore
    .getAll()
    .map((c) => `${c.name}=${c.value}`)
    .join('; ');

  const res = await fetch(`${API_BASE}/api/v1/studios/${studioId}/style-refresh-interval`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      ...(cookieHeader ? { Cookie: cookieHeader } : {}),
    },
    body: JSON.stringify({ styleRefreshIntervalMinutes }),
    cache: 'no-store',
  });

  if (!res.ok) {
    const body = await res.json().catch(() => null);
    return {
      ok: false,
      error: body?.error ?? `HTTP ${res.status}`,
    };
  }

  revalidatePath(`/admin/studios/${studioId}/knowledge-base`);
  return { ok: true };
}

// programStartDate is "YYYY-MM-DD" (Week 1's first day of a parsed
// week-by-week program document), or "" to clear it — see
// apps/api/internal/studios/program_schedule.go for how it's used.
export async function updateProgramStartDate(studioId: string, programStartDate: string) {
  const cookieStore = await cookies();
  const cookieHeader = cookieStore
    .getAll()
    .map((c) => `${c.name}=${c.value}`)
    .join('; ');

  const res = await fetch(`${API_BASE}/api/v1/studios/${studioId}/program-start-date`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      ...(cookieHeader ? { Cookie: cookieHeader } : {}),
    },
    body: JSON.stringify({ programStartDate }),
    cache: 'no-store',
  });

  if (!res.ok) {
    const body = await res.json().catch(() => null);
    return {
      ok: false,
      error: body?.error ?? `HTTP ${res.status}`,
    };
  }

  revalidatePath(`/admin/studios/${studioId}/knowledge-base`);
  return { ok: true };
}

// ----- knowledge gaps ("Needs Answers" tab) -----

export interface KnowledgeGap {
  id: string;
  studioId: string;
  conversationId?: string;
  leadId?: string;
  leadName?: string;
  question: string;
  status: 'open' | 'resolved' | 'dismissed';
  timesAsked: number;
  answer?: string;
  createdAt: string;
  updatedAt: string;
  resolvedAt?: string;
}

async function kbCookieHeader(): Promise<string> {
  const cookieStore = await cookies();
  return cookieStore
    .getAll()
    .map((c) => `${c.name}=${c.value}`)
    .join('; ');
}

export async function listKnowledgeGaps(
  studioId: string,
  status: 'open' | 'resolved' | 'dismissed' | '' = 'open'
): Promise<{ ok: boolean; error?: string; gaps?: KnowledgeGap[] }> {
  const ck = await kbCookieHeader();
  try {
    const qs = status ? `?status=${status}` : '';
    const res = await fetch(`${API_BASE}/api/v1/studios/${studioId}/knowledge-base/gaps${qs}`, {
      headers: { ...(ck ? { Cookie: ck } : {}) },
      cache: 'no-store',
    });
    if (!res.ok) return { ok: false, error: `HTTP ${res.status}` };
    const data = (await res.json()) as { gaps?: KnowledgeGap[] };
    return { ok: true, gaps: data.gaps ?? [] };
  } catch (err: any) {
    return { ok: false, error: err.message };
  }
}

export async function resolveKnowledgeGap(
  studioId: string,
  gapId: string,
  answer: string
): Promise<{ ok: boolean; error?: string; gap?: KnowledgeGap }> {
  const ck = await kbCookieHeader();
  const res = await fetch(`${API_BASE}/api/v1/studios/${studioId}/knowledge-base/gaps/${gapId}/resolve`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...(ck ? { Cookie: ck } : {}) },
    body: JSON.stringify({ answer }),
    cache: 'no-store',
  });
  type ErrBody = { error?: string; details?: Record<string, string> };
  const body = (await res.json().catch(() => null)) as (ErrBody & Partial<KnowledgeGap>) | null;
  if (!res.ok) {
    const detail = body?.details ? Object.values(body.details)[0] : undefined;
    return { ok: false, error: detail ?? body?.error ?? `HTTP ${res.status}` };
  }
  revalidatePath(`/admin/studios/${studioId}/knowledge-base`);
  return { ok: true, gap: body as KnowledgeGap };
}

export async function dismissKnowledgeGap(studioId: string, gapId: string): Promise<{ ok: boolean; error?: string }> {
  const ck = await kbCookieHeader();
  const res = await fetch(`${API_BASE}/api/v1/studios/${studioId}/knowledge-base/gaps/${gapId}/dismiss`, {
    method: 'POST',
    headers: { ...(ck ? { Cookie: ck } : {}) },
    cache: 'no-store',
  });
  if (!res.ok && res.status !== 204) {
    const body = (await res.json().catch(() => null)) as { error?: string } | null;
    return { ok: false, error: body?.error ?? `HTTP ${res.status}` };
  }
  revalidatePath(`/admin/studios/${studioId}/knowledge-base`);
  return { ok: true };
}
