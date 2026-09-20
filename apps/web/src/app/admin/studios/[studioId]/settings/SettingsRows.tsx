'use client';

import { useState } from 'react';
import { Check, Copy, Pencil, X, Loader2, Upload } from 'lucide-react';
import { Input } from '@/components/ui/Input';

// Shared layout for one field row inside a settings card: label + description
// on the left, current value / control on the right. Cards stack rows with a
// divider (`divide-y`) rather than each row drawing its own border.
export function SettingsRow({
  label,
  description,
  children,
}: {
  label: string;
  description?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-3 px-5 py-4 sm:flex-row sm:items-center sm:justify-between">
      <div className="min-w-0 sm:max-w-[55%]">
        <div className="text-sm font-bold text-zinc-800 dark:text-zinc-100">{label}</div>
        {description && <p className="mt-0.5 text-xs text-zinc-400">{description}</p>}
      </div>
      <div className="sm:shrink-0 sm:text-right">{children}</div>
    </div>
  );
}

export function SettingsCard({ title, children }: { title?: string; children: React.ReactNode }) {
  return (
    <div>
      {title && <h4 className="mb-2 text-xs font-bold uppercase tracking-wide text-zinc-400">{title}</h4>}
      <div className="divide-y divide-zinc-100 rounded-xl border border-zinc-200 bg-white dark:divide-zinc-800 dark:border-zinc-800 dark:bg-zinc-950">
        {children}
      </div>
    </div>
  );
}

// Read-only value shown in a monospace pill with a copy button — for
// immutable identifiers (slug, ID) that can never be edited here.
export function CopyRow({ label, description, value }: { label: string; description?: string; value: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <SettingsRow label={label} description={description}>
      <div className="flex items-center gap-1.5">
        <code className="max-w-[220px] truncate rounded-lg bg-zinc-100 px-2.5 py-1.5 text-xs text-zinc-700 dark:bg-zinc-900 dark:text-zinc-300">
          {value}
        </code>
        <button
          type="button"
          onClick={async () => {
            await navigator.clipboard.writeText(value);
            setCopied(true);
            setTimeout(() => setCopied(false), 1500);
          }}
          className="shrink-0 rounded-lg p-1.5 text-zinc-400 transition-colors hover:bg-zinc-100 hover:text-zinc-700 dark:hover:bg-zinc-900 dark:hover:text-zinc-200"
          aria-label={`Copy ${label}`}
        >
          {copied ? <Check className="h-3.5 w-3.5 text-emerald-500" /> : <Copy className="h-3.5 w-3.5" />}
        </button>
      </div>
    </SettingsRow>
  );
}

// A read-only row with no edit affordance at all (e.g. a computed status).
export function StaticRow({ label, description, children }: { label: string; description?: string; children: React.ReactNode }) {
  return <SettingsRow label={label} description={description}>{children}</SettingsRow>;
}

