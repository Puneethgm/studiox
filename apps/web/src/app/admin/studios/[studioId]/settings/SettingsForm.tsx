'use client';

import { useRouter } from 'next/navigation';
import { useState } from 'react';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { Input } from '@/components/ui/Input';
import { FieldError, FieldHint, Label } from '@/components/ui/Label';
import type { Studio } from '@/lib/types';
import { updateStudioSettings } from './actions';

export function SettingsForm({ studio }: { studio: Studio }) {
  const router = useRouter();
  const [name, setName] = useState(studio.name);
  const [brandColor, setBrandColor] = useState(studio.brandColor);
  const [logoUrl, setLogoUrl] = useState(studio.logoUrl);
  const [contactEmail, setContactEmail] = useState(studio.contactEmail);
  const [active, setActive] = useState(studio.active);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);

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
                {logoUrl ? (
                  // eslint-disable-next-line @next/next/no-img-element
                  <img src={logoUrl} alt="" className="h-12 w-12 rounded-2xl object-cover" />
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
              <button
                type="button"
                className="w-full rounded-xl py-2.5 text-sm font-semibold text-white shadow-sm"
                style={{ background: brandColor }}
                suppressHydrationWarning
              >
                Get in touch
              </button>
            </div>
          </div>
          <FieldHint>
            Slug is fixed (renaming would break shared links). Ask the platform admin if you need it changed.
          </FieldHint>
          </div>
        </div>
      </div>
    </div>
  );
}
