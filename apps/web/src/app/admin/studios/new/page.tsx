'use client';

import { useRouter } from 'next/navigation';
import { useState } from 'react';
import { Building2, Plus } from 'lucide-react';
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

export default function NewStudioPage() {
  const router = useRouter();
  const [name, setName] = useState('');
  const [slug, setSlug] = useState('');
  const [brandColor, setBrandColor] = useState('#7c3aed');
  const [logoUrl, setLogoUrl] = useState('');
  const [contactEmail, setContactEmail] = useState('');
  const [adminEmail, setAdminEmail] = useState('');
  const [adminPassword, setAdminPassword] = useState('');
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [submitting, setSubmitting] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setErrors({});
    setSubmitting(true);
    try {
      const res = await api<CreateResp>('/api/v1/admin/studios', {
        method: 'POST',
        json: { name, slug, brandColor, logoUrl, contactEmail, adminEmail, adminPassword },
      });
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
    <div className="space-y-8 pb-12">
      {/* Premium Glass Header */}
      <div
        className="relative overflow-hidden rounded-[26px] border border-white/30 p-6 backdrop-blur-2xl dark:border-white/5"
        style={{
          background: 'linear-gradient(135deg, rgba(255,255,255,0.30) 0%, rgba(237,233,254,0.22) 60%, rgba(219,234,254,0.20) 100%)',
          boxShadow: 'inset 0 0 0 1px rgba(255,255,255,0.2), 0 8px 32px rgba(139,92,246,0.07)',
        }}
      >
        <div className="pointer-events-none absolute -right-16 -top-16 h-48 w-48 rounded-full bg-brand-500/10 blur-[70px]" />
        
        <div className="relative flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex items-center gap-4">
            <div className="grid h-12 w-12 place-items-center rounded-2xl bg-gradient-to-br from-brand-500 to-violet-600 text-white shadow-lg shadow-brand-500/25">
              <Building2 className="h-6 w-6" />
            </div>
            <div>
              <h1 className="text-2xl font-black tracking-tight text-zinc-900 dark:text-white">New Studio</h1>
              <p className="mt-0.5 text-xs font-semibold text-zinc-500 dark:text-zinc-400">
                Configure the studio&rsquo;s identity and create its first admin login.
              </p>
            </div>
          </div>
        </div>
      </div>
      <div className="mx-auto max-w-2xl space-y-6">
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
                Used in public URLs: <code className="font-mono">/l/&lt;slug&gt;/&lt;campaign&gt;</code>
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
    </>
  );
}
