'use client';

import { useEffect, useState } from 'react';
import { Loader2, LayoutTemplate, ExternalLink, Pencil, Check, X } from 'lucide-react';
import type { Studio } from '@/lib/types';
import { Button } from '@/components/ui/Button';
import { Label } from '@/components/ui/Label';
import { Input } from '@/components/ui/Input';
import {
  updateStudioSettings,
  changeMyPassword,
  getWhatsAppSendSpacing,
  saveWhatsAppSendSpacing,
  getWhatsAppDailyMessageLimit,
  saveWhatsAppDailyMessageLimit,
  getGlofoxFirstSession,
  saveGlofoxFirstSession,
  getColdLeadThresholds,
  saveColdLeadThresholds,
  getInitialContactDelay,
  saveInitialContactDelay,
  getAIReplyDelay,
  saveAIReplyDelay,
  getSheetsSettings,
  saveSheetsSettings,
  listExternalLeadsSheets,
  createExternalLeadsSheet,
  updateExternalLeadsSheet,
  deleteExternalLeadsSheet,
  type ExternalLeadsSheet,
  type ExternalLeadsSheetSettingsData,
} from './actions';
import {
  SettingsCard,
  SettingsRow,
  CopyRow,
  EditableTextRow,
  EditableSecretRow,
  ColorEditRow,
  ImageEditRow,
  ToggleRow,
} from './SettingsRows';

// The Server Actions return {ok, error, details}. `error` is often just the
// generic "validation failed" — the actually useful message is the first
// entry in `details` (e.g. {spreadsheetId: "required"}). Prefer that.
function bestError(res: { error?: string; details?: Record<string, string> }): string {
  if (res.details) {
    const first = Object.entries(res.details)[0];
    if (first) return `${first[0]}: ${first[1]}`;
  }
  return res.error ?? 'Failed to save';
}

function SectionHeader({ title, description }: { title: string; description: string }) {
  return (
    <div>
      <h3 className="text-base font-bold text-zinc-900 dark:text-white">{title}</h3>
      <p className="text-sm text-zinc-400">{description}</p>
    </div>
  );
}

async function uploadImage(studioId: string, file: File): Promise<string> {
  const formData = new FormData();
  // Must be 'file' — the backend handler reads r.FormFile("file"); sending it as
  // 'image' made every upload here 400 before it ever reached S3/disk.
  formData.append('file', file);
  const res = await fetch(`/api/v1/studios/${studioId}/social-posts/upload-image`, { method: 'POST', body: formData });
  if (!res.ok) {
    const body = await res.json().catch(() => null) as { error?: string } | null;
    throw new Error(body?.error ?? 'Upload failed');
  }
  // The endpoint returns { mediaUrl }, not { url } — reading `url` here silently
  // saved "undefined" as the logo, which is the second half of this same bug.
  const { mediaUrl } = await res.json();
  return mediaUrl as string;
}

const COUNTRY_CODES = [
  { code: '+65', name: 'Singapore (+65)' },
  { code: '+1', name: 'United States/Canada (+1)' },
  { code: '+44', name: 'United Kingdom (+44)' },
  { code: '+91', name: 'India (+91)' },
  { code: '+61', name: 'Australia (+61)' },
  { code: '+64', name: 'New Zealand (+64)' },
  { code: '+60', name: 'Malaysia (+60)' },
  { code: '+852', name: 'Hong Kong (+852)' },
  { code: '+63', name: 'Philippines (+63)' },
  { code: '+971', name: 'UAE (+971)' },
];

function parsePhone(fullPhone: string) {
  if (!fullPhone) return { countryCode: '+65', phoneNumber: '' };
  for (const c of COUNTRY_CODES) {
    if (fullPhone.startsWith(c.code)) {
      return { countryCode: c.code, phoneNumber: fullPhone.slice(c.code.length) };
    }
  }
  return { countryCode: '+65', phoneNumber: fullPhone };
}

