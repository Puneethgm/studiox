'use client';

import { useState, useEffect } from 'react';
import { Check, Sparkles, ArrowRight, Star } from 'lucide-react';
import Link from 'next/link';

type Tier = {
  name: string;
  price: number;
  cycle: string;
  description: string;
  features: string[];
  highlight?: boolean;
};

const FALLBACK_TIERS: Tier[] = [
  {
    name: 'Starter',
    price: 49,
    cycle: 'monthly',
    description: 'Perfect for single-location studios getting started with automated leads.',
    features: [
      '1 Connected Location',
      'AI Lead Response Assistant',
      'Basic FAQ Knowledge Base',
      'WhatsApp & Instagram DMs',
      'Email Support'
    ]
  },
  {
    name: 'Grow',
    price: 99,
    cycle: 'monthly',
    description: 'Designed for expanding studios seeking full automation pipelines.',
    features: [
      'Up to 3 Connected Locations',
      'Advanced RAG AI Conversations',
      'Unlimited FAQ Uploads',
      'Stripe Billing Integration',
      'WhatsApp, Instagram, & FB DMs'
    ],
    highlight: true
  },
  {
    name: 'Scale',
    price: 199,
    cycle: 'monthly',
    description: 'Comprehensive features for multi-studio chains and gym groups.',
    features: [
      'Unlimited Studio Locations',
      'Custom Voice AI Tuning',
      'Multi-agent Broadcasts',
      'Performance Dashboards',
      'Direct CRM Sync & Exports'
    ]
  },
  {
    name: 'Enterprise',
    price: 399,
    cycle: 'monthly',
    description: 'Bespoke solutions and dedicated hosting for large fitness franchises.',
    features: [
      'Custom SLA Agreements',
      'Dedicated Database Instance',
      'Custom Integration Development',
      'Whitelabel Dashboard Option',
      '24/7 Dedicated Phone Support'
    ]
  }
];

