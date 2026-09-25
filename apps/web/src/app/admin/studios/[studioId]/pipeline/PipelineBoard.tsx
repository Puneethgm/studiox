'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
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
import { ArrowRight, Inbox, MessageSquareText, AlertCircle, Snowflake, Loader2, CheckCircle2 } from 'lucide-react';
import { brandInitials } from '@/lib/color';
import { cn } from '@/lib/cn';
import { relativeTime } from '@/lib/datetime';
import { api } from '@/lib/api';
import type { ColdLead, Lead, LeadStatus } from '@/lib/types';
import { LEAD_STATUSES, LEAD_STATUS_LABELS } from '@/lib/types';
import { updatePipelineStatus, movePipelineColdLead } from '../leads/actions';

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
  coldLeads: initialColdLeads,
}: {
  studioId: string;
  initialByStatus: Record<LeadStatus, Lead[]>;
  counts: Record<LeadStatus, number>;
  overflowCounts: Record<LeadStatus, number>;
  coldLeads: ColdLead[];
}) {
  const [byStatus, setByStatus] = useState(initialByStatus);
  const [coldLeads, setColdLeads] = useState(initialColdLeads);

  // router.refresh() re-renders this already-mounted client component with
  // fresh server props — it does NOT remount it, so useState's initializer
  // above only ever runs once. Without this sync, every AutoRefresh tick and
  // every post-drag router.refresh() silently no-ops: the board keeps
  // showing whatever local optimistic edits happened, never the real
  // server-fetched state (e.g. Cold count staying frozen after a move).
  useEffect(() => {
    setByStatus(initialByStatus);
  }, [initialByStatus]);

  useEffect(() => {
    setColdLeads(initialColdLeads);
  }, [initialColdLeads]);

  const [activeLead, setActiveLead] = useState<Lead | null>(null);
  const [activeColdLead, setActiveColdLead] = useState<ColdLead | null>(null);
  const [pending, setPending] = useState<Set<string>>(new Set());
  const [error, setError] = useState<string | null>(null);
  // Paused is rarely used day-to-day — when nothing's actually paused, that
  // column slot shows Cold Leads instead of an empty "no leads yet" column.
  const showPausedColumn = (counts.paused ?? 0) > 0;
  const router = useRouter();

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 6 } }),
  );

  function handleDragStart(event: DragStartEvent) {
    setError(null);
    const lead = event.active.data.current?.lead as Lead | undefined;
    const coldLead = event.active.data.current?.coldLead as ColdLead | undefined;
    setActiveLead(lead ?? null);
    setActiveColdLead(coldLead ?? null);
  }

  async function handleDragEnd(event: DragEndEvent) {
    const lead = event.active.data.current?.lead as Lead | undefined;
    const coldLead = event.active.data.current?.coldLead as ColdLead | undefined;
    const targetStatus = event.over?.id as LeadStatus | undefined;
    setActiveLead(null);
    setActiveColdLead(null);

    if (coldLead) {
      if (!targetStatus) return;
      // Optimistic remove from the Cold column immediately. The lead being
      // created server-side means we don't have a full Lead object to drop
      // straight into the target column (email/fitnessPlan/createdAt/etc.
      // aren't known client-side) — router.refresh() re-runs the page's
      // server fetch right after the move completes instead, landing it in
      // the target column within one round trip rather than waiting up to
      // 4s for AutoRefresh's next tick.
      setColdLeads((prev) => prev.filter((l) => l.conversationId !== coldLead.conversationId));
      const res = await movePipelineColdLead(studioId, coldLead.conversationId, targetStatus);
      if (!res.ok) {
        setColdLeads((prev) => [coldLead, ...prev]);
        setError(`Couldn't move ${coldLead.name || 'contact'}: ${res.error}`);
        return;
      }
      router.refresh();
      return;
    }

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
          {LEAD_STATUSES.map((status) =>
            status === 'paused' && !showPausedColumn ? (
              <ColdColumn key="cold" studioId={studioId} leads={coldLeads} onRemove={(id) => setColdLeads((prev) => prev.filter((l) => l.conversationId !== id))} />
            ) : (
              <PipelineColumn
                key={status}
                status={status}
                count={counts[status] ?? 0}
                leads={byStatus[status]}
                overflow={overflowCounts[status] ?? 0}
                studioId={studioId}
                pending={pending}
              />
            ),
          )}
        </div>
      </div>
      <DragOverlay>
        {activeLead ? (
          <LeadCardVisual
            lead={activeLead}
            cfg={COLUMN_CONFIG[activeLead.status]}
            dragging
          />
        ) : activeColdLead ? (
          <ColdLeadCardVisual lead={activeColdLead} dragging />
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
// Cold Leads column — takes the Paused slot when nothing's actually
// paused. Conversations that have gone dead (never replied, or replied
// once then stalled 7+ days with no progress), including WhatsApp Web
// history backfilled before this platform was ever connected — those have
// no lead at all, so this list is keyed by conversationId, not lead id.
// ─────────────────────────────────────────────────────

const COLD_COLOR = '#f59e0b';

function ColdColumn({
  studioId, leads, onRemove,
}: {
  studioId: string;
  leads: ColdLead[];
  onRemove: (conversationId: string) => void;
}) {
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [sending, setSending] = useState(false);
  const [result, setResult] = useState<string | null>(null);

  function toggleOne(conversationId: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(conversationId)) next.delete(conversationId);
      else next.add(conversationId);
      return next;
    });
  }

  async function reEngage() {
    if (selected.size === 0) return;
    setSending(true);
    setResult(null);
    try {
      const res = await api<{ sent: number }>(`/api/v1/studios/${studioId}/messaging/leads/cold/re-engage`, {
        method: 'POST',
        json: { conversationIds: Array.from(selected) },
      });
      selected.forEach((id) => onRemove(id));
      setSelected(new Set());
      setResult(`Sent to ${res.sent}.`);
    } catch (e) {
      setResult(e instanceof Error ? e.message : 'Failed to re-engage');
    } finally {
      setSending(false);
    }
  }

  return (
    <section
      className="flex h-full flex-col overflow-hidden rounded-[20px] backdrop-blur-2xl border bg-amber-500/5 dark:bg-amber-950/20 border-amber-500/20 dark:border-amber-500/10"
      style={{ boxShadow: 'inset 0 0 0 1px rgba(255,255,255,0.10), 0 4px 20px rgba(0,0,0,0.04)' }}
      aria-label="Cold Leads"
    >
      <div className="h-1 w-full shrink-0" style={{ background: `linear-gradient(90deg, ${COLD_COLOR} 0%, ${COLD_COLOR}70 100%)` }} />

      <header className="flex shrink-0 items-center justify-between gap-2 px-4 py-3">
        <div className="flex items-center gap-2">
          <Snowflake className="h-3.5 w-3.5 text-amber-500" />
          <h3 className="text-xs font-black uppercase tracking-[0.14em] text-amber-600 dark:text-amber-400">Cold</h3>
        </div>
        <span className="rounded-full bg-amber-500/10 px-2 py-0.5 text-[11px] font-black tabular-nums text-amber-600 dark:text-amber-400">
          {leads.length}
        </span>
      </header>

      {leads.length > 0 && (
        <div className="flex shrink-0 items-center justify-between gap-2 px-2.5 pb-2">
          <button
            type="button"
            onClick={() => setSelected(selected.size === leads.length ? new Set() : new Set(leads.map((l) => l.conversationId)))}
            className="text-[10px] font-bold text-amber-600 hover:text-amber-700 dark:text-amber-400"
          >
            {selected.size === leads.length ? 'Deselect All' : 'Select All'}
          </button>
          <button
            type="button"
            onClick={reEngage}
            disabled={selected.size === 0 || sending}
            className="flex items-center gap-1 rounded-full bg-amber-500 px-2.5 py-1 text-[10px] font-bold text-white disabled:opacity-50"
          >
            {sending && <Loader2 className="h-3 w-3 animate-spin" />}
            Re-engage {selected.size > 0 ? `(${selected.size})` : ''}
          </button>
        </div>
      )}

      <div className="flex flex-1 flex-col gap-1.5 overflow-y-auto no-scrollbar px-2.5 pb-3">
        {leads.length === 0 ? (
          <div className="flex flex-1 items-center justify-center rounded-xl border-2 border-dashed border-amber-500/20 py-8 text-center text-slate-400">
            <div>
              <div className="mx-auto mb-2 grid h-8 w-8 place-items-center rounded-xl bg-amber-500/10">
                <Snowflake className="h-4 w-4 text-amber-500" />
              </div>
              <p className="text-[11px] font-semibold">Nothing cold</p>
            </div>
          </div>
        ) : (
          <>
            {leads.map((lead) => (
              <ColdLeadCard
                key={lead.conversationId}
                lead={lead}
                checked={selected.has(lead.conversationId)}
                onToggle={() => toggleOne(lead.conversationId)}
              />
            ))}
          </>
        )}
        {result && (
          <div className="flex items-center gap-1.5 px-1 text-[10px] font-semibold text-emerald-600 dark:text-emerald-400">
            <CheckCircle2 className="h-3 w-3 shrink-0" />
            {result}
          </div>
        )}
      </div>
    </section>
  );
}