export function GeneralSection({ studio, onChange }: { studio: Studio; onChange: (s: Studio) => void }) {
  async function saveField(field: keyof Studio, value: unknown) {
    const res = await updateStudioSettings(studio.id, studio.slug, { [field]: value } as any);
    if (res.ok) onChange({ ...studio, [field]: value } as Studio);
    return res;
  }

  const initialPhone = parsePhone(studio.contactPhone || '');
  const [phoneCountryCode, setPhoneCountryCode] = useState(initialPhone.countryCode);

  return (
    <div className="space-y-6">
      <SectionHeader title="General" description="Studio identity and branding." />

      <SettingsCard title="Studio Identity">
        <EditableTextRow
          label="Studio name"
          description="Shown across the admin dashboard and public pages."
          value={studio.name}
          onSave={(v) => saveField('name', v)}
        />
        <CopyRow
          label="Studio slug"
          description="Used in the public booking link. Cannot be changed after creation."
          value={studio.slug}
        />
        <CopyRow label="Studio ID" description="Stable identifier used internally and in the API." value={studio.id} />
      </SettingsCard>

      <SettingsCard title="Contact & Trial">
        <EditableTextRow
          label="Contact email"
          value={studio.contactEmail || ''}
          onSave={(v) => saveField('contactEmail', v)}
        />
        <SettingsRow label="Contact phone">
          <ContactPhoneEditor
            countryCode={phoneCountryCode}
            phoneNumber={initialPhone.phoneNumber}
            onCountryCodeChange={setPhoneCountryCode}
            onSave={async (countryCode, phoneNumber) => {
              const cleanPhone = phoneNumber.replace(/\D/g, '');
              const fullPhone = cleanPhone ? `${countryCode}${cleanPhone}` : '';
              return saveField('contactPhone', fullPhone);
            }}
          />
        </SettingsRow>
        <EditableTextRow
          label="Trial price (S$)"
          description="Shown/charged on the trial payment page. S$0 falls back to your lowest active plan (or S$25 with none)."
          value={((studio.trialAmountSgd ?? 0) / 100).toFixed(2)}
          onSave={(v) => saveField('trialAmountSgd', Math.round((parseFloat(v) || 0) * 100))}
        />
      </SettingsCard>

      <SettingsCard title="Brand">
        <ColorEditRow
          label="Brand color"
          description="Used for the public booking page and studio-admin accents."
          value={studio.brandColor}
          onSave={(v) => saveField('brandColor', v)}
        />
        <ImageEditRow
          label="Logo"
          description="Square image works best. Shown in the sidebar and public pages."
          value={studio.logoUrl}
          onSave={(v) => saveField('logoUrl', v)}
          onUpload={(f) => uploadImage(studio.id, f)}
        />
      </SettingsCard>

      <SettingsCard title="Status">
        <ToggleRow
          label="Studio is active"
          description="Inactive studios stop accepting public form submissions."
          checked={studio.active}
          onSave={(v) => saveField('active', v)}
        />
        <ToggleRow
          label="Managed by 1Hero"
          description="Super admins can access this studio. Uncheck to restrict access."
          checked={studio.managedBy1Hero || false}
          onSave={(v) => saveField('managedBy1Hero', v)}
        />
      </SettingsCard>
    </div>
  );
}

function ContactPhoneEditor({
  countryCode,
  phoneNumber,
  onCountryCodeChange,
  onSave,
}: {
  countryCode: string;
  phoneNumber: string;
  onCountryCodeChange: (v: string) => void;
  onSave: (countryCode: string, phoneNumber: string) => Promise<{ ok: boolean; error?: string }>;
}) {
  const [editing, setEditing] = useState(false);
  const [code, setCode] = useState(countryCode);
  const [phone, setPhone] = useState(phoneNumber);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function save() {
    setSaving(true);
    setError(null);
    const res = await onSave(code, phone);
    setSaving(false);
    if (!res.ok) {
      setError(res.error ?? 'Failed to save');
      return;
    }
    onCountryCodeChange(code);
    setEditing(false);
  }

  if (editing) {
    return (
      <div>
        <div className="flex items-center gap-1.5">
          <select
            value={code}
            onChange={(e) => setCode(e.target.value)}
            className="h-8 rounded-lg border border-zinc-200 bg-white px-2 text-xs font-semibold text-zinc-800 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-100"
          >
            {COUNTRY_CODES.map((c) => (
              <option key={c.code} value={c.code}>
                {c.code}
              </option>
            ))}
          </select>
          <Input value={phone} onChange={(e) => setPhone(e.target.value)} placeholder="e.g. 81234567" className="h-8 w-32 text-sm" autoFocus />
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
            className="shrink-0 rounded-lg p-1.5 text-zinc-400 hover:bg-zinc-100 dark:hover:bg-zinc-900"
            aria-label="Cancel"
          >
            <X className="h-3.5 w-3.5" />
          </button>
        </div>
        {error && <p className="mt-1 text-xs font-medium text-red-500">{error}</p>}
      </div>
    );
  }

  return (
    <div className="flex items-center gap-1.5">
      <span className={`text-sm ${phoneNumber ? 'text-zinc-800 dark:text-zinc-100' : 'italic text-zinc-400'}`}>
        {phoneNumber ? `${countryCode} ${phoneNumber}` : 'Not set'}
      </span>
      <button
        type="button"
        onClick={() => setEditing(true)}
        className="shrink-0 rounded-lg p-1.5 text-zinc-400 transition-colors hover:bg-zinc-100 hover:text-zinc-700 dark:hover:bg-zinc-900 dark:hover:text-zinc-200"
        aria-label="Edit contact phone"
      >
        <Pencil className="h-3.5 w-3.5" />
      </button>
    </div>
  );
}