export default function PricingPage() {
  const [loadingTier, setLoadingTier] = useState<string | null>(null);
  const [tiers, setTiers] = useState<Tier[]>(FALLBACK_TIERS);
  const [loadingPlans, setLoadingPlans] = useState(true);

  useEffect(() => {
    fetch('/api/v1/public/platform/plans')
      .then((res) => res.json())
      .then((data) => {
        if (Array.isArray(data) && data.length > 0) {
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
    <div className="min-h-screen bg-[#FAF9F6] font-sans text-slate-800 selection:bg-violet-600/10 selection:text-violet-900 overflow-x-hidden">
      
      {/* Background Soft Glows */}
      <div className="absolute top-0 right-0 -z-10 h-[600px] w-[600px] rounded-full bg-violet-100/50 blur-[130px] pointer-events-none" />
      <div className="absolute top-[400px] left-0 -z-10 h-[700px] w-[700px] rounded-full bg-indigo-100/40 blur-[120px] pointer-events-none" />

      {/* ── Navigation (Full Width) ─────────────────── */}
      <header className="sticky top-0 z-50 w-full border-b border-slate-200/50 bg-[#FAF9F6]/75 backdrop-blur-md">
        <div className="mx-auto flex h-18 w-full max-w-[1600px] items-center justify-between px-6 md:px-12 py-4">
          <div className="flex items-center gap-3">
            <Link href="/" className="flex items-center gap-3">
              <div className="grid h-10 w-10 place-items-center rounded-xl bg-gradient-to-br from-violet-600 to-indigo-600 text-white shadow-lg shadow-violet-600/10">
                <Sparkles className="h-5 w-5" />
              </div>
              <span className="text-xl font-black tracking-tight text-slate-900">1herosocial.ai</span>
            </Link>
          </div>

          <nav className="hidden lg:flex items-center gap-10">
            <Link href="/#simulator" className="text-sm font-semibold text-slate-600 hover:text-violet-600 transition-colors">Lead Stream</Link>
            <Link href="/#calculator" className="text-sm font-semibold text-slate-600 hover:text-violet-600 transition-colors">ROI Calculator</Link>
            <Link href="/#features" className="text-sm font-semibold text-slate-600 hover:text-violet-600 transition-colors">Platform Features</Link>
            <Link href="/pricing" className="text-sm font-semibold text-slate-600 hover:text-violet-600 transition-colors">Pricing Plans</Link>
          </nav>

          <div className="flex items-center gap-6">
            <Link href="/login" className="text-sm font-semibold text-slate-600 hover:text-violet-600 transition-colors">
              Log in
            </Link>
          </div>
        </div>
      </header>

      {/* ── Pricing Hero Split Layout (Wide Viewport) ── */}
      <section className="relative pt-12 pb-10 md:pt-28 md:pb-16 px-4 md:px-12 max-w-[1600px] mx-auto">
        <div className="grid grid-cols-1 lg:grid-cols-12 gap-10 lg:gap-8 items-center">
          
          {/* Left Text Column */}
          <div className="lg:col-span-7 space-y-6 text-left">
            <div className="inline-flex items-center gap-2 bg-violet-600/5 border border-violet-600/10 text-violet-700 text-xs font-bold px-4 py-1.5 rounded-full backdrop-blur-sm">
              <Star className="h-3.5 w-3.5 fill-violet-600 text-violet-600" />
              Flexible Subscription Plans
            </div>

            <h1 className="text-3xl sm:text-5xl lg:text-6xl xl:text-7xl font-extrabold tracking-tight text-slate-900 leading-[1.1] lg:leading-[1.05]">
              Transparent Pricing.
              <span className="block mt-1 md:mt-2 text-transparent bg-clip-text bg-gradient-to-r from-violet-600 via-fuchsia-600 to-indigo-600">
                Built to Scale Networks.
              </span>
            </h1>

            <p className="text-base sm:text-lg md:text-xl text-slate-600 max-w-2xl leading-relaxed">
              No hidden fees. Scale operations across physical studios while utilizing custom RAG knowledge bases, Stripe integrations, and visual messaging pipelines.
            </p>
          </div>

          {/* Right Column Badge Grid */}
          <div className="lg:col-span-5 relative grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div className="bg-white border border-slate-200 p-5 rounded-2xl shadow-sm text-left">
              <div className="h-10 w-10 rounded-lg bg-emerald-50 text-emerald-600 flex items-center justify-center font-bold mb-4">L</div>
              <h3 className="font-bold text-slate-900 text-sm">Flexible Billing</h3>
              <p className="text-xs text-slate-500 mt-1">Upgrade, downgrade, or cancel your subscription plan at any time.</p>
            </div>
            <div className="bg-white border border-slate-200 p-5 rounded-2xl shadow-sm text-left">
              <div className="h-10 w-10 rounded-lg bg-violet-50 text-violet-600 flex items-center justify-center font-bold mb-4">V</div>
              <h3 className="font-bold text-slate-900 text-sm">Vector Processing</h3>
              <p className="text-xs text-slate-500 mt-1">Included FAQ uploads are embedded instantly to power your studio agents.</p>
            </div>
          </div>
        </div>
      </section>

      {/* ── Pricing Cards Grid (Single Row, Full Width) ── */}
      <section className="px-4 md:px-12 pb-20 md:pb-28 max-w-[1600px] mx-auto">
        <div className={`grid grid-cols-1 md:grid-cols-2 gap-8 items-stretch ${
          tiers.length === 4 ? 'xl:grid-cols-4' :
          tiers.length === 3 ? 'lg:grid-cols-3' :
          'lg:grid-cols-2'
        }`}>
          {tiers.map((tier, idx) => (
            <div
              key={tier.name}
              className={`rounded-[32px] p-5 sm:p-8 flex flex-col justify-between transition-all duration-300 relative text-left ${
                tier.highlight
                  ? 'bg-white border-2 border-violet-500 shadow-xl shadow-violet-900/[0.04] lg:scale-105 z-10'
                  : 'bg-white border border-slate-200 hover:border-violet-300 hover:shadow-lg'
              }`}
            >
              {tier.highlight && (
                <div className="absolute -top-4 left-1/2 -translate-x-1/2 bg-violet-600 text-white text-xs font-bold px-4 py-1.5 rounded-full uppercase tracking-widest shadow-md">
                  Most Popular
                </div>
              )}
              
              <div>
                <div className={`mb-6 h-12 w-12 flex items-center justify-center rounded-2xl ${
                  tier.highlight ? 'bg-violet-100 text-violet-600' : 'bg-slate-100 text-slate-600'
                }`}>
                  <Sparkles className="w-6 h-6" />
                </div>
                
                <h3 className="text-2xl font-black text-slate-900 mb-2">{tier.name}</h3>
                <p className="text-sm text-slate-500 min-h-[40px] mb-6 font-medium">
                  {tier.description}
                </p>
                
                <div className="mb-8">
                  <span className="text-5xl font-black tracking-tight text-slate-900">
                    ${tier.price}
                  </span>
                  <span className="text-sm font-bold text-slate-400 ml-2">
                    {tier.cycle?.toLowerCase() === 'one-time' ? '/one-time' : '/month'}
                  </span>
                </div>

                <div className="border-t border-slate-100 pt-6">
                  <p className="text-xs font-extrabold text-slate-950 uppercase tracking-wider mb-4">
                    Key Features Included:
                  </p>
                  <ul className="space-y-3.5">
                    {tier.features.map((feature) => (
                      <li key={feature} className="flex gap-x-3 text-sm text-slate-600">
                        <Check className={`h-5 w-5 flex-none ${tier.highlight ? 'text-violet-600' : 'text-slate-700'}`} aria-hidden="true" />
                        <span className="leading-relaxed font-semibold">{feature}</span>
                      </li>
                    ))}
                  </ul>
                </div>
              </div>

              <div className="mt-10">
                <button
                  onClick={() => handleSelectPlan(tier.name)}
                  disabled={loadingTier !== null}
                  className={`w-full py-4 rounded-xl text-base font-bold transition-all ${
                    tier.highlight
                      ? 'bg-gradient-to-r from-violet-600 to-indigo-600 text-white hover:from-violet-700 hover:to-indigo-700 shadow-lg shadow-violet-600/20 hover:shadow-violet-600/30'
                      : 'bg-slate-50 text-slate-800 border border-slate-200 hover:border-violet-300 hover:bg-violet-50'
                  }`}
                >
                  {loadingTier === tier.name ? 'Processing...' : `Get Started with ${tier.name}`}
                </button>
              </div>
            </div>
          ))}
        </div>
      </section>

      {/* ── Footer ──────────────────────────────────── */}
      <footer className="bg-slate-50 text-slate-500 border-t border-slate-200/80">
        <div className="mx-auto max-w-[1600px] px-6 md:px-12 py-16">
          <div className="flex flex-col lg:flex-row justify-between gap-12 mb-12">
            {/* Brand */}
            <div className="max-w-xs text-left">
              <div className="flex items-center gap-2.5 mb-6">
                <div className="grid h-10 w-10 place-items-center rounded-xl bg-gradient-to-br from-violet-600 to-indigo-600 text-white shadow-md">
                  <Sparkles className="h-5 w-5" />
                </div>
                <span className="text-xl font-black tracking-tight text-slate-900">1herosocial.ai</span>
              </div>
              <p className="text-sm text-slate-500 leading-relaxed">
                The modern, central workspace built for multi-location fitness and wellness studios to optimize lead conversions.
              </p>
            </div>

            {/* Sitemap */}
            <div className="grid grid-cols-2 sm:grid-cols-3 gap-8 text-sm text-left">
              <div>
                <h4 className="font-bold text-slate-800 mb-4">Product</h4>
                <ul className="space-y-3 text-slate-500">
                  <li><Link href="/#features" className="hover:text-violet-600 transition-colors">Features</Link></li>
                  <li><Link href="/pricing" className="hover:text-violet-600 transition-colors">Pricing</Link></li>
                  <li><Link href="/#simulator" className="hover:text-violet-600 transition-colors">Lead Stream</Link></li>
                </ul>
              </div>
              <div>
                <h4 className="font-bold text-slate-800 mb-4">Account</h4>
                <ul className="space-y-3 text-slate-500">
                  <li><Link href="/login" className="hover:text-violet-600 transition-colors">Log In</Link></li>
                  <li><Link href="/pricing" className="hover:text-violet-600 transition-colors">Register</Link></li>
                </ul>
              </div>
              <div>
                <h4 className="font-bold text-slate-800 mb-4">Legal</h4>
                <ul className="space-y-3 text-slate-500">
                  <li><a href="#" className="hover:text-violet-600 transition-colors">Privacy Policy</a></li>
                  <li><a href="#" className="hover:text-violet-600 transition-colors">Terms of Service</a></li>
                </ul>
              </div>
            </div>
          </div>

          <div className="border-t border-slate-200 pt-8 flex flex-col sm:flex-row items-center justify-between gap-4 text-xs text-slate-400">
            <p>© 2026 1herosocial.ai. All rights reserved.</p>
            <p>Built for fitness & wellness studios worldwide 🌏</p>
          </div>
        </div>
      </footer>
    </div>
  );
}
