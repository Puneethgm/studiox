'use client';

import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { Suspense, useState } from 'react';
import { ArrowLeft, CheckCircle2, Download } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { FieldError, FieldHint, Label } from '@/components/ui/Label';

function RecoverForm() {
  const token = useSearchParams().get('token') ?? '';
  const [email, setEmail] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      const res = await fetch('/api/v1/recover', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token, email }),
      });

      if (!res.ok) {
        const body = await res.json().catch(() => null);
        setError(body?.error ?? 'Could not recover this account.');
        return;
      }

      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'account-archive.zip';
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);

      setDone(true);
    } catch {
      setError('Could not recover this account. Try again.');
    } finally {
      setSubmitting(false);
    }
  }

  if (!token) {
    return (
      <div className="text-center">
        <h2 className="text-2xl font-black tracking-tight text-zinc-900 dark:text-white">Invalid link</h2>
        <p className="mt-4 text-sm font-medium text-zinc-500 dark:text-zinc-400">
          This recovery link is missing its token. Go back to your email and use the link exactly as sent.
        </p>
        <Link href="/" className="mt-8 inline-flex items-center gap-2 text-sm font-bold text-brand-600 hover:underline dark:text-brand-400">
          <ArrowLeft className="h-4 w-4" /> Back home
        </Link>
      </div>
    );
  }

  if (done) {
    return (
      <div className="text-center">
        <div className="mx-auto mb-6 grid h-14 w-14 place-items-center rounded-2xl bg-emerald-500/10 text-emerald-600 dark:text-emerald-400">
          <CheckCircle2 className="h-6 w-6" />
        </div>
        <h2 className="text-2xl font-black tracking-tight text-zinc-900 dark:text-white">Download started</h2>
        <p className="mt-4 text-sm font-medium text-zinc-500 dark:text-zinc-400">
          Your account archive (account-archive.zip) should now be downloading. If nothing happened, check your
          browser&apos;s download prompt or pop-up blocker.
        </p>
        <Link href="/" className="mt-8 inline-flex items-center gap-2 text-sm font-bold text-brand-600 hover:underline dark:text-brand-400">
          <ArrowLeft className="h-4 w-4" /> Back home
        </Link>
      </div>
    );
  }

  return (
    <>
      <div className="mb-10 text-center">
        <div className="mx-auto mb-6 grid h-14 w-14 place-items-center rounded-2xl bg-brand-500/10 text-brand-600 dark:text-brand-400">
          <Download className="h-6 w-6" />
        </div>
        <h2 className="text-3xl font-black tracking-tight text-zinc-900 dark:text-white">Recover your data</h2>
        <p className="mt-4 text-sm font-medium text-zinc-500 dark:text-zinc-400">
          A recovery archive of your deleted account is available for 7 days from the day it was deleted. Confirm
          your registered email to download it. After 7 days, the archive expires and can no longer be recovered.
        </p>
      </div>
      <form onSubmit={onSubmit} className="space-y-6">
        <div className="space-y-2.5">
          <Label htmlFor="email" className="ml-2 text-[10px] font-black uppercase tracking-[0.2em] text-zinc-400 dark:text-zinc-500">Account Email</Label>
          <Input
            id="email"
            type="email"
            autoComplete="email"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="you@studio.com"
            className="h-14 rounded-[20px] border-white/20 bg-white/50 px-5 text-base shadow-sm backdrop-blur-md dark:border-white/5 dark:bg-black/20"
          />
          <FieldHint>This must match the studio&apos;s registered account email.</FieldHint>
        </div>
        <FieldError message={error ?? undefined} />
        <Button type="submit" className="h-14 w-full rounded-[24px] text-lg font-black" size="lg" loading={submitting}>
          Download archive
        </Button>
      </form>
      <Link href="/" className="mt-8 flex items-center justify-center gap-2 text-sm font-bold text-zinc-500 hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200">
        <ArrowLeft className="h-4 w-4" /> Back home
      </Link>
    </>
  );
}

export default function RecoverPage() {
  return (
    <main className="flex min-h-screen w-full items-center justify-center bg-slate-50 px-6 text-zinc-900 dark:bg-neutral-950 dark:text-zinc-100">
      <div className="w-full max-w-md">
        <div className="glass-container p-1">
          <div className="rounded-[40px] bg-white/40 p-10 backdrop-blur-3xl dark:bg-neutral-900/40 sm:p-12">
            <Suspense fallback={null}>
              <RecoverForm />
            </Suspense>
          </div>
        </div>
      </div>
    </main>
  );
}
