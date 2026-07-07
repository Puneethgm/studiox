'use client';

import { useEffect, useState } from 'react';
import { BellOff, Loader2, User } from 'lucide-react';
import { api } from '@/lib/api';
import { cn } from '@/lib/cn';
import { brandInitials } from '@/lib/color';
import type { Conversation, Lead } from '@/lib/types';
import { LEAD_STATUS_LABELS } from '@/lib/types';

// Right-side "Contact Details" panel shown when a conversation is open.
// Surfaces the linked lead's core fields and a Do Not Disturb switch: when
// enabled, the backend silences all automated messaging (autocontact
// follow-ups, AI/decision-tree replies) for this lead without touching its
// pipeline status.
export function ContactDetailsPanel({
  studioId,
  conversation,
}: {
  studioId: string;
  conversation: Conversation;
}) {
  const [lead, setLead] = useState<Lead | null>(null);
  const [loading, setLoading] = useState(false);
  const [dndSaving, setDndSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const leadId = conversation.leadId;

  useEffect(() => {
    setLead(null);
    setError(null);
    if (!leadId) return;
    setLoading(true);
    api<Lead>(`/api/v1/studios/${studioId}/leads/${leadId}`)
      .then(setLead)
      .catch((err: any) => setError(err?.message || 'Failed to load contact.'))
      .finally(() => setLoading(false));
  }, [studioId, leadId]);

  async function toggleDND() {
    if (!lead || dndSaving) return;
    const nextEnabled = !lead.dndEnabled;
    setDndSaving(true);
    setError(null);
    // Optimistic update.
    setLead({ ...lead, dndEnabled: nextEnabled });
    try {
      const updated = await api<Lead>(`/api/v1/studios/${studioId}/leads/${lead.id}/dnd`, {
        method: 'PATCH',
        json: { enabled: nextEnabled },
      });
      setLead(updated);
    } catch (err: any) {
      // Roll back on failure.
      setLead(lead);
      setError(err?.message || 'Failed to update Do Not Disturb.');
    } finally {
      setDndSaving(false);
    }
  }

  return (
    <aside className="hidden w-80 shrink-0 flex-col border-l border-white/20 lg:flex bg-white/10 dark:border-white/5 dark:bg-neutral-950/10">
      <div className="flex h-14 shrink-0 items-center justify-between border-b border-white/20 px-5 dark:border-white/5">
        <h2 className="text-sm font-black uppercase tracking-[0.15em] text-violet-600 dark:text-violet-400">
          Contact Details
        </h2>
      </div>

      <div className="flex-1 overflow-y-auto no-scrollbar px-5 py-5">
        {!leadId ? (
          <div className="flex flex-col items-center gap-2 py-10 text-center">
            <div className="grid h-10 w-10 place-items-center rounded-xl bg-white/30 dark:bg-white/5">
              <User className="h-4 w-4 text-zinc-400" />
            </div>
            <p className="text-[11px] font-semibold text-zinc-400">
              No lead linked to this conversation.
            </p>
          </div>
        ) : loading ? (
          <div className="grid h-32 place-items-center">
            <Loader2 className="h-5 w-5 animate-spin text-zinc-400" />
          </div>
        ) : lead ? (
          <div className="space-y-5">
            <div className="flex items-center gap-3">
              <span
                className="grid h-11 w-11 shrink-0 place-items-center rounded-2xl bg-gradient-to-br from-brand-500 to-violet-500 text-xs font-black text-white shadow-sm"
                aria-hidden
              >
                {brandInitials(lead.name)}
              </span>
              <div className="min-w-0">
                <div className="truncate text-sm font-bold text-zinc-900 dark:text-zinc-100">
                  {lead.name}
                </div>
                <div className="truncate text-[11px] font-semibold text-zinc-400">
                  {LEAD_STATUS_LABELS[lead.status]}
                </div>
              </div>
            </div>

            {/* Do Not Disturb toggle */}
            <div className="flex items-center gap-3 rounded-2xl border border-white/20 bg-white/20 p-3 dark:border-white/5 dark:bg-white/5">
              <button
                onClick={toggleDND}
                disabled={dndSaving}
                className={cn(
                  'relative inline-flex h-6 w-11 shrink-0 cursor-pointer items-center rounded-full transition-colors duration-300 focus:outline-none focus:ring-2 focus:ring-brand-500 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50',
                  lead.dndEnabled ? 'bg-rose-500' : 'bg-zinc-300 dark:bg-zinc-700',
                )}
                role="switch"
                aria-checked={lead.dndEnabled}
                title={lead.dndEnabled ? 'Turn off Do Not Disturb' : 'Turn on Do Not Disturb'}
              >
                <span
                  className={cn(
                    'pointer-events-none flex h-5 w-5 items-center justify-center rounded-full bg-white shadow-md ring-0 transition-transform duration-300 ease-out',
                    lead.dndEnabled ? 'translate-x-5' : 'translate-x-0.5',
                  )}
                >
                  {dndSaving ? (
                    <Loader2 className="h-3 w-3 animate-spin text-zinc-500" />
                  ) : (
                    <BellOff className={cn('h-2.5 w-2.5', lead.dndEnabled ? 'text-rose-600' : 'text-zinc-400')} />
                  )}
                </span>
              </button>
              <div className="min-w-0">
                <div className="text-xs font-black uppercase tracking-wider text-zinc-700 dark:text-zinc-200">
                  Do Not Disturb
                </div>
                <p className="text-[10px] font-semibold leading-snug text-zinc-400">
                  {lead.dndEnabled
                    ? 'Automated messages are silenced for this contact.'
                    : 'Stops all automated follow-ups and AI replies.'}
                </p>
              </div>
            </div>

            {error && (
              <p className="text-[11px] font-bold text-rose-500 dark:text-rose-400">{error}</p>
            )}

            {/* Contact fields */}
            <div className="space-y-3">
              <Field label="Email" value={lead.email} />
              <Field label="Phone" value={lead.phone} />
              <Field label="Fitness Plan" value={lead.fitnessPlan} />
              <Field label="Source" value={lead.source} />
              {lead.assignedTo && <Field label="Assigned To" value={lead.assignedTo} />}
              {lead.notes && <Field label="Notes" value={lead.notes} multiline />}
            </div>
          </div>
        ) : (
          <p className="text-[11px] font-semibold text-rose-500">{error || 'Could not load contact.'}</p>
        )}
      </div>
    </aside>
  );
}

function Field({ label, value, multiline }: { label: string; value: string; multiline?: boolean }) {
  if (!value) return null;
  return (
    <div>
      <div className="text-[9px] font-black uppercase tracking-wider text-zinc-400">{label}</div>
      <div
        className={cn(
          'mt-0.5 text-xs font-semibold text-zinc-700 dark:text-zinc-200',
          multiline ? 'whitespace-pre-wrap' : 'truncate',
        )}
      >
        {value}
      </div>
    </div>
  );
}
