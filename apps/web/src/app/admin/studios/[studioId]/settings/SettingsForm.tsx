'use client';

import { useRouter } from 'next/navigation';
import { useState, useEffect } from 'react';
import { Eye, EyeOff, Database, Building, Calendar, Cpu, Lock, Save } from 'lucide-react';
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
  const [metaAppId, setMetaAppId] = useState(studio.metaAppId || '');
  const [metaAppSecret, setMetaAppSecret] = useState(studio.metaAppSecret || '');
  const [active, setActive] = useState(studio.active);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);
  
  const [activeSection, setActiveSection] = useState<'general' | 'availability' | 'integrations' | 'security'>('general');
  
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

  const [showGeminiApiKey, setShowGeminiApiKey] = useState(false);
  const [showMetaAppSecret, setShowMetaAppSecret] = useState(false);

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
        metaAppId,
        metaAppSecret,
      });
      if (!result.ok) {
        setErrors(result.details ?? { _: result.error });
        return;
      }
      if (typeof window !== 'undefined' && window.history.length > 1) {
        router.back();
      } else {
        router.push(`/admin/studios/${studio.id}`);
      }
    } finally {
      setSaving(false);
    }
  }

  async function onSubmitConfigOnly(e: React.FormEvent) {
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
        metaAppId,
        metaAppSecret,
      });
      if (!result.ok) {
        setErrors(result.details ?? { _: result.error });
        return;
      }
      alert('Integrations saved successfully.');
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
    <div className="flex flex-col gap-8 lg:flex-row lg:items-start">
      {/* Left Sidebar Navigation */}
      <div className="w-full lg:w-64 shrink-0 flex flex-row lg:flex-col gap-1 overflow-x-auto lg:overflow-visible pb-2 lg:pb-0 border-b lg:border-b-0 lg:border-r border-white/10 lg:pr-6 scrollbar-none">
        {[
          { id: 'general', label: 'General Info', icon: Building },
          { id: 'availability', label: 'Availability', icon: Calendar },
          { id: 'integrations', label: 'Integrations', icon: Cpu },
          { id: 'security', label: 'Security', icon: Lock },
        ].map((item) => {
          const Icon = item.icon;
          const isActive = activeSection === item.id;
          return (
            <button
              key={item.id}
              type="button"
              onClick={() => setActiveSection(item.id as any)}
              className={`flex items-center gap-3 rounded-2xl px-4 py-3 text-xs font-black uppercase tracking-wider transition-all duration-200 shrink-0 whitespace-nowrap ${
                isActive
                  ? 'bg-gradient-to-r from-brand-500 to-violet-600 text-white shadow-lg shadow-brand-500/20'
                  : 'text-zinc-500 hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200 hover:bg-white/10 dark:hover:bg-neutral-800/30'
              }`}
            >
              <Icon className="h-4 w-4 shrink-0" />
              {item.label}
            </button>
          );
        })}
      </div>

      {/* Right Content Pane */}
      <div className="flex-1 min-w-0">
        {activeSection === 'general' && (
          <div className="grid gap-6 lg:grid-cols-3">
            <div className="lg:col-span-2">
              <form onSubmit={onSubmit} className="overflow-hidden rounded-[24px] border border-white/30 bg-white/20 backdrop-blur-2xl dark:border-white/5 dark:bg-neutral-900/30 p-6 space-y-5">
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
                    <div className="flex items-center gap-2 mt-1">
                      <div className="relative h-10 w-12 shrink-0 overflow-hidden rounded-xl border border-white/10 bg-white/5">
                        <input
                          type="color"
                          id="brandColor"
                          value={brandColor}
                          onChange={(e) => setBrandColor(e.target.value)}
                          className="absolute -inset-2 h-14 w-16 cursor-pointer border-0 p-0 bg-transparent"
                          suppressHydrationWarning
                        />
                      </div>
                      <Input
                        value={brandColor}
                        onChange={(e) => setBrandColor(e.target.value)}
                        invalid={!!errors.brandColor}
                        className="font-mono text-xs"
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

                <div className="flex items-center gap-3 p-3 rounded-2xl border border-white/10 bg-white/10 dark:bg-neutral-800/10">
                  <input
                    type="checkbox"
                    id="active"
                    checked={active}
                    onChange={(e) => setActive(e.target.checked)}
                    className="h-4 w-4 rounded border-white/20 text-brand-500 focus:ring-brand-500 bg-white/10 cursor-pointer"
                    suppressHydrationWarning
                  />
                  <div>
                    <Label htmlFor="active" className="mb-0 cursor-pointer text-xs font-black uppercase tracking-wider">Studio is active</Label>
                    <p className="text-[10px] text-zinc-400">Inactive studios stop accepting public form submissions.</p>
                  </div>
                </div>

                <FieldError message={errors._} />

                <div className="flex items-center justify-end gap-2 border-t border-white/10 pt-5">
                  <Button variant="ghost" type="button" onClick={() => router.back()}>
                    Cancel
                  </Button>
                  <Button 
                    type="submit" 
                    loading={saving}
                    className="bg-gradient-to-r from-brand-500 to-violet-600 hover:from-brand-600 hover:to-violet-700 text-white text-xs font-black uppercase tracking-widest shadow-lg shadow-brand-500/25 rounded-xl h-10 px-6"
                  >
                    Save Changes
                  </Button>
                </div>
              </form>
            </div>

            {/* Live Preview Column */}
            <div className="lg:col-span-1">
              <div className="overflow-hidden rounded-[24px] border border-white/30 bg-white/20 backdrop-blur-2xl dark:border-white/5 dark:bg-neutral-900/30">
                <div className="border-b border-white/20 px-6 py-4 dark:border-white/5">
                  <h3 className="text-xs font-black uppercase tracking-wider text-zinc-400">Live Preview</h3>
                </div>
                <div className="p-6 space-y-4">
                  <div className="rounded-2xl border border-white/10 bg-white/10 p-4 dark:bg-neutral-800/20">
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
                          className="block w-full rounded-xl py-2.5 text-center text-xs font-bold text-white shadow-md transition-transform hover:scale-[1.02]"
                          style={{ background: brandColor }}
                          suppressHydrationWarning
                        >
                          Get in touch
                        </a>
                      ) : (
                        <button
                          type="button"
                          disabled
                          className="w-full rounded-xl py-2.5 text-xs font-semibold text-white shadow-sm opacity-60"
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
            </div>
          </div>
        )}

        {activeSection === 'availability' && (
          <AvailabilitySettings studio={studio} />
        )}

        {activeSection === 'integrations' && (
          <div className="space-y-6">
            <form onSubmit={onSubmitConfigOnly} className="overflow-hidden rounded-[24px] border border-white/30 bg-white/20 backdrop-blur-2xl dark:border-white/5 dark:bg-neutral-900/30 p-6 space-y-5">
              <div>
                <h3 className="text-xs font-black uppercase tracking-wider text-zinc-400 mb-4">AI & Meta Integrations</h3>
              </div>

              <div>
                <Label htmlFor="geminiApiKey">Gemini API Key</Label>
                <div className="relative mt-1">
                  <Input
                    id="geminiApiKey"
                    type={showGeminiApiKey ? 'text' : 'password'}
                    placeholder="AIzaSy..."
                    invalid={!!errors.geminiApiKey}
                    value={geminiApiKey}
                    onChange={(e) => setGeminiApiKey(e.target.value)}
                    className="pr-12"
                  />
                  <button
                    type="button"
                    className="absolute right-3 top-1/2 -translate-y-1/2 text-zinc-500 transition hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200"
                    onClick={() => setShowGeminiApiKey(!showGeminiApiKey)}
                  >
                    {showGeminiApiKey ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                  </button>
                </div>
                <FieldHint>Configure the Gemini API Key to enable AI-driven template and post generation.</FieldHint>
                <FieldError message={errors.geminiApiKey} />
              </div>

              <div>
                <Label htmlFor="metaAppId">Meta App ID</Label>
                <Input
                  id="metaAppId"
                  type="text"
                  placeholder="e.g. 2405726999940224"
                  invalid={!!errors.metaAppId}
                  value={metaAppId}
                  onChange={(e) => setMetaAppId(e.target.value)}
                />
                <FieldHint>The custom Facebook Developer App ID for Facebook/Instagram integration.</FieldHint>
                <FieldError message={errors.metaAppId} />
              </div>

              <div>
                <Label htmlFor="metaAppSecret">Meta App Secret</Label>
                <div className="relative mt-1">
                  <Input
                    id="metaAppSecret"
                    type={showMetaAppSecret ? 'text' : 'password'}
                    placeholder="e.g. d2d2fad32..."
                    invalid={!!errors.metaAppSecret}
                    value={metaAppSecret}
                    onChange={(e) => setMetaAppSecret(e.target.value)}
                    className="pr-12"
                  />
                  <button
                    type="button"
                    className="absolute right-3 top-1/2 -translate-y-1/2 text-zinc-500 transition hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200"
                    onClick={() => setShowMetaAppSecret(!showMetaAppSecret)}
                  >
                    {showMetaAppSecret ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                  </button>
                </div>
                <FieldHint>The custom Meta App Secret for validating incoming webhook events.</FieldHint>
                <FieldError message={errors.metaAppSecret} />
              </div>

              <FieldError message={errors._} />

              <div className="flex items-center justify-end border-t border-white/10 pt-5">
                <Button 
                  type="submit" 
                  loading={saving}
                  className="bg-gradient-to-r from-brand-500 to-violet-600 hover:from-brand-600 hover:to-violet-700 text-white text-xs font-black uppercase tracking-widest shadow-lg shadow-brand-500/25 rounded-xl h-10 px-6"
                >
                  Save API Config
                </Button>
              </div>
            </form>

            {/* Google Sheets Card */}
            <div className="overflow-hidden rounded-[24px] border border-white/30 bg-white/20 backdrop-blur-2xl dark:border-white/5 dark:bg-neutral-900/30">
              <div className="border-b border-white/20 px-6 py-4 dark:border-white/5 flex items-center gap-2">
                <Database className="h-4 w-4 text-brand-500" />
                <h3 className="text-xs font-black uppercase tracking-wider text-zinc-400">Google Sheets Sync</h3>
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

                <div className="flex items-center gap-3 p-3 rounded-2xl border border-white/10 bg-white/10 dark:bg-neutral-800/10">
                  <input
                    type="checkbox"
                    id="sheetsActive"
                    checked={sheetsActive}
                    onChange={(e) => setSheetsActive(e.target.checked)}
                    className="h-4 w-4 rounded border-white/20 text-brand-500 focus:ring-brand-500 bg-white/10 cursor-pointer"
                  />
                  <div>
                    <Label htmlFor="sheetsActive" className="mb-0 cursor-pointer text-xs font-black uppercase tracking-wider">Enable Google Sheets Sync</Label>
                    <p className="text-[10px] text-zinc-400">Automatically synchronize lead submissions with your Google Spreadsheet.</p>
                  </div>
                </div>

                {sheetsSuccess ? (
                  <p className="text-xs font-black text-emerald-500 uppercase tracking-wider">{sheetsSuccess}</p>
                ) : null}
                {sheetsError ? (
                  <p className="text-xs font-black text-rose-500 uppercase tracking-wider">{sheetsError}</p>
                ) : null}

                <div className="flex items-center justify-end border-t border-white/10 pt-4">
                  <Button 
                    type="submit" 
                    loading={sheetsSaving}
                    className="bg-gradient-to-r from-brand-500 to-violet-600 hover:from-brand-600 hover:to-violet-700 text-white text-xs font-black uppercase tracking-widest shadow-lg shadow-brand-500/25 rounded-xl h-10 px-6"
                  >
                    Save Connection
                  </Button>
                </div>
              </form>
            </div>
          </div>
        )}

        {activeSection === 'security' && (
          <div className="max-w-xl">
            <div className="overflow-hidden rounded-[24px] border border-white/30 bg-white/20 backdrop-blur-2xl dark:border-white/5 dark:bg-neutral-900/30">
              <div className="border-b border-white/20 px-6 py-4 dark:border-white/5">
                <h3 className="text-xs font-black uppercase tracking-wider text-zinc-400">Change Password</h3>
              </div>
              <form onSubmit={onChangePassword} className="space-y-4 p-6">
                <div>
                  <Label htmlFor="currentPassword">Current password</Label>
                  <div className="relative mt-1">
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
                  <div className="relative mt-1">
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
                  <div className="relative mt-1">
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
                  <p className="text-xs font-black text-emerald-500 uppercase tracking-wider">{passwordSuccess}</p>
                ) : null}
                <FieldError message={passwordErrors._} />

                <div className="flex items-center justify-end border-t border-white/10 pt-4">
                  <Button 
                    type="submit" 
                    loading={changingPassword}
                    className="bg-gradient-to-r from-brand-500 to-violet-600 hover:from-brand-600 hover:to-violet-700 text-white text-xs font-black uppercase tracking-widest shadow-lg shadow-brand-500/25 rounded-xl h-10 px-6"
                  >
                    Update password
                  </Button>
                </div>
              </form>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
