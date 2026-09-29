'use client';

import Link from 'next/link';
import { useRouter, useSearchParams } from 'next/navigation';
import { Suspense, useState } from 'react';
import { ArrowLeft, CheckCircle2, Eye, EyeOff } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { FieldError, Label } from '@/components/ui/Label';
import { ApiError, api } from '@/lib/api';

function ResetPasswordForm() {
  const router = useRouter();
  const token = useSearchParams().get('token') ?? '';
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await api('/api/v1/auth/reset-password', {
        method: 'POST',
        json: { token, newPassword, confirmPassword },
      });
      setDone(true);
      setTimeout(() => router.push('/login'), 2000);
    } catch (err) {
      if (err instanceof ApiError && err.details) {
        const first = Object.values(err.details)[0];
        setError(first ?? 'Could not reset your password.');
      } else if (err instanceof ApiError && err.status === 400) {
        setError('This reset link is invalid or has expired. Request a new one.');
      } else {
        setError('Could not reset your password. Try again.');
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
          This password reset link is missing its token. Request a new one below.
        </p>
        <Link href="/forgot-password" className="mt-8 inline-flex items-center gap-2 text-sm font-bold text-brand-600 hover:underline dark:text-brand-400">
          <ArrowLeft className="h-4 w-4" /> Request a new link
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
        <h2 className="text-2xl font-black tracking-tight text-zinc-900 dark:text-white">Password updated</h2>
        <p className="mt-4 text-sm font-medium text-zinc-500 dark:text-zinc-400">Taking you to sign in&hellip;</p>
      </div>
    );
  }

  return (
    <>
      <div className="mb-10 text-center">
        <h2 className="text-3xl font-black tracking-tight text-zinc-900 dark:text-white">Set a new password</h2>
        <p className="mt-4 text-sm font-medium text-zinc-500 dark:text-zinc-400">Choose a new password for your account.</p>
      </div>
      <form onSubmit={onSubmit} className="space-y-6">
        <div className="space-y-2.5">
          <Label htmlFor="newPassword" className="ml-2 text-[10px] font-black uppercase tracking-[0.2em] text-zinc-400 dark:text-zinc-500">New Password</Label>
          <div className="relative">
            <Input
              id="newPassword"
              type={showPassword ? 'text' : 'password'}
              autoComplete="new-password"
              required
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              placeholder="••••••••"
              className="h-14 rounded-[20px] border-white/20 bg-white/50 px-5 pr-12 text-base shadow-sm backdrop-blur-md dark:border-white/5 dark:bg-black/20"
            />
            <button
              type="button"
              aria-label={showPassword ? 'Hide password' : 'Show password'}
              className="absolute right-4 top-1/2 -translate-y-1/2 text-zinc-500 hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200"
              onClick={() => setShowPassword((v) => !v)}
            >
              {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
            </button>
          </div>
        </div>
        <div className="space-y-2.5">
          <Label htmlFor="confirmPassword" className="ml-2 text-[10px] font-black uppercase tracking-[0.2em] text-zinc-400 dark:text-zinc-500">Confirm Password</Label>
          <Input
            id="confirmPassword"
            type={showPassword ? 'text' : 'password'}
            autoComplete="new-password"
            required
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            placeholder="••••••••"
            className="h-14 rounded-[20px] border-white/20 bg-white/50 px-5 text-base shadow-sm backdrop-blur-md dark:border-white/5 dark:bg-black/20"
          />
        </div>
        <FieldError message={error ?? undefined} />
        <Button type="submit" className="h-14 w-full rounded-[24px] text-lg font-black" size="lg" loading={submitting}>
          Reset password
        </Button>
      </form>
      <Link href="/login" className="mt-8 flex items-center justify-center gap-2 text-sm font-bold text-zinc-500 hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200">
        <ArrowLeft className="h-4 w-4" /> Back to sign in
      </Link>
    </>
  );
}

export default function ResetPasswordPage() {
  return (
    <main className="flex min-h-screen w-full items-center justify-center bg-slate-50 px-6 text-zinc-900 dark:bg-neutral-950 dark:text-zinc-100">
      <div className="w-full max-w-md">
        <div className="glass-container p-1">
          <div className="rounded-[40px] bg-white/40 p-10 backdrop-blur-3xl dark:bg-neutral-900/40 sm:p-12">
            <Suspense fallback={null}>
              <ResetPasswordForm />
            </Suspense>
          </div>
        </div>
      </div>
    </main>
  );
}
