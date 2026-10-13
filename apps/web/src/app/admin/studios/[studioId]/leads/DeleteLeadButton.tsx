'use client';

import { useState, useEffect } from 'react';
import { createPortal } from 'react-dom';
import { useRouter } from 'next/navigation';
import { Trash2, Loader2, AlertTriangle } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { deleteLeadAction } from './actions';

interface DeleteLeadButtonProps {
  studioId: string;
  leadId: string;
  leadName: string;
}

// A lead with payment/subscription history is refused server-side (see
// DeleteLead's doc comment) rather than silently losing those records —
// surfaced here as an inline error instead of a generic failure toast.
//
// A centered modal, not an inline Yes/No strip over the row/card: the
// inline version overlapped the card's own content in the Pipeline board
// (not enough room on a compact card) and looked cramped in the Leads
// list too.
export function DeleteLeadButton({ studioId, leadId, leadName }: DeleteLeadButtonProps) {
  const router = useRouter();
  const [mounted, setMounted] = useState(false);
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setMounted(true);
  }, []);

  async function handleDelete() {
    setLoading(true);
    setError(null);
    const res = await deleteLeadAction(studioId, leadId);
    setLoading(false);
    if (res.ok) {
      router.refresh();
      setOpen(false);
    } else {
      setError(res.error);
    }
  }

  return (
    <>
      <button
        onClick={(e) => { e.preventDefault(); e.stopPropagation(); setOpen(true); }}
        className="rounded-full p-1.5 text-zinc-400 hover:bg-red-500/10 hover:text-red-500"
        title="Delete lead"
      >
        <Trash2 className="h-3.5 w-3.5" />
      </button>

      {open && mounted && createPortal(
        <div
          className="fixed inset-0 z-[60] flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm"
          onClick={(e) => { e.preventDefault(); e.stopPropagation(); if (!loading) setOpen(false); }}
        >
          <div
            className="w-full max-w-sm overflow-hidden rounded-[24px] border border-white/30 bg-white/90 p-6 shadow-2xl backdrop-blur-2xl dark:border-white/5 dark:bg-zinc-900/90"
            style={{ boxShadow: '0 20px 50px rgba(0,0,0,0.3)' }}
            onClick={(e) => { e.preventDefault(); e.stopPropagation(); }}
          >
            <div className="flex items-start gap-3">
              <div className="grid h-10 w-10 shrink-0 place-items-center rounded-full bg-red-500/10 text-red-500">
                <AlertTriangle className="h-5 w-5" />
              </div>
              <div className="min-w-0">
                <h2 className="text-base font-black tracking-tight text-zinc-900 dark:text-white">Delete lead?</h2>
                <p className="mt-1 text-sm text-zinc-500 dark:text-zinc-400">
                  {leadName ? <>Permanently delete <strong className="text-zinc-700 dark:text-zinc-200">{leadName}</strong>?</> : 'Permanently delete this lead?'}{' '}
                  This can&apos;t be undone — their conversation history is kept, but the lead record itself is gone.
                </p>
              </div>
            </div>

            {error && (
              <p className="mt-4 rounded-xl bg-red-50 px-3 py-2 text-xs font-semibold text-red-600 dark:bg-red-950/30 dark:text-red-400">
                {error}
              </p>
            )}

            <div className="mt-5 flex justify-end gap-2">
              <Button type="button" variant="ghost" size="sm" disabled={loading} onClick={() => setOpen(false)}>
                Cancel
              </Button>
              <Button
                type="button"
                variant="danger"
                size="sm"
                onClick={handleDelete}
                disabled={loading}
              >
                {loading ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : 'Delete'}
              </Button>
            </div>
          </div>
        </div>,
        document.body,
      )}
    </>
  );
}
