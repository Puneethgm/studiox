'use client';

import { useRouter } from 'next/navigation';
import { useState } from 'react';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { FieldError, Label } from '@/components/ui/Label';
import { Select } from '@/components/ui/Select';
import { Textarea } from '@/components/ui/Textarea';

interface Errors {
  name?: string;
  email?: string;
  phone?: string;
  fitnessPlan?: string;
  _?: string;
}

export function LeadForm({
  studioSlug,
  campaignSlug,
  fitnessPlans,
  brandColor,
}: {
  studioSlug: string;
  campaignSlug: string;
  fitnessPlans: string[];
  brandColor: string;
}) {
  const router = useRouter();
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [phone, setPhone] = useState('');
  const [fitnessPlan, setFitnessPlan] = useState(fitnessPlans[0] ?? '');
  const [goals, setGoals] = useState('');
  const [errors, setErrors] = useState<Errors>({});
  const [submitting, setSubmitting] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setErrors({});
    setSubmitting(true);
    try {
      const res = await fetch(
        `/api/v1/public/studios/${encodeURIComponent(studioSlug)}/campaigns/${encodeURIComponent(campaignSlug)}/leads`,
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ name, email, phone, fitnessPlan, goals }),
        },
      );
      if (res.status === 422) {
        const body = await res.json();
        setErrors(body.details ?? { _: body.error ?? 'Please fix the highlighted fields' });
        return;
      }
      if (!res.ok) {
        setErrors({ _: 'Something went wrong. Please try again.' });
        return;
      }
      router.push(
        `/l/${encodeURIComponent(studioSlug)}/${encodeURIComponent(campaignSlug)}/thank-you`,
      );
    } catch {
      setErrors({ _: 'Network error. Please try again.' });
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form onSubmit={onSubmit} className="space-y-5" noValidate>
      <div>
        <Label htmlFor="name">Full name</Label>
        <Input
          id="name"
          autoComplete="name"
          required
          invalid={!!errors.name}
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="Jane Doe"
          suppressHydrationWarning
        />
        <FieldError message={errors.name} />
      </div>
      <div>
        <Label htmlFor="email">Email</Label>
        <Input
          id="email"
          type="email"
          autoComplete="email"
          required
          invalid={!!errors.email}
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          placeholder="you@example.com"
          suppressHydrationWarning
        />
        <FieldError message={errors.email} />
      </div>
      <div>
        <Label htmlFor="phone">Phone</Label>
        <Input
          id="phone"
          type="tel"
          autoComplete="tel"
          required
          invalid={!!errors.phone}
          value={phone}
          onChange={(e) => setPhone(e.target.value)}
          placeholder="+1 555 123 4567"
          suppressHydrationWarning
        />
        <FieldError message={errors.phone} />
      </div>
      <div>
        <Label htmlFor="fitnessPlan">Which plan are you interested in?</Label>
        <Select
          id="fitnessPlan"
          required
          invalid={!!errors.fitnessPlan}
          value={fitnessPlan}
          onChange={(e) => setFitnessPlan(e.target.value)}
          suppressHydrationWarning
        >
          {fitnessPlans.map((p) => (
            <option key={p} value={p}>
              {p}
            </option>
          ))}
        </Select>
        <FieldError message={errors.fitnessPlan} />
      </div>
      <div>
        <Label htmlFor="goals">Anything specific you&rsquo;d like to share? (optional)</Label>
        <Textarea
          id="goals"
          rows={3}
          value={goals}
          onChange={(e) => setGoals(e.target.value)}
          placeholder="Goals, preferred timings, past experience…"
          suppressHydrationWarning
        />
      </div>
      <FieldError message={errors._} />
      <Button
        type="submit"
        loading={submitting}
        className="w-full h-12 shadow-lg"
        style={{ background: brandColor }}
        suppressHydrationWarning
      >
        Get in touch
      </Button>
    </form>
  );
}
