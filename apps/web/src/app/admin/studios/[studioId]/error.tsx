'use client';

import { useEffect } from 'react';
import { AlertTriangle, RefreshCw, ArrowLeft } from 'lucide-react';
import Link from 'next/link';
import { useParams } from 'next/navigation';

export default function StudioError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  const params = useParams();
  const studioId = params?.studioId as string | undefined;

  useEffect(() => {
    console.error('[Studio Error]', error);
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
          {error.message || 'An unexpected error occurred while loading this studio page.'}
        </p>
        {error.digest && (
          <p className="font-mono text-[10px] text-zinc-400">
            Error ID: {error.digest}
          </p>
        )}
      </div>
      <div className="flex gap-3">
        {studioId && (
          <Link
            href={`/admin/studios/${studioId}`}
            className="inline-flex items-center gap-2 rounded-2xl border border-white/20 bg-white/20 px-5 py-3 text-sm font-black uppercase tracking-widest text-zinc-600 shadow-sm backdrop-blur-md transition-all hover:bg-white/30 dark:text-zinc-300 dark:hover:bg-white/10"
          >
            <ArrowLeft className="h-4 w-4" />
            Studio Home
          </Link>
        )}
        <button
          onClick={reset}
          className="inline-flex items-center gap-2 rounded-2xl bg-gradient-to-r from-brand-500 to-violet-600 px-6 py-3 text-sm font-black uppercase tracking-widest text-white shadow-lg shadow-brand-500/25 transition-all hover:from-brand-600 hover:to-violet-700"
        >
          <RefreshCw className="h-4 w-4" />
          Try Again
        </button>
      </div>
    </div>
  );
}