// Click-the-pencil-to-edit text field — shows the value as plain text until
// the admin opts into editing it, matching the reference row interaction
// instead of an always-open input. Pass `multiline` for a small textarea.
export function EditableTextRow({
  label,
  description,
  value,
  placeholder = 'Not set',
  multiline = false,
  disabled = false,
  disabledHint,
  onSave,
}: {
  label: string;
  description?: string;
  value: string;
  placeholder?: string;
  multiline?: boolean;
  disabled?: boolean;
  disabledHint?: string;
  onSave: (next: string) => Promise<{ ok: boolean; error?: string }>;
}) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(value);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function save() {
    if (draft === value) {
      setEditing(false);
      return;
    }
    setSaving(true);
    setError(null);
    const res = await onSave(draft);
    setSaving(false);
    if (!res.ok) {
      setError(res.error ?? 'Failed to save');
      return;
    }
    setEditing(false);
  }

  if (editing) {
    const inputEl = multiline ? (
      <textarea
        autoFocus
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Escape') {
            setDraft(value);
            setEditing(false);
          }
        }}
        rows={3}
        className="w-full min-w-[220px] rounded border border-zinc-200 bg-white px-3 py-2 text-sm text-zinc-900 focus-visible:outline-none focus-visible:border-[color:var(--brand,#7c3aed)] focus-visible:ring-1 focus-visible:ring-[color:var(--brand,#7c3aed)] dark:border-zinc-800 dark:bg-zinc-950 dark:text-zinc-100"
      />
    ) : (
      <Input
        autoFocus
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter') save();
          if (e.key === 'Escape') {
            setDraft(value);
            setEditing(false);
          }
        }}
        className="h-8 w-48 text-sm"
      />
    );
    return (
      <SettingsRow label={label} description={description}>
        <div className="flex items-start gap-1.5">
          {inputEl}
          <button
            type="button"
            onClick={save}
            disabled={saving}
            className="mt-0.5 shrink-0 rounded-lg p-1.5 text-emerald-600 transition-colors hover:bg-emerald-50 disabled:opacity-50 dark:text-emerald-400 dark:hover:bg-emerald-500/10"
            aria-label="Save"
          >
            {saving ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Check className="h-3.5 w-3.5" />}
          </button>
          <button
            type="button"
            onClick={() => {
              setDraft(value);
              setEditing(false);
            }}
            disabled={saving}
            className="mt-0.5 shrink-0 rounded-lg p-1.5 text-zinc-400 transition-colors hover:bg-zinc-100 hover:text-zinc-700 dark:hover:bg-zinc-900"
            aria-label="Cancel"
          >
            <X className="h-3.5 w-3.5" />
          </button>
        </div>
        {error && <p className="mt-1 text-xs font-medium text-red-500">{error}</p>}
      </SettingsRow>
    );
  }

  return (
    <SettingsRow label={label} description={disabled && disabledHint ? disabledHint : description}>
      <div className="flex items-center gap-1.5">
        <span className={`max-w-[260px] truncate text-sm ${value ? 'text-zinc-800 dark:text-zinc-100' : 'italic text-zinc-400'}`}>
          {value || placeholder}
        </span>
        <button
          type="button"
          onClick={() => setEditing(true)}
          disabled={disabled}
          className="shrink-0 rounded-lg p-1.5 text-zinc-400 transition-colors hover:bg-zinc-100 hover:text-zinc-700 disabled:pointer-events-none disabled:opacity-30 dark:hover:bg-zinc-900 dark:hover:text-zinc-200"
          aria-label={`Edit ${label}`}
        >
          <Pencil className="h-3.5 w-3.5" />
        </button>
      </div>
    </SettingsRow>
  );
}

// Same click-to-edit interaction, but for a masked secret (API key, client
// secret) — never shows the real value, just whether one is set.
export function EditableSecretRow({
  label,
  description,
  hasValue,
  onSave,
}: {
  label: string;
  description?: string;
  hasValue: boolean;
  onSave: (next: string) => Promise<{ ok: boolean; error?: string }>;
}) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState('');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [justSaved, setJustSaved] = useState(false);

  async function save() {
    if (!draft) {
      setEditing(false);
      return;
    }
    setSaving(true);
    setError(null);
    const res = await onSave(draft);
    setSaving(false);
    if (!res.ok) {
      setError(res.error ?? 'Failed to save');
      return;
    }
    setDraft('');
    setEditing(false);
    setJustSaved(true);
  }

  if (editing) {
    return (
      <SettingsRow label={label} description={description}>
        <div className="flex items-center gap-1.5">
          <Input
            autoFocus
            type="password"
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            placeholder="Paste new value…"
            onKeyDown={(e) => {
              if (e.key === 'Enter') save();
              if (e.key === 'Escape') setEditing(false);
            }}
            className="h-8 w-48 text-sm"
          />
          <button
            type="button"
            onClick={save}
            disabled={saving}
            className="shrink-0 rounded-lg p-1.5 text-emerald-600 transition-colors hover:bg-emerald-50 disabled:opacity-50 dark:text-emerald-400 dark:hover:bg-emerald-500/10"
            aria-label="Save"
          >
            {saving ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Check className="h-3.5 w-3.5" />}
          </button>
          <button
            type="button"
            onClick={() => setEditing(false)}
            disabled={saving}
            className="shrink-0 rounded-lg p-1.5 text-zinc-400 transition-colors hover:bg-zinc-100 hover:text-zinc-700 dark:hover:bg-zinc-900"
            aria-label="Cancel"
          >
            <X className="h-3.5 w-3.5" />
          </button>
        </div>
        {error && <p className="mt-1 text-xs font-medium text-red-500">{error}</p>}
      </SettingsRow>
    );
  }

  return (
    <SettingsRow label={label} description={description}>
      <div className="flex items-center gap-1.5">
        {justSaved ? (
          <span className="text-xs font-semibold text-emerald-600 dark:text-emerald-400">Saved</span>
        ) : (
          <span className={`text-sm ${hasValue ? 'text-zinc-800 dark:text-zinc-100' : 'italic text-zinc-400'}`}>
            {hasValue ? '••••••••••••' : 'Not set'}
          </span>
        )}
        <button
          type="button"
          onClick={() => setEditing(true)}
          className="shrink-0 rounded-lg p-1.5 text-zinc-400 transition-colors hover:bg-zinc-100 hover:text-zinc-700 dark:hover:bg-zinc-900 dark:hover:text-zinc-200"
          aria-label={`Edit ${label}`}
        >
          <Pencil className="h-3.5 w-3.5" />
        </button>
      </div>
    </SettingsRow>
  );
}