export function BookingSection({ studio }: { studio: Studio }) {
  return (
    <div className="space-y-6">
      <SectionHeader title="Trial Page" description="The customizable checkout page leads land on to book a trial." />
      <div className="rounded-xl border border-zinc-200 bg-white p-5 dark:border-zinc-800 dark:bg-zinc-950">
        <h4 className="text-sm font-bold text-zinc-900 dark:text-white">Customize the Trial Payment Page</h4>
        <p className="mt-1 text-xs text-zinc-400">
          Drag-and-drop the text, images, video, and payment form blocks shown on your public trial checkout page.
        </p>
        <div className="mt-4 flex flex-wrap gap-2">
          <a
            href={`/admin/studios/${studio.id}/settings/trial-page`}
            target="_blank"
            rel="noreferrer"
            className="inline-flex items-center gap-1.5 rounded-xl bg-[var(--brand,#7c3aed)] px-3.5 py-2 text-xs font-bold text-white transition hover:brightness-110"
          >
            <LayoutTemplate className="h-3.5 w-3.5" />
            Customize Trial Payment Page
          </a>
          <a
            href={`/trial-details/studio/${studio.slug}`}
            target="_blank"
            rel="noreferrer"
            className="inline-flex items-center gap-1.5 rounded-xl border border-zinc-200 bg-white px-3.5 py-2 text-xs font-bold text-zinc-600 transition-colors hover:bg-zinc-50 dark:border-zinc-800 dark:bg-zinc-950 dark:text-zinc-300 dark:hover:bg-zinc-900"
          >
            <ExternalLink className="h-3.5 w-3.5" />
            Preview
          </a>
        </div>
      </div>
    </div>
  );
}

