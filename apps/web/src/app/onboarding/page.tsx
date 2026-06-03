'use client';

import { useState, Suspense } from 'react';
import { useSearchParams } from 'next/navigation';
import { Button } from '@/components/ui/Button';

function OnboardingForm() {
  const searchParams = useSearchParams();
  const sessionId = searchParams.get('session_id');

  const [studioName, setStudioName] = useState('');
  const [adminPassword, setAdminPassword] = useState('');
  const [status, setStatus] = useState<'idle' | 'loading' | 'success' | 'error'>('idle');
  const [errorMessage, setErrorMessage] = useState('');

  if (!sessionId) {
    return <div className="text-center p-8">Invalid session. No session ID found.</div>;
  }

  const handleCompleteOnboarding = async (e: React.FormEvent) => {
    e.preventDefault();
    setStatus('loading');
    setErrorMessage('');

    try {
      const res = await fetch('/api/v1/public/platform/provision', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          sessionId,
          studioName,
          adminPassword,
        }),
      });

      if (!res.ok) {
        const errorData = await res.json();
        throw new Error(errorData.message || 'Failed to provision studio');
      }

      setStatus('success');
      setTimeout(() => {
        window.location.href = '/login';
      }, 2000);
    } catch (e: any) {
      console.error(e);
      setStatus('error');
      setErrorMessage(e.message);
    }
  };

  return (
    <div className="max-w-md w-full bg-white dark:bg-neutral-900 rounded-3xl shadow-xl p-8 border border-slate-200 dark:border-neutral-800">
      <h1 className="text-2xl font-bold text-center mb-6 text-slate-900 dark:text-white">Welcome to 1herosocial.ai!</h1>
      {status === 'success' ? (
        <div className="text-center text-green-600 dark:text-green-400 font-medium animate-in fade-in zoom-in duration-300">
          <p>Your studio has been created successfully!</p>
          <p className="text-sm mt-2 text-slate-500 dark:text-slate-400">Redirecting to login...</p>
        </div>
      ) : (
        <form onSubmit={handleCompleteOnboarding} className="space-y-4 animate-in fade-in duration-300">
          <p className="text-sm text-slate-600 dark:text-slate-400 mb-4 text-center">
            Payment successful! Let's get your studio set up.
          </p>
          <div>
            <label className="block text-xs font-black uppercase tracking-wider text-slate-700 dark:text-slate-400 mb-1">Studio Name</label>
            <input
              type="text"
              required
              value={studioName}
              onChange={(e) => setStudioName(e.target.value)}
              className="w-full rounded-xl border border-slate-300 dark:border-neutral-700 bg-white dark:bg-neutral-800 px-3 py-2 text-sm focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500 text-slate-900 dark:text-white"
              placeholder="e.g. FitPro Studio"
            />
          </div>
          <div>
            <label className="block text-xs font-black uppercase tracking-wider text-slate-700 dark:text-slate-400 mb-1">Admin Password</label>
            <input
              type="password"
              required
              minLength={8}
              value={adminPassword}
              onChange={(e) => setAdminPassword(e.target.value)}
              className="w-full rounded-xl border border-slate-300 dark:border-neutral-700 bg-white dark:bg-neutral-800 px-3 py-2 text-sm focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500 text-slate-900 dark:text-white"
              placeholder="Minimum 8 characters"
            />
          </div>
          {status === 'error' && (
            <p className="text-sm text-red-500 font-medium bg-red-500/10 p-3 rounded-lg">{errorMessage}</p>
          )}
          <Button
            type="submit"
            className="w-full mt-6 shadow-lg shadow-brand-500/20"
            loading={status === 'loading'}
          >
            Complete Setup
          </Button>
        </form>
      )}
    </div>
  );
}

export default function OnboardingPage() {
  return (
    <div className="min-h-screen bg-slate-50 dark:bg-slate-950 flex items-center justify-center p-4 bg-[radial-gradient(ellipse_at_top,_var(--tw-gradient-stops))] from-brand-100/20 via-slate-50 to-slate-50 dark:from-brand-900/20 dark:via-slate-950 dark:to-slate-950">
      <Suspense fallback={<div className="h-8 w-8 animate-spin rounded-full border-2 border-brand-500 border-t-transparent" />}>
        <OnboardingForm />
      </Suspense>
    </div>
  );
}
