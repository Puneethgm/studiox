'use client';

import { useParams, useRouter } from 'next/navigation';
import { useState } from 'react';
import { Megaphone, Plus, CheckCircle2, AlertCircle, X } from 'lucide-react';
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
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' } | null>(null);

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
      setToast({ message: 'Campaign created successfully. Redirecting...', type: 'success' });
      router.refresh();
      setTimeout(() => {
        router.push(`/admin/studios/${studioId}/campaigns/${c.id}`);
      }, 1500);
    } catch (err) {
      if (err instanceof ApiError && err.details) {
        setErrors(err.details);
        setToast({ message: 'Validation failed. Please correct the fields.', type: 'error' });
      } else if (err instanceof ApiError && err.code === 'slug_taken') {
        setErrors({ slug: 'this slug is already in use within this studio' });
        setToast({ message: 'This URL slug is already in use.', type: 'error' });
      } else {
        setErrors({ _: 'failed to create campaign' });
        setToast({ message: 'Failed to create campaign.', type: 'error' });
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
                Public URL: <code className="font-mono">{'/l/<studio>/<slug>'}</code>
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
      {/* Custom Floating Toast Notification */}
      {toast && (
        <div className={`fixed bottom-6 right-6 z-[9999] p-4 rounded-2xl border backdrop-blur-xl shadow-2xl flex items-center gap-3 animate-in slide-in-from-bottom-5 fade-in duration-300 min-w-[320px] ${
          toast.type === 'success' 
            ? 'border-emerald-500/30 bg-white/90 dark:bg-zinc-900/90' 
            : 'border-red-500/30 bg-white/90 dark:bg-zinc-900/90'
        }`}>
          <div className={`h-8 w-8 rounded-xl flex items-center justify-center shrink-0 ${
            toast.type === 'success' 
              ? 'bg-emerald-500/10 text-emerald-500' 
              : 'bg-red-500/10 text-red-500'
          }`}>
            {toast.type === 'success' ? <CheckCircle2 className="w-5 h-5" /> : <AlertCircle className="w-5 h-5" />}
          </div>
          <div className="flex-1 text-left">
            <p className="text-xs font-black uppercase tracking-wider text-zinc-800 dark:text-zinc-100">
              {toast.type === 'success' ? 'Success' : 'Error'}
            </p>
            <p className="text-[10px] text-zinc-550 dark:text-zinc-400 font-semibold mt-0.5">{toast.message}</p>
          </div>
          <button 
            onClick={() => setToast(null)} 
            className="text-zinc-400 hover:text-zinc-655 dark:hover:text-white p-1 rounded-lg transition-colors"
          >
            <X className="w-4 h-4" />
          </button>
        </div>
      )}
    </div>
  );
}
