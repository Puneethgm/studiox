'use client';

import { useSearchParams } from 'next/navigation';
import { Suspense, useEffect, useState } from 'react';
import Link from 'next/link';
import { CheckCircle2, Home, ArrowRight, Receipt } from 'lucide-react';

function PaymentSuccessContent() {
  const params = useSearchParams();
  const studio = params.get('studio') || 'your studio';
  const sessionId = params.get('session_id');
  const [show, setShow] = useState(false);

  useEffect(() => {
    // Trigger animation after mount
    const t = setTimeout(() => setShow(true), 100);
    return () => clearTimeout(t);
  }, []);

  return (
    <div className="min-h-screen bg-gradient-to-br from-slate-950 via-violet-950 to-slate-900 flex items-center justify-center p-6">
      {/* Animated background blobs */}
      <div className="pointer-events-none fixed inset-0 overflow-hidden">
        <div className="absolute -top-40 -right-40 h-96 w-96 rounded-full bg-violet-600/20 blur-3xl animate-pulse" />
        <div className="absolute -bottom-40 -left-40 h-96 w-96 rounded-full bg-emerald-600/15 blur-3xl animate-pulse" style={{ animationDelay: '1s' }} />
        <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 h-64 w-64 rounded-full bg-brand-500/10 blur-3xl" />
      </div>

      <div
        className={`relative z-10 w-full max-w-md transition-all duration-700 ${
          show ? 'opacity-100 translate-y-0' : 'opacity-0 translate-y-8'
        }`}
      >
        {/* Card */}
        <div className="overflow-hidden rounded-[32px] border border-white/10 bg-white/5 backdrop-blur-2xl shadow-2xl">
          {/* Top gradient bar */}
          <div className="h-1.5 w-full bg-gradient-to-r from-emerald-400 via-teal-400 to-violet-500" />

          <div className="p-8 text-center space-y-6">
            {/* Success icon with ring animation */}
            <div className="relative inline-flex">
              <div className="absolute inset-0 rounded-full bg-emerald-500/20 animate-ping" />
              <div className="relative grid h-20 w-20 place-items-center rounded-full bg-emerald-500/20 border border-emerald-500/30">
                <CheckCircle2 className="h-10 w-10 text-emerald-400" />
              </div>
            </div>

            {/* Text */}
            <div className="space-y-2">
              <h1 className="text-2xl font-black text-white tracking-tight">
                Payment Successful! 🎉
              </h1>
              <p className="text-sm text-zinc-400">
                Your trial session at{' '}
                <span className="font-bold text-white capitalize">
                  {studio.replace(/-/g, ' ')}
                </span>{' '}
                is confirmed.
              </p>
            </div>

            {/* Info box */}
            <div className="rounded-2xl border border-white/10 bg-white/5 p-4 text-left space-y-3">
              <div className="flex items-center gap-3">
                <div className="h-8 w-8 rounded-xl bg-emerald-500/20 flex items-center justify-center shrink-0">
                  <CheckCircle2 className="h-4 w-4 text-emerald-400" />
                </div>
                <div>
                  <p className="text-xs font-bold text-white">Payment Received</p>
                  <p className="text-[10px] text-zinc-500">Your receipt will be emailed to you</p>
                </div>
              </div>
              <div className="flex items-center gap-3">
                <div className="h-8 w-8 rounded-xl bg-violet-500/20 flex items-center justify-center shrink-0">
                  <Receipt className="h-4 w-4 text-violet-400" />
                </div>
                <div>
                  <p className="text-xs font-bold text-white">Session Booked</p>
                  <p className="text-[10px] text-zinc-500">You&apos;ll receive a WhatsApp confirmation shortly</p>
                </div>
              </div>
            </div>

            {/* What's next */}
            <div className="space-y-2">
              <p className="text-[10px] uppercase tracking-widest font-black text-zinc-500">What&apos;s next</p>
              <div className="text-xs text-zinc-400 bg-white/5 rounded-2xl p-3 border border-white/10 text-left">
                ✅ Please arrive <strong className="text-white">10 minutes early</strong> to your session.<br />
                📱 Check your <strong className="text-white">WhatsApp</strong> for a confirmation message.<br />
                💪 Get ready to start your fitness journey!
              </div>
            </div>

            {sessionId && (
              <p className="text-[9px] text-zinc-600 font-mono break-all">
                Ref: {sessionId}
              </p>
            )}
          </div>

          {/* Footer */}
          <div className="border-t border-white/10 p-4 flex gap-2">
            <a
              href="/"
              className="flex-1 flex items-center justify-center gap-2 py-2.5 rounded-xl bg-white/5 hover:bg-white/10 text-xs font-bold text-zinc-400 hover:text-white transition-all"
            >
              <Home className="h-3.5 w-3.5" />
              Home
            </a>
            <a
              href={`/l/${studio}`}
              className="flex-1 flex items-center justify-center gap-2 py-2.5 rounded-xl bg-gradient-to-r from-violet-600 to-brand-500 text-xs font-black text-white shadow-lg shadow-violet-500/25 transition-all hover:scale-[1.02]"
            >
              Visit Studio
              <ArrowRight className="h-3.5 w-3.5" />
            </a>
          </div>
        </div>
      </div>
    </div>
  );
}

export default function PaymentSuccessPage() {
  return (
    <Suspense fallback={
      <div className="min-h-screen bg-slate-950 flex items-center justify-center">
        <div className="h-8 w-8 rounded-full border-2 border-violet-500 border-t-transparent animate-spin" />
      </div>
    }>
      <PaymentSuccessContent />
    </Suspense>
  );
}
