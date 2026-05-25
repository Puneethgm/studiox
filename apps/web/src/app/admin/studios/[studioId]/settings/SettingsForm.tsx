'use client';

import { useRouter } from 'next/navigation';
import { useState, useEffect } from 'react';
import { Eye, EyeOff, Database } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { FieldError, FieldHint, Label } from '@/components/ui/Label';
import type { Studio } from '@/lib/types';
import { changeMyPassword, updateStudioSettings, getSheetsSettings, saveSheetsSettings } from './actions';
import { AvailabilitySettings } from './AvailabilitySettings';




export function SettingsForm({ studio, previewHref }: { studio: Studio; previewHref: string | null }) {
  const router = useRouter();
  const [name, setName] = useState(studio.name);
  const [brandColor, setBrandColor] = useState(studio.brandColor);
  const [logoUrl, setLogoUrl] = useState(studio.logoUrl);
  const [logoError, setLogoError] = useState(false);
  useEffect(() => {
    setLogoError(false);
  }, [logoUrl]);
  const [contactEmail, setContactEmail] = useState(studio.contactEmail);
  const [geminiApiKey, setGeminiApiKey] = useState(studio.geminiApiKey || '');
  const [active, setActive] = useState(studio.active);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [showCurrentPassword, setShowCurrentPassword] = useState(false);
  const [showNewPassword, setShowNewPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);
  const [passwordErrors, setPasswordErrors] = useState<Record<string, string>>({});
  const [changingPassword, setChangingPassword] = useState(false);
  const [passwordSuccess, setPasswordSuccess] = useState<string | null>(null);

  const [spreadsheetId, setSpreadsheetId] = useState('');
  const [tabName, setTabName] = useState('Leads');
  const [sheetsActive, setSheetsActive] = useState(false);
  const [sheetsSaving, setSheetsSaving] = useState(false);
  const [sheetsSuccess, setSheetsSuccess] = useState<string | null>(null);
  const [sheetsError, setSheetsError] = useState<string | null>(null);

  useEffect(() => {
    getSheetsSettings(studio.id).then((res) => {
      if (res.ok && res.data) {
        setSpreadsheetId(res.data.spreadsheetId || '');
        setTabName(res.data.tabName || 'Leads');
        setSheetsActive(res.data.active || false);
      }
    });
  }, [studio.id]);

  async function onSaveSheetsSettings(e: React.FormEvent) {
    e.preventDefault();
    setSheetsError(null);
    setSheetsSuccess(null);
    setSheetsSaving(true);
    try {
      const res = await saveSheetsSettings(studio.id, {
        spreadsheetId,
        tabName,
        active: sheetsActive,
      });
      if (res.ok) {
        setSheetsSuccess('Google Sheets connection saved successfully.');
      } else {
        setSheetsError(res.error || 'Failed to save settings.');
      }
    } catch (err: any) {
      setSheetsError(err.message || 'An error occurred.');
    } finally {
      setSheetsSaving(false);
    }
  }

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setErrors({});
    setSaving(true);
    try {
      const result = await updateStudioSettings(studio.id, studio.slug, {
        name,
        brandColor,
        logoUrl,
        contactEmail,
        active,
        geminiApiKey,
      });
      if (!result.ok) {
        setErrors(result.details ?? { _: result.error });
        return;
      }
      // The Server Action revalidated the studio overview / studios list /
      // public form for this slug — no manual refresh needed on return.
      if (typeof window !== 'undefined' && window.history.length > 1) {
        router.back();
      } else {
        router.push(`/admin/studios/${studio.id}`);
      }
    } finally {
      setSaving(false);
    }
  }

  async function onChangePassword(e: React.FormEvent) {
    e.preventDefault();
    setPasswordErrors({});
    setPasswordSuccess(null);

    if (newPassword !== confirmPassword) {
      setPasswordErrors({ confirmPassword: 'Passwords do not match' });
      return;
    }

    setChangingPassword(true);
    try {
      const result = await changeMyPassword({
        currentPassword,
        newPassword,
        confirmPassword,
      });
      if (!result.ok) {
        setPasswordErrors(result.details ?? { _: result.error });
        return;
      }

      setCurrentPassword('');
      setNewPassword('');
      setConfirmPassword('');
      setPasswordSuccess('Password updated successfully.');
    } finally {
      setChangingPassword(false);
    }
  }

  return (
    <div className="grid gap-6 md:grid-cols-3">
      <div className="md:col-span-2">
        <div className="overflow-hidden rounded-[24px] border border-white/30 bg-white/30 backdrop-blur-2xl dark:border-white/5 dark:bg-neutral-900/30" style={{ boxShadow: 'inset 0 0 0 1px rgba(255,255,255,0.15), 0 4px 16px rgba(0,0,0,0.05)' }}>
          <div className="border-b border-white/20 px-6 py-4 dark:border-white/5">
            <h3 className="text-sm font-black uppercase tracking-[0.15em] text-zinc-400">Identity</h3>
          </div>
          <form onSubmit={onSubmit} className="space-y-5 p-6">
            <div>
              <Label htmlFor="name">Studio name</Label>
              <Input
                id="name"
                required
                invalid={!!errors.name}
                value={name}
                onChange={(e) => setName(e.target.value)}
              />
              <FieldError message={errors.name} />
            </div>

            <div className="grid gap-5 sm:grid-cols-2">
              <div>
                <Label htmlFor="brandColor">Brand color</Label>
                <div className="flex items-center gap-2">
                  <input
                    type="color"
                    id="brandColor"
                    value={brandColor}
                    onChange={(e) => setBrandColor(e.target.value)}
                    className="h-10 w-12 cursor-pointer rounded-md border border-slate-300 bg-white p-1 dark:border-slate-700 dark:bg-slate-900"
                    suppressHydrationWarning
                  />
                  <Input
                    value={brandColor}
                    onChange={(e) => setBrandColor(e.target.value)}
                    invalid={!!errors.brandColor}
                    className="font-mono"
                  />
                </div>
                <FieldError message={errors.brandColor} />
              </div>
              <div>
                <Label htmlFor="logoUrl">Logo URL</Label>
                <Input
                  id="logoUrl"
                  value={logoUrl}
                  onChange={(e) => setLogoUrl(e.target.value)}
                />
                <FieldHint>Square image works best.</FieldHint>
              </div>
            </div>

            <div>
              <Label htmlFor="contactEmail">Contact email</Label>
              <Input
                id="contactEmail"
                type="email"
                invalid={!!errors.contactEmail}
                value={contactEmail}
                onChange={(e) => setContactEmail(e.target.value)}
              />
              <FieldError message={errors.contactEmail} />
            </div>

            <div>
              <Label htmlFor="geminiApiKey">Gemini API Key</Label>
              <Input
                id="geminiApiKey"
                type="password"
                placeholder="AIzaSy..."
                invalid={!!errors.geminiApiKey}
                value={geminiApiKey}
                onChange={(e) => setGeminiApiKey(e.target.value)}
              />
              <FieldHint>Configure the Gemini API Key to enable AI-driven template generation.</FieldHint>
              <FieldError message={errors.geminiApiKey} />
            </div>

            <div className="flex items-center gap-3">
              <input
                type="checkbox"
                id="active"
                checked={active}
                onChange={(e) => setActive(e.target.checked)}
                className="h-4 w-4 rounded border-slate-300 text-[color:var(--brand,#7c3aed)] focus:ring-[color:var(--brand,#7c3aed)]"
                suppressHydrationWarning
              />
              <Label htmlFor="active" className="mb-0">Studio is active</Label>
            </div>
            <FieldHint>Inactive studios stop accepting public form submissions.</FieldHint>

            <FieldError message={errors._} />

            <div className="flex items-center justify-end gap-2 border-t border-slate-100 pt-5 dark:border-slate-800/60">
              <Button variant="ghost" type="button" onClick={() => router.back()}>
                Cancel
              </Button>
              <Button type="submit" loading={saving}>Save & back</Button>
            </div>
          </form>
        </div>

        {/* Availability Settings */}
        <AvailabilitySettings studio={studio} />
      </div>

      <div>
        <div className="overflow-hidden rounded-[24px] border border-white/30 bg-white/30 backdrop-blur-2xl dark:border-white/5 dark:bg-neutral-900/30" style={{ boxShadow: 'inset 0 0 0 1px rgba(255,255,255,0.15), 0 4px 16px rgba(0,0,0,0.05)' }}>
          <div className="border-b border-white/20 px-6 py-4 dark:border-white/5">
            <h3 className="text-sm font-black uppercase tracking-[0.15em] text-zinc-400">Live Preview</h3>
          </div>
          <div className="p-6">
          <div className="rounded-xl bg-slate-50 p-4 dark:bg-slate-900/40">
            <div className="flex items-center gap-3">
              <span
                className="grid h-12 w-12 place-items-center rounded-2xl text-base font-bold text-white shadow-md"
                style={{ background: brandColor }}
              >
                {logoUrl && !logoError ? (
                  // eslint-disable-next-line @next/next/no-img-element
                  <img src={logoUrl} alt="" className="h-12 w-12 rounded-2xl object-cover" onError={() => setLogoError(true)} />
                ) : (
                  (name || studio.name).slice(0, 2).toUpperCase()
                )}
              </span>
              <div className="min-w-0">
                <div className="truncate text-sm font-semibold">{name || studio.name}</div>
                <div className="truncate font-mono text-xs text-slate-500">/{studio.slug}</div>
              </div>
            </div>
            <div className="mt-4">
              {previewHref ? (
                <a
                  href={previewHref}
                  target="_blank"
                  rel="noreferrer"
                  className="block w-full rounded-xl py-2.5 text-center text-sm font-semibold text-white shadow-sm"
                  style={{ background: brandColor }}
                  suppressHydrationWarning
                >
                  Get in touch
                </a>
              ) : (
                <button
                  type="button"
                  disabled
                  className="w-full rounded-xl py-2.5 text-sm font-semibold text-white shadow-sm opacity-60"
                  style={{ background: brandColor }}
                  suppressHydrationWarning
                >
                  Create a campaign first
                </button>
              )}
            </div>
          </div>
          <FieldHint>
            Slug is fixed (renaming would break shared links). Ask the platform admin if you need it changed.
          </FieldHint>
          </div>
        </div>


        <div className="mt-6 overflow-hidden rounded-[24px] border border-white/30 bg-white/30 backdrop-blur-2xl dark:border-white/5 dark:bg-neutral-900/30" style={{ boxShadow: 'inset 0 0 0 1px rgba(255,255,255,0.15), 0 4px 16px rgba(0,0,0,0.05)' }}>
          <div className="border-b border-white/20 px-6 py-4 dark:border-white/5">
            <h3 className="text-sm font-black uppercase tracking-[0.15em] text-zinc-400">Password</h3>
          </div>
          <form onSubmit={onChangePassword} className="space-y-4 p-6">
            <div>
              <Label htmlFor="currentPassword">Current password</Label>
              <div className="relative">
                <Input
                  id="currentPassword"
                  type={showCurrentPassword ? 'text' : 'password'}
                  autoComplete="current-password"
                  invalid={!!passwordErrors.currentPassword}
                  value={currentPassword}
                  onChange={(e) => setCurrentPassword(e.target.value)}
                  className="pr-12"
                />
                <button
                  type="button"
                  aria-label={showCurrentPassword ? 'Hide current password' : 'Show current password'}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-zinc-500 transition hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200"
                  onClick={() => setShowCurrentPassword((v) => !v)}
                >
                  {showCurrentPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </button>
              </div>
              <FieldError message={passwordErrors.currentPassword} />
            </div>

            <div>
              <Label htmlFor="newPassword">New password</Label>
              <div className="relative">
                <Input
                  id="newPassword"
                  type={showNewPassword ? 'text' : 'password'}
                  autoComplete="new-password"
                  invalid={!!passwordErrors.newPassword}
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  className="pr-12"
                />
                <button
                  type="button"
                  aria-label={showNewPassword ? 'Hide new password' : 'Show new password'}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-zinc-500 transition hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200"
                  onClick={() => setShowNewPassword((v) => !v)}
                >
                  {showNewPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </button>
              </div>
              <FieldHint>Use at least 8 characters.</FieldHint>
              <FieldError message={passwordErrors.newPassword} />
            </div>

            <div>
              <Label htmlFor="confirmPassword">Re-enter new password</Label>
              <div className="relative">
                <Input
                  id="confirmPassword"
                  type={showConfirmPassword ? 'text' : 'password'}
                  autoComplete="new-password"
                  invalid={!!passwordErrors.confirmPassword}
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  className="pr-12"
                />
                <button
                  type="button"
                  aria-label={showConfirmPassword ? 'Hide re-entered password' : 'Show re-entered password'}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-zinc-500 transition hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200"
                  onClick={() => setShowConfirmPassword((v) => !v)}
                >
                  {showConfirmPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </button>
              </div>
              <FieldError message={passwordErrors.confirmPassword} />
            </div>

            {passwordSuccess ? (
              <p className="text-sm font-medium text-emerald-600 dark:text-emerald-400">{passwordSuccess}</p>
            ) : null}
            <FieldError message={passwordErrors._} />

            <div className="flex items-center justify-end border-t border-slate-100 pt-4 dark:border-slate-800/60">
              <Button type="submit" loading={changingPassword}>Update password</Button>
            </div>
          </form>
        </div>

        <div className="mt-6 overflow-hidden rounded-[24px] border border-white/30 bg-white/30 backdrop-blur-2xl dark:border-white/5 dark:bg-neutral-900/30" style={{ boxShadow: 'inset 0 0 0 1px rgba(255,255,255,0.15), 0 4px 16px rgba(0,0,0,0.05)' }}>
          <div className="border-b border-white/20 px-6 py-4 dark:border-white/5 flex items-center gap-2">
            <Database className="h-4 w-4 text-zinc-400" />
            <h3 className="text-sm font-black uppercase tracking-[0.15em] text-zinc-400">Google Sheets Sync</h3>
          </div>
          <form onSubmit={onSaveSheetsSettings} className="space-y-4 p-6">
            <div>
              <Label htmlFor="spreadsheetId">Spreadsheet ID</Label>
              <Input
                id="spreadsheetId"
                placeholder="1aBc...Xyz"
                value={spreadsheetId}
                onChange={(e) => setSpreadsheetId(e.target.value)}
              />
              <FieldHint>The ID of your Google Sheet tracker</FieldHint>
            </div>

            <div>
              <Label htmlFor="tabName">Tab Name</Label>
              <Input
                id="tabName"
                placeholder="Leads"
                value={tabName}
                onChange={(e) => setTabName(e.target.value)}
              />
            </div>

            <div className="flex items-center gap-3">
              <input
                type="checkbox"
                id="sheetsActive"
                checked={sheetsActive}
                onChange={(e) => setSheetsActive(e.target.checked)}
                className="h-4 w-4 rounded border-slate-300 text-[color:var(--brand,#7c3aed)] focus:ring-[color:var(--brand,#7c3aed)]"
              />
              <Label htmlFor="sheetsActive" className="mb-0">Enable sync</Label>
            </div>

            {sheetsSuccess ? (
              <p className="text-sm font-medium text-emerald-600 dark:text-emerald-400">{sheetsSuccess}</p>
            ) : null}
            {sheetsError ? (
              <p className="text-sm font-medium text-rose-600 dark:text-rose-400">{sheetsError}</p>
            ) : null}

            <div className="flex items-center justify-end border-t border-slate-100 pt-4 dark:border-slate-800/60">
              <Button type="submit" loading={sheetsSaving}>Save Connection</Button>
            </div>
          </form>
        </div>
      </div>
    </div>
  );
}
