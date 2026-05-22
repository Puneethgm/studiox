'use client';

import { useRouter } from 'next/navigation';
import { useState } from 'react';
import { Button } from '@/components/ui/Button';
import { FieldError, FieldHint } from '@/components/ui/Label';
import { Textarea } from '@/components/ui/Textarea';
import { updateCampaignFitnessPlans } from './plans-actions';

export function CampaignPlansEditor({
  studioId,
  campaignId,
  studioSlug,
  campaignSlug,
  initialPlans,
}: {
  studioId: string;
  campaignId: string;
  studioSlug: string;
  campaignSlug: string;
  initialPlans: string[];
}) {
  const router = useRouter();
  const [plansText, setPlansText] = useState(initialPlans.join('\n'));
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);

  const previewPlans = plansText
    .split('\n')
    .map((plan) => plan.trim())
    .filter(Boolean);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setErrors({});
    setSaved(false);
    setSaving(true);
    try {
      const fitnessPlans = plansText
        .split('\n')
        .map((plan) => plan.trim())
        .filter(Boolean);
      const result = await updateCampaignFitnessPlans({
        studioId,
        campaignId,
        studioSlug,
        campaignSlug,
        fitnessPlans,
      });
      if (!result.ok) {
        setErrors(result.details ?? { _: result.error });
        return;
      }
      setSaved(true);
      router.refresh();
    } finally {
      setSaving(false);
    }
  }

  return (
    <form onSubmit={onSubmit} className="space-y-4">
      <div>
        <label className="mb-2 block text-sm font-semibold text-zinc-900 dark:text-zinc-100">
          Fitness plans (one per line)
        </label>
        <Textarea
          rows={6}
          value={plansText}
          onChange={(e) => setPlansText(e.target.value)}
          invalid={!!errors.fitnessPlans}
          placeholder="Yoga\nHIIT\nPersonal Training\nGroup Class"
        />
        <FieldHint>This list controls the dropdown on the public form.</FieldHint>
        <FieldError message={errors.fitnessPlans} />
      </div>

      <div className="flex flex-wrap gap-2">
        {previewPlans.map((plan) => (
          <span
            key={plan}
            className="rounded-full px-3 py-1 text-xs font-medium"
            style={{ background: 'var(--brand-soft, rgba(124,58,237,0.08))', color: 'var(--brand, #7c3aed)' }}
          >
            {plan}
          </span>
        ))}
      </div>

      {saved ? <p className="text-sm font-medium text-emerald-600 dark:text-emerald-400">Saved.</p> : null}
      <FieldError message={errors._} />

      <div className="flex justify-end">
        <Button type="submit" loading={saving}>
          Save plans
        </Button>
      </div>
    </form>
  );
}