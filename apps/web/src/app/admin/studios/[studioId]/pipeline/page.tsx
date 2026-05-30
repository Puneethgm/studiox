import Link from 'next/link';
import { ArrowRight, Inbox, MessageSquareText, GitBranch } from 'lucide-react';
import { EmptyState } from '@/components/ui/EmptyState';
import { serverFetch } from '@/lib/auth';
import { brandInitials } from '@/lib/color';
import { cn } from '@/lib/cn';
import { relativeTime } from '@/lib/datetime';
import type { Lead, LeadStatus } from '@/lib/types';
import { LEAD_STATUSES, LEAD_STATUS_LABELS } from '@/lib/types';

interface ListResp {
  leads: Lead[];
  total: number;
}

interface LeadStats {
  total: number;
  byStatus: Record<LeadStatus, number>;
}

const COLUMN_CAP = 50;

// Per-status visual config
const COLUMN_CONFIG: Record<LeadStatus, {
  color: string;
  lightBg: string;
  border: string;
  glow: string;
  pill: string;
  pillText: string;
  avatarRing: string;
}> = {
  new: {
    color: '#0ea5e9',
    lightBg: 'rgba(240,249,255,0.45)',
    border: 'rgba(125,211,252,0.40)',
    glow: 'rgba(14,165,233,0.12)',
    pill: 'rgba(14,165,233,0.12)',
    pillText: '#0284c7',
    avatarRing: 'rgba(14,165,233,0.30)',
  },
  contacted: {
    color: '#7c3aed',
    lightBg: 'rgba(245,243,255,0.45)',
    border: 'rgba(196,181,253,0.40)',
    glow: 'rgba(124,58,237,0.12)',
    pill: 'rgba(124,58,237,0.10)',
    pillText: '#6d28d9',
    avatarRing: 'rgba(124,58,237,0.25)',
  },
  trial_booked: {
    color: '#f59e0b',
    lightBg: 'rgba(255,251,235,0.45)',
    border: 'rgba(252,211,77,0.40)',
    glow: 'rgba(245,158,11,0.12)',
    pill: 'rgba(245,158,11,0.10)',
    pillText: '#d97706',
    avatarRing: 'rgba(245,158,11,0.30)',
  },
  member: {
    color: '#10b981',
    lightBg: 'rgba(236,253,245,0.45)',
    border: 'rgba(110,231,183,0.40)',
    glow: 'rgba(16,185,129,0.12)',
    pill: 'rgba(16,185,129,0.10)',
    pillText: '#059669',
    avatarRing: 'rgba(16,185,129,0.25)',
  },
  dropped: {
    color: '#94a3b8',
    lightBg: 'rgba(248,250,252,0.45)',
    border: 'rgba(203,213,225,0.40)',
    glow: 'rgba(148,163,184,0.10)',
    pill: 'rgba(148,163,184,0.10)',
    pillText: '#64748b',
    avatarRing: 'rgba(148,163,184,0.25)',
  },
  paused: {
    color: '#6366f1',
    lightBg: 'rgba(238,242,255,0.45)',
    border: 'rgba(199,210,254,0.40)',
    glow: 'rgba(99,102,241,0.12)',
    pill: 'rgba(99,102,241,0.10)',
    pillText: '#4f46e5',
    avatarRing: 'rgba(99,102,241,0.25)',
  },
};

const AVATAR_PALETTE = [
  '#0ea5e9', '#6366f1', '#7c3aed', '#a855f7', '#ec4899',
  '#f43f5e', '#f97316', '#eab308', '#22c55e', '#14b8a6',
];

