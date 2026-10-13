'use client';

import { useState, useEffect } from 'react';
import { createPortal } from 'react-dom';
import { useRouter } from 'next/navigation';
import { Sparkles, X, AlertTriangle, CheckCircle2, AlertCircle } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { useLeadSelection } from './LeadSelectionContext';
import { enableAIForLeadsAction } from './actions';

// Floating bar that appears once at least one lead row is checked —
// enables AI for exactly those leads (see enableAIForLeadsAction), as
// opposed to EnableAIButton's whole-status bulk action.
export function EnableAIForSelectedBar({ studioId }: { studioId: string }) {
  const router = useRouter();
  const { selected, clear } = useLeadSelection();
  const [mounted, setMounted] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<{ ok: boolean; message: string } | null>(null);

  useEffect(() => {
    setMounted(true);
  }, []);

  if (selected.size === 0) return null;

  async function handleConfirm() {
    setLoading(true);
    setResult(null);
    const res = await enableAIForLeadsAction(studioId, Array.from(selected));
    setLoading(false);
    if (res.ok) {
      setResult({ ok: true, message: res.message || `Enabled AI for ${res.enqueued ?? 0} lead(s).` });
      router.refresh();
      setTimeout(() => {
        clear();
        setConfirming(false);
        setResult(null);
      }, 2000);
    } else {
      setResult({ ok: false, message: res.error || 'Failed to enable AI.' });
    }
  }

  return (
    <>
      <div className="fixed bottom-6 left-1/2 z-40 flex -translate-x-1/2 items-center gap-3 rounded-full border border-white/30 bg-white/90 px-4 py-2.5 shadow-2xl backdrop-blur-xl dark:border-white/10 dark:bg-zinc-900/90">
        <span className="text-xs font-bold text-zinc-700 dark:text-zinc-200">
          {selected.size} lead{selected.size === 1 ? '' : 's'} selected
        </span>
        <Button size="sm" leftIcon={<Sparkles className="h-3.5 w-3.5" />} onClick={() => setConfirming(true)}>
          Enable AI for Selected
        </Button>
        <button
          onClick={clear}
          className="rounded-full p-1 text-zinc-400 hover:bg-zinc-200/50 dark:hover:bg-zinc-800/50"
          title="Clear selection"
        >
          <X className="h-4 w-4" />
        </button>
      </div>

      {confirming && mounted && createPortal(
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm">
          <div
            className="w-full max-w-md overflow-hidden rounded-[24px] border border-white/30 bg-white/90 p-6 shadow-2xl backdrop-blur-2xl dark:border-white/5 dark:bg-zinc-900/90"
            style={{ boxShadow: '0 20px 50px rgba(0,0,0,0.3)' }}
          >
            <div className="flex items-start gap-3">
              <div className="grid h-10 w-10 shrink-0 place-items-center rounded-full bg-amber-500/10 text-amber-500">
                <AlertTriangle className="h-5 w-5" />
              </div>
              <div className="min-w-0">
                <h2 className="text-base font-black tracking-tight text-zinc-900 dark:text-white">
                  Enable AI for {selected.size} lead{selected.size === 1 ? '' : 's'}?
                </h2>
                <p className="mt-1 text-sm text-zinc-500 dark:text-zinc-400">
                  Each one gets a real WhatsApp message now: a context-aware follow-up if they already have a
                  conversation, or the normal greeting if not. Sends go out spaced apart, not all at once. This can&apos;t be undone.
                </p>
              </div>
            </div>

            {result && (
              <div
                className={`mt-4 flex items-start gap-2 rounded-xl px-3 py-2 text-xs font-semibold ${
                  result.ok
                    ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                    : 'bg-red-50 text-red-600 dark:bg-red-950/30 dark:text-red-400'
                }`}
              >
                {result.ok ? <CheckCircle2 className="h-4 w-4 shrink-0" /> : <AlertCircle className="h-4 w-4 shrink-0" />}
                <span>{result.message}</span>
              </div>
            )}

            <div className="mt-5 flex justify-end gap-2">
              <Button type="button" variant="ghost" size="sm" disabled={loading} onClick={() => setConfirming(false)}>
                Cancel
              </Button>
              <Button type="button" size="sm" loading={loading} onClick={handleConfirm}>
                Yes, enable AI
              </Button>
            </div>
          </div>
        </div>,
        document.body,
      )}
    </>
  );
}