// Color swatch + hex value, click the swatch/pencil to edit both the native
// color picker and the hex text together.
export function ColorEditRow({
  label,
  description,
  value,
  onSave,
}: {
  label: string;
  description?: string;
  value: string;
  onSave: (next: string) => Promise<{ ok: boolean; error?: string }>;
}) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(value);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function save() {
    if (draft === value) {
      setEditing(false);
      return;
    }
    setSaving(true);
    setError(null);
    const res = await onSave(draft);
    setSaving(false);
    if (!res.ok) {
      setError(res.error ?? 'Failed to save');
      return;
    }
    setEditing(false);
  }

  if (editing) {
    return (
      <SettingsRow label={label} description={description}>
        <div className="flex items-center gap-1.5">
          <div className="relative h-8 w-9 shrink-0 overflow-hidden rounded-lg border border-zinc-200 dark:border-zinc-700">
            <input
              type="color"
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              className="absolute -inset-1 h-10 w-11 cursor-pointer border-0 bg-transparent p-0"
            />
          </div>
          <Input value={draft} onChange={(e) => setDraft(e.target.value)} className="h-8 w-28 font-mono text-xs" />
          <button
            type="button"
            onClick={save}
            disabled={saving}
            className="shrink-0 rounded-lg p-1.5 text-emerald-600 transition-colors hover:bg-emerald-50 disabled:opacity-50 dark:text-emerald-400 dark:hover:bg-emerald-500/10"
            aria-label="Save"
          >
            {saving ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Check className="h-3.5 w-3.5" />}
          </button>
          <button
            type="button"
            onClick={() => {
              setDraft(value);
              setEditing(false);
            }}
            disabled={saving}
            className="shrink-0 rounded-lg p-1.5 text-zinc-400 transition-colors hover:bg-zinc-100 hover:text-zinc-700 dark:hover:bg-zinc-900"
            aria-label="Cancel"
          >
            <X className="h-3.5 w-3.5" />
          </button>
        </div>
        {error && <p className="mt-1 text-xs font-medium text-red-500">{error}</p>}
      </SettingsRow>
    );
  }

  return (
    <SettingsRow label={label} description={description}>
      <div className="flex items-center gap-1.5">
        <span className="h-5 w-5 shrink-0 rounded-full border border-zinc-200 dark:border-zinc-700" style={{ background: value }} />
        <span className="font-mono text-sm text-zinc-800 dark:text-zinc-100">{value}</span>
        <button
          type="button"
          onClick={() => setEditing(true)}
          className="shrink-0 rounded-lg p-1.5 text-zinc-400 transition-colors hover:bg-zinc-100 hover:text-zinc-700 dark:hover:bg-zinc-900 dark:hover:text-zinc-200"
          aria-label={`Edit ${label}`}
        >
          <Pencil className="h-3.5 w-3.5" />
        </button>
      </div>
    </SettingsRow>
  );
}

