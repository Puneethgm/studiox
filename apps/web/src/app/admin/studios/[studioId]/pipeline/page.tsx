import { Inbox } from 'lucide-react';
import { EmptyState } from '@/components/ui/EmptyState';
import { serverFetch } from '@/lib/auth';
import type { Lead, LeadStatus } from '@/lib/types';
import { LEAD_STATUSES } from '@/lib/types';
import { AutoRefresh } from '@/components/AutoRefresh';
import { PipelineBoard } from './PipelineBoard';

interface ListResp {
  leads: Lead[];
  total: number;
}

interface LeadStats {
  total: number;
  byStatus: Record<LeadStatus, number>;
}

const COLUMN_CAP = 50;

const COLUMN_CONFIG: Record<LeadStatus, { color: string; pill: string; pillText: string }> = {
  new: { color: '#0ea5e9', pill: 'rgba(14,165,233,0.12)', pillText: '#0284c7' },
  contacted: { color: '#7c3aed', pill: 'rgba(124,58,237,0.10)', pillText: '#6d28d9' },
  trial_booked: { color: '#f59e0b', pill: 'rgba(245,158,11,0.10)', pillText: '#d97706' },
  member: { color: '#10b981', pill: 'rgba(16,185,129,0.10)', pillText: '#059669' },
  dropped: { color: '#94a3b8', pill: 'rgba(148,163,184,0.10)', pillText: '#64748b' },
  paused: { color: '#6366f1', pill: 'rgba(99,102,241,0.10)', pillText: '#4f46e5' },
};

export default async function PipelinePage({
  params,
}: {
  params: Promise<{ studioId: string }>;
}) {
  const { studioId } = await params;

  const [stats, ...buckets] = await Promise.all([
    serverFetch<LeadStats>(`/api/v1/studios/${studioId}/leads/stats`),
    ...LEAD_STATUSES.map((s) =>
      serverFetch<ListResp>(
        `/api/v1/studios/${studioId}/leads?status=${s}&limit=${COLUMN_CAP}`,
      ),
    ),
  ]);

  const byStatus = LEAD_STATUSES.reduce(
    (acc, status, i) => {
      acc[status] = buckets[i]?.leads ?? [];
      return acc;
    },
    {} as Record<LeadStatus, Lead[]>,
  );

  const overflowCounts = LEAD_STATUSES.reduce(
    (acc, status) => {
      acc[status] = (stats.byStatus[status] ?? 0) - (byStatus[status]?.length ?? 0);
      return acc;
    },
    {} as Record<LeadStatus, number>,
  );

  const activeCount =
    (stats.byStatus.new ?? 0) +
    (stats.byStatus.contacted ?? 0) +
    (stats.byStatus.trial_booked ?? 0) +
    (stats.byStatus.paused ?? 0);
  const memberCount = stats.byStatus.member ?? 0;
  const conversionPct =
    stats.total > 0 ? Math.round((memberCount / stats.total) * 100) : 0;

  return (
    <div className="flex h-[calc(100vh-10rem)] lg:h-[calc(100vh-11rem)] flex-col gap-4">
      <AutoRefresh intervalMs={4000} />

      {/* Sleek inline stats bar */}
      <div className="flex flex-wrap items-center justify-between gap-4 px-2">
        <div className="text-[11px] font-semibold text-zinc-400 dark:text-zinc-500">
          Total: <span className="font-extrabold text-zinc-800 dark:text-zinc-200">{stats.total} leads</span> · Active: <span className="font-extrabold text-zinc-800 dark:text-zinc-200">{activeCount} active</span> · Conversion: <span className="font-extrabold text-zinc-800 dark:text-zinc-200">{conversionPct}%</span>
          <span className="ml-2 text-zinc-300 dark:text-zinc-600">· Drag a card to change its stage</span>
        </div>

        {/* Stage count pills */}
        <div className="flex flex-wrap items-center gap-2">
          {LEAD_STATUSES.map((status) => {
            const cfg = COLUMN_CONFIG[status];
            const n = stats.byStatus[status] ?? 0;
            if (!n) return null;
            return (
              <div
                key={status}
                className="flex items-center gap-1 rounded-full px-2.5 py-0.5 text-[9px] font-black uppercase tracking-wider"
                style={{ background: cfg.pill, color: cfg.pillText }}
              >
                <span
                  className="h-1.5 w-1.5 rounded-full"
                  style={{ background: cfg.color }}
                />
                {n}
              </div>
            );
          })}
        </div>
      </div>

      {/* ── Board ── */}
      {stats.total === 0 ? (
        <div
          className="flex-1 overflow-hidden rounded-[22px] border border-white/30 backdrop-blur-2xl dark:border-white/5"
          style={{ background: 'rgba(255,255,255,0.22)' }}
        >
          <EmptyState
            icon={<Inbox className="h-5 w-5" />}
            title="No leads yet"
            description="Once people submit a campaign form, they'll show up here grouped by status."
          />
        </div>
      ) : (
        <PipelineBoard
          studioId={studioId}
          initialByStatus={byStatus}
          counts={stats.byStatus}
          overflowCounts={overflowCounts}
        />
      )}
    </div>
  );
}
