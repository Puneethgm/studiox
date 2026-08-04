'use client';

import { useEffect, useState } from 'react';
import { CheckCircle2, CreditCard } from 'lucide-react';

interface LeadInfo {
  leadName: string;
  studioName: string;
  alreadyPurchased: boolean;
}

const fieldClass =
  'w-full rounded-xl border border-white/10 bg-white/5 px-3.5 py-2.5 text-sm text-white placeholder:text-zinc-500 ' +
  'focus:outline-none focus:border-brand-500 focus:ring-1 focus:ring-brand-500 transition-all';

export function TrialDetailsForm({ leadId }: { leadId: string }) {
  const [info, setInfo] = useState<LeadInfo | null>(null);
  const [loadError, setLoadError] = useState(false);
  const [fullName, setFullName] = useState('');
  const [gender, setGender] = useState('');
  const [dateOfBirth, setDateOfBirth] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    (async () => {
      try {
        const res = await fetch(`/api/v1/public/leads/${encodeURIComponent(leadId)}/trial-checkout`);
        if (!res.ok) {
          setLoadError(true);
          return;
        }
        const data: LeadInfo = await res.json();
        setInfo(data);
        setFullName(data.leadName || '');
      } catch {
        setLoadError(true);
      }
    })();
  }, [leadId]);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (submitting) return;
    setError(null);
    if (!fullName.trim()) {
      setError('Please enter your full name.');
      return;
    }
    setSubmitting(true);
    try {
      const res = await fetch(`/api/v1/public/leads/${encodeURIComponent(leadId)}/trial-checkout`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ fullName: fullName.trim(), gender, dateOfBirth }),
      });
      if (!res.ok) {
        setError('Something went wrong. Please try again.');
        setSubmitting(false);
        return;
      }
      const data = await res.json();
      if (data.checkoutUrl) {
        window.location.href = data.checkoutUrl;
      } else {
        setError('Could not start payment. Please try again.');
        setSubmitting(false);
      }
    } catch {
      setError('Network error. Please try again.');
      setSubmitting(false);
    }
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-slate-950 via-violet-950 to-slate-900 flex items-center justify-center p-6">
      <div className="pointer-events-none fixed inset-0 overflow-hidden">
        <div className="absolute -top-40 -right-40 h-96 w-96 rounded-full bg-violet-600/20 blur-3xl animate-pulse" />
        <div className="absolute -bottom-40 -left-40 h-96 w-96 rounded-full bg-emerald-600/15 blur-3xl animate-pulse" style={{ animationDelay: '1s' }} />
      </div>

      <div className="relative z-10 w-full max-w-md">
        <div className="overflow-hidden rounded-[32px] border border-white/10 bg-white/5 backdrop-blur-2xl shadow-2xl">
          <div className="h-1.5 w-full bg-gradient-to-r from-violet-500 via-brand-500 to-emerald-400" />

          <div className="p-8 space-y-6">
            {loadError ? (
              <div className="text-center space-y-2">
                <p className="text-sm font-bold text-white">This link isn&apos;t valid.</p>
                <p className="text-xs text-zinc-400">Please contact the studio for a new booking link.</p>
              </div>
            ) : !info ? (
              <div className="flex justify-center py-10">
                <div className="h-8 w-8 rounded-full border-2 border-violet-500 border-t-transparent animate-spin" />
              </div>
            ) : info.alreadyPurchased ? (
              <div className="text-center space-y-3">
                <div className="relative inline-flex">
                  <div className="grid h-16 w-16 place-items-center rounded-full bg-emerald-500/20 border border-emerald-500/30">
                    <CheckCircle2 className="h-8 w-8 text-emerald-400" />
                  </div>
                </div>
                <h1 className="text-xl font-black text-white">You&apos;re all set!</h1>
                <p className="text-sm text-zinc-400">
                  Your trial at <span className="font-bold text-white">{info.studioName}</span> is already confirmed.
                </p>
              </div>
            ) : (
              <>
                <div className="text-center space-y-1.5">
                  <h1 className="text-xl font-black text-white tracking-tight">Almost there! 🎉</h1>
                  <p className="text-sm text-zinc-400">
                    A few quick details for your trial at <span className="font-bold text-white">{info.studioName}</span>, then straight to secure payment.
                  </p>
                </div>

                <form onSubmit={onSubmit} className="space-y-4">
                  <div>
                    <label className="mb-1.5 block text-[10px] font-black uppercase tracking-wider text-zinc-500">
                      Full Name
                    </label>
                    <input
                      type="text"
                      required
                      value={fullName}
                      onChange={(e) => setFullName(e.target.value)}
                      placeholder="Your full name"
                      className={fieldClass}
                    />
                  </div>

                  <div>
                    <label className="mb-1.5 block text-[10px] font-black uppercase tracking-wider text-zinc-500">
                      Gender <span className="normal-case text-zinc-600">(optional)</span>
                    </label>
                    <select
                      value={gender}
                      onChange={(e) => setGender(e.target.value)}
                      className={fieldClass + ' appearance-none'}
                    >
                      <option value="" className="bg-slate-900">Prefer not to say</option>
                      <option value="female" className="bg-slate-900">Female</option>
                      <option value="male" className="bg-slate-900">Male</option>
                      <option value="other" className="bg-slate-900">Other</option>
                    </select>
                  </div>

                  <div>
                    <label className="mb-1.5 block text-[10px] font-black uppercase tracking-wider text-zinc-500">
                      Date of Birth <span className="normal-case text-zinc-600">(optional)</span>
                    </label>
                    <input
                      type="date"
                      value={dateOfBirth}
                      onChange={(e) => setDateOfBirth(e.target.value)}
                      className={fieldClass}
                    />
                  </div>

                  {error && <p className="text-xs font-semibold text-red-400">{error}</p>}

                  <button
                    type="submit"
                    disabled={submitting}
                    className="flex w-full items-center justify-center gap-2 rounded-xl bg-gradient-to-r from-violet-600 to-brand-500 py-3 text-sm font-black text-white shadow-lg shadow-violet-500/25 transition-all hover:scale-[1.02] disabled:opacity-60 disabled:hover:scale-100"
                  >
                    {submitting ? (
                      <div className="h-4 w-4 rounded-full border-2 border-white/40 border-t-white animate-spin" />
                    ) : (
                      <CreditCard className="h-4 w-4" />
                    )}
                    Continue to Payment
                  </button>
                </form>
              </>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
