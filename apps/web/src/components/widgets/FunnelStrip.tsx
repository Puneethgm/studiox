// Horizontal-bar visualization of lead counts at each funnel stage.
// Pure SVG/CSS — no charts library. Each row's bar width is proportional
// to the largest stage so you can read the shape at a glance.

import Link from 'next/link';
import { ArrowRight } from 'lucide-react';
import { cn } from '@/lib/cn';
import type { LeadStatus } from '@/lib/types';
import { LEAD_STATUS_LABELS } from '@/lib/types';

export interface FunnelStripProps {
  byStatus: Record<LeadStatus, number>;
  total: number;
  /** Studio id for the "Open pipeline →" link. */
  studioId: string;
  className?: string;
}

// Order matters: visual top-to-bottom of the funnel.
const ORDER: LeadStatus[] = ['new', 'contacted', 'trial_booked', 'member', 'dropped'];

const COLORS: Record<LeadStatus, string> = {
  new:          '#0ea5e9', // sky
  contacted:    'var(--brand, #7c3aed)',
  trial_booked: '#f59e0b', // amber
  member:       '#10b981', // emerald
  dropped:      '#94a3b8', // slate
};

const BAR_GRADIENTS: Record<LeadStatus, string> = {
  new:          'linear-gradient(90deg, #0ea5e9, #38bdf8)',
  contacted:    'linear-gradient(90deg, var(--brand, #7c3aed), #a78bfa)',
  trial_booked: 'linear-gradient(90deg, #f59e0b, #fbbf24)',
  member:       'linear-gradient(90deg, #10b981, #34d399)',
  dropped:      'linear-gradient(90deg, #94a3b8, #cbd5e1)',
};

export function FunnelStrip({ byStatus, total, studioId, className }: FunnelStripProps) {
  const max = Math.max(1, ...ORDER.map((s) => byStatus[s] ?? 0));

  return (
    <div className={cn('', className)}>
      <div className="mb-6 flex items-end justify-between gap-3">
        <div>
          <div className="text-[10px] font-black uppercase tracking-[0.2em] text-zinc-400">
            Pipeline at a glance
          </div>
          <div className="mt-2 text-3xl font-black tracking-tight text-zinc-900 dark:text-white">
            {total} <span className="text-base font-semibold text-zinc-400">leads total</span>
          </div>
        </div>
        <Link
          href={`/admin/studios/${studioId}/pipeline`}
          className="inline-flex items-center gap-1.5 rounded-2xl bg-white/50 px-4 py-2 text-xs font-bold text-brand-600 backdrop-blur-md transition-all hover:bg-brand-500 hover:text-white hover:shadow-lg hover:shadow-brand-500/20 dark:bg-white/5 dark:text-brand-400"
        >
          Open pipeline
          <ArrowRight className="h-3.5 w-3.5" />
        </Link>
      </div>

      <ul className="space-y-4">
        {ORDER.map((status) => {
          const count = byStatus[status] ?? 0;
          const pct = (count / max) * 100;
          const sharePct = total === 0 ? 0 : Math.round((count / total) * 100);
          return (
            <li key={status} className="grid grid-cols-[7rem,1fr,4rem] items-center gap-4 sm:grid-cols-[8rem,1fr,5rem]">
              <span className="truncate text-xs font-bold text-zinc-600 dark:text-zinc-300">
                {LEAD_STATUS_LABELS[status]}
              </span>
              <div className="relative h-8 overflow-hidden rounded-2xl bg-white/40 backdrop-blur-sm dark:bg-white/5">
                <div
                  className="absolute inset-y-0 left-0 rounded-2xl transition-all duration-700 ease-out"
                  style={{ width: `${pct}%`, background: BAR_GRADIENTS[status], boxShadow: `0 4px 12px ${COLORS[status]}30` }}
                />
              </div>
              <span className="text-right text-xs tabular-nums">
                <span className="font-black text-zinc-900 dark:text-white">{count}</span>
                <span className="ml-1 font-semibold text-zinc-400">{sharePct}%</span>
              </span>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