export function SecuritySection({ studio }: { studio: Studio }) {
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);
  const [success, setSuccess] = useState(false);

  async function onChangePassword(e: React.FormEvent) {
    e.preventDefault();
    setErrors({});
    setSuccess(false);
    if (newPassword !== confirmPassword) {
      setErrors({ confirmPassword: 'Passwords do not match' });
      return;
    }
    setSaving(true);
    try {
      const res = await changeMyPassword({ currentPassword, newPassword, confirmPassword });
      if (!res.ok) {
        setErrors(res.details ?? { _: res.error });
        return;
      }
      setCurrentPassword('');
      setNewPassword('');
      setConfirmPassword('');
      setSuccess(true);
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="space-y-6">
      <SectionHeader title="Security" description="Account password and irreversible actions." />

      <div>
        <h4 className="mb-2 text-xs font-bold uppercase tracking-wide text-zinc-400">Change Password</h4>
        <form onSubmit={onChangePassword} className="space-y-3 rounded-xl border border-zinc-200 bg-white p-5 dark:border-zinc-800 dark:bg-zinc-950">
          <div>
            <Label htmlFor="currentPassword">Current password</Label>
            <Input
              id="currentPassword"
              type="password"
              value={currentPassword}
              onChange={(e) => setCurrentPassword(e.target.value)}
              invalid={!!errors.currentPassword}
              required
            />
          </div>
          <div>
            <Label htmlFor="newPassword">New password</Label>
            <Input
              id="newPassword"
              type="password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              invalid={!!errors.newPassword}
              required
            />
          </div>
          <div>
            <Label htmlFor="confirmPassword">Confirm new password</Label>
            <Input
              id="confirmPassword"
              type="password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              invalid={!!errors.confirmPassword}
              required
            />
            {errors.confirmPassword && <p className="mt-1 text-xs font-medium text-red-500">{errors.confirmPassword}</p>}
          </div>
          {errors._ && <p className="text-xs font-medium text-red-500">{errors._}</p>}
          {success && <p className="text-xs font-medium text-emerald-600 dark:text-emerald-400">Password updated.</p>}
          <Button type="submit" loading={saving} size="sm">
            {saving && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
            Update Password
          </Button>
        </form>
      </div>

      <DangerZone studioId={studio.id} />
    </div>
  );
}

function DangerZone({ studioId }: { studioId: string }) {
  const [confirming, setConfirming] = useState(false);
  const [confirmText, setConfirmText] = useState('');
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function onDelete() {
    setDeleting(true);
    setError(null);
    try {
      const res = await fetch(`/api/v1/me/studios/${studioId}/delete-account`, { method: 'POST' });
      if (!res.ok) {
        const body = await res.json().catch(() => null);
        setError(body?.error ?? 'Failed to delete account');
        return;
      }
      window.location.href = '/login';
    } finally {
      setDeleting(false);
    }
  }

  return (
    <div>
      <h4 className="mb-2 text-xs font-bold uppercase tracking-wide text-red-500">Danger Zone</h4>
      <div className="rounded-xl border border-red-200 bg-red-50 p-5 dark:border-red-900/40 dark:bg-red-950/10">
        <p className="text-sm font-bold text-red-600 dark:text-red-400">Delete Account</p>
        <p className="mt-1 text-xs text-red-600/70 dark:text-red-400/70">
          Permanently delete your account and all data. There is no going back.
        </p>
        {!confirming ? (
          <Button
            type="button"
            variant="outline"
            className="mt-3 border-red-300 text-red-600 hover:bg-red-100 dark:border-red-900/50 dark:text-red-400 dark:hover:bg-red-950/30"
            onClick={() => setConfirming(true)}
          >
            Delete Account
          </Button>
        ) : (
          <div className="mt-3 space-y-2">
            <Label htmlFor="confirmDelete" className="text-red-600 dark:text-red-400">
              Type DELETE to confirm
            </Label>
            <Input id="confirmDelete" value={confirmText} onChange={(e) => setConfirmText(e.target.value)} className="max-w-xs" />
            {error && <p className="text-xs font-medium text-red-500">{error}</p>}
            <div className="flex gap-2">
              <Button
                type="button"
                loading={deleting}
                disabled={confirmText !== 'DELETE'}
                className="bg-red-600 hover:bg-red-700 text-white"
                onClick={onDelete}
              >
                Confirm Delete
              </Button>
              <Button type="button" variant="outline" onClick={() => setConfirming(false)} disabled={deleting}>
                Cancel
              </Button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function EditableNumberRow({
  label,
  description,
  value,
  suffix,
  onSave,
}: {
  label: string;
  description?: string;
  value: number;
  suffix?: string;
  onSave: (next: number) => Promise<{ ok: boolean; error?: string }>;
}) {
  return (
    <EditableTextRow
      label={label}
      description={description}
      value={suffix ? `${value} ${suffix}` : String(value)}
      onSave={async (v) => {
        const n = Math.max(0, parseInt(v, 10) || 0);
        return onSave(n);
      }}
    />
  );
}

export function IntegrationsSection({ studio, onChange }: { studio: Studio; onChange: (patch: Partial<Studio>) => void }) {
  const [loading, setLoading] = useState(true);
  const [sendSpacing, setSendSpacing] = useState(20);
  const [dailyMessageLimit, setDailyMessageLimit] = useState(48);
  const [glofoxFirstSession, setGlofoxFirstSession] = useState(false);
  const [coldNeverRepliedDays, setColdNeverRepliedDays] = useState(1);
  const [coldStalledDays, setColdStalledDays] = useState(7);
  const [initialDelayMinutes, setInitialDelayMinutes] = useState(0);
  const [aiReplyDelay, setAiReplyDelay] = useState(0);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const [spacingRes, dailyLimitRes, glofoxRes, coldRes, delayRes, aiDelayRes] = await Promise.all([
        getWhatsAppSendSpacing(studio.id),
        getWhatsAppDailyMessageLimit(studio.id),
        getGlofoxFirstSession(studio.id),
        getColdLeadThresholds(studio.id),
        getInitialContactDelay(studio.id),
        getAIReplyDelay(studio.id),
      ]);
      if (cancelled) return;
      if (spacingRes.ok && spacingRes.data) setSendSpacing(spacingRes.data.whatsappSendSpacingSeconds);
      if (dailyLimitRes.ok && dailyLimitRes.data) setDailyMessageLimit(dailyLimitRes.data.whatsappDailyMessageLimit);
      if (glofoxRes.ok && glofoxRes.data) setGlofoxFirstSession(glofoxRes.data.enabled);
      if (coldRes.ok && coldRes.data) {
        setColdNeverRepliedDays(coldRes.data.coldNeverRepliedDays);
        setColdStalledDays(coldRes.data.coldStalledDays);
      }
      if (delayRes.ok && delayRes.data) setInitialDelayMinutes(delayRes.data.initialContactDelayMinutes);
      if (aiDelayRes.ok && aiDelayRes.data) setAiReplyDelay(aiDelayRes.data.aiReplyDelaySeconds);
      setLoading(false);
    })();
    return () => {
      cancelled = true;
    };
  }, [studio.id]);

  async function saveField(field: 'metaAppId' | 'metaAppSecret' | 'googleClientId' | 'googleClientSecret' | 'googleDeveloperToken', value: string) {
    const res = await updateStudioSettings(studio.id, studio.slug, { [field]: value });
    if (res.ok) {
      const hasFlag: Record<string, keyof Studio> = {
        metaAppSecret: 'hasMetaAppSecret',
        googleClientSecret: 'hasGoogleClientSecret',
        googleDeveloperToken: 'hasGoogleDeveloperToken',
      };
      const patch: Partial<Studio> = { [field]: value } as any;
      if (hasFlag[field]) patch[hasFlag[field]] = true as any;
      onChange(patch);
    }
    return res;
  }

  return (
    <div className="space-y-6">
      <SectionHeader title="Integrations" description="Meta, Google Ads, and message pacing." />

      <SettingsCard title="Meta App Settings">
        <EditableTextRow
          label="Meta App ID"
          description="The custom Facebook Developer App ID for Facebook/Instagram integration."
          value={studio.metaAppId || ''}
          onSave={(v) => saveField('metaAppId', v)}
        />
        <EditableSecretRow
          label="Meta App Secret"
          description="Kept encrypted. Only used server-side to sign API requests."
          hasValue={!!studio.hasMetaAppSecret}
          onSave={(v) => saveField('metaAppSecret', v)}
        />
      </SettingsCard>

      <SettingsCard title="Google Ads Integration">
        <EditableTextRow
          label="Google Client ID"
          description="The OAuth Client ID for the Google Ads integration."
          value={studio.googleClientId || ''}
          onSave={(v) => saveField('googleClientId', v)}
        />
        <EditableSecretRow
          label="Google Client Secret"
          description="The OAuth Client Secret for the Google Ads integration."
          hasValue={!!studio.hasGoogleClientSecret}
          onSave={(v) => saveField('googleClientSecret', v)}
        />
        <EditableSecretRow
          label="Google Developer Token"
          description="Required to call the Google Ads API."
          hasValue={!!studio.hasGoogleDeveloperToken}
          onSave={(v) => saveField('googleDeveloperToken', v)}
        />
      </SettingsCard>

      <SettingsCard title="Message Timing">
        {loading ? (
          <div className="flex items-center gap-2 px-5 py-4 text-sm text-zinc-400">
            <Loader2 className="h-4 w-4 animate-spin" />
            Loading…
          </div>
        ) : (
          <>
            <EditableNumberRow
              label="WhatsApp Message Pacing"
              description="Gap between consecutive WhatsApp messages sent on the same number (0-300s)."
              value={sendSpacing}
              suffix="sec"
              onSave={async (n) => {
                const res = await saveWhatsAppSendSpacing(studio.id, n);
                if (res.ok) setSendSpacing(n);
                return res;
              }}
            />
            <EditableNumberRow
              label="WhatsApp Daily Message Limit"
              description="Max automated/AI/Manual Actions WhatsApp messages per day, Singapore time (0 = unlimited). Live typed replies aren't counted."
              value={dailyMessageLimit}
              suffix="/day"
              onSave={async (n) => {
                const res = await saveWhatsAppDailyMessageLimit(studio.id, n);
                if (res.ok) setDailyMessageLimit(n);
                return res;
              }}
            />
            <ToggleRow
              label="Glofox First-Session Message"
              description="Automatically WhatsApp members after their first attended Glofox session. Off by default. Turning it on only covers sessions attended from that moment on, never past ones."
              checked={glofoxFirstSession}
              onSave={async (v) => {
                const res = await saveGlofoxFirstSession(studio.id, v);
                if (res.ok) setGlofoxFirstSession(v);
                return res;
              }}
            />
            <EditableNumberRow
              label="Cold Lead: Never Replied"
              description="Days of silence before a contacted-but-never-replied lead shows up in the Pipeline's Cold column. A background scan applies this roughly every 15 minutes."
              value={coldNeverRepliedDays}
              suffix="days"
              onSave={async (n) => {
                const res = await saveColdLeadThresholds(studio.id, n, coldStalledDays);
                if (res.ok) setColdNeverRepliedDays(n);
                return res;
              }}
            />
            <EditableNumberRow
              label="Cold Lead: Went Quiet"
              description="Days of no activity after a lead replied at least once before it's flagged Cold."
              value={coldStalledDays}
              suffix="days"
              onSave={async (n) => {
                const res = await saveColdLeadThresholds(studio.id, coldNeverRepliedDays, n);
                if (res.ok) setColdStalledDays(n);
                return res;
              }}
            />
            <EditableNumberRow
              label="Initial Message Delay"
              description="How long to wait before the very first message to a new lead (minutes)."
              value={initialDelayMinutes}
              suffix="min"
              onSave={async (n) => {
                const res = await saveInitialContactDelay(studio.id, n);
                if (res.ok) setInitialDelayMinutes(n);
                return res;
              }}
            />
            <EditableNumberRow
              label="AI Reply Delay"
              description="How long the AI waits before replying within an already-started conversation (seconds)."
              value={aiReplyDelay}
              suffix="sec"
              onSave={async (n) => {
                const res = await saveAIReplyDelay(studio.id, n);
                if (res.ok) setAiReplyDelay(n);
                return res;
              }}
            />
          </>
        )}
      </SettingsCard>
    </div>
  );
}

export function SheetsSection({ studio }: { studio: Studio }) {
  const [loading, setLoading] = useState(true);
  const [spreadsheetId, setSpreadsheetId] = useState('');
  const [tabName, setTabName] = useState('Leads');
  const [active, setActive] = useState(false);

  const [extSheets, setExtSheets] = useState<ExternalLeadsSheet[]>([]);
  const [newSheetId, setNewSheetId] = useState('');
  const [newSheetTab, setNewSheetTab] = useState('Sheet1');
  const [addingSheet, setAddingSheet] = useState(false);
  const [addError, setAddError] = useState('');

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const [sheetsRes, extRes] = await Promise.all([getSheetsSettings(studio.id), listExternalLeadsSheets(studio.id)]);
      if (cancelled) return;
      if (sheetsRes.ok && sheetsRes.data) {
        setSpreadsheetId(sheetsRes.data.spreadsheetId || '');
        setTabName(sheetsRes.data.tabName || 'Leads');
        setActive(sheetsRes.data.active);
      }
      if (extRes.ok && extRes.sheets) setExtSheets(extRes.sheets);
      setLoading(false);
    })();
    return () => {
      cancelled = true;
    };
  }, [studio.id]);

  async function saveMain(patch: Partial<{ spreadsheetId: string; tabName: string; active: boolean }>) {
    const next = { spreadsheetId, tabName, active, ...patch };
    const res = await saveSheetsSettings(studio.id, next);
    if (res.ok) {
      setSpreadsheetId(next.spreadsheetId);
      setTabName(next.tabName);
      setActive(next.active);
      return { ok: true };
    }
    return { ok: false, error: bestError(res) };
  }

  // Only the fields the API's request struct declares — the API's JSON decoder rejects
  // unknown fields, and a fetched sheet also carries `id`, `studioId`, timestamps.
  function sheetBody(sheet: ExternalLeadsSheetSettingsData): ExternalLeadsSheetSettingsData {
    return {
      spreadsheetId: sheet.spreadsheetId,
      tabName: sheet.tabName,
      nameColumn: sheet.nameColumn,
      firstNameColumn: sheet.firstNameColumn,
      lastNameColumn: sheet.lastNameColumn,
      emailColumn: sheet.emailColumn,
      phoneColumn: sheet.phoneColumn,
      sourceColumn: sheet.sourceColumn,
      notesColumn: sheet.notesColumn,
      dateColumn: sheet.dateColumn,
      hotLeadColumn: sheet.hotLeadColumn,
      trialPurchasedColumn: sheet.trialPurchasedColumn,
      continueAiAfterGreeting: sheet.continueAiAfterGreeting,
      autoContactEnabled: sheet.autoContactEnabled,
      active: sheet.active,
    };
  }

  async function saveExtSheet(id: string, patch: Partial<ExternalLeadsSheetSettingsData>) {
    const current = extSheets.find((x) => x.id === id);
    if (!current) return { ok: false, error: 'Sheet not found' };
    const next = { ...current, ...patch };
    const res = await updateExternalLeadsSheet(studio.id, id, sheetBody(next));
    if (res.ok) {
      setExtSheets((list) => list.map((x) => (x.id === id ? { ...next, ...(res.sheet ?? {}) } : x)));
      return { ok: true };
    }
    return { ok: false, error: bestError(res) };
  }

  async function removeExtSheet(id: string) {
    const res = await deleteExternalLeadsSheet(studio.id, id);
    if (res.ok) setExtSheets((list) => list.filter((x) => x.id !== id));
    return res;
  }

  async function addExtSheet() {
    setAddError('');
    if (!newSheetId.trim()) {
      setAddError('Enter a spreadsheet ID or URL.');
      return;
    }
    setAddingSheet(true);
    try {
      const res = await createExternalLeadsSheet(studio.id, {
        spreadsheetId: newSheetId.trim(),
        tabName: newSheetTab.trim() || 'Sheet1',
        nameColumn: '',
        firstNameColumn: 'A',
        lastNameColumn: 'B',
        emailColumn: 'C',
        phoneColumn: 'D',
        sourceColumn: '',
        notesColumn: '',
        dateColumn: '',
        hotLeadColumn: '',
        trialPurchasedColumn: '',
        continueAiAfterGreeting: true,
        autoContactEnabled: true,
        active: false,
      });
      if (res.ok && res.sheet) {
        setExtSheets((list) => [...list, res.sheet as ExternalLeadsSheet]);
        setNewSheetId('');
        setNewSheetTab('Sheet1');
      } else {
        setAddError(bestError(res));
      }
    } finally {
      setAddingSheet(false);
    }
  }

  if (loading) {
    return (
      <div className="flex items-center gap-2 text-sm text-zinc-400">
        <Loader2 className="h-4 w-4 animate-spin" />
        Loading…
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <SectionHeader title="Google Sheets" description="Lead sync and CSV column imports." />

      <SettingsCard title="Google Sheets Sync">
        <EditableTextRow
          label="Spreadsheet ID or URL"
          description="Required before sync can be turned on."
          value={spreadsheetId}
          onSave={(v) => saveMain({ spreadsheetId: v })}
        />
        <ToggleRow
          label="Enable sync"
          description="Every new lead is also written to this Google Sheet."
          checked={active}
          disabled={!spreadsheetId}
          disabledHint="Set a Spreadsheet ID above first."
          onSave={(v) => saveMain({ active: v })}
        />
        <EditableTextRow
          label="Tab name"
          value={tabName}
          disabled={!spreadsheetId}
          disabledHint="Set a Spreadsheet ID above first."
          onSave={(v) => saveMain({ tabName: v })}
        />
      </SettingsCard>

      <div>
        <h4 className="mb-1 text-xs font-bold uppercase tracking-wide text-zinc-400">External Leads Sheets (Import)</h4>
        <p className="mb-3 text-xs text-zinc-400">
          Add as many Google Sheets as you need. Each one is polled on its own and its new rows become leads, using its
          own tab, column mapping and options.
        </p>
        <div className="space-y-3">
          {extSheets.map((sheet, i) => (
            <ExternalSheetBlock
              key={sheet.id}
              index={i + 1}
              sheet={sheet}
              onSave={(patch) => saveExtSheet(sheet.id, patch)}
              onRemove={() => removeExtSheet(sheet.id)}
            />
          ))}
          {extSheets.length === 0 && (
            <p className="rounded-xl border border-dashed border-zinc-200 px-4 py-3 text-xs text-zinc-400 dark:border-zinc-800">
              No external sheets yet. Add one below.
            </p>
          )}
        </div>

        <div className="mt-3 rounded-xl border border-zinc-200 bg-white p-4 dark:border-zinc-800 dark:bg-zinc-950">
          <p className="mb-3 text-sm font-semibold text-zinc-800 dark:text-zinc-100">Add another sheet</p>
          <div className="grid gap-3 sm:grid-cols-[1fr_180px_auto]">
            <Input
              value={newSheetId}
              onChange={(e) => setNewSheetId(e.target.value)}
              placeholder="Spreadsheet ID or URL"
              invalid={!!addError}
            />
            <Input value={newSheetTab} onChange={(e) => setNewSheetTab(e.target.value)} placeholder="Tab name" />
            <Button type="button" onClick={addExtSheet} disabled={addingSheet} suppressHydrationWarning>
              {addingSheet ? 'Adding…' : 'Add sheet'}
            </Button>
          </div>
          {addError && <p className="mt-2 text-xs font-medium text-red-500">{addError}</p>}
        </div>
      </div>
    </div>
  );
}

