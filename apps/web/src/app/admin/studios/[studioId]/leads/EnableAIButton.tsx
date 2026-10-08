'use client';

import { useState, useEffect } from 'react';
import { createPortal } from 'react-dom';
import { useRouter } from 'next/navigation';
import { Sparkles, X, CheckCircle2, AlertCircle, AlertTriangle } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Label } from '@/components/ui/Label';
import { LEAD_STATUSES, LEAD_STATUS_LABELS, type LeadStatus } from '@/lib/types';
import { enableAutoContactForStatusAction } from './actions';

interface EnableAIButtonProps {
  studioId: string;
}

// The Leads page's "Enable AI" control: pick one or more lead statuses, and
// every current lead at any of them (that hasn't already been contacted)
// gets connected to the AI auto-contact worker right away — a deliberate,
// explicit bulk action instead of something that happens automatically on import.
export function EnableAIButton({ studioId }: EnableAIButtonProps) {
  const router = useRouter();
  const [mounted, setMounted] = useState(false);
  const [isOpen, setIsOpen] = useState(false);
  const [selected, setSelected] = useState<LeadStatus[]>(['new']);
  const [confirming, setConfirming] = useState(false);
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<{ ok: boolean; message: string } | null>(null);
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null);

  useEffect(() => {
    setMounted(true);
  }, []);

  useEffect(() => {
    if (!toast) return;
    const t = setTimeout(() => setToast(null), 4000);
    return () => clearTimeout(t);
  }, [toast]);

  function close() {
    setIsOpen(false);
    setConfirming(false);
    setResult(null);
  }

  function toggleStatus(s: LeadStatus) {
    setConfirming(false);
    setSelected((prev) => (prev.includes(s) ? prev.filter((x) => x !== s) : [...prev, s]));
  }

  async function handleConfirm() {
    setLoading(true);
    setResult(null);
    try {
      const res = await enableAutoContactForStatusAction(studioId, selected);
      if (res.ok) {
        const msg = res.message || `Enabled AI for ${res.enqueued ?? 0} lead(s).`;
        setResult({ ok: true, message: msg });
        setToast({ message: msg, type: 'success' });
        router.refresh();
        setTimeout(close, 2000);
      } else {
        setResult({ ok: false, message: res.error || 'Failed to enable AI.' });
        setToast({ message: res.error || 'Failed to enable AI.', type: 'error' });
      }
    } catch (err: any) {
      setResult({ ok: false, message: err.message || 'An error occurred.' });
      setToast({ message: err.message || 'An error occurred.', type: 'error' });
    } finally {
      setLoading(false);
    }
  }

  return (
    <>
      <Button
        onClick={() => setIsOpen(true)}
        variant="ghost"
        size="sm"
        leftIcon={<Sparkles className="h-3.5 w-3.5" />}
        className="rounded-xl border border-white/20 bg-white/10 px-3 py-1.5 text-xs font-bold text-zinc-700 hover:bg-white/20 dark:text-zinc-200 dark:hover:bg-neutral-800/50 shadow-sm shrink-0"
      >
        Enable AI
      </Button>

      {isOpen && mounted && createPortal(
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm">
          <div
            className="w-full max-w-md overflow-hidden rounded-[28px] border border-white/30 bg-white/80 p-6 shadow-2xl backdrop-blur-2xl dark:border-white/5 dark:bg-zinc-900/80"
            style={{ boxShadow: '0 20px 50px rgba(0,0,0,0.3)' }}
          >
            <div className="flex items-center justify-between border-b border-zinc-200/50 pb-4 dark:border-zinc-800/50">
              <h2 className="text-lg font-black tracking-tight text-zinc-900 dark:text-white">Enable AI Auto-Contact</h2>
              <button onClick={close} className="rounded-full p-1 text-zinc-400 hover:bg-zinc-200/50 dark:hover:bg-zinc-800/50">
                <X className="h-5 w-5" />
              </button>
            </div>

            <div className="mt-5 space-y-5">
              <div>
                <Label>Lead statuses to connect</Label>
                <div className="mt-1.5 grid grid-cols-2 gap-2">
                  {LEAD_STATUSES.map((s) => {
                    const checked = selected.includes(s);
                    return (
                      <label
                        key={s}
                        className={`flex items-center gap-2 rounded-xl border px-3 py-2 text-xs font-semibold cursor-pointer transition-colors ${
                          checked
                            ? 'border-brand-500 bg-brand-500/10 text-brand-700 dark:text-brand-300'
                            : 'border-zinc-200 bg-white text-zinc-600 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-300'
                        }`}
                      >
                        <input
                          type="checkbox"
                          checked={checked}
                          onChange={() => toggleStatus(s)}
                          className="h-3.5 w-3.5 rounded border-zinc-300 text-brand-500 focus:ring-brand-500 dark:border-zinc-700"
                        />
                        {LEAD_STATUS_LABELS[s]}
                      </label>
                    );
                  })}
                </div>
                <p className="mt-1.5 text-[11px] text-zinc-400">
                  Only leads currently at a checked status, and not already contacted, will be connected —
                  nothing happens to leads at any other status.
                </p>
              </div>

              {!confirming && selected.length > 0 ? (
                <div className="flex items-start gap-2 rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-xs font-semibold text-amber-700 dark:border-amber-800/50 dark:bg-amber-900/20 dark:text-amber-400">
                  <AlertTriangle className="h-4 w-4 shrink-0 mt-0.5" />
                  <span>
                    This will immediately WhatsApp every eligible lead at{' '}
                    {selected.map((s) => `"${LEAD_STATUS_LABELS[s]}"`).join(', ')} with the automatic opening message.
                    This can't be undone.
                  </span>
                </div>
              ) : null}

              {result && (
                <div
                  className={`flex items-start gap-3 rounded-2xl p-4 text-sm font-medium ${
                    result.ok
                      ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                      : 'bg-rose-500/10 text-rose-600 dark:text-rose-400'
                  }`}
                >
                  {result.ok ? <CheckCircle2 className="h-5 w-5 shrink-0" /> : <AlertCircle className="h-5 w-5 shrink-0" />}
                  <span>{result.message}</span>
                </div>
              )}

              <div className="flex items-center justify-end gap-3 border-t border-zinc-200/50 pt-4 dark:border-zinc-800/50">
                <Button type="button" variant="ghost" onClick={close} disabled={loading}>
                  Cancel
                </Button>
                {confirming ? (
                  <Button type="button" loading={loading} onClick={handleConfirm}>
                    Yes, connect them
                  </Button>
                ) : (
                  <Button type="button" disabled={selected.length === 0} onClick={() => setConfirming(true)}>
                    Continue
                  </Button>
                )}
              </div>
            </div>
          </div>
        </div>,
        document.body
      )}

      {toast && (
        <div className={`fixed bottom-6 right-6 z-[9999] p-4 rounded-2xl border backdrop-blur-xl shadow-2xl flex items-center gap-3 animate-in slide-in-from-bottom-5 fade-in duration-300 min-w-[320px] ${
          toast.type === 'success'
            ? 'border-emerald-500/30 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
            : 'border-rose-500/30 bg-rose-500/10 text-rose-600 dark:text-rose-400'
        }`}>
          {toast.type === 'success' ? <CheckCircle2 className="h-5 w-5 shrink-0" /> : <AlertCircle className="h-5 w-5 shrink-0" />}
          <span className="text-sm font-semibold">{toast.message}</span>
        </div>
      )}
    </>
  );
}
