'use client';

import { useEffect, useRef, useState } from 'react';
import { FileText, MessageCircle, Send, X } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { api, ApiError } from '@/lib/api';
import { cn } from '@/lib/cn';

interface TestChatTurn {
  role: 'user' | 'assistant';
  text: string;
  // Uploaded knowledge-base document name(s) that backed this answer —
  // undefined/empty when no KB chunk was used (e.g. a greeting).
  sources?: string[];
}

interface TestChatResponse {
  reply: string;
  sources?: string[];
}

// The AI's greeting ("Good morning/afternoon/evening") is normally timed to
// the recipient's phone-number country, but there's no real recipient in
// Test Chat — so instead it uses whoever's actually testing it: this admin's
// own browser/system clock (apps/api's buildPrompt via TestChatRequest.timezone).
// Sent automatically with every message, no admin input needed.
function clientTimezone(): string | undefined {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone;
  } catch {
    return undefined;
  }
}

// A right-side slide-in "assistant" drawer for the Knowledge Base admin
// page — lets an admin type test questions and see how the AI would
// actually answer (real KB retrieval + LLM pipeline, apps/api's
// AIWorker.TestChat), without creating any real conversation/message/lead.
// Multi-turn: the running transcript is kept client-side only and resent
// as `history` on each new message — nothing is persisted server-side.
export function TestChatDrawer({
  studioId,
  openSignal,
}: {
  studioId: string;
  // Bump this number from a parent (e.g. the "Test it now" button on the
  // embedding-complete popup) to force the drawer open — an uncontrolled
  // component otherwise, so this only opens it, it never closes it.
  openSignal?: number;
}) {
  const [open, setOpen] = useState(false);
  const [turns, setTurns] = useState<TestChatTurn[]>([]);
  const [input, setInput] = useState('');
  const [sending, setSending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const endRef = useRef<HTMLLIElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    if (openSignal) setOpen(true);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [openSignal]);

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [turns, sending]);

  // Auto-grow the input as the admin types a longer test question, instead
  // of the text overflowing a fixed-height single-line box. Caps out and
  // scrolls internally past ~6 lines so the drawer layout doesn't blow up.
  useEffect(() => {
    const el = textareaRef.current;
    if (!el) return;
    el.style.height = 'auto';
    el.style.height = `${Math.min(el.scrollHeight, 144)}px`;
  }, [input]);

  async function handleSend() {
    const message = input.trim();
    if (!message || sending) return;

    setError(null);
    setInput('');
    const history = turns;
    setTurns((prev) => [...prev, { role: 'user', text: message }]);
    setSending(true);

    try {
      const res = await api<TestChatResponse>(
        `/api/v1/studios/${studioId}/knowledge-base/test-chat`,
        { method: 'POST', json: { message, history, timezone: clientTimezone() } },
      );
      setTurns((prev) => [...prev, { role: 'assistant', text: res.reply, sources: res.sources }]);
    } catch (err) {
      const msg = err instanceof ApiError ? err.message : 'Something went wrong — try again.';
      setError(msg);
    } finally {
      setSending(false);
    }
  }

  return (
    <>
      {/* Launcher — fixed bottom-right, "like an assistant" widget */}
      <button
        type="button"
        onClick={() => setOpen(true)}
        className={cn(
          'fixed bottom-6 right-6 z-40 flex items-center gap-2 rounded-full px-4 py-3 text-sm font-bold text-white shadow-lg shadow-violet-500/25',
          'bg-[var(--brand,#7c3aed)] hover:brightness-110',
          'active:scale-[0.97] hover:scale-[1.03] transition-all',
          open && 'pointer-events-none opacity-0',
        )}
      >
        <MessageCircle className="h-4 w-4" />
        Test Chat
      </button>

      {/* Backdrop (mobile/narrow screens) */}
      <div
        className={cn(
          'fixed inset-0 z-40 bg-black/30 transition-opacity sm:hidden',
          open ? 'opacity-100' : 'pointer-events-none opacity-0',
        )}
        onClick={() => setOpen(false)}
      />

      {/* Drawer */}
      <div
        className={cn(
          'fixed inset-y-0 right-0 z-50 flex w-full flex-col border-l border-zinc-200 bg-white shadow-2xl transition-transform duration-300 ease-out sm:w-[28rem] dark:border-zinc-800 dark:bg-zinc-950',
          open ? 'translate-x-0' : 'translate-x-full',
        )}
      >
        <div className="flex items-center justify-between border-b border-zinc-200 px-5 py-4 dark:border-zinc-800">
          <div>
            <h3 className="text-sm font-black uppercase tracking-wide text-zinc-800 dark:text-zinc-100">Test Chat</h3>
            <p className="text-xs text-zinc-400 dark:text-zinc-500">Ask questions the way a customer would</p>
          </div>
          <button
            type="button"
            onClick={() => setOpen(false)}
            className="rounded-lg p-1.5 text-zinc-400 hover:bg-zinc-100 hover:text-zinc-700 dark:hover:bg-zinc-800"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto px-4 py-4">
          {turns.length === 0 ? (
            <p className="px-2 py-8 text-center text-xs text-zinc-400 dark:text-zinc-500">
              Ask a question to test how the AI would answer using this studio&apos;s knowledge base — nothing here creates a real conversation.
            </p>
          ) : (
            <ul className="space-y-3">
              {turns.map((t, i) => (
                <TestChatBubble key={i} turn={t} />
              ))}
              {sending && (
                <li className="flex items-start">
                  <div className="rounded-lg border border-zinc-200 bg-white px-3 py-2 text-xs text-zinc-400 dark:border-zinc-700 dark:bg-zinc-900">
                    <span className="animate-pulse">Thinking…</span>
                  </div>
                </li>
              )}
              <li ref={endRef} className="h-1" />
            </ul>
          )}
          {error && (
            <p className="mt-3 rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-xs font-semibold text-rose-600 dark:border-rose-900/40 dark:bg-rose-900/10 dark:text-rose-400">
              {error}
            </p>
          )}
        </div>

        <div className="flex items-end gap-2 border-t border-zinc-200 p-3 dark:border-zinc-800">
          <textarea
            ref={textareaRef}
            rows={1}
            className="max-h-36 flex-1 resize-none overflow-y-auto rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm leading-normal placeholder:text-slate-400 focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500 dark:border-slate-700 dark:bg-slate-900 dark:placeholder:text-slate-500"
            placeholder="Ask a test question… (Shift+Enter for a new line)"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                handleSend();
              }
            }}
            disabled={sending}
          />
          <Button size="sm" onClick={handleSend} disabled={sending || !input.trim()} leftIcon={<Send className="h-3.5 w-3.5" />}>
            Send
          </Button>
        </div>
      </div>
    </>
  );
}

