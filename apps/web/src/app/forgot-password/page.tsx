'use client';

import Link from 'next/link';
import { useState } from 'react';
import { ArrowLeft, Mail } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { FieldError, Label } from '@/components/ui/Label';
import { ApiError, api } from '@/lib/api';

export default function ForgotPasswordPage() {
  const [email, setEmail] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [sent, setSent] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      await api('/api/v1/auth/forgot-password', { method: 'POST', json: { email } });
      setSent(true);
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        setError('No account found with that email.');
      } else {
        setError('Could not send the reset email. Try again.');
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="flex min-h-screen w-full items-center justify-center bg-slate-50 px-6 text-zinc-900 dark:bg-neutral-950 dark:text-zinc-100">
      <div className="w-full max-w-md">
        <div className="glass-container p-1">
          <div className="rounded-[40px] bg-white/40 p-10 backdrop-blur-3xl dark:bg-neutral-900/40 sm:p-12">
            {sent ? (
              <div className="text-center">
                <div className="mx-auto mb-6 grid h-14 w-14 place-items-center rounded-2xl bg-brand-500/10 text-brand-600 dark:text-brand-400">
                  <Mail className="h-6 w-6" />
                </div>
                <h2 className="text-2xl font-black tracking-tight text-zinc-900 dark:text-white">Check your email</h2>
                <p className="mt-4 text-sm font-medium leading-relaxed text-zinc-500 dark:text-zinc-400">
                  We&rsquo;ve sent a link to <span className="font-bold text-zinc-700 dark:text-zinc-200">{email}</span> to reset your password. It&rsquo;s valid for 1 hour and can only be used once.
                </p>
                <Link href="/login" className="mt-8 inline-flex items-center gap-2 text-sm font-bold text-brand-600 hover:underline dark:text-brand-400">
                  <ArrowLeft className="h-4 w-4" /> Back to sign in
                </Link>
              </div>
            ) : (
              <>
                <div className="mb-10 text-center">
                  <h2 className="text-3xl font-black tracking-tight text-zinc-900 dark:text-white">Forgot password?</h2>
                  <p className="mt-4 text-sm font-medium text-zinc-500 dark:text-zinc-400">
                    Enter your email and we&rsquo;ll send you a link to reset it.
                  </p>
                </div>
                <form onSubmit={onSubmit} className="space-y-6">
                  <div className="space-y-2.5">
                    <Label htmlFor="email" className="ml-2 text-[10px] font-black uppercase tracking-[0.2em] text-zinc-400 dark:text-zinc-500">Email Address</Label>
                    <Input
                      id="email"
                      type="email"
                      autoComplete="email"
                      required
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                      placeholder="you@example.com"
                      className="h-14 rounded-[20px] border-white/20 bg-white/50 px-5 text-base shadow-sm backdrop-blur-md dark:border-white/5 dark:bg-black/20"
                    />
                  </div>
                  <FieldError message={error ?? undefined} />
                  <Button type="submit" className="h-14 w-full rounded-[24px] text-lg font-black" size="lg" loading={submitting}>
                    Send reset link
                  </Button>
                </form>
                <Link href="/login" className="mt-8 flex items-center justify-center gap-2 text-sm font-bold text-zinc-500 hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200">
                  <ArrowLeft className="h-4 w-4" /> Back to sign in
                </Link>
              </>
            )}
          </div>
        </div>
      </div>
    </main>
  );
}
