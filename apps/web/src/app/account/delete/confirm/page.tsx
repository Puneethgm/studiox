'use client';

import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { Suspense, useState } from 'react';
import { AlertTriangle, ArrowLeft, CheckCircle2 } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { FieldError } from '@/components/ui/Label';
import { ApiError, api } from '@/lib/api';

function ConfirmDeleteForm() {
  const token = useSearchParams().get('token') ?? '';
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState(false);

  async function onConfirm() {
    setError(null);
    setSubmitting(true);
    try {
      await api('/api/v1/account/delete/confirm', {
        method: 'POST',
        json: { token },
      });
      setDone(true);
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message || 'This deletion link is invalid or has expired.');
      } else {
        setError('Could not confirm deletion. Try again.');
      }
    } finally {
      setSubmitting(false);
    }
  }

  if (!token) {
    return (
      <div className="text-center">
        <h2 className="text-2xl font-black tracking-tight text-zinc-900 dark:text-white">Invalid link</h2>
        <p className="mt-4 text-sm font-medium text-zinc-500 dark:text-zinc-400">
          This deletion link is missing its token. Go back to your email and use the link exactly as sent.
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
        <h2 className="text-2xl font-black tracking-tight text-zinc-900 dark:text-white">Account deleted</h2>
        <p className="mt-4 text-sm font-medium text-zinc-500 dark:text-zinc-400">
          Your account and its data have been deleted. We&apos;ve sent a separate email with a recovery link — it
          stays valid for 7 days, after which the backup is removed for good.
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
        <div className="mx-auto mb-6 grid h-14 w-14 place-items-center rounded-2xl bg-red-500/10 text-red-600 dark:text-red-400">
          <AlertTriangle className="h-6 w-6" />
        </div>
        <h2 className="text-3xl font-black tracking-tight text-zinc-900 dark:text-white">Delete this account?</h2>
        <p className="mt-4 text-sm font-medium text-zinc-500 dark:text-zinc-400">
          This permanently deletes the account, its studio, and all associated conversations and data. This action
          cannot be undone from here — a recovery window follows, but only via a separate link sent after deletion.
        </p>
      </div>
      <FieldError message={error ?? undefined} />
      <Button
        type="button"
        onClick={onConfirm}
        loading={submitting}
        className="mt-6 h-14 w-full rounded-[24px] bg-red-600 text-lg font-black hover:bg-red-700"
        size="lg"
      >
        Yes, permanently delete this account
      </Button>
      <Link href="/" className="mt-8 flex items-center justify-center gap-2 text-sm font-bold text-zinc-500 hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200">
        <ArrowLeft className="h-4 w-4" /> Cancel, keep my account
      </Link>
    </>
  );
}

export default function ConfirmDeletePage() {
  return (
    <main className="flex min-h-screen w-full items-center justify-center bg-slate-50 px-6 text-zinc-900 dark:bg-neutral-950 dark:text-zinc-100">
      <div className="w-full max-w-md">
        <div className="glass-container p-1">
          <div className="rounded-[40px] bg-white/40 p-10 backdrop-blur-3xl dark:bg-neutral-900/40 sm:p-12">
            <Suspense fallback={null}>
              <ConfirmDeleteForm />
            </Suspense>
          </div>
        </div>
      </div>
    </main>
  );
}