// Draggable wrapper — same drag-out-to-another-column support as a normal
// lead card. The checkbox still works for click-to-select because dnd-kit's
// PointerSensor requires a few pixels of movement before a drag activates
// (see activationConstraint above), so a plain click passes through.
function ColdLeadCard({
  lead, checked, onToggle,
}: {
  lead: ColdLead;
  checked: boolean;
  onToggle: () => void;
}) {
  const { attributes, listeners, setNodeRef, isDragging } = useDraggable({
    id: `cold-${lead.conversationId}`,
    data: { coldLead: lead },
  });

  return (
    <div ref={setNodeRef} {...listeners} {...attributes} className={cn('touch-none', isDragging && 'opacity-40')}>
      <ColdLeadCardVisual lead={lead} checked={checked} onToggle={onToggle} />
    </div>
  );
}

function ColdLeadCardVisual({
  lead, checked, onToggle, dragging,
}: {
  lead: ColdLead;
  checked?: boolean;
  onToggle?: () => void;
  dragging?: boolean;
}) {
  return (
    <label
      className={cn(
        'flex items-start gap-2 rounded-[14px] border border-amber-500/15 bg-white/60 p-2.5 backdrop-blur-xl dark:bg-neutral-900/30',
        dragging ? 'shadow-xl rotate-2' : 'cursor-pointer hover:bg-white/80 dark:hover:bg-neutral-900/50',
      )}
    >
      <input
        type="checkbox"
        checked={checked ?? false}
        onChange={onToggle}
        disabled={dragging}
        className="mt-0.5 shrink-0 rounded"
      />
      <div className="min-w-0 flex-1">
        <div className="truncate text-[12px] font-bold text-zinc-900 dark:text-zinc-100">
          {lead.name || 'Unnamed contact'}
        </div>
        <div className="truncate text-[10px] text-zinc-400">{lead.phone}</div>
        <div className="mt-1 flex flex-wrap items-center gap-1">
          {lead.status && (lead.status as LeadStatus) in LEAD_STATUS_LABELS && (
            <span
              className="rounded-full px-1.5 py-0.5 text-[8px] font-black uppercase tracking-wider"
              style={{
                background: COLUMN_CONFIG[lead.status as LeadStatus].pill,
                color: COLUMN_CONFIG[lead.status as LeadStatus].pillText,
              }}
            >
              {LEAD_STATUS_LABELS[lead.status as LeadStatus]}
            </span>
          )}
          <span
            className={cn(
              'rounded-full px-1.5 py-0.5 text-[8px] font-black uppercase tracking-wider',
              lead.reason === 'stalled'
                ? 'bg-orange-100 text-orange-600 dark:bg-orange-500/10 dark:text-orange-400'
                : 'bg-zinc-100 text-zinc-500 dark:bg-zinc-800 dark:text-zinc-400',
            )}
          >
            {lead.reason === 'stalled' ? 'Stalled' : 'Never replied'}
          </span>
          {!lead.leadId && (
            <span className="rounded-full bg-sky-100 px-1.5 py-0.5 text-[8px] font-black uppercase tracking-wider text-sky-600 dark:bg-sky-500/10 dark:text-sky-400">
              Imported
            </span>
          )}
        </div>
        {lead.lastMessageAt && (
          <div className="mt-1 text-[9px] font-semibold uppercase tracking-wider text-zinc-400">
            {relativeTime(lead.lastMessageAt)}
          </div>
        )}
      </div>
    </label>
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
