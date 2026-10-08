'use client';

import { useEffect, useState } from 'react';
import { AlertCircle, CheckCircle2, HelpCircle, Loader2, MessageSquareText, X } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { listKnowledgeGaps, resolveKnowledgeGap, dismissKnowledgeGap, type KnowledgeGap } from './actions';

function timeAgo(iso: string): string {
  const diffMs = Date.now() - new Date(iso).getTime();
  const mins = Math.round(diffMs / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.round(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  const days = Math.round(hrs / 24);
  return `${days}d ago`;
}

// One open question: shows what the customer asked, when, how many times, and (if
// linked) who — with an inline answer box that, on save, appends the Q&A straight
// into the knowledge base and re-embeds it. Answering here is the whole point of
// this tab: no separate trip to the Instructions tab is needed.
function GapCard({ gap, onAnswered, onDismissed }: { gap: KnowledgeGap; onAnswered: () => void; onDismissed: () => void }) {
  const [answering, setAnswering] = useState(false);
  const [answer, setAnswer] = useState('');
  const [saving, setSaving] = useState(false);
  const [dismissing, setDismissing] = useState(false);
  const [error, setError] = useState('');

  async function submit() {
    if (!answer.trim()) {
      setError('Enter an answer before adding it to the knowledge base.');
      return;
    }
    setSaving(true);
    setError('');
    const res = await resolveKnowledgeGap(gap.studioId, gap.id, answer.trim());
    setSaving(false);
    if (res.ok) {
      onAnswered();
    } else {
      setError(res.error ?? 'Failed to save');
    }
  }

  async function dismiss() {
    setDismissing(true);
    const res = await dismissKnowledgeGap(gap.studioId, gap.id);
    setDismissing(false);
    if (res.ok) onDismissed();
    else setError(res.error ?? 'Failed to dismiss');
  }

  return (
    <div className="rounded-xl border border-zinc-200 bg-white p-4 dark:border-zinc-800 dark:bg-zinc-950">
      <div className="flex items-start justify-between gap-3">
        <div className="flex min-w-0 items-start gap-2.5">
          <HelpCircle className="mt-0.5 h-4 w-4 shrink-0 text-amber-500" />
          <div className="min-w-0">
            <p className="text-sm font-semibold text-zinc-800 dark:text-zinc-100">{gap.question}</p>
            <p className="mt-1 text-[11px] text-zinc-400">
              {gap.leadName ? `${gap.leadName} · ` : ''}
              Asked {gap.timesAsked > 1 ? `${gap.timesAsked} times, ` : ''}
              {timeAgo(gap.createdAt)}
            </p>
          </div>
        </div>
        {!answering && (
          <div className="flex shrink-0 items-center gap-1.5">
            <Button type="button" variant="outline" className="h-7 px-2.5 text-xs" onClick={() => setAnswering(true)}>
              Add answer
            </Button>
            <button
              type="button"
              onClick={dismiss}
              disabled={dismissing}
              title="Dismiss — won't be added to the knowledge base"
              className="rounded-lg p-1.5 text-zinc-400 transition-colors hover:bg-zinc-100 hover:text-zinc-600 disabled:opacity-50 dark:hover:bg-zinc-900 dark:hover:text-zinc-300"
            >
              {dismissing ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <X className="h-3.5 w-3.5" />}
            </button>
          </div>
        )}
      </div>

      {answering && (
        <div className="mt-3 space-y-2 border-t border-zinc-100 pt-3 dark:border-zinc-800">
          <textarea
            autoFocus
            value={answer}
            onChange={(e) => setAnswer(e.target.value)}
            rows={3}
            placeholder="Type the answer here — it's added to the knowledge base exactly as written and the AI can use it on the very next message."
            className="w-full rounded-xl border border-zinc-200 bg-white p-3 text-sm text-zinc-800 focus:border-brand-500 focus:outline-none dark:border-zinc-800 dark:bg-zinc-900 dark:text-zinc-100"
          />
          {error && (
            <p className="flex items-center gap-1.5 text-xs font-medium text-rose-600 dark:text-rose-400">
              <AlertCircle className="h-3.5 w-3.5 shrink-0" />
              {error}
            </p>
          )}
          <div className="flex justify-end gap-2">
            <Button
              type="button"
              variant="ghost"
              className="h-8 text-xs"
              onClick={() => {
                setAnswering(false);
                setAnswer('');
                setError('');
              }}
              disabled={saving}
            >
              Cancel
            </Button>
            <Button type="button" className="h-8 text-xs" onClick={submit} loading={saving}>
              Add to knowledge base
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}

export function NeedsAnswersTab({ studioId }: { studioId: string }) {
  const [gaps, setGaps] = useState<KnowledgeGap[] | null>(null);
  const [error, setError] = useState('');
  const [justAnswered, setJustAnswered] = useState(0);

  async function load() {
    const res = await listKnowledgeGaps(studioId, 'open');
    if (res.ok) {
      setGaps(res.gaps ?? []);
      setError('');
    } else {
      setError(res.error ?? 'Failed to load');
    }
  }

  useEffect(() => {
    void load();
    // Light polling so a question that comes in while this tab is open shows up
    // without a manual refresh — matches the Inbox's own auto-refresh cadence.
    const id = setInterval(load, 15000);
    return () => clearInterval(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [studioId]);

  function removeGap(id: string) {
    setGaps((prev) => (prev ? prev.filter((g) => g.id !== id) : prev));
  }

  return (
    <div className="max-w-2xl space-y-4">
      <div className="rounded-xl border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-950">
        <div className="flex items-center gap-2 border-b border-zinc-200 px-6 py-4 dark:border-zinc-800">
          <MessageSquareText className="h-4 w-4 text-zinc-400" />
          <h3 className="text-sm font-bold text-zinc-900 dark:text-white">Questions the AI couldn&apos;t answer</h3>
        </div>
        <div className="p-6">
          <p className="mb-4 text-xs text-zinc-400">
            Whenever a customer asks something the knowledge base doesn&apos;t cover, the AI hands the
            conversation to your team instead of guessing — and the question lands here. Answer it once and
            it&apos;s added straight into the knowledge base, so the AI can handle it next time.
          </p>

          {justAnswered > 0 && (
            <div className="mb-4 flex items-center gap-2 rounded-xl border border-emerald-500/20 bg-emerald-500/5 p-3 text-sm font-semibold text-emerald-600 dark:text-emerald-400">
              <CheckCircle2 className="h-4 w-4 shrink-0" />
              <span>Added to the knowledge base. The AI can use this starting now.</span>
            </div>
          )}

          {error && (
            <div className="mb-4 flex items-center gap-2 rounded-xl border border-rose-500/20 bg-rose-500/5 p-3 text-sm font-semibold text-rose-600 dark:text-rose-400">
              <AlertCircle className="h-4 w-4 shrink-0" />
              <span>{error}</span>
            </div>
          )}

          {gaps === null ? (
            <div className="flex items-center gap-2 py-8 text-sm text-zinc-400">
              <Loader2 className="h-4 w-4 animate-spin" />
              Loading…
            </div>
          ) : gaps.length === 0 ? (
            <div className="flex flex-col items-center gap-2 rounded-xl border border-dashed border-zinc-200 py-10 text-center dark:border-zinc-800">
              <CheckCircle2 className="h-6 w-6 text-emerald-500" />
              <p className="text-sm font-semibold text-zinc-600 dark:text-zinc-300">All caught up</p>
              <p className="max-w-xs text-xs text-zinc-400">
                Nothing waiting right now. New questions the AI can&apos;t answer will show up here as they come in.
              </p>
            </div>
          ) : (
            <div className="space-y-3">
              {gaps.map((gap) => (
                <GapCard
                  key={gap.id}
                  gap={gap}
                  onAnswered={() => {
                    removeGap(gap.id);
                    setJustAnswered((n) => n + 1);
                  }}
                  onDismissed={() => removeGap(gap.id)}
                />
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