// A text/table segment parsed out of an AI reply — replies are asked to use
// markdown pipe-tables for schedules (see buildPrompt in ai_worker.go), but
// on real channels (WhatsApp/SMS) that just shows as plain pipe characters.
// This drawer is the one surface that can actually render it as a table.
type Segment =
  | { kind: 'text'; content: string }
  | { kind: 'table'; headers: string[]; rows: string[][] };

function parseMarkdownish(text: string): Segment[] {
  const lines = text.split('\n');
  const segments: Segment[] = [];
  let buffer: string[] = [];

  const flushText = () => {
    const content = buffer.join('\n').trim();
    if (content) segments.push({ kind: 'text', content });
    buffer = [];
  };

  const isTableRow = (line: string) => /^\s*\|.*\|\s*$/.test(line);
  const isSeparatorRow = (line: string) => /^\s*\|?[\s:|-]+\|?\s*$/.test(line) && line.includes('-');
  const splitRow = (line: string) =>
    line.trim().replace(/^\|/, '').replace(/\|$/, '').split('|').map((c) => c.trim());

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i] ?? '';
    const nextLine = lines[i + 1];
    if (isTableRow(line) && nextLine !== undefined && isSeparatorRow(nextLine)) {
      flushText();
      const headers = splitRow(line);
      let j = i + 2;
      const rows: string[][] = [];
      let rowLine = lines[j];
      while (rowLine !== undefined && isTableRow(rowLine)) {
        rows.push(splitRow(rowLine));
        j++;
        rowLine = lines[j];
      }
      segments.push({ kind: 'table', headers, rows });
      i = j - 1;
    } else {
      buffer.push(line);
    }
  }
  flushText();
  return segments;
}