const EXT_MAPPING_FIELDS: [keyof ExternalLeadsSheetSettingsData, string, string | undefined][] = [
  ['nameColumn', 'Full name column', 'e.g. A'],
  ['firstNameColumn', 'First name column', undefined],
  ['lastNameColumn', 'Last name column', undefined],
  ['emailColumn', 'Email column', undefined],
  ['phoneColumn', 'Phone column', undefined],
  ['sourceColumn', 'Source column', undefined],
  ['notesColumn', 'Notes column', undefined],
  ['dateColumn', 'Date column', undefined],
  ['hotLeadColumn', 'Hot lead column', undefined],
  ['trialPurchasedColumn', 'Trial purchased column', undefined],
];

// One external leads sheet: its connection settings, options and column mapping, plus a
// remove button. Collapsed by default when there are several, so the page stays short.
function ExternalSheetBlock({
  index,
  sheet,
  onSave,
  onRemove,
}: {
  index: number;
  sheet: ExternalLeadsSheet;
  onSave: (patch: Partial<ExternalLeadsSheetSettingsData>) => Promise<{ ok: boolean; error?: string }>;
  onRemove: () => Promise<{ ok: boolean; error?: string }>;
}) {
  const [confirmRemove, setConfirmRemove] = useState(false);
  const [removing, setRemoving] = useState(false);
  const [error, setError] = useState('');
  const shortId = sheet.spreadsheetId.length > 14 ? `${sheet.spreadsheetId.slice(0, 6)}…${sheet.spreadsheetId.slice(-6)}` : sheet.spreadsheetId;

  async function remove() {
    setRemoving(true);
    setError('');
    const res = await onRemove();
    setRemoving(false);
    if (!res.ok) setError(res.error ?? 'Failed to remove');
  }

  return (
    <details className="group rounded-xl border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-950" open={index === 1}>
      <summary className="flex cursor-pointer list-none items-center justify-between gap-3 px-4 py-3">
        <span className="min-w-0">
          <span className="block text-sm font-semibold text-zinc-800 dark:text-zinc-100">
            Sheet {index} · {sheet.tabName || 'Sheet1'}
          </span>
          <span className="block truncate text-xs text-zinc-400">{shortId}</span>
        </span>
        <span
          className={`shrink-0 rounded-full px-2 py-0.5 text-[10px] font-bold ${
            sheet.active
              ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
              : 'bg-zinc-100 text-zinc-500 dark:bg-zinc-900 dark:text-zinc-400'
          }`}
        >
          {sheet.active ? 'Importing' : 'Paused'}
        </span>
      </summary>

      <div className="space-y-4 border-t border-zinc-100 p-4 dark:border-zinc-800">
        <SettingsCard>
          <EditableTextRow label="Spreadsheet ID or URL" value={sheet.spreadsheetId} onSave={(v) => onSave({ spreadsheetId: v })} />
          <ToggleRow
            label="Enable import"
            description="Poll this Google Sheet for new rows."
            checked={sheet.active}
            onSave={(v) => onSave({ active: v })}
          />
          <EditableTextRow label="Tab name" value={sheet.tabName} onSave={(v) => onSave({ tabName: v })} />
          <ToggleRow
            label="Auto-contact new leads"
            description="Automatically start the WhatsApp outreach flow for leads imported from this sheet."
            checked={sheet.autoContactEnabled}
            onSave={(v) => onSave({ autoContactEnabled: v })}
          />
          <ToggleRow
            label="Continue AI after greeting"
            description="Let the AI keep replying after the initial greeting, instead of handing off to staff."
            checked={sheet.continueAiAfterGreeting}
            onSave={(v) => onSave({ continueAiAfterGreeting: v })}
          />
        </SettingsCard>

        <SettingsCard title="Column Mapping">
          {EXT_MAPPING_FIELDS.map(([field, label, placeholder]) => (
            <EditableTextRow
              key={field}
              label={label}
              placeholder={placeholder}
              value={String(sheet[field] ?? '')}
              onSave={(v) => onSave({ [field]: v } as Partial<ExternalLeadsSheetSettingsData>)}
            />
          ))}
        </SettingsCard>

        <div className="flex items-center justify-between gap-3">
          <p className="text-xs text-zinc-400">Removing a sheet stops importing from it. Leads already imported are kept.</p>
          {confirmRemove ? (
            <span className="flex shrink-0 items-center gap-2">
              <Button type="button" variant="outline" onClick={() => setConfirmRemove(false)} suppressHydrationWarning>
                Keep
              </Button>
              <Button
                type="button"
                onClick={remove}
                disabled={removing}
                className="bg-red-600 text-white hover:bg-red-700"
                suppressHydrationWarning
              >
                {removing ? 'Removing…' : 'Yes, remove'}
              </Button>
            </span>
          ) : (
            <Button type="button" variant="outline" onClick={() => setConfirmRemove(true)} className="shrink-0" suppressHydrationWarning>
              Remove sheet
            </Button>
          )}
        </div>
        {error && <p className="text-xs font-medium text-red-500">{error}</p>}
      </div>
    </details>
  );
}
