'use client';

import { useRouter, useSearchParams, usePathname } from 'next/navigation';
import { useState, useEffect } from 'react';
import { Eye, EyeOff, Database, Building, Calendar, Cpu, Lock, Save, CheckCircle2, X, DollarSign, Upload, Loader2, Timer } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { FieldError, FieldHint, Label } from '@/components/ui/Label';
import type { Studio } from '@/lib/types';
import {
  changeMyPassword,
  updateStudioSettings,
  getSheetsSettings,
  saveSheetsSettings,
  getExternalLeadsSheetSettings,
  saveExternalLeadsSheetSettings,
  getWhatsAppSendSpacing,
  saveWhatsAppSendSpacing,
  getInitialContactDelay,
  saveInitialContactDelay,
  getAIReplyDelay,
  saveAIReplyDelay,
} from './actions';
import { AvailabilitySettings } from './AvailabilitySettings';
import { PlansManagement } from './PlansManagement';

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

type SettingsSection = 'general' | 'plans' | 'availability' | 'booking' | 'sheets' | 'integrations' | 'security' | 'billing';
const VALID_SECTIONS: SettingsSection[] = ['general', 'plans', 'availability', 'booking', 'sheets', 'integrations', 'security', 'billing'];

export function SettingsForm({ studio, previewHref, initialPlans }: { studio: Studio; previewHref: string | null; initialPlans: any[] }) {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const initialSection = (VALID_SECTIONS.includes(searchParams.get('tab') as SettingsSection)
    ? searchParams.get('tab')
    : 'general') as SettingsSection;

  const [name, setName] = useState(studio.name);
  const [brandColor, setBrandColor] = useState(studio.brandColor);
  const [logoUrl, setLogoUrl] = useState(studio.logoUrl);
  const [logoError, setLogoError] = useState(false);
  useEffect(() => {
    setLogoError(false);
  }, [logoUrl]);
  const [contactEmail, setContactEmail] = useState(studio.contactEmail);
  const initialPhone = parsePhone(studio.contactPhone || '');
  const [contactPhoneCountryCode, setContactPhoneCountryCode] = useState(initialPhone.countryCode);
  const [contactPhone, setContactPhone] = useState(initialPhone.phoneNumber);
  const [geminiApiKey, setGeminiApiKey] = useState('');
  const [groqApiKey, setGroqApiKey] = useState('');
  const [metaAppId, setMetaAppId] = useState(studio.metaAppId || '');
  const [metaAppSecret, setMetaAppSecret] = useState('');
  const [googleClientId, setGoogleClientId] = useState(studio.googleClientId || '');
  const [googleClientSecret, setGoogleClientSecret] = useState('');
  const [active, setActive] = useState(studio.active);
  const [managedBy1Hero, setManagedBy1Hero] = useState(studio.managedBy1Hero || false);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);
  
  const [activeSection, _setActiveSection] = useState<SettingsSection>(initialSection);
  const setActiveSection = (section: SettingsSection) => {
    _setActiveSection(section);
    const p = new URLSearchParams(searchParams.toString());
    p.set('tab', section);
    router.replace(`${pathname}?${p.toString()}`, { scroll: false });
  };
  
  // Booking page media
  const [bookingHeroImageUrl, setBookingHeroImageUrl] = useState(studio.bookingHeroImageUrl || '');
  const [bookingHeroVideoUrl, setBookingHeroVideoUrl] = useState(studio.bookingHeroVideoUrl || '');
  const [mediaUploading, setMediaUploading] = useState<'image' | 'video' | null>(null);
  const [mediaSaving, setMediaSaving] = useState(false);

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

  // External (third-party company) leads sheet — read-only polling, separate
  // from the Google Sheets Sync export above.
  const [extSpreadsheetId, setExtSpreadsheetId] = useState('');
  const [extTabName, setExtTabName] = useState('Sheet1');
  const [extNameColumn, setExtNameColumn] = useState('');
  const [extFirstNameColumn, setExtFirstNameColumn] = useState('A');
  const [extLastNameColumn, setExtLastNameColumn] = useState('B');
  const [extEmailColumn, setExtEmailColumn] = useState('C');
  const [extPhoneColumn, setExtPhoneColumn] = useState('D');
  const [extSourceColumn, setExtSourceColumn] = useState('');
  const [extNotesColumn, setExtNotesColumn] = useState('');
  const [extDateColumn, setExtDateColumn] = useState('');
  const [extHotLeadColumn, setExtHotLeadColumn] = useState('');
  const [extTrialPurchasedColumn, setExtTrialPurchasedColumn] = useState('');
  // No dedicated input here (the toggle lives in the Inbox top bar) — just
  // round-tripped so saving from this form doesn't silently reset it.
  const [extContinueAIAfterGreeting, setExtContinueAIAfterGreeting] = useState(true);
  const [extAutoContactEnabled, setExtAutoContactEnabled] = useState(true);
  const [extActive, setExtActive] = useState(false);
  const [extSaving, setExtSaving] = useState(false);
  const [extError, setExtError] = useState<string | null>(null);
  const [sendSpacingSeconds, setSendSpacingSeconds] = useState(20);
  const [sendSpacingSaving, setSendSpacingSaving] = useState(false);
  const [sendSpacingError, setSendSpacingError] = useState<string | null>(null);
  const [initialDelayValue, setInitialDelayValue] = useState(0);
  const [initialDelayUnit, setInitialDelayUnit] = useState<'minutes' | 'hours'>('minutes');
  const [aiReplyDelaySeconds, setAiReplyDelaySeconds] = useState(0);
  const [initialDelaySaving, setInitialDelaySaving] = useState(false);
  const [initialDelayError, setInitialDelayError] = useState<string | null>(null);
  const [metaSaving, setMetaSaving] = useState(false);
  const [googleSaving, setGoogleSaving] = useState(false);
  const [showGoogleClientSecret, setShowGoogleClientSecret] = useState(false);
  const [geminiSaving, setGeminiSaving] = useState(false);
  const [groqSaving, setGroqSaving] = useState(false);
  const [showGroqApiKey, setShowGroqApiKey] = useState(false);

  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null);

  const showToast = (message: string, type: 'success' | 'error' = 'success') => {
    setToast({ message, type });
  };

  const [showGeminiApiKey, setShowGeminiApiKey] = useState(false);
  const [showMetaAppSecret, setShowMetaAppSecret] = useState(false);
  const [uploadingLogo, setUploadingLogo] = useState(false);

  useEffect(() => {
    // Fetch both sheet integrations together — the external (import) sheet's
    // Spreadsheet ID prefills from the primary (export) sheet's ID when the
    // former hasn't been set yet, since studios usually reuse the same sheet.
    void (async () => {
      const [sheetsRes, extRes] = await Promise.all([
        getSheetsSettings(studio.id),
        getExternalLeadsSheetSettings(studio.id),
      ]);

      let primarySpreadsheetId = '';
      if (sheetsRes.ok && sheetsRes.data) {
        primarySpreadsheetId = sheetsRes.data.spreadsheetId || '';
        setSpreadsheetId(primarySpreadsheetId);
        setTabName(sheetsRes.data.tabName || 'Leads');
        setSheetsActive(sheetsRes.data.active || false);
      }

      if (extRes.ok && extRes.data) {
        setExtSpreadsheetId(extRes.data.spreadsheetId || primarySpreadsheetId);
        setExtTabName(extRes.data.tabName || 'Sheet1');
        setExtNameColumn(extRes.data.nameColumn || '');
        setExtFirstNameColumn(extRes.data.firstNameColumn || 'A');
        setExtLastNameColumn(extRes.data.lastNameColumn || 'B');
        setExtEmailColumn(extRes.data.emailColumn || 'C');
        setExtPhoneColumn(extRes.data.phoneColumn || 'D');
        setExtSourceColumn(extRes.data.sourceColumn || '');
        setExtNotesColumn(extRes.data.notesColumn || '');
        setExtDateColumn(extRes.data.dateColumn || '');
        setExtHotLeadColumn(extRes.data.hotLeadColumn || '');
        setExtContinueAIAfterGreeting(extRes.data.continueAiAfterGreeting ?? true);
        setExtAutoContactEnabled(extRes.data.autoContactEnabled ?? true);
        setExtTrialPurchasedColumn(extRes.data.trialPurchasedColumn || '');
        setExtActive(extRes.data.active || false);
      }
    })();
    void (async () => {
      const res = await getWhatsAppSendSpacing(studio.id);
      if (res.ok && res.data) {
        setSendSpacingSeconds(res.data.whatsappSendSpacingSeconds ?? 20);
      }
    })();
    void (async () => {
      const res = await getInitialContactDelay(studio.id);
      if (res.ok && res.data) {
        const minutes = res.data.initialContactDelayMinutes ?? 0;
        if (minutes > 0 && minutes % 60 === 0) {
          setInitialDelayValue(minutes / 60);
          setInitialDelayUnit('hours');
        } else {
          setInitialDelayValue(minutes);
          setInitialDelayUnit('minutes');
        }
      }
    })();
    void (async () => {
      const res = await getAIReplyDelay(studio.id);
      if (res.ok && res.data) {
        setAiReplyDelaySeconds(res.data.aiReplyDelaySeconds ?? 0);
      }
    })();
  }, [studio.id]);

  async function onSaveSendSpacing(e: React.FormEvent) {
    e.preventDefault();
    setSendSpacingError(null);
    setSendSpacingSaving(true);
    try {
      const res = await saveWhatsAppSendSpacing(studio.id, sendSpacingSeconds);
      if (res.ok) {
        showToast('WhatsApp message pacing saved successfully.');
      } else {
        setSendSpacingError(res.error || 'Failed to save pacing.');
      }
    } catch (err: any) {
      setSendSpacingError(err.message || 'An error occurred.');
    } finally {
      setSendSpacingSaving(false);
    }
  }

  async function onSaveInitialDelay(e: React.FormEvent) {
    e.preventDefault();
    setInitialDelayError(null);
    setInitialDelaySaving(true);
    try {
      const minutes = initialDelayUnit === 'hours' ? initialDelayValue * 60 : initialDelayValue;
      const [delayRes, aiReplyRes] = await Promise.all([
        saveInitialContactDelay(studio.id, minutes),
        saveAIReplyDelay(studio.id, aiReplyDelaySeconds),
      ]);
      if (delayRes.ok && aiReplyRes.ok) {
        showToast('Message timing saved successfully.');
      } else {
        setInitialDelayError(delayRes.error || aiReplyRes.error || 'Failed to save timing.');
      }
    } catch (err: any) {
      setInitialDelayError(err.message || 'An error occurred.');
    } finally {
      setInitialDelaySaving(false);
    }
  }

  async function onSaveExternalLeadsSheetSettings(e: React.FormEvent) {
    e.preventDefault();
    setExtError(null);
    setExtSaving(true);
    try {
      const res = await saveExternalLeadsSheetSettings(studio.id, {
        spreadsheetId: extSpreadsheetId,
        tabName: extTabName,
        nameColumn: extNameColumn,
        firstNameColumn: extFirstNameColumn,
        lastNameColumn: extLastNameColumn,
        emailColumn: extEmailColumn,
        phoneColumn: extPhoneColumn,
        sourceColumn: extSourceColumn,
        notesColumn: extNotesColumn,
        dateColumn: extDateColumn,
        hotLeadColumn: extHotLeadColumn,
        trialPurchasedColumn: extTrialPurchasedColumn,
        continueAiAfterGreeting: extContinueAIAfterGreeting,
        autoContactEnabled: extAutoContactEnabled,
        active: extActive,
      });
      if (res.ok) {
        showToast('External leads sheet connection saved successfully.');
      } else {
        setExtError(res.error || 'Failed to save settings.');
      }
    } catch (err: any) {
      setExtError(err.message || 'An error occurred.');
    } finally {
      setExtSaving(false);
    }
  }

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
      const cleanPhone = contactPhone.replace(/\D/g, '');
      const fullPhone = cleanPhone ? `${contactPhoneCountryCode}${cleanPhone}` : '';
      const result = await updateStudioSettings(studio.id, studio.slug, {
        name,
        brandColor,
        logoUrl,
        contactEmail,
        contactPhone: fullPhone,
        active,
        managedBy1Hero,
        trialAmountSgd: Math.round(trialAmountSgd * 100),
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

  async function onSaveGroqKey(e: React.FormEvent) {
    e.preventDefault();
    setErrors({});
    setGroqSaving(true);
    try {
      const result = await updateStudioSettings(studio.id, studio.slug, {
        groqApiKey,
      });
      if (!result.ok) {
        setErrors(result.details ?? { _: result.error });
        return;
      }
      showToast('Groq API Key saved successfully.');
    } finally {
      setGroqSaving(false);
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

  async function onSaveGoogleConfig(e: React.FormEvent) {
    e.preventDefault();
    setErrors({});
    setGoogleSaving(true);
    try {
      const result = await updateStudioSettings(studio.id, studio.slug, {
        googleClientId,
        googleClientSecret,
      });
      if (!result.ok) {
        setErrors(result.details ?? { _: result.error });
        return;
      }
      showToast('Google Ads config saved successfully.');
    } finally {
      setGoogleSaving(false);
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
          { id: 'booking', label: 'Booking Page', icon: Eye },
          { id: 'sheets', label: 'Google Sheets', icon: Database },
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
              className={`flex items-center gap-3 rounded px-4 py-3 text-xs font-black uppercase tracking-wider transition-all duration-200 shrink-0 whitespace-nowrap ${
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
              <form onSubmit={onSubmit} className="overflow-hidden rounded border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-950 p-6 space-y-5">
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

                <div>
                  <Label htmlFor="contactPhone">Contact phone</Label>
                  <div className="flex gap-2">
                    <select
                      value={contactPhoneCountryCode}
                      onChange={(e) => setContactPhoneCountryCode(e.target.value)}
                      className="h-10 rounded-xl border border-white/20 bg-white/5 dark:bg-slate-950 dark:border-slate-800 text-slate-900 dark:text-slate-100 px-3 py-2 text-sm focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500 w-[110px] shrink-0"
                    >
                      {COUNTRY_CODES.map((c) => (
                        <option key={c.code} value={c.code}>
                          {c.code}
                        </option>
                      ))}
                    </select>
                    <Input
                      id="contactPhone"
                      type="tel"
                      placeholder="e.g. 81234567"
                      invalid={!!errors.contactPhone}
                      value={contactPhone}
                      onChange={(e) => setContactPhone(e.target.value)}
                    />
                  </div>
                  <FieldError message={errors.contactPhone} />
                </div>

                <div>
                  <Label htmlFor="trialAmountSgd">Trial Price (S$)</Label>
                  <Input
                    id="trialAmountSgd"
                    type="number"
                    min={0}
                    step={0.01}
                    value={trialAmountSgd}
                    onChange={(e) => setTrialAmountSgd(Math.max(0, Number(e.target.value) || 0))}
                  />
                  <FieldHint>
                    The price shown and charged on the trial payment page (including the shareable static signup link).
                    If left at S$0, the page falls back to a default of S$25.
                  </FieldHint>
                </div>

                <div className="flex items-center gap-3 p-3 rounded border border-zinc-100 bg-zinc-50 dark:bg-zinc-900/50 dark:border-zinc-800/50">
                  <input
                    type="checkbox"
                    id="active"
                    checked={active}
                    onChange={(e) => setActive(e.target.checked)}
                    className="h-4 w-4 rounded border-zinc-300 text-brand-500 focus:ring-brand-500 cursor-pointer"
                    suppressHydrationWarning
                  />
                  <div>
                    <Label htmlFor="active" className="mb-0 cursor-pointer text-xs font-black uppercase tracking-wider">Studio is active</Label>
                    <p className="text-[10px] text-zinc-400">Inactive studios stop accepting public form submissions.</p>
                  </div>
                </div>

                <div className="flex items-center gap-3 p-3 rounded border border-zinc-100 bg-zinc-50 dark:bg-zinc-900/50 dark:border-zinc-800/50">
                  <input
                    type="checkbox"
                    id="managedBy1Hero"
                    checked={managedBy1Hero}
                    onChange={(e) => setManagedBy1Hero(e.target.checked)}
                    className="h-4 w-4 rounded border-zinc-300 text-brand-500 focus:ring-brand-500 cursor-pointer"
                    suppressHydrationWarning
                  />
                  <div>
                    <Label htmlFor="managedBy1Hero" className="mb-0 cursor-pointer text-xs font-black uppercase tracking-wider">Managed by 1Hero</Label>
                    <p className="text-[10px] text-zinc-400">Super admins can access this studio. Uncheck to restrict access.</p>
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
                    className="bg-brand-500 hover:bg-brand-600 text-white text-xs font-black uppercase tracking-widest rounded h-10 px-6"
                  >
                    Save Changes
                  </Button>
                </div>
              </form>
            </div>

            {/* Live Preview Column */}
            <div className="lg:col-span-1">
              <div className="overflow-hidden rounded border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-950">
                <div className="border-b border-zinc-200 px-6 py-4 dark:border-zinc-800">
                  <h3 className="text-xs font-black uppercase tracking-wider text-zinc-400">Live Preview</h3>
                </div>
                <div className="p-6 space-y-4">
                  <div className="rounded border border-zinc-100 bg-zinc-50 p-4 dark:bg-neutral-800/20">
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

        {activeSection === 'booking' && (
          <div className="space-y-6">
            <div className="overflow-hidden rounded border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-950 p-6 space-y-5">
              <div>
                <h3 className="text-sm font-black text-zinc-950 dark:text-white">Booking Page Media</h3>
                <p className="text-xs text-zinc-400 mt-1">Add a hero image or background video shown to leads on your public booking/checkout page.</p>
              </div>

              {/* Hero Image */}
              <div className="space-y-2">
                <Label htmlFor="heroImage">Hero Image</Label>
                <div className="flex gap-2">
                  <input
                    id="heroImage"
                    type="url"
                    value={bookingHeroImageUrl}
                    onChange={e => setBookingHeroImageUrl(e.target.value)}
                    placeholder="https://… or upload below"
                    className="flex-1 rounded-xl border border-zinc-200 dark:border-white/10 bg-white dark:bg-neutral-800 px-3 py-2 text-sm text-zinc-800 dark:text-white placeholder:text-zinc-400 focus:outline-none focus:border-zinc-400"
                  />
                  <label className="cursor-pointer">
                    <input type="file" accept="image/*" className="hidden"
                      onChange={async e => {
                        const file = e.target.files?.[0]; if (!file) return;
                        setMediaUploading('image');
                        try {
                          const fd = new FormData(); fd.append('image', file);
                          const res = await fetch(`/api/v1/studios/${studio.id}/social-posts/upload-image`, { method: 'POST', body: fd });
                          if (!res.ok) throw new Error('Upload failed');
                          const { url } = await res.json();
                          setBookingHeroImageUrl(url);
                        } catch { alert('Upload failed'); }
                        finally { setMediaUploading(null); }
                      }}
                    />
                    <span className={`inline-flex items-center gap-1.5 rounded-xl border px-3 py-2 text-xs font-bold transition-all ${mediaUploading === 'image' ? 'bg-zinc-200 text-zinc-400' : 'border-zinc-200 dark:border-white/10 bg-white dark:bg-neutral-800 text-zinc-600 dark:text-zinc-300 hover:bg-zinc-50 dark:hover:bg-neutral-700 cursor-pointer'}`}>
                      {mediaUploading === 'image' ? <Loader2 className="h-3 w-3 animate-spin" /> : <Upload className="h-3 w-3" />}
                      Upload
                    </span>
                  </label>
                </div>
                {bookingHeroImageUrl && (
                  <div className="relative mt-2 rounded-xl overflow-hidden border border-zinc-200 dark:border-white/10 h-36">
                    {/* eslint-disable-next-line @next/next/no-img-element */}
                    <img src={bookingHeroImageUrl} alt="Hero preview" className="w-full h-full object-cover" />
                    <button type="button" onClick={() => setBookingHeroImageUrl('')}
                      className="absolute top-2 right-2 h-6 w-6 rounded-full bg-black/50 text-white flex items-center justify-center hover:bg-black/70 transition-colors">
                      <X className="h-3 w-3" />
                    </button>
                  </div>
                )}
              </div>

              {/* Hero Video */}
              <div className="space-y-2">
                <Label htmlFor="heroVideo">Background Video URL</Label>
                <div className="flex gap-2">
                  <input
                    id="heroVideo"
                    type="url"
                    value={bookingHeroVideoUrl}
                    onChange={e => setBookingHeroVideoUrl(e.target.value)}
                    placeholder="https://… (mp4 or YouTube embed)"
                    className="flex-1 rounded-xl border border-zinc-200 dark:border-white/10 bg-white dark:bg-neutral-800 px-3 py-2 text-sm text-zinc-800 dark:text-white placeholder:text-zinc-400 focus:outline-none focus:border-zinc-400"
                  />
                  {bookingHeroVideoUrl && (
                    <button type="button" onClick={() => setBookingHeroVideoUrl('')}
                      className="rounded-xl border border-zinc-200 dark:border-white/10 bg-white dark:bg-neutral-800 px-3 py-2 text-xs font-bold text-zinc-500 hover:bg-zinc-50 dark:hover:bg-neutral-700 transition-colors">
                      <X className="h-3 w-3" />
                    </button>
                  )}
                </div>
                <p className="text-[11px] text-zinc-400">Paste an .mp4 URL. Video plays muted in the background of the booking page.</p>
              </div>

              <Button
                type="button"
                loading={mediaSaving}
                onClick={async () => {
                  setMediaSaving(true);
                  try {
                    const res = await fetch(`/api/v1/me/studios/${studio.id}`, {
                      method: 'PUT',
                      headers: { 'Content-Type': 'application/json' },
                      body: JSON.stringify({ bookingHeroImageUrl, bookingHeroVideoUrl }),
                    });
                    if (!res.ok) throw new Error('Save failed');
                    showToast('Booking page media saved!');
                  } catch { showToast('Failed to save.'); }
                  finally { setMediaSaving(false); }
                }}
                className="text-sm font-bold"
              >
                <Save className="h-4 w-4" />Save Media
              </Button>
            </div>
          </div>
        )}

        {activeSection === 'sheets' && (
          <div className="grid gap-6 lg:grid-cols-2 items-start">
            {/* Google Sheets Card */}
            <div className="overflow-hidden rounded border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-950">
              <div className="border-b border-zinc-200 px-6 py-4 dark:border-zinc-800 flex items-center gap-2">
                <Database className="h-4 w-4 text-brand-500" />
                <h3 className="text-xs font-black uppercase tracking-wider text-zinc-400">Google Sheets Sync</h3>
              </div>
              <form onSubmit={onSaveSheetsSettings} className="space-y-4 p-6">
                <div>
                  <Label htmlFor="spreadsheetId">Spreadsheet ID or URL</Label>
                  <Input
                    id="spreadsheetId"
                    placeholder="Paste full Google Sheets link or just the ID"
                    value={spreadsheetId}
                    onChange={(e) => setSpreadsheetId(e.target.value)}
                  />
                  <FieldHint>Paste the full Google Sheets URL or just the spreadsheet ID — both work</FieldHint>
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

                <div className="flex items-center gap-3 p-3 rounded border border-zinc-100 bg-zinc-50 dark:bg-zinc-900/50 dark:border-zinc-800/50">
                  <input
                    type="checkbox"
                    id="sheetsActive"
                    checked={sheetsActive}
                    onChange={(e) => setSheetsActive(e.target.checked)}
                    className="h-4 w-4 rounded border-zinc-300 text-brand-500 focus:ring-brand-500 cursor-pointer"
                  />
                  <div>
                    <Label htmlFor="sheetsActive" className="mb-0 cursor-pointer text-xs font-black uppercase tracking-wider">Enable Google Sheets Sync</Label>
                    <p className="text-[10px] text-zinc-400">Automatically synchronize lead submissions with your Google Spreadsheet.</p>
                  </div>
                </div>

                {sheetsError ? (
                  <p className="text-xs font-black text-rose-500 uppercase tracking-wider">{sheetsError}</p>
                ) : null}

                <div className="flex items-center justify-end border-t border-zinc-200 dark:border-zinc-800 pt-4">
                  <Button
                    type="submit"
                    loading={sheetsSaving}
                    className="bg-brand-500 hover:bg-brand-600 text-white text-xs font-black uppercase tracking-widest rounded h-10 px-6"
                  >
                    Save Connection
                  </Button>
                </div>
              </form>
            </div>

            {/* External Leads Sheet Card — read-only import from a third-party company's sheet */}
            <div className="overflow-hidden rounded border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-950">
              <div className="border-b border-zinc-200 px-6 py-4 dark:border-zinc-800 flex items-center gap-2">
                <Database className="h-4 w-4 text-brand-500" />
                <h3 className="text-xs font-black uppercase tracking-wider text-zinc-400">External Leads Sheet (Import)</h3>
              </div>
              <form onSubmit={onSaveExternalLeadsSheetSettings} className="space-y-4 p-6">
                <p className="text-[10px] text-zinc-400">
                  Poll a read-only Google Sheet owned by an external company for new lead rows.
                  Share the sheet with our service account as <strong>Viewer</strong> — we never write to it.
                  New rows are imported as leads and automatically trigger the WhatsApp automation.
                </p>

                <div>
                  <Label htmlFor="extSpreadsheetId">Spreadsheet ID or URL</Label>
                  <Input
                    id="extSpreadsheetId"
                    placeholder="Paste full Google Sheets link or just the ID"
                    value={extSpreadsheetId}
                    onChange={(e) => setExtSpreadsheetId(e.target.value)}
                  />
                  <FieldHint>Paste the full Google Sheets URL or just the ID — both work</FieldHint>
                </div>

                <div>
                  <Label htmlFor="extTabName">Tab Name</Label>
                  <Input
                    id="extTabName"
                    placeholder="Sheet1"
                    value={extTabName}
                    onChange={(e) => setExtTabName(e.target.value)}
                  />
                </div>

                <div className="grid grid-cols-2 gap-4 sm:grid-cols-3">
                  <div>
                    <Label htmlFor="extFirstNameColumn">First Name Col</Label>
                    <Input
                      id="extFirstNameColumn"
                      placeholder="A"
                      value={extFirstNameColumn}
                      onChange={(e) => setExtFirstNameColumn(e.target.value)}
                    />
                  </div>
                  <div>
                    <Label htmlFor="extLastNameColumn">Last Name Col</Label>
                    <Input
                      id="extLastNameColumn"
                      placeholder="B"
                      value={extLastNameColumn}
                      onChange={(e) => setExtLastNameColumn(e.target.value)}
                    />
                  </div>
                  <div>
                    <Label htmlFor="extNameColumn">Full Name Col</Label>
                    <Input
                      id="extNameColumn"
                      placeholder="(optional)"
                      value={extNameColumn}
                      onChange={(e) => setExtNameColumn(e.target.value)}
                    />
                  </div>
                  <div>
                    <Label htmlFor="extEmailColumn">Email Col</Label>
                    <Input
                      id="extEmailColumn"
                      placeholder="C"
                      value={extEmailColumn}
                      onChange={(e) => setExtEmailColumn(e.target.value)}
                    />
                  </div>
                  <div>
                    <Label htmlFor="extPhoneColumn">Phone Col</Label>
                    <Input
                      id="extPhoneColumn"
                      placeholder="D"
                      value={extPhoneColumn}
                      onChange={(e) => setExtPhoneColumn(e.target.value)}
                    />
                  </div>
                  <div>
                    <Label htmlFor="extSourceColumn">Source Col</Label>
                    <Input
                      id="extSourceColumn"
                      placeholder="(optional)"
                      value={extSourceColumn}
                      onChange={(e) => setExtSourceColumn(e.target.value)}
                    />
                  </div>
                  <div>
                    <Label htmlFor="extNotesColumn">Notes Col</Label>
                    <Input
                      id="extNotesColumn"
                      placeholder="(optional)"
                      value={extNotesColumn}
                      onChange={(e) => setExtNotesColumn(e.target.value)}
                    />
                  </div>
                  <div>
                    <Label htmlFor="extDateColumn">Date Col</Label>
                    <Input
                      id="extDateColumn"
                      placeholder="(optional)"
                      value={extDateColumn}
                      onChange={(e) => setExtDateColumn(e.target.value)}
                    />
                  </div>
                  <div>
                    <Label htmlFor="extHotLeadColumn">Hot Lead Col</Label>
                    <Input
                      id="extHotLeadColumn"
                      placeholder="(optional)"
                      value={extHotLeadColumn}
                      onChange={(e) => setExtHotLeadColumn(e.target.value)}
                    />
                  </div>
                  <div>
                    <Label htmlFor="extTrialPurchasedColumn">Trial Purchased Col</Label>
                    <Input
                      id="extTrialPurchasedColumn"
                      placeholder="(optional)"
                      value={extTrialPurchasedColumn}
                      onChange={(e) => setExtTrialPurchasedColumn(e.target.value)}
                    />
                  </div>
                </div>
                <FieldHint>
                  Hot Lead Col values HOT/WARM/COLD: only WARM leads get auto-contacted. Trial Purchased Col = YES also skips auto-contact.
                </FieldHint>
                <FieldHint>
                  Use the full name column if the sheet has one combined column instead of separate first/last name columns.
                </FieldHint>

                <div className="flex items-center gap-3 p-3 rounded border border-zinc-100 bg-zinc-50 dark:bg-zinc-900/50 dark:border-zinc-800/50">
                  <input
                    type="checkbox"
                    id="extActive"
                    checked={extActive}
                    onChange={(e) => setExtActive(e.target.checked)}
                    className="h-4 w-4 rounded border-zinc-300 text-brand-500 focus:ring-brand-500 cursor-pointer"
                  />
                  <div>
                    <Label htmlFor="extActive" className="mb-0 cursor-pointer text-xs font-black uppercase tracking-wider">Enable External Sheet Import</Label>
                    <p className="text-[10px] text-zinc-400">Automatically poll this sheet every few minutes and import new leads.</p>
                  </div>
                </div>

                <div className="flex items-center gap-3 p-3 rounded border border-zinc-100 bg-zinc-50 dark:bg-zinc-900/50 dark:border-zinc-800/50">
                  <input
                    type="checkbox"
                    id="extAutoContactEnabled"
                    checked={extAutoContactEnabled}
                    onChange={(e) => setExtAutoContactEnabled(e.target.checked)}
                    className="h-4 w-4 rounded border-zinc-300 text-brand-500 focus:ring-brand-500 cursor-pointer"
                  />
                  <div>
                    <Label htmlFor="extAutoContactEnabled" className="mb-0 cursor-pointer text-xs font-black uppercase tracking-wider">Auto-Contact Imported Leads</Label>
                    <p className="text-[10px] text-zinc-400">
                      When on, new leads imported from this sheet get the initial WhatsApp message and follow-up sequence automatically.
                      When off, leads are imported silently — no messages are sent.
                    </p>
                  </div>
                </div>

                {extError ? (
                  <p className="text-xs font-black text-rose-500 uppercase tracking-wider">{extError}</p>
                ) : null}

                <div className="flex items-center justify-end border-t border-zinc-200 dark:border-zinc-800 pt-4">
                  <Button
                    type="submit"
                    loading={extSaving}
                    className="bg-brand-500 hover:bg-brand-600 text-white text-xs font-black uppercase tracking-widest rounded h-10 px-6"
                  >
                    Save Connection
                  </Button>
                </div>
              </form>
            </div>
          </div>
        )}

        {activeSection === 'integrations' && (
          <div className="grid gap-6 lg:grid-cols-2 items-start">
              {/* Meta App Settings Card */}
              <form onSubmit={onSaveMetaConfig} className="overflow-hidden rounded border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-950 p-6 space-y-5">
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
                      placeholder={studio.hasMetaAppSecret ? '••••••••••••••••' : 'e.g. d2d2fad32...'}
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

                <div className="flex items-center justify-end border-t border-zinc-200 dark:border-zinc-800 pt-4">
                  <Button 
                    type="submit" 
                    loading={metaSaving}
                    className="bg-brand-500 hover:bg-brand-600 text-white text-xs font-black uppercase tracking-widest rounded h-10 px-6"
                  >
                    Save Meta Config
                  </Button>
                </div>
              </form>

              {/* Google Ads Integration Card */}
              <form onSubmit={onSaveGoogleConfig} className="overflow-hidden rounded border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-950 p-6 space-y-5">
                <div>
                  <h3 className="text-xs font-black uppercase tracking-wider text-zinc-400">Google Ads Integration</h3>
                </div>

                <div>
                  <Label htmlFor="googleClientId">Google Client ID</Label>
                  <Input
                    id="googleClientId"
                    type="text"
                    placeholder="e.g. 123456789-abc.apps.googleusercontent.com"
                    invalid={!!errors.googleClientId}
                    value={googleClientId}
                    onChange={(e) => setGoogleClientId(e.target.value)}
                  />
                  <FieldHint>The OAuth Client ID for the Google Ads integration.</FieldHint>
                  <FieldError message={errors.googleClientId} />
                </div>

                <div>
                  <Label htmlFor="googleClientSecret">Google Client Secret</Label>
                  <div className="relative mt-1">
                    <Input
                      id="googleClientSecret"
                      type={showGoogleClientSecret ? 'text' : 'password'}
                      placeholder={studio.hasGoogleClientSecret ? '••••••••••••••••' : 'GOCSPX-...'}
                      invalid={!!errors.googleClientSecret}
                      value={googleClientSecret}
                      onChange={(e) => setGoogleClientSecret(e.target.value)}
                      className="pr-12"
                    />
                    <button
                      type="button"
                      className="absolute right-3 top-1/2 -translate-y-1/2 text-zinc-500 transition hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200"
                      onClick={() => setShowGoogleClientSecret(!showGoogleClientSecret)}
                    >
                      {showGoogleClientSecret ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                    </button>
                  </div>
                  <FieldHint>The OAuth Client Secret for the Google Ads integration.</FieldHint>
                  <FieldError message={errors.googleClientSecret} />
                </div>

                <FieldError message={errors._} />

                <div className="flex items-center justify-end border-t border-zinc-200 dark:border-zinc-800 pt-4">
                  <Button
                    type="submit"
                    loading={googleSaving}
                    className="bg-brand-500 hover:bg-brand-600 text-white text-xs font-black uppercase tracking-widest rounded h-10 px-6"
                  >
                    Save Google Config
                  </Button>
                </div>
              </form>

              {/* Gemini AI Integration Card */}
              <form onSubmit={onSaveGeminiKey} className="overflow-hidden rounded border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-950 p-6 space-y-5">
                <div>
                  <h3 className="text-xs font-black uppercase tracking-wider text-zinc-400">Gemini AI Integration</h3>
                </div>

                <div>
                  <Label htmlFor="geminiApiKey">Gemini API Key</Label>
                  <div className="relative mt-1">
                    <Input
                      id="geminiApiKey"
                      type={showGeminiApiKey ? 'text' : 'password'}
                      placeholder={studio.hasGeminiApiKey ? '••••••••••••••••' : 'AIzaSy...'}
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

                <div className="flex items-center justify-end border-t border-zinc-200 dark:border-zinc-800 pt-4">
                  <Button 
                    type="submit" 
                    loading={geminiSaving}
                    className="bg-brand-500 hover:bg-brand-600 text-white text-xs font-black uppercase tracking-widest rounded h-10 px-6"
                  >
                    Save Gemini Key
                  </Button>
                </div>
              </form>

              {/* Groq AI Integration Card */}
              <form onSubmit={onSaveGroqKey} className="overflow-hidden rounded border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-950 p-6 space-y-5">
                <div>
                  <h3 className="text-xs font-black uppercase tracking-wider text-zinc-400">Groq AI Integration</h3>
                </div>

                <div>
                  <Label htmlFor="groqApiKey">Groq API Key</Label>
                  <div className="relative mt-1">
                    <Input
                      id="groqApiKey"
                      type={showGroqApiKey ? 'text' : 'password'}
                      placeholder={studio.hasGroqApiKey ? '••••••••••••••••' : 'gsk_...'}
                      invalid={!!errors.groqApiKey}
                      value={groqApiKey}
                      onChange={(e) => setGroqApiKey(e.target.value)}
                      className="pr-12"
                    />
                    <button
                      type="button"
                      className="absolute right-3 top-1/2 -translate-y-1/2 text-zinc-500 transition hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200"
                      onClick={() => setShowGroqApiKey(!showGroqApiKey)}
                    >
                      {showGroqApiKey ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                    </button>
                  </div>
                  <FieldHint>Groq key enables Llama 3.1 8B (primary) and Llama 3.3 70B (fallback) for fast, cheap AI replies.</FieldHint>
                  <FieldError message={errors.groqApiKey} />
                </div>

                <FieldError message={errors._} />

                <div className="flex items-center justify-end border-t border-zinc-200 dark:border-zinc-800 pt-4">
                  <Button
                    type="submit"
                    loading={groqSaving}
                    className="bg-brand-500 hover:bg-brand-600 text-white text-xs font-black uppercase tracking-widest rounded h-10 px-6"
                  >
                    Save Groq Key
                  </Button>
                </div>
              </form>

            {/* WhatsApp Message Pacing — throttle between consecutive sends */}
            <div className="overflow-hidden rounded border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-950">
              <div className="border-b border-zinc-200 px-6 py-4 dark:border-zinc-800 flex items-center gap-2">
                <Timer className="h-4 w-4 text-brand-500" />
                <h3 className="text-xs font-black uppercase tracking-wider text-zinc-400">WhatsApp Message Pacing</h3>
              </div>
              <form onSubmit={onSaveSendSpacing} className="space-y-4 p-6">
                <p className="text-[10px] text-zinc-400">
                  Controls the gap between consecutive WhatsApp messages sent on the same connected number —
                  lead to lead, one at a time. Applies everywhere, including a batch of newly-imported leads or a
                  number freshly connected: each one waits its turn instead of firing at once. A larger gap
                  reduces the risk of the number being flagged by WhatsApp during bulk sends.
                </p>

                <div className="max-w-xs">
                  <Label htmlFor="sendSpacingSeconds">Delay Between Messages (seconds)</Label>
                  <Input
                    id="sendSpacingSeconds"
                    type="number"
                    min={0}
                    max={300}
                    value={sendSpacingSeconds}
                    onChange={(e) => setSendSpacingSeconds(Math.max(0, Math.min(300, Number(e.target.value) || 0)))}
                  />
                  <FieldHint>e.g. 20 = wait 20 seconds after each WhatsApp message before sending the next one on this number. 0-300s.</FieldHint>
                </div>

                {sendSpacingError ? (
                  <p className="text-xs font-black text-rose-500 uppercase tracking-wider">{sendSpacingError}</p>
                ) : null}

                <div className="flex items-center justify-end border-t border-zinc-200 dark:border-zinc-800 pt-4">
                  <Button
                    type="submit"
                    loading={sendSpacingSaving}
                    className="bg-brand-500 hover:bg-brand-600 text-white text-xs font-black uppercase tracking-widest rounded h-10 px-6"
                  >
                    Save Pacing
                  </Button>
                </div>
              </form>
            </div>

            {/* Initial Message Delay — wait before the very first auto-contact send */}
            <div className="overflow-hidden rounded border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-950">
              <div className="border-b border-zinc-200 px-6 py-4 dark:border-zinc-800 flex items-center gap-2">
                <Timer className="h-4 w-4 text-brand-500" />
                <h3 className="text-xs font-black uppercase tracking-wider text-zinc-400">Message Timing</h3>
              </div>
              <form onSubmit={onSaveInitialDelay} className="space-y-4 p-6">
                <p className="text-[10px] text-zinc-400">
                  How long to wait before sending the very first WhatsApp message to a newly-created lead
                  (right after connecting WhatsApp, or a batch of leads coming in at once). Set to 0 to send immediately.
                </p>

                <div className="flex items-end gap-3">
                  <div className="max-w-[140px]">
                    <Label htmlFor="initialDelayValue">Wait</Label>
                    <Input
                      id="initialDelayValue"
                      type="number"
                      min={0}
                      max={initialDelayUnit === 'hours' ? 24 : 1440}
                      value={initialDelayValue}
                      onChange={(e) => setInitialDelayValue(Math.max(0, Number(e.target.value) || 0))}
                    />
                  </div>
                  <div className="max-w-[160px]">
                    <Label htmlFor="initialDelayUnit">Unit</Label>
                    <select
                      id="initialDelayUnit"
                      value={initialDelayUnit}
                      onChange={(e) => setInitialDelayUnit(e.target.value as 'minutes' | 'hours')}
                      className="h-10 w-full rounded border border-zinc-200 bg-white px-3 text-sm text-zinc-900 focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500 dark:border-zinc-800 dark:bg-zinc-950 dark:text-zinc-100"
                    >
                      <option value="minutes">Minutes</option>
                      <option value="hours">Hours</option>
                    </select>
                  </div>
                </div>
                <FieldHint>e.g. 15 minutes, or 1 hour, before the first message goes out after a lead comes in.</FieldHint>

                <div className="max-w-xs border-t border-zinc-200 dark:border-zinc-800 pt-4">
                  <Label htmlFor="aiReplyDelaySeconds">AI Reply Delay (seconds)</Label>
                  <Input
                    id="aiReplyDelaySeconds"
                    type="number"
                    min={0}
                    max={300}
                    value={aiReplyDelaySeconds}
                    onChange={(e) => setAiReplyDelaySeconds(Math.max(0, Math.min(300, Number(e.target.value) || 0)))}
                  />
                  <FieldHint>
                    Separate from the delay above — this is how long the AI waits before replying to a message
                    within an already-started conversation (not the first outreach). e.g. 8 = wait 8 seconds
                    after a lead messages in before the AI's reply goes out. 0 = instant.
                  </FieldHint>
                </div>

                {initialDelayError ? (
                  <p className="text-xs font-black text-rose-500 uppercase tracking-wider">{initialDelayError}</p>
                ) : null}

                <div className="flex items-center justify-end border-t border-zinc-200 dark:border-zinc-800 pt-4">
                  <Button
                    type="submit"
                    loading={initialDelaySaving}
                    className="bg-brand-500 hover:bg-brand-600 text-white text-xs font-black uppercase tracking-widest rounded h-10 px-6"
                  >
                    Save Timing
                  </Button>
                </div>
              </form>
            </div>
        </div>
        )}

        {activeSection === 'security' && (
          <div className="max-w-xl">
            <div className="overflow-hidden rounded border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-950">
              <div className="border-b border-zinc-200 px-6 py-4 dark:border-zinc-800">
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

                <div className="flex items-center justify-end border-t border-zinc-200 dark:border-zinc-800 pt-4">
                  <Button
                    type="submit"
                    loading={changingPassword}
                    className="bg-brand-500 hover:bg-brand-600 text-white text-xs font-black uppercase tracking-widest rounded h-10 px-6"
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
  const [showConfirmDialog, setShowConfirmDialog] = useState(false);

  const handleDelete = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');

    if (!deleteEmail) {
      setError('Please enter your email to confirm');
      return;
    }

    setShowConfirmDialog(true);
  };

  const handleConfirmDelete = async () => {
    setShowConfirmDialog(false);
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
    <>
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

      {/* Custom Confirmation Dialog Modal */}
      {showConfirmDialog && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
          <div className="w-full max-w-md mx-4 rounded-2xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 shadow-2xl overflow-hidden">
            <div className="bg-red-50 dark:bg-red-950/30 border-b border-red-200 dark:border-red-900/30 px-6 py-4">
              <h2 className="text-lg font-black text-red-900 dark:text-red-100">Final Confirmation</h2>
            </div>

            <div className="p-6 space-y-4">
              <div className="bg-red-100/50 dark:bg-red-950/50 border border-red-300 dark:border-red-800 rounded-lg p-4">
                <p className="text-sm font-semibold text-red-900 dark:text-red-100 mb-2">⚠️ This action cannot be undone.</p>
                <p className="text-sm text-red-800 dark:text-red-200">
                  You are about to permanently delete:
                </p>
                <ul className="mt-3 space-y-1 text-sm text-red-800 dark:text-red-200 ml-4">
                  <li>✗ Your account and login credentials</li>
                  <li>✗ All studio data and settings</li>
                  <li>✗ All campaigns and leads</li>
                  <li>✗ All messages and conversations</li>
                  <li>✗ All uploaded files and documents</li>
                </ul>
              </div>

              <p className="text-sm text-slate-600 dark:text-slate-400">
                This will delete all data associated with <strong>{studioName}</strong>. Are you absolutely sure?
              </p>
            </div>

            <div className="border-t border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-800/50 px-6 py-4 flex items-center gap-3">
              <Button
                type="button"
                variant="ghost"
                onClick={() => setShowConfirmDialog(false)}
                className="flex-1"
              >
                Cancel
              </Button>
              <Button
                type="button"
                onClick={handleConfirmDelete}
                disabled={deleting}
                className="flex-1 bg-red-600 hover:bg-red-700 text-white font-bold"
              >
                {deleting ? 'Deleting...' : 'Yes, Delete Everything'}
              </Button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}

function PlatformBillingManager({ studio }: { studio: Studio }) {
  const [plans, setPlans] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const [syncing, setSyncing] = useState(false);
  const [syncedTier, setSyncedTier] = useState<string | null>(null);

  useEffect(() => {
    // Only trigger a full reload when returning from Stripe checkout (upgrade=success).
    // For normal page loads, just display the current tier without reloading,
    // to avoid an infinite refresh loop caused by minor tier string mismatches.
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
        // ONLY reload if the user just came back from Stripe (upgrade=success).
        // Never reload on a plain tier mismatch — that causes an infinite loop.
        if (justUpgraded && data.synced) {
          const searchParams = new URLSearchParams(window.location.search);
          searchParams.delete('upgrade');
          const newSearch = searchParams.toString();
          const newUrl = window.location.pathname + (newSearch ? `?${newSearch}` : '');
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

      <div className={`relative overflow-hidden rounded border border-brand-500 bg-brand-50/50 dark:bg-brand-950/20 p-6 transition-all duration-300 shadow-sm`}>
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

      <div className="grid gap-4 sm:grid-cols-2">
        {plans.map((plan, idx) => {
          const isCurrent = currentTier === plan.name;
          // Inline colors so Tailwind JIT doesn't purge them
          const accentHex = ['#a1a1aa', '#7c3aed', '#8b5cf6', '#10b981'][idx % 4];
          return (
            <div
              key={plan.name || idx}
              style={{ borderTopColor: accentHex }}
              className={`group relative flex flex-col rounded border-t-4 overflow-hidden transition-all duration-200 cursor-default ${
                isCurrent
                  ? 'border border-brand-500 bg-brand-50 dark:bg-brand-950/30 shadow-md'
                  : 'border border-zinc-200 bg-white dark:bg-zinc-900 dark:border-zinc-700 hover:border-brand-400 hover:shadow-md hover:shadow-brand-500/10'
              }`}
            >
              {/* Card Header */}
              <div className={`px-5 py-4 ${isCurrent ? 'bg-brand-50 dark:bg-brand-950/20' : 'bg-zinc-50 dark:bg-zinc-900'}`}>
                <div className="flex items-center justify-between">
                  <h4 className="text-sm font-black uppercase tracking-widest text-zinc-800 dark:text-zinc-100">
                    {plan.name}
                  </h4>
                  {isCurrent && (
                    <span className="inline-flex items-center gap-1 rounded bg-brand-500 px-2 py-0.5 text-[10px] font-black uppercase tracking-wider text-white">
                      <CheckCircle2 className="h-3 w-3" /> Active
                    </span>
                  )}
                </div>
                <div className="mt-2 flex items-baseline gap-1">
                  <span className="text-3xl font-black tracking-tight text-zinc-900 dark:text-white">
                    S${plan.price}
                  </span>
                  {plan.cycle?.toLowerCase() !== 'one-time' ? (
                    <span className="text-xs font-semibold text-zinc-400">/mo</span>
                  ) : (
                    <span className="text-xs font-semibold text-zinc-400">one-time</span>
                  )}
                </div>
                <p className="mt-1 text-[11px] text-zinc-500 leading-relaxed">{plan.description}</p>
              </div>

              {/* Feature List */}
              <div className="flex-1 space-y-2 border-t border-zinc-200 dark:border-zinc-800 px-5 py-4">
                {plan.features?.map((f: string, i: number) => (
                  <div key={i} className="flex items-start gap-2">
                    <CheckCircle2 className="h-3.5 w-3.5 shrink-0 text-emerald-500 mt-0.5" />
                    <span className="text-xs text-zinc-600 dark:text-zinc-300">{f}</span>
                  </div>
                ))}
              </div>

              {/* Action Button */}
              <div className="border-t border-zinc-200 dark:border-zinc-800 px-5 py-4">
                <Button
                  variant={isCurrent ? 'secondary' : 'primary'}
                  onClick={() => handleAction(plan.name, hasSubscription)}
                  loading={actionLoading === plan.name}
                  disabled={isCurrent}
                  className="w-full text-xs font-black uppercase tracking-wider rounded h-9"
                >
                  {isCurrent ? '✓ Current Plan' : (hasSubscription ? 'Switch to This Plan' : 'Select Plan')}
                </Button>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
