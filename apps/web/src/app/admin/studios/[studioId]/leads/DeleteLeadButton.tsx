'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { Trash2, Loader2 } from 'lucide-react';
import { deleteLeadAction } from './actions';

interface DeleteLeadButtonProps {
  studioId: string;
  leadId: string;
  leadName: string;
}

// A lead with payment/subscription history is refused server-side (see
// DeleteLead's doc comment) rather than silently losing those records —
// surfaced here as an inline error instead of a generic failure toast.
export function DeleteLeadButton({ studioId, leadId, leadName }: DeleteLeadButtonProps) {
  const router = useRouter();
  const [confirming, setConfirming] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleDelete(e: React.MouseEvent) {
    e.preventDefault();
    e.stopPropagation();
    setLoading(true);
    setError(null);
    const res = await deleteLeadAction(studioId, leadId);
    setLoading(false);
    if (res.ok) {
      router.refresh();
    } else {
      setError(res.error);
      setConfirming(false);
    }
  }

  if (confirming) {
    return (
      <div
        className="flex items-center gap-1.5 rounded-full bg-white px-2 py-1 shadow-lg ring-1 ring-zinc-200 dark:bg-zinc-900 dark:ring-zinc-700"
        onClick={(e) => { e.preventDefault(); e.stopPropagation(); }}
      >
        <span className="text-[10px] font-bold text-zinc-600 dark:text-zinc-300">Delete {leadName || 'this lead'}?</span>
        <button
          onClick={handleDelete}
          disabled={loading}
          className="rounded-full bg-red-500 px-2 py-0.5 text-[10px] font-bold text-white hover:bg-red-600 disabled:opacity-50"
        >
          {loading ? <Loader2 className="h-3 w-3 animate-spin" /> : 'Yes'}
        </button>
        <button
          onClick={(e) => { e.preventDefault(); e.stopPropagation(); setConfirming(false); }}
          className="rounded-full px-2 py-0.5 text-[10px] font-bold text-zinc-500 hover:bg-zinc-100 dark:hover:bg-zinc-800"
        >
          No
        </button>
      </div>
    );
  }

  if (error) {
    return (
      <div
        className="max-w-[160px] rounded-xl bg-red-50 px-2 py-1 text-[10px] font-semibold text-red-600 dark:bg-red-950/30 dark:text-red-400"
        onClick={(e) => { e.preventDefault(); e.stopPropagation(); setError(null); }}
        title="Click to dismiss"
      >
        {error}
      </div>
    );
  }

  return (
    <button
      onClick={(e) => { e.preventDefault(); e.stopPropagation(); setConfirming(true); }}
      className="rounded-full p-1.5 text-zinc-400 hover:bg-red-500/10 hover:text-red-500"
      title="Delete lead"
    >
      <Trash2 className="h-3.5 w-3.5" />
    </button>
  );
}