// A small "Sources (N)" badge under an AI reply — click to open a popover
// listing the exact uploaded knowledge-base document name(s) that backed
// that answer, so an admin can verify retrieval is pulling from the right
// file before enabling live AI replies. Renders nothing when there are no
// sources (e.g. a greeting, or a question the KB has no relevant content for).
function SourcesBadge({ sources }: { sources: string[] }) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    function onClickOutside(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener('mousedown', onClickOutside);
    return () => document.removeEventListener('mousedown', onClickOutside);
  }, [open]);

  if (sources.length === 0) return null;

  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex items-center gap-1 rounded-full border border-zinc-200 bg-zinc-50 px-2 py-0.5 text-[9px] font-bold uppercase tracking-wide text-zinc-500 transition-colors hover:border-[var(--brand,#7c3aed)] hover:text-[var(--brand,#7c3aed)] dark:border-zinc-700 dark:bg-zinc-800 dark:text-zinc-400"
      >
        <FileText className="h-2.5 w-2.5" />
        {sources.length === 1 ? 'Source' : `Sources (${sources.length})`}
      </button>
      {open && (
        <div className="absolute left-0 top-full z-10 mt-1 w-56 rounded-lg border border-zinc-200 bg-white p-2 shadow-lg dark:border-zinc-700 dark:bg-zinc-900">
          <p className="mb-1 px-1 text-[9px] font-black uppercase tracking-wider text-zinc-400">Answered using</p>
          <ul className="space-y-1">
            {sources.map((s, i) => (
              <li
                key={i}
                className="flex items-center gap-1.5 rounded px-1 py-0.5 text-[11px] font-medium text-zinc-700 dark:text-zinc-200"
              >
                <FileText className="h-3 w-3 shrink-0 text-zinc-400" />
                <span className="truncate">{s}</span>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

function TestChatBubble({ turn }: { turn: TestChatTurn }) {
  const isUser = turn.role === 'user';
  const segments = isUser ? null : parseMarkdownish(turn.text);
  const hasTable = segments?.some((s) => s.kind === 'table') ?? false;

  return (
    <li className={cn('flex flex-col gap-0.5', isUser ? 'items-end' : 'items-start')}>
      <span className="px-1 text-[9px] font-black uppercase tracking-wider text-zinc-400">
        {isUser ? 'You' : 'AI'}
      </span>
      <div
        className={cn(
          'rounded-lg border px-3 py-2 text-xs shadow-sm',
          hasTable ? 'max-w-full' : 'max-w-[85%]',
          isUser
            ? 'border-slate-300 bg-slate-100 text-slate-800 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100'
            : 'border-zinc-200 bg-white text-zinc-800 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-100',
        )}
      >
        {isUser || !segments ? (
          <div className="whitespace-pre-wrap font-medium leading-relaxed">{turn.text}</div>
        ) : (
          <div className="space-y-2">
            {segments.map((seg, i) =>
              seg.kind === 'text' ? (
                <div key={i} className="whitespace-pre-wrap font-medium leading-relaxed">{seg.content}</div>
              ) : (
                <div key={i} className="overflow-x-auto">
                  <table className="min-w-[280px] border-collapse text-[11px]">
                    <thead>
                      <tr>
                        {seg.headers.map((h, hi) => (
                          <th
                            key={hi}
                            className="border border-zinc-200 bg-zinc-50 px-2 py-1 text-left font-bold dark:border-zinc-700 dark:bg-zinc-800"
                          >
                            {h}
                          </th>
                        ))}
                      </tr>
                    </thead>
                    <tbody>
                      {seg.rows.map((row, ri) => (
                        <tr key={ri}>
                          {row.map((cell, ci) => (
                            <td key={ci} className="border border-zinc-200 px-2 py-1 dark:border-zinc-700">
                              {cell}
                            </td>
                          ))}
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ),
            )}
          </div>
        )}
      </div>
      {!isUser && turn.sources && turn.sources.length > 0 && <SourcesBadge sources={turn.sources} />}
    </li>
  );
}
