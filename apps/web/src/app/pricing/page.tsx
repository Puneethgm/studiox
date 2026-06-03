import Link from 'next/link';
import { Check } from 'lucide-react';
import { Button } from '@/components/ui/Button';

const tiers = [
  {
    name: 'Trial Pass',
    price: 300,
    cycle: 'One-time',
    description: 'Entry-level setup to validate AI integration and lead generation.',
    features: [
      '1 Connected Channel',
      'Basic AI Auto-Replies (200/mo)',
      '1-day automated follow-up',
      'Google Sheets contact sync',
    ],
  },
  {
    name: 'Growth Tier',
    price: 999,
    cycle: 'Monthly',
    description: 'Automate workflows, payments, and client acquisition.',
    features: [
      '3 Connected Channels',
      'Full AI Auto-Replies (2,000/mo)',
      'Dedicated Knowledge Base',
      'Visual drag-and-drop Pipeline',
      'Stripe account integration',
    ],
  },
  {
    name: 'Pro Tier',
    price: 1299,
    cycle: 'Monthly',
    description: 'For active studios looking to scale reach via social media and paid advertising.',
    features: [
      '8 Connected Channels',
      'Extended AI Auto-Replies (10,000/mo)',
      'Dual model routing (Gemini + Claude)',
      'Advanced Social Planner',
      'Google Ads Channel Integration',
      'Studio Plan Option (Scheduling)',
    ],
    highlight: true,
  },
  {
    name: 'Enterprise Tier',
    price: 1599,
    cycle: 'Monthly',
    description: 'Maximum scale, custom branding, and multi-location management.',
    features: [
      'Unlimited Connected Channels',
      'Unlimited AI Auto-Replies',
      'Multi-Location Hub',
      'Whitelabel Dashboard',
      'Enterprise Studio Plan Option',
      'Priority Support SLA',
    ],
  },
];

export default function PricingPage() {
  return (
    <div className="min-h-screen bg-slate-50 py-24 sm:py-32 dark:bg-slate-950">
      <div className="mx-auto max-w-7xl px-6 lg:px-8">
        <div className="mx-auto max-w-4xl text-center">
          <h2 className="text-base font-semibold leading-7 text-brand-600">Pricing</h2>
          <p className="mt-2 text-4xl font-bold tracking-tight text-slate-900 dark:text-white sm:text-5xl">
            Choose the right plan for your studio
          </p>
        </div>
        <p className="mx-auto mt-6 max-w-2xl text-center text-lg leading-8 text-slate-600 dark:text-slate-400">
          Unlock advanced messaging automation, AI tools, and CRM features tailored for your studio's scale.
        </p>
        <div className="isolate mx-auto mt-16 grid max-w-md grid-cols-1 gap-y-8 sm:mt-20 lg:mx-0 lg:max-w-none lg:grid-cols-4 lg:gap-x-8 lg:gap-y-0">
          {tiers.map((tier, tierIdx) => (
            <div
              key={tier.name}
              className={`rounded-3xl p-8 ring-1 xl:p-10 ${
                tier.highlight
                  ? 'bg-brand-600 text-white ring-brand-600 shadow-2xl scale-105 z-10 relative'
                  : 'bg-white text-slate-900 ring-slate-200 dark:bg-slate-900 dark:text-white dark:ring-slate-800 relative z-0'
              }`}
            >
              <h3 className={`text-lg font-semibold leading-8 ${tier.highlight ? 'text-white' : 'text-slate-900 dark:text-white'}`}>
                {tier.name}
              </h3>
              <p className={`mt-4 text-sm leading-6 ${tier.highlight ? 'text-brand-100' : 'text-slate-600 dark:text-slate-400'}`}>
                {tier.description}
              </p>
              <p className="mt-6 flex items-baseline gap-x-1">
                <span className={`text-4xl font-bold tracking-tight ${tier.highlight ? 'text-white' : 'text-slate-900 dark:text-white'}`}>
                  ${tier.price}
                </span>
                <span className={`text-sm font-semibold leading-6 ${tier.highlight ? 'text-brand-100' : 'text-slate-600 dark:text-slate-400'}`}>
                  /{tier.cycle}
                </span>
              </p>
              <Button
                variant={tier.highlight ? 'secondary' : 'default'}
                className="mt-6 w-full"
                onClick={() => alert('Stripe checkout integration pending backend webhook routing.')}
              >
                Select Plan
              </Button>
              <ul className={`mt-8 space-y-3 text-sm leading-6 ${tier.highlight ? 'text-brand-50' : 'text-slate-600 dark:text-slate-400'}`}>
                {tier.features.map((feature) => (
                  <li key={feature} className="flex gap-x-3">
                    <Check className={`h-6 w-5 flex-none ${tier.highlight ? 'text-brand-200' : 'text-brand-600'}`} aria-hidden="true" />
                    {feature}
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
