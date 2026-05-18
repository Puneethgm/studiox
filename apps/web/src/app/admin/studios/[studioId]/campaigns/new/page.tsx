'use client';

import { useParams, useRouter } from 'next/navigation';
import { useState } from 'react';
import { Megaphone, Plus } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { Input } from '@/components/ui/Input';
import { FieldError, FieldHint, Label } from '@/components/ui/Label';
import { Textarea } from '@/components/ui/Textarea';
import { ApiError, api } from '@/lib/api';
import type { Campaign } from '@/lib/types';

export default function NewCampaignPage() {
  const router = useRouter();
  const params = useParams<{ studioId: string }>();
  const studioId = params.studioId;

  const [name, setName] = useState('');
  const [slug, setSlug] = useState('');
  const [description, setDescription] = useState('');
  const [plansText, setPlansText] = useState('Yoga\nHIIT\nPersonal Training\nGroup Class');
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [submitting, setSubmitting] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setErrors({});
    setSubmitting(true);
    const fitnessPlans = plansText
      .split('\n')
      .map((p) => p.trim())
      .filter(Boolean);
    try {
      const c = await api<Campaign>(`/api/v1/studios/${studioId}/campaigns`, {
        method: 'POST',
        json: { name, slug, description, fitnessPlans },
      });
      router.push(`/admin/studios/${studioId}/campaigns/${c.id}`);
      router.refresh();
    } catch (err) {
      if (err instanceof ApiError && err.details) {
        setErrors(err.details);
      } else if (err instanceof ApiError && err.code === 'slug_taken') {
        setErrors({ slug: 'this slug is already in use within this studio' });
      } else {
        setErrors({ _: 'failed to create campaign' });
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
              <Megaphone className="h-6 w-6" />
            </div>
            <div>
              <h1 className="text-2xl font-black tracking-tight text-zinc-900 dark:text-white">Create Campaign</h1>
              <p className="mt-0.5 text-xs font-semibold text-zinc-500 dark:text-zinc-400">
                Configure a new lead magnet and generate a high-converting landing page.
              </p>
            </div>
          </div>
        </div>
      </div>
      <div className="mx-auto max-w-2xl">
        <Card>
          <form onSubmit={onSubmit} className="space-y-5">
            <div>
              <Label htmlFor="name">Campaign name</Label>
              <Input
                id="name"
                placeholder="Spring Promo 2026"
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
                placeholder="auto-generated if empty"
                invalid={!!errors.slug}
                value={slug}
                onChange={(e) => setSlug(e.target.value)}
              />
              <FieldHint>
                Public URL: <code className="font-mono">/l/&lt;studio&gt;/&lt;slug&gt;</code>
              </FieldHint>
              <FieldError message={errors.slug} />
            </div>

            <div>
              <Label htmlFor="description">Description (optional)</Label>
              <Textarea
                id="description"
                rows={3}
                placeholder="Shown above the form on the public page"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
              />
            </div>

            <div>
              <Label htmlFor="plans">Fitness plans (one per line)</Label>
              <Textarea
                id="plans"
                rows={5}
                invalid={!!errors.fitnessPlans}
                value={plansText}
                onChange={(e) => setPlansText(e.target.value)}
              />
              <FieldHint>These appear as choices on the public form.</FieldHint>
              <FieldError message={errors.fitnessPlans} />
            </div>

            <FieldError message={errors._} />

            <div className="flex items-center justify-end gap-2 border-t border-slate-100 pt-5 dark:border-slate-800/60">
              <Button variant="ghost" type="button" onClick={() => router.back()}>
                Cancel
              </Button>
              <Button type="submit" loading={submitting} leftIcon={<Plus className="h-4 w-4" />}>
                Create campaign
              </Button>
            </div>
          </form>
        </Card>
      </div>
    </div>
  );
}
