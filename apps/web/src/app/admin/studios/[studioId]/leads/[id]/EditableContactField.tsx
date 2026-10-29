'use client';

import { useState } from 'react';
import { updateLeadContact } from './actions';

// Click-to-edit Email/Phone row for the Leads page's "Contact details" card
// — same interaction as the Inbox's lead panel (click to edit, Enter/blur
// saves, Escape cancels), but backed by the server action above instead of
// a direct client-side fetch, since this page is otherwise a server
// component.
export function EditableContactField({
  studioId,
  leadId,
  label,
  field,
  value,
}: {
  studioId: string;
  leadId: string;
  label: string;
  field: 'email' | 'phone';
  value: string;
}) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(value || '');
  const [current, setCurrent] = useState(value || '');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function commit() {
    setEditing(false);
    if (draft === current) return;
    setSaving(true);
    setError(null);
    try {
      const result = await updateLeadContact(studioId, leadId, { [field]: draft });
      if (!result.ok) {
        setDraft(current);
        setError(result.error);
      } else {
        setCurrent(draft);
      }
    } finally {
      setSaving(false);
    }
  }

  return (
    <div>
      <dt className="text-[10px] font-black uppercase tracking-[0.18em] text-zinc-400">{label}</dt>
      {editing ? (
        <input
          type={field === 'email' ? 'email' : 'text'}
          autoFocus
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onBlur={commit}
          onKeyDown={(e) => {
            if (e.key === 'Enter') e.currentTarget.blur();
            if (e.key === 'Escape') {
              setDraft(current);
              setEditing(false);
            }
          }}
          className="mt-1.5 w-full rounded border border-brand-500/50 bg-white/60 px-1.5 py-0.5 text-sm font-semibold text-zinc-900 focus:outline-none dark:bg-white/10 dark:text-zinc-100"
        />
      ) : (
        <button
          type="button"
          onClick={() => setEditing(true)}
          disabled={saving}
          className="mt-1.5 block w-full break-words rounded px-0.5 py-0 text-left font-semibold text-zinc-900 transition-colors hover:bg-white/10 dark:text-zinc-100"
        >
          {saving ? 'Saving…' : current || '—'}
        </button>
      )}
      {error && <p className="mt-1 text-[11px] font-bold text-rose-500">{error}</p>}
    </div>
  );
}
