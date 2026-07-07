'use client';

import { useState } from 'react';
import Link from 'next/link';
import {
  DndContext,
  DragOverlay,
  PointerSensor,
  useSensor,
  useSensors,
  useDraggable,
  useDroppable,
  type DragEndEvent,
  type DragStartEvent,
} from '@dnd-kit/core';
import { ArrowRight, Inbox, MessageSquareText, AlertCircle } from 'lucide-react';
import { brandInitials } from '@/lib/color';
import { cn } from '@/lib/cn';
import { relativeTime } from '@/lib/datetime';
import type { Lead, LeadStatus } from '@/lib/types';
import { LEAD_STATUSES, LEAD_STATUS_LABELS } from '@/lib/types';
import { updatePipelineStatus } from '../leads/actions';

// Per-status visual config
const COLUMN_CONFIG: Record<LeadStatus, {
  color: string;
  glow: string;
  pill: string;
  pillText: string;
}> = {
  new: { color: '#0ea5e9', glow: 'rgba(14,165,233,0.12)', pill: 'rgba(14,165,233,0.12)', pillText: '#0284c7' },
  contacted: { color: '#7c3aed', glow: 'rgba(124,58,237,0.12)', pill: 'rgba(124,58,237,0.10)', pillText: '#6d28d9' },
  trial_booked: { color: '#f59e0b', glow: 'rgba(245,158,11,0.12)', pill: 'rgba(245,158,11,0.10)', pillText: '#d97706' },
  member: { color: '#10b981', glow: 'rgba(16,185,129,0.12)', pill: 'rgba(16,185,129,0.10)', pillText: '#059669' },
  dropped: { color: '#94a3b8', glow: 'rgba(148,163,184,0.10)', pill: 'rgba(148,163,184,0.10)', pillText: '#64748b' },
  paused: { color: '#6366f1', glow: 'rgba(99,102,241,0.12)', pill: 'rgba(99,102,241,0.10)', pillText: '#4f46e5' },
};

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

const COLUMN_CAP = 50;

export function PipelineBoard({
  studioId,
  initialByStatus,
  counts,
  overflowCounts,
}: {
  studioId: string;
  initialByStatus: Record<LeadStatus, Lead[]>;
  counts: Record<LeadStatus, number>;
  overflowCounts: Record<LeadStatus, number>;
}) {
  const [byStatus, setByStatus] = useState(initialByStatus);
  const [activeLead, setActiveLead] = useState<Lead | null>(null);
  const [pending, setPending] = useState<Set<string>>(new Set());
  const [error, setError] = useState<string | null>(null);

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 6 } }),
  );

  function handleDragStart(event: DragStartEvent) {
    setError(null);
    const lead = event.active.data.current?.lead as Lead | undefined;
    setActiveLead(lead ?? null);
  }

  async function handleDragEnd(event: DragEndEvent) {
    const lead = event.active.data.current?.lead as Lead | undefined;
    const targetStatus = event.over?.id as LeadStatus | undefined;
    setActiveLead(null);

    if (!lead || !targetStatus || targetStatus === lead.status) {
      return;
    }

    const fromStatus = lead.status;
    const movedLead: Lead = { ...lead, status: targetStatus };

    // Optimistic move between columns.
    setByStatus((prev) => ({
      ...prev,
      [fromStatus]: prev[fromStatus].filter((l) => l.id !== lead.id),
      [targetStatus]: [movedLead, ...prev[targetStatus]],
    }));
    setPending((prev) => new Set(prev).add(lead.id));

    const res = await updatePipelineStatus(studioId, lead.id, targetStatus);

    setPending((prev) => {
      const next = new Set(prev);
      next.delete(lead.id);
      return next;
    });

    if (!res.ok) {
      // Roll back on failure.
      setByStatus((prev) => ({
        ...prev,
        [targetStatus]: prev[targetStatus].filter((l) => l.id !== lead.id),
        [fromStatus]: [lead, ...prev[fromStatus]],
      }));
      setError(`Couldn't move ${lead.name}: ${res.error}`);
    }
  }

  return (
    <DndContext sensors={sensors} onDragStart={handleDragStart} onDragEnd={handleDragEnd}>
      {error && (
        <div className="mb-2 flex items-center gap-2 rounded-xl border border-rose-500/20 bg-rose-500/10 px-3 py-2 text-[11px] font-bold text-rose-600 dark:text-rose-400">
          <AlertCircle className="h-3.5 w-3.5 shrink-0" />
          {error}
        </div>
      )}
      <div className="flex-1 overflow-x-auto pb-2">
        <div className="grid h-full min-w-[1300px] grid-cols-6 gap-4 xl:min-w-0">
          {LEAD_STATUSES.map((status) => (
            <PipelineColumn
              key={status}
              status={status}
              count={counts[status] ?? 0}
              leads={byStatus[status]}
              overflow={overflowCounts[status] ?? 0}
              studioId={studioId}
              pending={pending}
            />
          ))}
        </div>
      </div>
      <DragOverlay>
        {activeLead ? (
          <LeadCardVisual
            lead={activeLead}
            cfg={COLUMN_CONFIG[activeLead.status]}
            dragging
          />
        ) : null}
      </DragOverlay>
    </DndContext>
  );
}

