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

export async function updateKnowledgeBase(
  studioId: string,
  studioSlug: string,
  knowledgeBase: string,
  knowledgeBaseFiles: { name: string; url: string; text: string }[]
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
