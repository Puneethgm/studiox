'use client';

import { useState, useEffect } from 'react';
import { Check } from 'lucide-react';
import { Button } from '@/components/ui/Button';

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
    <div className="min-h-screen bg-[#FAF9F7] font-sans text-slate-900 selection:bg-brand-500/30">
      {/* ── Navigation Bar ──────────────────────── */}
      <header className="sticky top-0 z-50 w-full border-b border-[#EAE8E2] bg-[#FAF9F7]/80 backdrop-blur-xl">
        <div className="mx-auto flex h-20 max-w-7xl items-center justify-between px-6 lg:px-8">
          <div className="flex items-center gap-2">
            <div className="grid h-10 w-10 place-items-center rounded-xl bg-gradient-to-br from-brand-500 to-violet-600 text-white shadow-lg shadow-brand-500/20">
              <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="lucide lucide-sparkles"><path d="M9.937 15.5A2 2 0 0 0 8.5 14.063l-6.135-1.582a.5.5 0 0 1 0-.962L8.5 9.936A2 2 0 0 0 9.937 8.5l1.582-6.135a.5.5 0 0 1 .963 0L14.063 8.5A2 2 0 0 0 15.5 9.937l6.135 1.581a.5.5 0 0 1 0 .964L15.5 14.063a2 2 0 0 0-1.437 1.437l-1.582 6.135a.5.5 0 0 1-.963 0z"/><path d="M20 3v4"/><path d="M22 5h-4"/><path d="M4 17v2"/><path d="M5 18H3"/></svg>
            </div>
            <span className="text-xl font-black tracking-tight text-[#1A1A1A]">1herosocial.ai</span>
          </div>
          
          <nav className="hidden md:flex gap-8">
            <a href="/#features" className="text-sm font-medium text-[#525252] hover:text-[#1A1A1A] transition-colors">
              Features
            </a>
            <a href="/pricing" className="text-sm font-medium text-[#525252] hover:text-[#1A1A1A] transition-colors">
              Pricing
            </a>
          </nav>

          <div className="flex items-center gap-4">
            <a href="/login">
              <button className="hidden sm:inline-flex text-sm font-medium text-[#525252] hover:text-[#1A1A1A] transition-colors">
                Log in
              </button>
            </a>
            <button className="bg-[#1A1A1A] text-white hover:bg-black px-4 py-2 rounded-lg text-sm font-medium transition-colors">
              Get Started
            </button>
          </div>
        </div>
      </header>

      {/* ── Pricing Content ──────────────────────── */}
      <main className="mx-auto max-w-7xl px-6 lg:px-8 py-24 sm:py-32">
        <div className="mb-16 text-center max-w-2xl mx-auto">
          <h1 className="text-4xl sm:text-5xl font-medium tracking-tight text-[#1A1A1A] mb-4">Plans & Pricing</h1>
          <p className="text-lg text-[#525252]">Scale your studio with the power of AI. Choose a plan that fits your growth and start automating your lead conversion today.</p>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
          {tiers.map((tier, idx) => (
            <div
              key={tier.name}
              className="bg-white border border-[#EAE8E2] rounded-[24px] p-8 flex flex-col hover:border-[#D0CDC5] transition-colors shadow-sm hover:shadow-md"
            >
              <div className="mb-6 h-12 w-12 text-[#1A1A1A]">
                {/* Minimalist icon representation based on Claude's aesthetic */}
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" className="w-8 h-8">
                  <circle cx="12" cy="12" r="3" />
                  <path d="M12 2v3M12 19v3M2 12h3M19 12h3M5 5l2 2M17 17l2 2M5 19l2-2M17 5l2 2" strokeLinecap="round"/>
                </svg>
              </div>
              <h3 className="text-2xl font-medium text-[#1A1A1A] mb-2">{tier.name}</h3>
              <p className="text-sm text-[#525252] min-h-[40px] mb-6">
                {tier.description}
              </p>
              
              <div className="mb-8">
                <span className="text-4xl font-medium text-[#1A1A1A]">
                  ${tier.price}
                </span>
                <p className="text-xs text-[#525252] mt-2">
                  {tier.cycle?.toLowerCase() === 'one-time' ? 'One-time payment' : 'Per month'}
                </p>
              </div>

              <button
                onClick={() => handleSelectPlan(tier.name)}
                disabled={loadingTier !== null}
                className={`w-full py-3 rounded-xl text-sm font-medium transition-colors border ${
                  tier.highlight
                    ? 'bg-[#1A1A1A] text-white hover:bg-black border-transparent shadow-md'
                    : 'bg-white text-[#1A1A1A] border-[#EAE8E2] hover:bg-[#FAF9F7]'
                }`}
              >
                {loadingTier === tier.name ? 'Processing...' : `Try ${tier.name}`}
              </button>

              <div className="mt-8 flex-1">
                <p className="text-sm font-medium text-[#1A1A1A] mb-4">
                  {idx === 0 ? 'Includes:' : `Everything in ${tiers[idx - 1].name}, plus:`}
                </p>
                <ul className="space-y-4">
                  {tier.features.map((feature) => (
                    <li key={feature} className="flex gap-x-3 text-sm text-[#525252]">
                      <Check className="h-5 w-5 flex-none text-[#1A1A1A]" aria-hidden="true" />
                      <span className="leading-relaxed">{feature}</span>
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
