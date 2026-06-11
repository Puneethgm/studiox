'use client';

import { useRouter } from 'next/navigation';
import { useState } from 'react';
import { Building2, Plus, Sparkles, Users, Zap, Palette } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { Input } from '@/components/ui/Input';
import { FieldError, FieldHint, Label } from '@/components/ui/Label';
import { ApiError, api } from '@/lib/api';
import type { Studio } from '@/lib/types';

interface CreateResp {
  studio: Studio;
  adminId: string;
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

export default function NewStudioPage() {
  const router = useRouter();
  const [name, setName] = useState('');
  const [slug, setSlug] = useState('');
  const [brandColor, setBrandColor] = useState('#7c3aed');
  const [logoUrl, setLogoUrl] = useState('');
  const [contactEmail, setContactEmail] = useState('');
  const [contactPhoneCountryCode, setContactPhoneCountryCode] = useState('+65');
  const [contactPhone, setContactPhone] = useState('');
  const [adminEmail, setAdminEmail] = useState('');
  const [adminPassword, setAdminPassword] = useState('');
  const [socialPlannerEnabled, setSocialPlannerEnabled] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [submitting, setSubmitting] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setErrors({});
    setSubmitting(true);
    try {
      const cleanPhone = contactPhone.replace(/\D/g, '');
      const fullPhone = cleanPhone ? `${contactPhoneCountryCode}${cleanPhone}` : '';
      const res = await api<CreateResp>('/api/v1/admin/studios', {
        method: 'POST',
        json: {
          name,
          slug,
          brandColor,
          logoUrl,
          contactEmail,
          contactPhone: fullPhone,
          adminEmail,
          adminPassword,
          socialPlannerEnabled
        },
      });
      sessionStorage.setItem('studiox_toast', JSON.stringify({
        message: `Studio "${name}" has been created successfully.`,
        type: 'success'
      }));
      router.push(`/admin/studios/${res.studio.id}`);
      router.refresh();
    } catch (err) {
      if (err instanceof ApiError && err.details) {
        setErrors(err.details);
      } else {
        setErrors({ _: 'failed to create studio' });
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="min-h-[calc(100vh-64px)] flex flex-col lg:flex-row gap-6 lg:gap-0">
      {/* Left Panel - Premium Visual Design */}
      <div className="flex-1 hidden lg:flex flex-col justify-between bg-gradient-to-br from-brand-700 via-brand-600 to-violet-700 p-12 relative overflow-hidden lg:rounded-2xl lg:m-6" style={{
        backgroundImage: 'linear-gradient(to right, rgba(124, 58, 237, 0.4) 0%, rgba(124, 58, 237, 0.6) 100%), url("/platform-bg.png")',
        backgroundSize: 'cover',
        backgroundPosition: 'center',
        backgroundRepeat: 'no-repeat',
      }}>
        {/* Animated gradient orbs */}
        <div className="absolute inset-0">
          <div className="absolute -left-32 -top-32 h-96 w-96 rounded-full bg-white/5 blur-3xl animate-pulse" />
          <div className="absolute -right-48 -bottom-48 h-[500px] w-[500px] rounded-full bg-violet-300/5 blur-3xl animate-pulse" style={{ animationDelay: '1s' }} />
          <div className="absolute top-1/3 left-1/4 h-80 w-80 rounded-full bg-brand-400/5 blur-3xl" />
        </div>

        <div className="relative z-10 space-y-12">
          {/* Header with Icon */}
          <div className="space-y-6">
            <div className="inline-flex items-center justify-center h-20 w-20 rounded-2xl bg-gradient-to-br from-white/30 to-white/10 backdrop-blur-xl border border-white/40 shadow-2xl shadow-black/20">
              <Building2 className="h-10 w-10 text-white" />
            </div>

            <div className="space-y-3">
              <h2 className="text-5xl font-black text-white leading-tight">
                Launch Your <span className="bg-gradient-to-r from-yellow-200 via-pink-200 to-blue-200 bg-clip-text text-transparent">Studio Today</span>
              </h2>
              <p className="text-lg text-white/80 font-medium leading-relaxed max-w-md">
                Turn your fitness vision into reality. Set up in minutes and start capturing leads.
              </p>
            </div>
          </div>

          {/* Feature Cards with Icons */}
          <div className="space-y-3">
            {/* Feature 1 */}
            <div className="group flex gap-4 p-4 rounded-2xl bg-white/10 hover:bg-white/20 backdrop-blur-md border border-white/20 transition-all duration-300">
              <div className="flex-shrink-0 h-12 w-12 rounded-xl bg-gradient-to-br from-pink-400 to-rose-500 flex items-center justify-center shadow-lg">
                <Palette className="h-6 w-6 text-white" />
              </div>
              <div className="flex-1">
                <h3 className="text-white font-bold text-base leading-tight">Complete Branding</h3>
                <p className="text-white/70 text-sm mt-0.5">Logo, colors, and identity across all public forms</p>
              </div>
            </div>

            {/* Feature 2 */}
            <div className="group flex gap-4 p-4 rounded-2xl bg-white/10 hover:bg-white/20 backdrop-blur-md border border-white/20 transition-all duration-300">
              <div className="flex-shrink-0 h-12 w-12 rounded-xl bg-gradient-to-br from-blue-400 to-cyan-500 flex items-center justify-center shadow-lg">
                <Users className="h-6 w-6 text-white" />
              </div>
              <div className="flex-1">
                <h3 className="text-white font-bold text-base leading-tight">Admin Control</h3>
                <p className="text-white/70 text-sm mt-0.5">Secure login for your first admin user</p>
              </div>
            </div>

            {/* Feature 3 */}
            <div className="group flex gap-4 p-4 rounded-2xl bg-white/10 hover:bg-white/20 backdrop-blur-md border border-white/20 transition-all duration-300">
              <div className="flex-shrink-0 h-12 w-12 rounded-xl bg-gradient-to-br from-amber-400 to-orange-500 flex items-center justify-center shadow-lg">
                <Zap className="h-6 w-6 text-white" />
              </div>
              <div className="flex-1">
                <h3 className="text-white font-bold text-base leading-tight">AI Social Planner</h3>
                <p className="text-white/70 text-sm mt-0.5">Optional: AI-powered ad creation & scheduling</p>
              </div>
            </div>

            {/* Feature 4 */}
            <div className="group flex gap-4 p-4 rounded-2xl bg-white/10 hover:bg-white/20 backdrop-blur-md border border-white/20 transition-all duration-300">
              <div className="flex-shrink-0 h-12 w-12 rounded-xl bg-gradient-to-br from-emerald-400 to-teal-500 flex items-center justify-center shadow-lg">
                <Sparkles className="h-6 w-6 text-white" />
              </div>
              <div className="flex-1">
                <h3 className="text-white font-bold text-base leading-tight">Instant Go-Live</h3>
                <p className="text-white/70 text-sm mt-0.5">Your lead capture page live at <code className="text-white/90 bg-white/10 px-1.5 py-0.5 rounded text-xs font-mono">l/your-slug</code></p>
              </div>
            </div>
          </div>
        </div>

        {/* Bottom CTA or Badge */}
        <div className="relative z-10 flex items-center gap-3 pt-4 border-t border-white/20">
          <div className="h-2 w-2 rounded-full bg-emerald-400 animate-pulse" />
          <span className="text-sm text-white/70 font-medium">Setup typically takes 3-5 minutes</span>
        </div>
      </div>

      {/* Right Panel - Form */}
      <div className="flex-1 p-8 lg:p-12 overflow-y-auto">
        <div className="max-w-2xl mx-auto space-y-6">
        <Card title="Studio identity">
          <form id="studio-form" onSubmit={onSubmit} className="space-y-5">
            <div>
              <Label htmlFor="name">Studio name</Label>
              <Input
                id="name"
                placeholder="Yoga Bliss Singapore"
                required
                invalid={!!errors.name}
                value={name}
                onChange={(e) => setName(e.target.value)}
              />
              <FieldError message={errors.name} />
            </div>

            <div>
              <Label htmlFor="slug">URL slug (optional)</Label>
              <Input
                id="slug"
                placeholder="auto-generated from name"
                invalid={!!errors.slug}
                value={slug}
                onChange={(e) => setSlug(e.target.value)}
              />
              <FieldHint>
                Used in public URLs: <code className="font-mono">{'/l/<slug>/<campaign>'}</code>
              </FieldHint>
              <FieldError message={errors.slug} />
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
                <Label htmlFor="logoUrl">Logo URL (optional)</Label>
                <Input
                  id="logoUrl"
                  placeholder="https://..."
                  value={logoUrl}
                  onChange={(e) => setLogoUrl(e.target.value)}
                />
                <FieldHint>Square image works best. Used on the public form.</FieldHint>
              </div>
            </div>

            <div>
              <Label htmlFor="contactEmail">Contact email (optional)</Label>
              <Input
                id="contactEmail"
                type="email"
                placeholder="hello@studio.com"
                invalid={!!errors.contactEmail}
                value={contactEmail}
                onChange={(e) => setContactEmail(e.target.value)}
              />
              <FieldError message={errors.contactEmail} />
            </div>

            <div>
              <Label htmlFor="contactPhone">Contact phone (optional)</Label>
              <div className="flex gap-2">
                <select
                  value={contactPhoneCountryCode}
                  onChange={(e) => setContactPhoneCountryCode(e.target.value)}
                  className="h-10 rounded-xl border border-slate-200 bg-white dark:bg-slate-950 dark:border-slate-800 text-slate-900 dark:text-slate-100 px-3 py-2 text-sm focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500 w-[110px] shrink-0"
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

            <div className="flex items-center gap-3 rounded-lg border border-slate-200 bg-slate-50 p-4 dark:border-slate-800 dark:bg-slate-900/50">
              <input
                type="checkbox"
                id="socialPlannerEnabled"
                checked={socialPlannerEnabled}
                onChange={(e) => setSocialPlannerEnabled(e.target.checked)}
                className="h-4 w-4 rounded border-slate-300 text-brand-600 focus:ring-brand-600 dark:border-slate-600 dark:bg-slate-700"
              />
              <div className="space-y-1 leading-none">
                <Label htmlFor="socialPlannerEnabled" className="cursor-pointer">Enable Social Planner</Label>
                <p className="text-xs text-slate-500 dark:text-slate-400">Give this studio access to the AI-driven ad creation and schedule management features.</p>
              </div>
            </div>
          </form>
        </Card>

        <Card title="First admin login" subtitle="The studio admin uses this to sign in.">
          <div className="space-y-5">
            <div>
              <Label htmlFor="adminEmail">Admin email</Label>
              <Input
                id="adminEmail"
                type="email"
                form="studio-form"
                required
                invalid={!!errors.adminEmail}
                value={adminEmail}
                onChange={(e) => setAdminEmail(e.target.value)}
                placeholder="admin@studio.com"
              />
              <FieldError message={errors.adminEmail} />
            </div>
            <div>
              <Label htmlFor="adminPassword">Temporary password</Label>
              <Input
                id="adminPassword"
                type="password"
                form="studio-form"
                required
                invalid={!!errors.adminPassword}
                value={adminPassword}
                onChange={(e) => setAdminPassword(e.target.value)}
                placeholder="At least 8 characters"
              />
              <FieldHint>Share this securely with the studio admin. They sign in at the same /login page.</FieldHint>
              <FieldError message={errors.adminPassword} />
            </div>
          </div>
        </Card>

        <FieldError message={errors._} />

        <div className="flex items-center justify-end gap-2">
          <Button variant="ghost" type="button" onClick={() => router.back()}>
            Cancel
          </Button>
          <Button
            type="submit"
            form="studio-form"
            loading={submitting}
            leftIcon={<Plus className="h-4 w-4" />}
          >
            Create studio
          </Button>
        </div>
        </div>
      </div>
    </div>
  );
}
