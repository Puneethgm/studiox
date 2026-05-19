'use client';

import { useRouter } from 'next/navigation';
import { useState } from 'react';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { FieldError, Label } from '@/components/ui/Label';
import { Select } from '@/components/ui/Select';
import { Textarea } from '@/components/ui/Textarea';
import { LEAD_STATUSES, LEAD_STATUS_LABELS, type Lead, type LeadStatus } from '@/lib/types';
import { updateLeadStatus } from './actions';

export function LeadEditor({ studioId, lead }: { studioId: string; lead: Lead }) {
  const router = useRouter();
  const [status, setStatus] = useState<LeadStatus>(lead.status);
  const [notes, setNotes] = useState(lead.notes);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const dirty = status !== lead.status || notes !== lead.notes;

  async function save() {
    setSaving(true);
    setError(null);
    try {
      const result = await updateLeadStatus(studioId, lead.id, { status, notes });
      if (!result.ok) {
        setError(result.error);
        return;
      }
      // The Server Action revalidated the destination's RSC cache, so the
      // pipeline / leads list / dashboard will all show the new status
      // without a manual refresh.
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
    <Card title="Update status">
      <div className="space-y-5">
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
            rows={6}
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            placeholder="Call notes, next steps, blockers…"
          />
        </div>
        <FieldError message={error ?? undefined} />
        <div className="flex items-center justify-end gap-2 border-t border-slate-100 pt-4 dark:border-slate-800/60">
          <Button variant="ghost" size="sm" type="button" onClick={() => router.back()}>
            Cancel
          </Button>
          <Button onClick={save} disabled={!dirty} loading={saving} size="sm">
            Save & back
          </Button>
        </div>
      </div>
    </Card>
  );
}