// Image thumbnail + URL editor with an inline upload option.
export function ImageEditRow({
  label,
  description,
  value,
  onSave,
  onUpload,
  aspectWide = false,
}: {
  label: string;
  description?: string;
  value: string;
  onSave: (next: string) => Promise<{ ok: boolean; error?: string }>;
  onUpload: (file: File) => Promise<string>;
  aspectWide?: boolean;
}) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(value);
  const [saving, setSaving] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function save(next = draft) {
    if (next === value) {
      setEditing(false);
      return;
    }
    setSaving(true);
    setError(null);
    const res = await onSave(next);
    setSaving(false);
    if (!res.ok) {
      setError(res.error ?? 'Failed to save');
      return;
    }
    setEditing(false);
  }

  if (editing) {
    return (
      <SettingsRow label={label} description={description}>
        <div className="flex flex-col items-end gap-1.5">
          <div className="flex items-center gap-1.5">
            <Input value={draft} onChange={(e) => setDraft(e.target.value)} placeholder="https://…" className="h-8 w-48 text-xs" />
            <label className="shrink-0 cursor-pointer rounded-lg border border-zinc-200 p-1.5 text-zinc-500 transition-colors hover:bg-zinc-100 dark:border-zinc-700 dark:hover:bg-zinc-900">
              <input
                type="file"
                accept="image/jpeg,image/png,image/webp,image/gif"
                className="hidden"
                disabled={uploading}
                onChange={async (e) => {
                  const file = e.target.files?.[0];
                  if (!file) return;
                  setUploading(true);
                  try {
                    const url = await onUpload(file);
                    setDraft(url);
                    await save(url);
                  } finally {
                    setUploading(false);
                  }
                }}
              />
              {uploading ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Upload className="h-3.5 w-3.5" />}
            </label>
            <button
              type="button"
              onClick={() => save()}
              disabled={saving}
              className="shrink-0 rounded-lg p-1.5 text-emerald-600 transition-colors hover:bg-emerald-50 disabled:opacity-50 dark:text-emerald-400 dark:hover:bg-emerald-500/10"
              aria-label="Save"
            >
              {saving ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Check className="h-3.5 w-3.5" />}
            </button>
            <button
              type="button"
              onClick={() => {
                setDraft(value);
                setEditing(false);
              }}
              disabled={saving}
              className="shrink-0 rounded-lg p-1.5 text-zinc-400 transition-colors hover:bg-zinc-100 hover:text-zinc-700 dark:hover:bg-zinc-900"
              aria-label="Cancel"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          </div>
          {error && <p className="text-xs font-medium text-red-500">{error}</p>}
        </div>
      </SettingsRow>
    );
  }

  return (
    <SettingsRow label={label} description={description}>
      <div className="flex items-center gap-2">
        <div
          className={`grid shrink-0 place-items-center overflow-hidden rounded-lg border border-zinc-200 bg-zinc-50 dark:border-zinc-700 dark:bg-zinc-900 ${
            aspectWide ? 'h-9 w-16' : 'h-9 w-9'
          }`}
        >
          {value ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img src={value} alt="" className="h-full w-full object-cover" />
          ) : (
            <span className="text-[9px] font-bold uppercase text-zinc-400">None</span>
          )}
        </div>
        <button
          type="button"
          onClick={() => setEditing(true)}
          className="shrink-0 rounded-lg p-1.5 text-zinc-400 transition-colors hover:bg-zinc-100 hover:text-zinc-700 dark:hover:bg-zinc-900 dark:hover:text-zinc-200"
          aria-label={`Edit ${label}`}
        >
          <Pencil className="h-3.5 w-3.5" />
        </button>
      </div>
    </SettingsRow>
  );
}

// A row whose "value" is a toggle switch rather than text.
export function ToggleRow({
  label,
  description,
  checked,
  disabled = false,
  disabledHint,
  onSave,
}: {
  label: string;
  description?: string;
  checked: boolean;
  disabled?: boolean;
  disabledHint?: string;
  onSave: (next: boolean) => Promise<{ ok: boolean; error?: string }>;
}) {
  const [saving, setSaving] = useState(false);
  const [current, setCurrent] = useState(checked);
  const [error, setError] = useState<string | null>(null);

  async function toggle() {
    const next = !current;
    setSaving(true);
    setError(null);
    const res = await onSave(next);
    setSaving(false);
    if (!res.ok) {
      setError(res.error ?? 'Failed to save');
      return;
    }
    setCurrent(next);
  }

  return (
    <SettingsRow label={label} description={disabled && disabledHint ? disabledHint : description}>
      <button
        type="button"
        onClick={toggle}
        disabled={saving || disabled}
        className={`relative h-6 w-11 shrink-0 rounded-full transition-colors disabled:cursor-not-allowed disabled:opacity-30 ${
          current ? 'bg-[var(--brand,#7c3aed)]' : 'bg-zinc-200 dark:bg-zinc-700'
        }`}
        aria-pressed={current}
        aria-label={label}
      >
        <span
          className={`absolute top-0.5 left-0.5 h-5 w-5 rounded-full bg-white shadow transition-transform ${
            current ? 'translate-x-[22px]' : 'translate-x-0'
          }`}
        />
      </button>
      {error && <p className="mt-1 text-xs font-medium text-red-500">{error}</p>}
    </SettingsRow>
  );
}
