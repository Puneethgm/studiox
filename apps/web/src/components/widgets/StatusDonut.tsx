// SVG donut chart for the lead status distribution. No deps; renders the
// 5 status slices with status colors, with the total in the center.

import type { LeadStatus } from '@/lib/types';
import { LEAD_STATUS_LABELS } from '@/lib/types';
import { cn } from '@/lib/cn';

export interface StatusDonutProps {
  byStatus: Record<LeadStatus, number>;
  total: number;
  className?: string;
}

const ORDER: LeadStatus[] = ['new', 'contacted', 'trial_booked', 'member', 'dropped'];

const COLORS: Record<LeadStatus, string> = {
  new:          '#0ea5e9',
  contacted:    'var(--brand, #7c3aed)',
  trial_booked: '#f59e0b',
  member:       '#10b981',
  dropped:      '#94a3b8',
};

export function StatusDonut({ byStatus, total, className }: StatusDonutProps) {
  // SVG geometry
  const size = 180;
  const cx = size / 2;
  const cy = size / 2;
  const r = 72;
  const strokeWidth = 20;
  const circumference = 2 * Math.PI * r;

  let cursor = 0;

  return (
    <div className={cn('', className)}>
      <div className="mb-5">
        <div className="text-[10px] font-black uppercase tracking-[0.2em] text-zinc-400">
          Status mix
        </div>
        <div className="mt-1 text-sm font-semibold text-zinc-500 dark:text-zinc-400">
          Where every lead currently stands
        </div>
      </div>

      <div className="grid items-center gap-6 sm:grid-cols-[auto,1fr]">
        <div className="relative grid place-items-center">
          <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`} className="-rotate-90">
            {/* Track */}
            <circle
              cx={cx}
              cy={cy}
              r={r}
              fill="none"
              strokeWidth={strokeWidth}
              className="stroke-white/30 dark:stroke-white/5"
            />
            {total > 0 &&
              ORDER.map((status) => {
                const count = byStatus[status] ?? 0;
                if (count === 0) return null;
                const length = (count / total) * circumference;
                const dasharray = `${length} ${circumference - length}`;
                const dashoffset = -cursor;
                cursor += length;
                return (
                  <circle
                    key={status}
                    cx={cx}
                    cy={cy}
                    r={r}
                    fill="none"
                    stroke={COLORS[status]}
                    strokeWidth={strokeWidth}
                    strokeDasharray={dasharray}
                    strokeDashoffset={dashoffset}
                    strokeLinecap="butt"
                    style={{ filter: `drop-shadow(0 2px 6px ${COLORS[status]}40)` }}
                  />
                );
              })}
          </svg>
          <div className="pointer-events-none absolute inset-0 grid place-items-center">
            <div className="text-center">
              <div className="text-4xl font-black tracking-tight text-zinc-900 dark:text-white">
                {total}
              </div>
              <div className="text-[10px] font-black uppercase tracking-[0.2em] text-zinc-400">
                Leads
              </div>
            </div>
          </div>
        </div>

        <ul className="space-y-3 text-sm">
          {ORDER.map((status) => {
            const count = byStatus[status] ?? 0;
            const sharePct = total === 0 ? 0 : Math.round((count / total) * 100);
            return (
              <li key={status} className="flex items-center justify-between gap-3 rounded-2xl bg-white/30 p-3 backdrop-blur-sm dark:bg-white/5">
                <span className="flex items-center gap-2.5 text-zinc-600 dark:text-zinc-300">
                  <span
                    aria-hidden
                    className="h-3 w-3 shrink-0 rounded-full shadow-sm"
                    style={{ background: COLORS[status], boxShadow: `0 2px 8px ${COLORS[status]}40` }}
                  />
                  <span className="truncate font-semibold">{LEAD_STATUS_LABELS[status]}</span>
                </span>
                <span className="shrink-0 tabular-nums">
                  <span className="font-black text-zinc-900 dark:text-white">{count}</span>
                  <span className="ml-1.5 text-xs font-semibold text-zinc-400">{sharePct}%</span>
                </span>
              </li>
            );
          })}
        </ul>
      </div>
    </div>
  );
}

