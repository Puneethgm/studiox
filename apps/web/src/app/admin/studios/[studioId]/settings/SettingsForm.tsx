'use client';

import { useRouter } from 'next/navigation';
import { useState, useEffect } from 'react';
import { Eye, EyeOff, Database, Building, Calendar, Cpu, Lock, Save, CheckCircle2, X, DollarSign } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { FieldError, FieldHint, Label } from '@/components/ui/Label';
import type { Studio } from '@/lib/types';
import { changeMyPassword, updateStudioSettings, getSheetsSettings, saveSheetsSettings } from './actions';
import { AvailabilitySettings } from './AvailabilitySettings';
import { PlansManagement } from './PlansManagement';

export function SettingsForm({ studio, previewHref, initialPlans }: { studio: Studio; previewHref: string | null; initialPlans: any[] }) {
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
  
  const [activeSection, setActiveSection] = useState<'general' | 'plans' | 'availability' | 'integrations' | 'security' | 'billing'>('general');
  
  // Trial Pricing (stored as cents/paise in the backend)
  const [trialAmountSgd, setTrialAmountSgd] = useState((studio.trialAmountSgd ?? 2500) / 100);
  const [pricingSaving, setPricingSaving] = useState(false);

  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [showCurrentPassword, setShowCurrentPassword] = useState(false);
  const [showNewPassword, setShowNewPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);
  const [passwordErrors, setPasswordErrors] = useState<Record<string, string>>({});
  const [changingPassword, setChangingPassword] = useState(false);

  const [spreadsheetId, setSpreadsheetId] = useState('');
  const [tabName, setTabName] = useState('Leads');
  const [sheetsActive, setSheetsActive] = useState(false);
  const [sheetsSaving, setSheetsSaving] = useState(false);
  const [sheetsError, setSheetsError] = useState<string | null>(null);
  const [metaSaving, setMetaSaving] = useState(false);
  const [geminiSaving, setGeminiSaving] = useState(false);

  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null);

  const showToast = (message: string, type: 'success' | 'error' = 'success') => {
    setToast({ message, type });
  };

  const [showGeminiApiKey, setShowGeminiApiKey] = useState(false);
  const [showMetaAppSecret, setShowMetaAppSecret] = useState(false);
  const [uploadingLogo, setUploadingLogo] = useState(false);

  useEffect(() => {
    void (async () => {
      const res = await getSheetsSettings(studio.id);
      if (res.ok && res.data) {
        setSpreadsheetId(res.data.spreadsheetId || '');
        setTabName(res.data.tabName || 'Leads');
        setSheetsActive(res.data.active || false);
      }
    })();
  }, [studio.id]);

  async function onSaveSheetsSettings(e: React.FormEvent) {
    e.preventDefault();
    setSheetsError(null);
    setSheetsSaving(true);
    try {
      const res = await saveSheetsSettings(studio.id, {
        spreadsheetId,
        tabName,
        active: sheetsActive,
      });
      if (res.ok) {
        showToast('Google Sheets connection saved successfully.');
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

  async function onSaveGeminiKey(e: React.FormEvent) {
    e.preventDefault();
    setErrors({});
    setGeminiSaving(true);
    try {
      const result = await updateStudioSettings(studio.id, studio.slug, {
        geminiApiKey,
      });
      if (!result.ok) {
        setErrors(result.details ?? { _: result.error });
        return;
      }
      showToast('Gemini API Key saved successfully.');
    } finally {
      setGeminiSaving(false);
    }
  }

  async function onSaveMetaConfig(e: React.FormEvent) {
    e.preventDefault();
    setErrors({});
    setMetaSaving(true);
    try {
      const result = await updateStudioSettings(studio.id, studio.slug, {
        metaAppId,
        metaAppSecret,
      });
      if (!result.ok) {
        setErrors(result.details ?? { _: result.error });
        return;
      }
      showToast('Meta Integration config saved successfully.');
    } finally {
      setMetaSaving(false);
    }
  }

  async function onChangePassword(e: React.FormEvent) {
    e.preventDefault();
    setPasswordErrors({});

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
      showToast('Password updated successfully.');
    } finally {
      setChangingPassword(false);
    }
  }

  async function onUploadLogo(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;

    setUploadingLogo(true);
    try {
      const formData = new FormData();
      formData.append('file', file);

      const token = localStorage.getItem('token');
      const res = await fetch(`/api/v1/me/studios/${studio.id}/logo`, {
        method: 'POST',
        body: formData,
        headers: {
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
      });

      if (!res.ok) {
        const error = await res.json().catch(() => ({ error: 'Upload failed' }));
        showToast(error.error || 'Failed to upload logo', 'error');
        return;
      }

      const data = await res.json();
      setLogoUrl(data.logoUrl);
      showToast('Logo uploaded successfully.');
    } catch (err: any) {
      showToast(err.message || 'Failed to upload logo', 'error');
    } finally {
      setUploadingLogo(false);
      // Reset file input
      e.target.value = '';
    }
  }

  return (
    <div className="flex flex-col gap-8 lg:flex-row lg:items-start">
      {/* Left Sidebar Navigation */}
      <div className="w-full lg:w-64 shrink-0 flex flex-row lg:flex-col gap-1 overflow-x-auto lg:overflow-visible pb-2 lg:pb-0 border-b lg:border-b-0 lg:border-r border-white/10 lg:pr-6 scrollbar-none">
        {[
          { id: 'general', label: 'General Info', icon: Building },
          { id: 'plans', label: 'Plans', icon: DollarSign },
          { id: 'availability', label: 'Availability', icon: Calendar },
          { id: 'integrations', label: 'Integrations', icon: Cpu },
          { id: 'security', label: 'Security', icon: Lock },
          { id: 'billing', label: 'Platform Billing', icon: DollarSign },
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

                <div>
                  <Label htmlFor="logoFile">Or upload image</Label>
                  <div className="flex items-center gap-2 mt-1">
                    <input
                      id="logoFile"
                      type="file"
                      accept="image/jpeg,image/png,image/webp,image/gif"
                      onChange={onUploadLogo}
                      disabled={uploadingLogo}
                      className="block w-full text-xs text-zinc-500 file:mr-4 file:py-2 file:px-4 file:rounded-xl file:border-0 file:text-xs file:font-semibold file:bg-gradient-to-r file:from-brand-500 file:to-violet-600 file:text-white file:cursor-pointer hover:file:from-brand-600 hover:file:to-violet-700 disabled:opacity-50 disabled:cursor-not-allowed"
                    />
                    {uploadingLogo && (
                      <span className="text-xs text-zinc-500">Uploading...</span>
                    )}
                  </div>
                  <FieldHint>JPEG, PNG, WebP, or GIF (max 5MB)</FieldHint>
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
          <AvailabilitySettings studio={studio} onSaveSuccess={(msg) => showToast(msg)} />
        )}

        {activeSection === 'plans' && (
          <PlansManagement studioId={studio.id} initialPlans={initialPlans} onSaveSuccess={(msg) => showToast(msg)} />
        )}

        {activeSection === 'integrations' && (
          <div className="grid gap-6 lg:grid-cols-2 items-start">
            <div className="space-y-6">
              {/* Meta App Settings Card */}
              <form onSubmit={onSaveMetaConfig} className="overflow-hidden rounded-[24px] border border-white/30 bg-white/20 backdrop-blur-2xl dark:border-white/5 dark:bg-neutral-900/30 p-6 space-y-5">
                <div>
                  <h3 className="text-xs font-black uppercase tracking-wider text-zinc-400">Meta App Settings</h3>
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

                <div className="flex items-center justify-end border-t border-white/10 pt-4">
                  <Button 
                    type="submit" 
                    loading={metaSaving}
                    className="bg-gradient-to-r from-brand-500 to-violet-600 hover:from-brand-600 hover:to-violet-700 text-white text-xs font-black uppercase tracking-widest shadow-lg shadow-brand-500/25 rounded-xl h-10 px-6"
                  >
                    Save Meta Config
                  </Button>
                </div>
              </form>

              {/* Gemini AI Integration Card */}
              <form onSubmit={onSaveGeminiKey} className="overflow-hidden rounded-[24px] border border-white/30 bg-white/20 backdrop-blur-2xl dark:border-white/5 dark:bg-neutral-900/30 p-6 space-y-5">
                <div>
                  <h3 className="text-xs font-black uppercase tracking-wider text-zinc-400">Gemini AI Integration</h3>
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

                <FieldError message={errors._} />

                <div className="flex items-center justify-end border-t border-white/10 pt-4">
                  <Button 
                    type="submit" 
                    loading={geminiSaving}
                    className="bg-gradient-to-r from-brand-500 to-violet-600 hover:from-brand-600 hover:to-violet-700 text-white text-xs font-black uppercase tracking-widest shadow-lg shadow-brand-500/25 rounded-xl h-10 px-6"
                  >
                    Save Gemini Key
                  </Button>
                </div>
              </form>
            </div>

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

            {/* Delete Account Section */}
            <DeleteAccountForm studioId={studio.id} studioName={studio.name} />
          </div>
        )}
      </div>

      {/* Custom Floating Toast Notification */}
      {toast && (
        <div className="fixed bottom-6 right-6 z-50 p-4 rounded-2xl border border-emerald-500/30 bg-white/90 dark:bg-zinc-900/90 backdrop-blur-xl shadow-2xl flex items-center gap-3 animate-in slide-in-from-bottom-5 fade-in duration-300 min-w-[320px]">
          <div className="h-8 w-8 rounded-xl bg-emerald-500/10 text-emerald-500 flex items-center justify-center shrink-0">
            <CheckCircle2 className="w-5 h-5" />
          </div>
          <div className="flex-1">
            <p className="text-xs font-black uppercase tracking-wider text-zinc-800 dark:text-zinc-100">Success</p>
            <p className="text-[10px] text-zinc-500 dark:text-zinc-400 font-semibold mt-0.5">{toast.message}</p>
          </div>
          <button 
            onClick={() => setToast(null)} 
            className="text-zinc-400 hover:text-zinc-600 dark:hover:text-white p-1 rounded-lg transition-colors"
          >
            <X className="w-4 h-4" />
          </button>
        </div>
      )}

      {activeSection === 'billing' && (
        <PlatformBillingManager studio={studio} />
      )}
    </div>
  );
}

function DeleteAccountForm({ studioId, studioName }: { studioId: string; studioName: string }) {
  const [showDeleteForm, setShowDeleteForm] = useState(false);
  const [deleteEmail, setDeleteEmail] = useState('');
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState('');

  const handleDelete = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');

    if (!deleteEmail) {
      setError('Please enter your email to confirm');
      return;
    }

    if (!window.confirm('⚠️ WARNING: This action cannot be undone. Your account and all data will be permanently deleted. Are you sure?')) {
      return;
    }

    setDeleting(true);
    try {
      const res = await fetch(`/api/v1/me/studios/${studioId}/delete-account`, {
        method: 'DELETE',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
        body: JSON.stringify({ email: deleteEmail }),
      });

      const data = await res.json();
      if (!res.ok) {
        setError(data.error || 'Failed to delete account');
        return;
      }

      alert('✅ Account deleted successfully. You will be logged out.');
      localStorage.removeItem('token');
      window.location.href = '/';
    } catch (err: any) {
      setError(err.message || 'Error deleting account');
    } finally {
      setDeleting(false);
    }
  };

  return (
    <div className="mt-6 max-w-xl">
      <div className="overflow-hidden rounded-[24px] border border-red-500/30 bg-red-500/5 backdrop-blur-2xl dark:border-red-900/30 dark:bg-red-950/10">
        <div className="border-b border-red-500/20 px-6 py-4 dark:border-red-900/20">
          <h3 className="text-xs font-black uppercase tracking-wider text-red-600 dark:text-red-400">Delete Account</h3>
          <p className="text-[10px] text-red-600/70 dark:text-red-400/70 mt-1">Permanently delete your account and all data</p>
        </div>

        {!showDeleteForm ? (
          <div className="p-6">
            <p className="text-sm text-zinc-600 dark:text-zinc-400 mb-4">
              Once you delete your account, there is no going back. Please be certain.
            </p>
            <Button
              onClick={() => setShowDeleteForm(true)}
              className="bg-red-600 hover:bg-red-700 text-white text-xs font-black uppercase tracking-widest rounded-xl h-10 px-6"
            >
              Delete Account Permanently
            </Button>
          </div>
        ) : (
          <form onSubmit={handleDelete} className="p-6 space-y-4">
            <div>
              <Label htmlFor="deleteEmail">Confirm your email</Label>
              <Input
                id="deleteEmail"
                type="email"
                placeholder="Enter your email to confirm"
                value={deleteEmail}
                onChange={(e) => setDeleteEmail(e.target.value)}
                className="mt-1"
              />
              <FieldHint>This is {studioName}'s account email</FieldHint>
              {error && <FieldError message={error} />}
            </div>

            <div className="bg-red-500/10 border border-red-500/20 rounded-lg p-3 text-xs text-red-700 dark:text-red-400">
              ⚠️ <strong>Warning:</strong> This will permanently delete your account, all conversations, and all data associated with {studioName}. This action cannot be undone.
            </div>

            <div className="flex items-center gap-2 border-t border-red-500/20 pt-4">
              <Button
                type="button"
                variant="ghost"
                onClick={() => {
                  setShowDeleteForm(false);
                  setDeleteEmail('');
                  setError('');
                }}
                className="flex-1 text-xs"
              >
                Cancel
              </Button>
              <Button
                type="submit"
                loading={deleting}
                className="flex-1 bg-red-600 hover:bg-red-700 text-white text-xs font-black"
              >
                Permanently Delete
              </Button>
            </div>
          </form>
        )}
      </div>
    </div>
  );
}

function PlatformBillingManager({ studio }: { studio: Studio }) {
  const [plans, setPlans] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const [syncing, setSyncing] = useState(false);
  const [syncedTier, setSyncedTier] = useState<string | null>(null);

  useEffect(() => {
    // Sync billing status from Stripe on page load (fallback when webhooks don't fire locally)
    // Always sync if returning from Stripe with upgrade=success
    const urlParams = new URLSearchParams(window.location.search);
    const justUpgraded = urlParams.get('upgrade') === 'success';

    const syncBilling = async () => {
      setSyncing(true);
      try {
        const res = await fetch(`/api/v1/me/studios/${studio.id}/billing/sync`, {
          method: 'POST',
          headers: { Authorization: `Bearer ${localStorage.getItem('token')}` },
        });
        const data = await res.json();
        if (data.synced && (data.tier !== studio.subscriptionTier || justUpgraded)) {
          // Tier changed — clean up URL and reload so AppShell picks up the new state
          const newUrl = window.location.pathname;
          window.history.replaceState({}, '', newUrl);
          window.location.reload();
          return;
        }
        setSyncedTier(data.tier || studio.subscriptionTier);
      } catch {
        setSyncedTier(studio.subscriptionTier || null);
      } finally {
        setSyncing(false);
      }
    };
    syncBilling();

    fetch('/api/v1/public/platform/plans')
      .then(res => res.json())
      .then(data => {
        if (Array.isArray(data)) setPlans(data);
      })
      .catch(err => console.error(err))
      .finally(() => setLoading(false));
  }, [studio.id]);


  const handleAction = async (planName: string, isUpgrade: boolean) => {
    setActionLoading(planName);
    try {
      const endpoint = 'billing/upgrade';
      const res = await fetch(`/api/v1/me/studios/${studio.id}/${endpoint}`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${localStorage.getItem('token')}`,
        },
        body: JSON.stringify({ tier: planName }),
      });
      const data = await res.json();
      if (data.url) {
        window.open(data.url, '_blank');
        // Poll for updated tier every 2 seconds after opening Stripe
        const pollInterval = setInterval(async () => {
          try {
            const syncRes = await fetch(`/api/v1/me/studios/${studio.id}/billing/sync`, {
              method: 'POST',
              headers: { Authorization: `Bearer ${localStorage.getItem('token')}` },
            });
            const syncData = await syncRes.json();
            if (syncData.synced && syncData.tier && syncData.tier !== studio.subscriptionTier) {
              clearInterval(pollInterval);
              window.location.reload();
            }
          } catch (err) {
            console.error('Sync poll error:', err);
          }
        }, 2000);
        // Stop polling after 5 minutes
        setTimeout(() => clearInterval(pollInterval), 5 * 60 * 1000);
      } else {
        alert(data.error || 'Failed to initialize checkout');
      }
    } catch (e) {
      console.error(e);
      alert('Network error initializing billing flow');
    } finally {
      setActionLoading(null);
    }
  };

  if (loading) {
    return <div className="p-8 text-center text-sm font-bold text-zinc-500">Loading plans...</div>;
  }

  const currentTier = studio.subscriptionTier || 'Trial Pass';
  const isCanceledOrPastDue = currentTier === 'past_due' || currentTier === 'canceled';
  const hasSubscription = currentTier !== 'Trial Pass' && !isCanceledOrPastDue && !!studio.subscriptionTier;

  return (
    <div className="space-y-6">
      <div>
        <h3 className="text-xs font-black uppercase tracking-wider text-zinc-400 mb-1">
          Platform Billing
        </h3>
        <p className="text-[10px] text-zinc-500">
          Manage your studio's platform subscription. Upgrade or downgrade your plan to unlock more features.
        </p>
      </div>

      <div className={`relative overflow-hidden rounded-[24px] border border-brand-500/30 bg-white/20 dark:bg-brand-950/20 backdrop-blur-2xl p-6 transition-all duration-300 shadow-lg shadow-brand-500/10`}>
        <div className="flex justify-between items-start mb-2">
          <div>
            <h4 className="text-xl font-black text-zinc-900 dark:text-white">Current Plan</h4>
            <p className="text-[10px] font-bold uppercase tracking-widest text-zinc-500 mt-1">
              {isCanceledOrPastDue ? 'Subscription Paused' : 'Currently Active'}
            </p>
          </div>
          <span className={`text-[10px] font-bold px-3 py-1 rounded-full uppercase tracking-wider transition-colors ${isCanceledOrPastDue ? 'bg-red-500/10 text-red-600 dark:text-red-400' : 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'}`}>
            {currentTier}
          </span>
        </div>
        {isCanceledOrPastDue && (
          <div className="mt-4 rounded-xl bg-red-500/20 p-4 border border-red-500/30 text-red-700 dark:text-red-400 font-bold text-xs">
            Warning: Your subscription payment is past due or canceled. Your access to other pages has been paused until the issue is resolved. Please select a plan below to re-activate your workspace!
          </div>
        )}
        {hasSubscription && (
          <div className="mt-6 border-t border-brand-500/20 pt-4">
            <Button
              variant="outline"
              size="sm"
              className="w-full text-xs font-bold"
              onClick={async () => {
                const res = await fetch(`/api/v1/me/studios/${studio.id}/billing/portal`, { method: 'POST' });
                const data = await res.json();
                if (data.url) window.open(data.url, '_blank');
              }}
            >
              Manage Billing Methods & Invoices
            </Button>
          </div>
        )}
      </div>

      <div className="grid gap-6 sm:grid-cols-2">
        {plans.map((plan, idx) => {
          const isCurrent = currentTier === plan.name;
          return (
            <div
              key={plan.name || idx}
              className={`relative overflow-hidden rounded-[24px] border ${
                isCurrent
                  ? 'border-brand-500/30 shadow-lg shadow-brand-500/10 bg-white/20 dark:bg-brand-950/20'
                  : 'border-white/10 bg-white/5 dark:bg-white/5 opacity-80'
              } backdrop-blur-2xl p-6 transition-all duration-300 flex flex-col`}
            >
              <div className="flex justify-between items-start mb-4">
                <div>
                  <h4 className="text-xl font-black text-zinc-900 dark:text-white flex items-center gap-2">
                    {plan.name}
                    {isCurrent && <CheckCircle2 className="h-4 w-4 text-brand-500" />}
                  </h4>
                  <p className="text-[10px] font-bold uppercase tracking-widest text-zinc-500 mt-1">
                    {plan.cycle?.toLowerCase() === 'one-time' ? 'One Time' : 'Monthly'}
                  </p>
                </div>
              </div>

              <div className="space-y-4 flex-1 flex flex-col">
                <p className="text-[11px] font-medium text-zinc-500 min-h-[32px] leading-relaxed">
                  {plan.description}
                </p>
                <div className="flex items-baseline gap-1 my-2">
                  <span className="text-3xl font-black tracking-tight text-zinc-900 dark:text-white">
                    S$ {plan.price}
                  </span>
                  {plan.cycle?.toLowerCase() !== 'one-time' && (
                    <span className="text-sm font-semibold text-zinc-500">
                      /mo
                    </span>
                  )}
                </div>
                <div className="space-y-2 flex-1">
                  {plan.features?.map((f: string, i: number) => (
                    <div key={i} className="flex items-start gap-2 text-sm text-zinc-700 dark:text-zinc-300">
                      <CheckCircle2 className="h-4 w-4 shrink-0 text-emerald-500 mt-0.5" />
                      <span>{f}</span>
                    </div>
                  ))}
                </div>
                <Button
                  variant={isCurrent ? 'outline' : 'primary'}
                  onClick={() => handleAction(plan.name, hasSubscription)}
                  loading={actionLoading === plan.name}
                  disabled={isCurrent}
                  className="w-full mt-4 rounded-xl border-white/20 hover:bg-white/10 text-xs font-bold uppercase tracking-wider"
                >
                  {isCurrent ? 'Current Plan' : (hasSubscription ? 'Change Plan' : 'Select Plan')}
                </Button>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
