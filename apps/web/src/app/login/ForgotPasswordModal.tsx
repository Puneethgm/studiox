'use client';

import { useState } from 'react';
import { KeyRound, Mail } from 'lucide-react';
import { Dialog, DialogHeader } from '@/components/ui/Dialog';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { FieldError, Label } from '@/components/ui/Label';
import { ApiError, api } from '@/lib/api';

export function ForgotPasswordModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [email, setEmail] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [sent, setSent] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function handleClose() {
    onClose();
    // Reset so reopening later starts fresh, not mid-"check your email" state.
    setSent(false);
    setEmail('');
    setError(null);
  }

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
    <Dialog open={open} onClose={handleClose} widthClassName="max-w-md">
      <DialogHeader icon={<KeyRound className="h-4 w-4 text-brand-600 dark:text-brand-400" />} title="Forgot password?" onClose={handleClose} />
      <div className="overflow-y-auto p-6">
        {sent ? (
          <div className="text-center">
            <div className="mx-auto mb-5 grid h-12 w-12 place-items-center rounded-2xl bg-brand-500/10 text-brand-600 dark:text-brand-400">
              <Mail className="h-5 w-5" />
            </div>
            <p className="text-sm font-semibold leading-relaxed text-zinc-600 dark:text-zinc-300">
              We&rsquo;ve sent a link to <span className="font-bold text-zinc-900 dark:text-white">{email}</span> to reset your password. It&rsquo;s valid for 1 hour and can only be used once.
            </p>
            <Button type="button" variant="secondary" className="mt-6 w-full rounded-xl" onClick={handleClose}>
              Done
            </Button>
          </div>
        ) : (
          <form onSubmit={onSubmit} className="space-y-5">
            <p className="text-sm text-zinc-500 dark:text-zinc-400">
              Enter your email and we&rsquo;ll send you a link to reset it.
            </p>
            <div className="space-y-2">
              <Label htmlFor="forgot-email" className="text-xs">Email Address</Label>
              <Input
                id="forgot-email"
                type="email"
                autoComplete="email"
                autoFocus
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="you@example.com"
              />
            </div>
            <FieldError message={error ?? undefined} />
            <Button type="submit" className="w-full rounded-xl" loading={submitting}>
              Send reset link
            </Button>
          </form>
        )}
      </div>
    </Dialog>
  );
}
