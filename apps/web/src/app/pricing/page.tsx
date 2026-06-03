'use client';

import { useState, useEffect } from 'react';
import { Check, Sparkles, ArrowRight } from 'lucide-react';
import Link from 'next/link';

type Tier = {
  name: string;
  price: number;
  cycle: string;
  description: string;
  features: string[];
  highlight?: boolean;
};

export default function PricingPage() {
  const [loadingTier, setLoadingTier] = useState<string | null>(null);
  const [tiers, setTiers] = useState<Tier[]>([]);
  const [loadingPlans, setLoadingPlans] = useState(true);

  useEffect(() => {
    fetch('/api/v1/public/platform/plans')
      .then((res) => res.json())
      .then((data) => {
        if (Array.isArray(data)) {
          setTiers(data);
        }
      })
      .catch((err) => console.error('Failed to fetch pricing tiers:', err))
      .finally(() => setLoadingPlans(false));
  }, []);

  const handleSelectPlan = async (tierName: string) => {
    setLoadingTier(tierName);
    try {
      const res = await fetch('/api/v1/public/platform/checkout', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ tier: tierName }),
      });
      if (!res.ok) {
        throw new Error('Failed to create checkout session');
      }
      const data = await res.json();
      if (data.url) {
        window.location.href = data.url;
      }
    } catch (e) {
      console.error(e);
      alert('Failed to initiate checkout. Please try again.');
    } finally {
      setLoadingTier(null);
    }
  };

  return (
    <div className="min-h-screen bg-[#FAF9F7] font-sans text-slate-900 selection:bg-violet-500/30">
      {/* ── Navigation Bar ──────────────────────── */}
      <header className="sticky top-0 z-50 w-full border-b border-[#EAE8E2] bg-[#FAF9F7]/80 backdrop-blur-xl">
        <div className="mx-auto flex h-18 max-w-7xl items-center justify-between px-6 lg:px-8 py-4">
          <div className="flex items-center gap-2.5">
            <Link href="/" className="flex items-center gap-2.5">
              <div className="grid h-9 w-9 place-items-center rounded-xl bg-gradient-to-br from-violet-600 to-indigo-700 text-white shadow-md">
                <Sparkles className="h-4 w-4" />
              </div>
              <span className="text-lg font-black tracking-tight text-[#1A1A1A]">1herosocial.ai</span>
            </Link>
          </div>
          
          <nav className="hidden md:flex items-center gap-8">
            <Link href="/#features" className="text-sm font-medium text-[#525252] hover:text-violet-600 transition-colors">
              Features
            </Link>
            <Link href="/pricing" className="text-sm font-medium text-[#525252] hover:text-violet-600 transition-colors">
              Pricing
            </Link>
          </nav>

          <div className="flex items-center gap-4">
            <Link href="/login" className="hidden sm:inline-flex text-sm font-medium text-[#525252] hover:text-violet-600 transition-colors">
              Log in
            </Link>
            <button className="flex items-center gap-2 bg-violet-600 text-white hover:bg-violet-700 px-4 py-2 rounded-lg text-sm font-medium transition-all shadow-md shadow-violet-600/20 hover:shadow-violet-600/40">
              Get Started <ArrowRight className="h-3.5 w-3.5" />
            </button>
          </div>
        </div>
      </header>

      {/* ── Pricing Content ──────────────────────── */}
      <main className="relative overflow-hidden mx-auto max-w-7xl px-6 lg:px-8 py-24 sm:py-32">
        <div className="absolute inset-0 -z-10">
          <div className="absolute top-0 left-1/2 -translate-x-1/2 w-[900px] h-[500px] bg-gradient-to-b from-violet-200/40 via-indigo-100/30 to-transparent rounded-full blur-3xl" />
        </div>

        <div className="mb-20 text-center max-w-2xl mx-auto">
          <h1 className="text-4xl sm:text-6xl font-black tracking-tight text-[#1A1A1A] mb-6">Plans & Pricing</h1>
          <p className="text-xl text-[#525252]">Scale your studio with the power of AI. Choose a plan that fits your growth and start automating your lead conversion today.</p>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
          {tiers.map((tier, idx) => (
            <div
              key={tier.name}
              className={`rounded-[32px] p-8 flex flex-col transition-all duration-300 relative ${
                tier.highlight
                  ? 'bg-white border-2 border-violet-500 shadow-xl shadow-violet-900/10 scale-105 z-10'
                  : 'bg-white/80 backdrop-blur-md border border-[#EAE8E2] hover:border-violet-300 hover:shadow-lg'
              }`}
            >
              {tier.highlight && (
                <div className="absolute -top-4 left-1/2 -translate-x-1/2 bg-violet-600 text-white text-xs font-bold px-4 py-1.5 rounded-full uppercase tracking-widest shadow-md">
                  Most Popular
                </div>
              )}
              
              <div className={`mb-6 h-12 w-12 flex items-center justify-center rounded-2xl ${
                tier.highlight ? 'bg-violet-100 text-violet-600' : 'bg-gray-100 text-gray-600'
              }`}>
                <Sparkles className="w-6 h-6" />
              </div>
              
              <h3 className="text-2xl font-black text-[#1A1A1A] mb-2">{tier.name}</h3>
              <p className="text-sm text-[#525252] min-h-[40px] mb-6">
                {tier.description}
              </p>
              
              <div className="mb-8">
                <span className="text-5xl font-black tracking-tight text-[#1A1A1A]">
                  ${tier.price}
                </span>
                <p className="text-sm font-medium text-[#737373] mt-2">
                  {tier.cycle?.toLowerCase() === 'one-time' ? 'One-time payment' : 'Per month'}
                </p>
              </div>

              <button
                onClick={() => handleSelectPlan(tier.name)}
                disabled={loadingTier !== null}
                className={`w-full py-4 rounded-xl text-base font-bold transition-all ${
                  tier.highlight
                    ? 'bg-violet-600 text-white hover:bg-violet-700 shadow-lg shadow-violet-600/30 hover:shadow-violet-600/50 hover:-translate-y-0.5'
                    : 'bg-[#FAF9F7] text-[#1A1A1A] border border-[#EAE8E2] hover:border-violet-300 hover:bg-violet-50'
                }`}
              >
                {loadingTier === tier.name ? 'Processing...' : `Try ${tier.name}`}
              </button>

              <div className="mt-8 flex-1">
                <p className="text-sm font-bold text-[#1A1A1A] mb-5">
                  {idx === 0 ? 'Includes:' : `Everything in ${tiers[idx - 1]?.name || ''}, plus:`}
                </p>
                <ul className="space-y-4">
                  {tier.features.map((feature) => (
                    <li key={feature} className="flex gap-x-3 text-sm text-[#525252]">
                      <Check className={`h-5 w-5 flex-none ${tier.highlight ? 'text-violet-600' : 'text-[#1A1A1A]'}`} aria-hidden="true" />
                      <span className="leading-relaxed font-medium">{feature}</span>
                    </li>
                  ))}
                </ul>
              </div>
            </div>
          ))}
        </div>
      </main>
    </div>
  );
}