function avatarColor(seed: string): string {
  let h = 0;
  for (let i = 0; i < seed.length; i++) {
    h = ((h << 5) - h + seed.charCodeAt(i)) | 0;
  }
  return AVATAR_PALETTE[Math.abs(h) % AVATAR_PALETTE.length]!;
}

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

  const activeCount =
    (stats.byStatus.new ?? 0) +
    (stats.byStatus.contacted ?? 0) +
    (stats.byStatus.trial_booked ?? 0) +
    (stats.byStatus.paused ?? 0);
  const memberCount = stats.byStatus.member ?? 0;
  const conversionPct =
    stats.total > 0 ? Math.round((memberCount / stats.total) * 100) : 0;

  return (
    <div className="flex h-[calc(100vh-8rem)] flex-col gap-5">

      {/* ── Compact glass header ── */}
      <div
        className="relative shrink-0 overflow-hidden rounded-[22px] border border-white/30 bg-white/30 px-5 py-4 backdrop-blur-2xl dark:border-white/5 dark:bg-neutral-900/30"
        style={{
          boxShadow: 'inset 0 0 0 1px rgba(255,255,255,0.20), 0 4px 16px rgba(139,92,246,0.06)',
        }}
      >
        <div className="pointer-events-none absolute -right-10 -top-10 h-32 w-32 rounded-full bg-brand-500/10 blur-3xl" />
        <div className="relative flex items-center justify-between gap-6">
          <div className="flex items-center gap-3">
            <div className="grid h-9 w-9 place-items-center rounded-xl bg-gradient-to-br from-brand-500 to-violet-600 text-white shadow-md shadow-brand-500/25">
              <GitBranch className="h-4 w-4" />
            </div>
            <div>
              <h1 className="text-lg font-black tracking-tight text-zinc-900 dark:text-white">Pipeline</h1>
              <p className="text-[11px] font-semibold text-zinc-400">
                {stats.total} leads · {activeCount} active · {conversionPct}% conversion
              </p>
            </div>
          </div>
          {/* Stage count pills */}
          <div className="hidden items-center gap-2 sm:flex">
            {LEAD_STATUSES.map((status) => {
              const cfg = COLUMN_CONFIG[status];
              const n = stats.byStatus[status] ?? 0;
              if (!n) return null;
              return (
                <div
                  key={status}
                  className="flex items-center gap-1 rounded-full px-2.5 py-1 text-[10px] font-black uppercase tracking-wider"
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
        <div className="flex-1 overflow-x-auto pb-2">
          <div className="grid h-full min-w-[1300px] grid-cols-6 gap-4 xl:min-w-0">
            {LEAD_STATUSES.map((status) => (
              <PipelineColumn
                key={status}
                status={status}
                count={stats.byStatus[status] ?? 0}
                leads={byStatus[status]}
                studioId={studioId}
              />
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

// ─────────────────────────────────────────────────────
const STATUS_STYLES: Record<LeadStatus, {
  bg: string;
  border: string;
  pill: string;
  pillText: string;
  avatarRing: string;
}> = {
  new: {
    bg: 'bg-sky-500/5 dark:bg-sky-950/20',
    border: 'border-sky-500/20 dark:border-sky-500/10',
    pill: 'bg-sky-500/10 dark:bg-sky-400/10',
    pillText: 'text-sky-600 dark:text-sky-400',
    avatarRing: 'border-sky-500/30 dark:border-sky-400/20',
  },
  contacted: {
    bg: 'bg-violet-500/5 dark:bg-violet-950/20',
    border: 'border-violet-500/20 dark:border-violet-500/10',
    pill: 'bg-violet-500/10 dark:bg-violet-400/10',
    pillText: 'text-violet-600 dark:text-violet-400',
    avatarRing: 'border-violet-500/25 dark:border-violet-400/20',
  },
  trial_booked: {
    bg: 'bg-amber-500/5 dark:bg-amber-950/20',
    border: 'border-amber-500/20 dark:border-amber-500/10',
    pill: 'bg-amber-500/10 dark:bg-amber-400/10',
    pillText: 'text-amber-600 dark:text-amber-400',
    avatarRing: 'border-amber-500/30 dark:border-amber-400/20',
  },
  member: {
    bg: 'bg-emerald-500/5 dark:bg-emerald-950/20',
    border: 'border-emerald-500/20 dark:border-emerald-500/10',
    pill: 'bg-emerald-500/10 dark:bg-emerald-400/10',
    pillText: 'text-emerald-600 dark:text-emerald-400',
    avatarRing: 'border-emerald-500/25 dark:border-emerald-400/20',
  },
  dropped: {
    bg: 'bg-slate-500/5 dark:bg-slate-950/20',
    border: 'border-slate-500/20 dark:border-slate-500/10',
    pill: 'bg-slate-500/10 dark:bg-slate-400/10',
    pillText: 'text-slate-600 dark:text-slate-400',
    avatarRing: 'border-slate-500/25 dark:border-slate-400/20',
  },
  paused: {
    bg: 'bg-indigo-500/5 dark:bg-indigo-950/20',
    border: 'border-indigo-500/20 dark:border-indigo-500/10',
    pill: 'bg-indigo-500/10 dark:bg-indigo-400/10',
    pillText: 'text-indigo-600 dark:text-indigo-400',
    avatarRing: 'border-indigo-500/25 dark:border-indigo-400/20',
  },
};

// Column
// ─────────────────────────────────────────────────────

function PipelineColumn({
  status, count, leads, studioId,
}: {
  status: LeadStatus;
  count: number;
  leads: Lead[];
  studioId: string;
}) {
  const cfg = COLUMN_CONFIG[status];
  const overflow = count - leads.length;

  return (
    <section
      className={cn(
        "flex h-full flex-col overflow-hidden rounded-[20px] backdrop-blur-2xl border",
        STATUS_STYLES[status].bg,
        STATUS_STYLES[status].border
      )}
      style={{
        boxShadow: `inset 0 0 0 1px rgba(255,255,255,0.10), 0 4px 20px rgba(0,0,0,0.04)`,
      }}
      aria-label={LEAD_STATUS_LABELS[status]}
    >
      {/* Gradient top bar */}
      <div
        className="h-1 w-full shrink-0"
        style={{ background: `linear-gradient(90deg, ${cfg.color} 0%, ${cfg.color}70 100%)` }}
      />

      {/* Column header */}
      <header className="flex shrink-0 items-center justify-between gap-2 px-4 py-3">
        <div className="flex items-center gap-2">
          <span
            className="h-2 w-2 rounded-full"
            style={{ background: cfg.color, boxShadow: `0 0 0 3px ${cfg.glow}` }}
          />
          <h3 className={cn("text-xs font-black uppercase tracking-[0.14em]", STATUS_STYLES[status].pillText)}>
            {LEAD_STATUS_LABELS[status]}
          </h3>
        </div>
        <span
          className={cn(
            "rounded-full px-2 py-0.5 text-[11px] font-black tabular-nums",
            STATUS_STYLES[status].pill,
            STATUS_STYLES[status].pillText
          )}
        >
          {count}
        </span>
      </header>

      {/* Cards scroll area */}
      <div className="flex flex-1 flex-col gap-2 overflow-y-auto no-scrollbar px-2.5 pb-3">
        {leads.length === 0 ? (
          <div
            className={cn(
              "flex flex-1 items-center justify-center rounded-xl border-2 border-dashed py-8 text-center",
              STATUS_STYLES[status].border,
              "text-slate-400"
            )}
          >
            <div>
              <div
                className={cn(
                  "mx-auto mb-2 grid h-8 w-8 place-items-center rounded-xl",
                  STATUS_STYLES[status].pill
                )}
              >
                <Inbox className={cn("h-4 w-4", STATUS_STYLES[status].pillText)} />
              </div>
              <p className="text-[11px] font-semibold">No leads yet</p>
            </div>
          </div>
        ) : (
          <>
            {leads.map((l) => (
              <LeadCard key={l.id} lead={l} studioId={studioId} cfg={cfg} />
            ))}
            {overflow > 0 && (
              <Link
                href={`/admin/studios/${studioId}/leads?status=${status}`}
                className={cn(
                  "mt-1 inline-flex items-center justify-center gap-1.5 rounded-2xl border border-white/40 bg-white/40 py-2.5 text-xs font-black backdrop-blur-sm transition-all hover:bg-white/60 dark:border-white/10 dark:bg-white/5",
                  STATUS_STYLES[status].pillText
                )}
              >
                +{overflow} more
                <ArrowRight className="h-3 w-3" />
              </Link>
            )}
          </>
        )}
      </div>
    </section>
  );
}

// ─────────────────────────────────────────────────────
// Lead card
// ─────────────────────────────────────────────────────

function LeadCard({
  lead, studioId, cfg,
}: {
  lead: Lead;
  studioId: string;
  cfg: typeof COLUMN_CONFIG[LeadStatus];
}) {
  const av = avatarColor(lead.name);

  return (
    <Link
      href={`/admin/studios/${studioId}/leads/${lead.id}`}
      className="group block rounded-[16px] p-3 backdrop-blur-xl transition-all duration-300 hover:-translate-y-0.5 hover:shadow-md border bg-white/70 border-white/45 dark:bg-neutral-900/30 dark:border-white/5"
      style={{
        boxShadow: 'inset 0 0 0 1px rgba(255,255,255,0.15), 0 2px 8px rgba(0,0,0,0.04)',
      }}
    >
      {/* Avatar + name row */}
      <div className="flex items-center gap-2.5">
        <span
          className={cn(
            "grid h-8 w-8 shrink-0 place-items-center rounded-xl text-[11px] font-black text-white shadow-sm border-2",
            STATUS_STYLES[lead.status].avatarRing
          )}
          style={{ background: av }}
          aria-hidden
        >
          {brandInitials(lead.name)}
        </span>
        <div className="min-w-0 flex-1">
          <div className="truncate text-[13px] font-bold leading-tight text-zinc-900 transition-colors group-hover:text-brand-600 dark:text-zinc-100">
            {lead.name}
          </div>
          <div className="truncate text-[10px] font-semibold leading-tight text-zinc-400">
            {lead.email}
          </div>
        </div>
      </div>

      {/* Plan + time row */}
      <div className="mt-2.5 flex items-center justify-between gap-2">
        <span
          className={cn(
            "inline-flex max-w-[70%] truncate rounded-full px-2 py-0.5 text-[10px] font-bold",
            STATUS_STYLES[lead.status].pill,
            STATUS_STYLES[lead.status].pillText
          )}
        >
          {lead.fitnessPlan}
        </span>
        <span
          className="shrink-0 text-[10px] font-semibold uppercase tracking-wider text-zinc-400"
          suppressHydrationWarning
        >
          {relativeTime(lead.createdAt)}
        </span>
      </div>

      {/* Notes preview */}
      {lead.notes && (
        <div
          className="mt-2.5 flex items-start gap-1.5 border-t pt-2 text-[10px] leading-snug text-zinc-500 border-zinc-200 dark:border-white/5"
        >
          <MessageSquareText className="mt-px h-3 w-3 shrink-0 text-zinc-300" />
          <span className="line-clamp-2">{lead.notes.split('\n').filter(Boolean).slice(0, 2).join(' · ')}</span>
        </div>
      )}
    </Link>
  );
}
