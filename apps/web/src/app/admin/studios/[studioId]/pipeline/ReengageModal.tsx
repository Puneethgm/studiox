'use client';

import { useEffect, useState } from 'react';
import { createPortal } from 'react-dom';
import { Loader2, X, FileText, Send } from 'lucide-react';
import { api, ApiError } from '@/lib/api';
import type { MessageTemplate } from '@/lib/types';

interface ReengagePrefsResp {
  message: string;
  templateId: string | null;
  defaultGreeting: string;
}

// Guarantees the box never opens empty.
const FALLBACK_MESSAGE =
  "Hi {{lead_first_name}}, still thinking about joining {{studio_name}}? We'd love to have you! Reply *1* to book a trial or *2* to become a member.";

export function ReengageModal({
  studioId,
  conversationIds,
  onClose,
  onSent,
}: {
  studioId: string;
  conversationIds: string[];
  onClose: () => void;
  onSent: (sentIds: string[], sentCount: number) => void;
}) {
  const [loading, setLoading] = useState(true);
  const [sending, setSending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [templates, setTemplates] = useState<MessageTemplate[]>([]);
  const [message, setMessage] = useState(FALLBACK_MESSAGE);
  const [templateId, setTemplateId] = useState<string | null>(null);
  const [templateName, setTemplateName] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    Promise.all([
      api<{ templates: MessageTemplate[] }>(`/api/v1/studios/${studioId}/messaging/templates`),
      api<ReengagePrefsResp>(`/api/v1/studios/${studioId}/messaging/leads/cold/reengage-prefs`),
    ])
      .then(([tmplResp, prefs]) => {
        if (cancelled) return;
        const list = tmplResp.templates ?? [];
        setTemplates(list);
        if (prefs.templateId) {
          const tmpl = list.find((t) => t.id === prefs.templateId);
          setTemplateId(prefs.templateId);
          setTemplateName(tmpl?.name ?? null);
          setMessage(prefs.message || tmpl?.body || prefs.defaultGreeting || FALLBACK_MESSAGE);
        } else if (prefs.message) {
          setMessage(prefs.message);
        } else {
          setMessage(prefs.defaultGreeting || FALLBACK_MESSAGE);
        }
      })
      .catch(() => {
        setError("Couldn't load your saved defaults — using a generic starter message instead.");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [studioId]);

  function applyTemplate(id: string) {
    const tmpl = templates.find((t) => t.id === id);
    if (!tmpl) return;
    setTemplateId(tmpl.id);
    setTemplateName(tmpl.name);
    setMessage(tmpl.body);
  }

  function useCustomText() {
    setTemplateId(null);
    setTemplateName(null);
  }

  async function send() {
    if (!message.trim() && !templateId) return;
    setSending(true);
    setError(null);
    try {
      const res = await api<{ sent: number }>(
        `/api/v1/studios/${studioId}/messaging/leads/cold/re-engage`,
        { method: 'POST', json: { conversationIds, message, templateId } },
      );
      onSent(conversationIds, res.sent);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Failed to re-engage');
    } finally {
      setSending(false);
    }
  }

  if (typeof document === 'undefined') return null;

  // Portalled to <body> — the Cold column's backdrop-blur would otherwise
  // trap a `position: fixed` child inside its own bounds.
  return createPortal(
    <div
      className="fixed inset-0 z-[9999] flex items-center justify-center bg-black/40 p-4"
      onClick={onClose}
    >
      <div
        className="w-full max-w-md rounded-2xl bg-white p-5 shadow-xl dark:bg-zinc-900"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="mb-3 flex items-center justify-between">
          <h3 className="text-sm font-bold text-zinc-800 dark:text-zinc-100">
            Re-engage {conversationIds.length} {conversationIds.length === 1 ? 'lead' : 'leads'}
          </h3>
          <button onClick={onClose} className="text-zinc-400 hover:text-zinc-600" title="Close">
            <X className="h-4 w-4" />
          </button>
        </div>

        {loading ? (
          <div className="flex items-center justify-center py-10 text-zinc-400">
            <Loader2 className="h-5 w-5 animate-spin" />
          </div>
        ) : (
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <label className="text-xs font-medium text-gray-500 uppercase tracking-wide">Message</label>
              {templateId && (
                <button
                  onClick={useCustomText}
                  className="text-[11px] font-medium text-violet-600 hover:text-violet-700"
                >
                  Use custom text instead
                </button>
              )}
            </div>

            {templateId ? (
              <div className="space-y-2">
                <div className="flex items-center gap-1.5 rounded border border-violet-200 bg-violet-50 px-2 py-1 text-[11px] font-medium text-violet-700">
                  <FileText className="h-3 w-3 shrink-0" />
                  {templateName || 'Saved template'}
                </div>
                <p className="rounded border border-gray-100 bg-gray-50 px-3 py-2 text-sm text-gray-600 whitespace-pre-wrap">
                  {message}
                </p>
              </div>
            ) : (
              <>
                <textarea
                  value={message}
                  onChange={(e) => setMessage(e.target.value)}
                  rows={5}
                  placeholder="e.g. Hi {{lead_first_name}}, still thinking about joining {{studio_name}}?"
                  className="w-full rounded border border-gray-200 px-3 py-2 text-sm resize-y"
                />
                {templates.length > 0 && (
                  <select
                    value=""
                    onChange={(e) => e.target.value && applyTemplate(e.target.value)}
                    className="w-full rounded border border-gray-200 px-2 py-1.5 text-xs text-gray-600"
                  >
                    <option value="">Or use a saved template (Inbox → Snippets)…</option>
                    {templates.map((t) => (
                      <option key={t.id} value={t.id}>
                        {t.name}
                      </option>
                    ))}
                  </select>
                )}
              </>
            )}

            <p className="text-[11px] text-gray-400">
              Placeholders: <code>{'{{lead_first_name}}'}</code>, <code>{'{{lead_name}}'}</code>,{' '}
              <code>{'{{studio_name}}'}</code>. Whatever you send becomes the default pre-fill next time.
            </p>

            {error && <p className="text-xs font-medium text-red-500">{error}</p>}

            <div className="flex justify-end gap-2 pt-1">
              <button
                onClick={onClose}
                className="rounded-lg px-3 py-1.5 text-xs font-medium text-zinc-500 hover:bg-zinc-100 dark:hover:bg-zinc-800"
              >
                Cancel
              </button>
              <button
                onClick={send}
                disabled={sending || (!message.trim() && !templateId)}
                className="flex items-center gap-1.5 rounded-lg bg-amber-500 px-3 py-1.5 text-xs font-bold text-white disabled:opacity-50"
              >
                {sending && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
                <Send className="h-3.5 w-3.5" />
                Send
              </button>
            </div>
          </div>
        )}
      </div>
    </div>,
    document.body,
  );
}
