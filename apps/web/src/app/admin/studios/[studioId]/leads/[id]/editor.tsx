'use client';

import { useRouter } from 'next/navigation';
import { useState } from 'react';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { Input } from '@/components/ui/Input';
import { FieldError, Label } from '@/components/ui/Label';
import { Select } from '@/components/ui/Select';
import { Textarea } from '@/components/ui/Textarea';
import { LEAD_STATUSES, LEAD_STATUS_LABELS, type Lead, type LeadStatus } from '@/lib/types';
import { updateLeadStatus } from './actions';

export function LeadEditor({ studioId, lead }: { studioId: string; lead: Lead }) {
  const router = useRouter();
  const [status, setStatus] = useState<LeadStatus>(lead.status);
  const [notes, setNotes] = useState(lead.notes);
  const [firstName, setFirstName] = useState(lead.firstName || '');
  const [lastName, setLastName] = useState(lead.lastName || '');
  const [contactMade, setContactMade] = useState(lead.contactMade || false);
  const [hotLead, setHotLead] = useState(lead.hotLead || false);
  const [trialPurchased, setTrialPurchased] = useState(lead.trialPurchased || false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const dirty =
    status !== lead.status ||
    notes !== lead.notes ||
    firstName !== (lead.firstName || '') ||
    lastName !== (lead.lastName || '') ||
    contactMade !== (lead.contactMade || false) ||
    hotLead !== (lead.hotLead || false) ||
    trialPurchased !== (lead.trialPurchased || false);

  async function save() {
    setSaving(true);
    setError(null);
    try {
      const result = await updateLeadStatus(studioId, lead.id, {
        status,
        notes,
        firstName,
        lastName,
        contactMade,
        hotLead,
        trialPurchased,
      });
      if (!result.ok) {
        setError(result.error);
        return;
      }
      if (typeof window !== 'undefined' && window.history.length > 1) {
        router.back();
      } else {
        router.push(`/admin/studios/${studioId}/leads`);
      }
    } finally {
      setSaving(false);
    }
  }

  return (
    <Card title="Update lead details">
      <div className="space-y-4">
        <div className="grid gap-3 sm:grid-cols-2">
          <div>
            <Label htmlFor="firstName">First name</Label>
            <Input
              id="firstName"
              value={firstName}
              onChange={(e) => setFirstName(e.target.value)}
            />
          </div>
          <div>
            <Label htmlFor="lastName">Last name</Label>
            <Input
              id="lastName"
              value={lastName}
              onChange={(e) => setLastName(e.target.value)}
            />
          </div>
        </div>

        <div>
          <Label htmlFor="status">Status</Label>
          <Select
            id="status"
            value={status}
            onChange={(e) => setStatus(e.target.value as LeadStatus)}
          >
            {LEAD_STATUSES.map((s) => (
              <option key={s} value={s}>
                {LEAD_STATUS_LABELS[s]}
              </option>
            ))}
          </Select>
        </div>

        <div>
          <Label htmlFor="notes">Internal notes</Label>
          <Textarea
            id="notes"
            rows={4}
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            placeholder="Call notes, next steps, blockers…"
          />
        </div>

        <div className="border-t border-slate-100 pt-3 dark:border-slate-800/60 space-y-2">
          <span className="text-[10px] font-black uppercase tracking-[0.18em] text-zinc-400 block mb-1">Tracking Flags</span>
          
          <div className="flex items-center gap-3">
            <input
              type="checkbox"
              id="contactMade"
              checked={contactMade}
              onChange={(e) => setContactMade(e.target.checked)}
              className="h-4 w-4 rounded border-slate-300 text-[color:var(--brand,#7c3aed)] focus:ring-[color:var(--brand,#7c3aed)]"
            />
            <Label htmlFor="contactMade" className="mb-0">Contact Made</Label>
          </div>

          <div className="flex items-center gap-3">
            <input
              type="checkbox"
              id="hotLead"
              checked={hotLead}
              onChange={(e) => setHotLead(e.target.checked)}
              className="h-4 w-4 rounded border-slate-300 text-[color:var(--brand,#7c3aed)] focus:ring-[color:var(--brand,#7c3aed)]"
            />
            <Label htmlFor="hotLead" className="mb-0">🔥 Hot Lead</Label>
          </div>

          <div className="flex items-center gap-3">
            <input
              type="checkbox"
              id="trialPurchased"
              checked={trialPurchased}
              onChange={(e) => setTrialPurchased(e.target.checked)}
              className="h-4 w-4 rounded border-slate-300 text-[color:var(--brand,#7c3aed)] focus:ring-[color:var(--brand,#7c3aed)]"
            />
            <Label htmlFor="trialPurchased" className="mb-0">🎟️ Trial Purchased</Label>
          </div>
        </div>

        <FieldError message={error ?? undefined} />
        <div className="flex items-center justify-end gap-2 border-t border-slate-100 pt-4 dark:border-slate-800/60">
          <Button variant="ghost" size="sm" type="button" onClick={() => router.back()}>
            Cancel
          </Button>
          <Button onClick={save} disabled={!dirty} loading={saving} size="sm">
            Save changes
          </Button>
        </div>
      </div>
    </Card>
  );
}