// ─────────────────────────────────────────────────────
// Column
// ─────────────────────────────────────────────────────

function PipelineColumn({
  status, count, leads, overflow, studioId, pending,
}: {
  status: LeadStatus;
  count: number;
  leads: Lead[];
  overflow: number;
  studioId: string;
  pending: Set<string>;
}) {
  const cfg = COLUMN_CONFIG[status];
  const { setNodeRef, isOver } = useDroppable({ id: status });

  return (
    <section
      ref={setNodeRef}
      className={cn(
        "flex h-full flex-col overflow-hidden rounded-[20px] backdrop-blur-2xl border transition-colors",
        STATUS_STYLES[status].bg,
        isOver ? "border-2" : STATUS_STYLES[status].border
      )}
      style={{
        boxShadow: isOver
          ? `inset 0 0 0 2px ${cfg.color}, 0 4px 20px rgba(0,0,0,0.04)`
          : `inset 0 0 0 1px rgba(255,255,255,0.10), 0 4px 20px rgba(0,0,0,0.04)`,
        borderColor: isOver ? cfg.color : undefined,
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
              <p className="text-[11px] font-semibold">
                {isOver ? 'Drop to move here' : 'No leads yet'}
              </p>
            </div>
          </div>
        ) : (
          <>
            {leads.map((l) => (
              <LeadCard key={l.id} lead={l} studioId={studioId} cfg={cfg} isPending={pending.has(l.id)} />
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
  lead, studioId, cfg, isPending,
}: {
  lead: Lead;
  studioId: string;
  cfg: typeof COLUMN_CONFIG[LeadStatus];
  isPending: boolean;
}) {
  const { attributes, listeners, setNodeRef, isDragging } = useDraggable({
    id: lead.id,
    data: { lead },
  });

  return (
    <div
      ref={setNodeRef}
      {...listeners}
      {...attributes}
      className={cn("touch-none", isDragging && "opacity-40")}
    >
      <LeadCardVisual lead={lead} cfg={cfg} studioId={studioId} isPending={isPending} />
    </div>
  );
}

function LeadCardVisual({
  lead, cfg, studioId, isPending, dragging,
}: {
  lead: Lead;
  cfg: typeof COLUMN_CONFIG[LeadStatus];
  studioId?: string;
  isPending?: boolean;
  dragging?: boolean;
}) {
  const av = avatarColor(lead.name);

  const content = (
    <div
      className={cn(
        "group block rounded-[16px] p-3 backdrop-blur-xl transition-all duration-300 border bg-white/70 border-white/45 dark:bg-neutral-900/30 dark:border-white/5",
        dragging ? "shadow-xl rotate-2" : "hover:-translate-y-0.5 hover:shadow-md cursor-grab active:cursor-grabbing",
        isPending && "opacity-60"
      )}
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
    </div>
  );

  if (dragging || !studioId) {
    return content;
  }

  return (
    <Link href={`/admin/studios/${studioId}/leads/${lead.id}`}>
      {content}
    </Link>
  );
}
