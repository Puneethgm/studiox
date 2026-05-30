'use client';

import { useEffect } from 'react';
import { AlertTriangle, RefreshCw } from 'lucide-react';

export default function AdminError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error('[Admin Error]', error);
  }, [error]);

  return (
    <div className="flex min-h-[60vh] flex-col items-center justify-center gap-6 text-center px-4">
      <div className="grid h-20 w-20 place-items-center rounded-3xl bg-red-500/10 text-red-500 ring-1 ring-red-500/20">
        <AlertTriangle className="h-10 w-10" />
      </div>
      <div className="space-y-2">
        <h2 className="text-2xl font-black tracking-tight text-zinc-900 dark:text-white">
          Something went wrong
        </h2>
        <p className="max-w-sm text-sm text-zinc-500 dark:text-zinc-400">
          {error.message || 'An unexpected error occurred. Please try again.'}
        </p>
        {error.digest && (
          <p className="font-mono text-[10px] text-zinc-400">
            Error ID: {error.digest}
          </p>
        )}
      </div>
      <button
        onClick={reset}
        className="inline-flex items-center gap-2 rounded-2xl bg-gradient-to-r from-brand-500 to-violet-600 px-6 py-3 text-sm font-black uppercase tracking-widest text-white shadow-lg shadow-brand-500/25 transition-all hover:from-brand-600 hover:to-violet-700 hover:shadow-brand-500/30"
      >
        <RefreshCw className="h-4 w-4" />
        Try Again
      </button>
    </div>
  );
}
