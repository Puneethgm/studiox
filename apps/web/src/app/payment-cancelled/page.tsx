'use client';

import { useSearchParams } from 'next/navigation';
import { Suspense } from 'react';
import { XCircle, Home, ArrowLeft } from 'lucide-react';

function PaymentCancelledContent() {
  const params = useSearchParams();
  const studio = params.get('studio') || 'your studio';

  return (
    <div className="min-h-screen bg-gradient-to-br from-slate-950 via-rose-950/30 to-slate-900 flex items-center justify-center p-6">
      <div className="pointer-events-none fixed inset-0 overflow-hidden">
        <div className="absolute -top-40 -right-40 h-96 w-96 rounded-full bg-rose-600/15 blur-3xl animate-pulse" />
        <div className="absolute -bottom-40 -left-40 h-96 w-96 rounded-full bg-slate-600/10 blur-3xl" />
      </div>

      <div className="relative z-10 w-full max-w-md">
        <div className="overflow-hidden rounded-[32px] border border-white/10 bg-white/5 backdrop-blur-2xl shadow-2xl">
          <div className="h-1.5 w-full bg-gradient-to-r from-rose-500 to-orange-500" />
          <div className="p-8 text-center space-y-6">
            <div className="grid h-20 w-20 mx-auto place-items-center rounded-full bg-rose-500/20 border border-rose-500/30">
              <XCircle className="h-10 w-10 text-rose-400" />
            </div>
            <div className="space-y-2">
              <h1 className="text-2xl font-black text-white tracking-tight">Payment Cancelled</h1>
              <p className="text-sm text-zinc-400">
                Your payment was cancelled. No charge was made.
              </p>
            </div>
            <div className="rounded-2xl border border-white/10 bg-white/5 p-4 text-xs text-zinc-400 text-left">
              Changed your mind? You can always try booking again by contacting{' '}
              <span className="text-white font-bold capitalize">{studio.replace(/-/g, ' ')}</span> on WhatsApp.
            </div>
          </div>
          <div className="border-t border-white/10 p-4 flex gap-2">
            <a
              href="/"
              className="flex-1 flex items-center justify-center gap-2 py-2.5 rounded-xl bg-white/5 hover:bg-white/10 text-xs font-bold text-zinc-400 hover:text-white transition-all"
            >
              <Home className="h-3.5 w-3.5" />
              Home
            </a>
            <button
              onClick={() => window.history.back()}
              className="flex-1 flex items-center justify-center gap-2 py-2.5 rounded-xl bg-gradient-to-r from-violet-600 to-brand-500 text-xs font-black text-white shadow-lg transition-all hover:scale-[1.02]"
            >
              <ArrowLeft className="h-3.5 w-3.5" />
              Try Again
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

export default function PaymentCancelledPage() {
  return (
    <Suspense fallback={
      <div className="min-h-screen bg-slate-950 flex items-center justify-center">
        <div className="h-8 w-8 rounded-full border-2 border-rose-500 border-t-transparent animate-spin" />
      </div>
    }>
      <PaymentCancelledContent />
    </Suspense>
  );
}
