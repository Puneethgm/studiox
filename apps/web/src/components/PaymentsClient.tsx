'use client';

import React, { useState, useEffect } from 'react';
import { 
  CreditCard, 
  ArrowUpRight, 
  TrendingUp, 
  CheckCircle, 
  ShieldCheck, 
  ArrowRight,
  Download,
  AlertCircle
} from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Badge } from '@/components/ui/Badge';
import { api } from '@/lib/api';

interface Invoice {
  id: string;
  date: string;
  amountSGD: number;
  amountINR: number;
  status: 'paid' | 'pending' | 'failed';
  plan: string;
}

export default function PaymentsClient({ studioId }: { studioId: string }) {
  const [currency, setCurrency] = useState<'SGD' | 'INR'>('SGD');
  const [plan, setPlan] = useState<'starter' | 'pro' | 'enterprise'>('pro');
  const [stripeStatus, setStripeStatus] = useState<'connected' | 'disconnected'>('disconnected');
  const [stripeAccountId, setStripeAccountId] = useState('');
  const [loading, setLoading] = useState(true);
  
  // Link Stripe Form state
  const [showForm, setShowForm] = useState(false);
  const [formStripeAccountId, setFormStripeAccountId] = useState('');
  const [formPublishableKey, setFormPublishableKey] = useState('');
  const [formSecretKey, setFormSecretKey] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  useEffect(() => {
    if (studioId === 'global') {
      setPlan('pro');
      setStripeStatus('disconnected');
      setLoading(false);
      return;
    }
    
    setLoading(true);
    api<{ stripeAccountId: string; stripePublishableKey: string; stripeSecretKey: string; subscriptionTier: string }>(
      `/api/v1/me/studios/${studioId}/payments`
    ).then((res) => {
      setPlan((res.subscriptionTier || 'pro') as any);
      if (res.stripeAccountId) {
        setStripeStatus('connected');
        setStripeAccountId(res.stripeAccountId);
        setFormStripeAccountId(res.stripeAccountId);
        setFormPublishableKey(res.stripePublishableKey || '');
        setFormSecretKey(res.stripeSecretKey || '');
      } else {
        setStripeStatus('disconnected');
      }
      setLoading(false);
    }).catch(() => {
      // Gracefully fall back — studio may not have Stripe configured yet
      setStripeStatus('disconnected');
      setLoading(false);
    });
  }, [studioId]);

  const handleLinkStripe = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    setFormError(null);
    try {
      await api(`/api/v1/me/studios/${studioId}/payments/stripe`, {
        method: 'POST',
        json: {
          stripeAccountId: formStripeAccountId,
          stripePublishableKey: formPublishableKey,
          stripeSecretKey: formSecretKey,
        }
      });
      setStripeAccountId(formStripeAccountId);
      setStripeStatus('connected');
      setShowForm(false);
    } catch (err: any) {
      setFormError(err.message || 'Failed to connect Stripe account');
    } finally {
      setSubmitting(false);
    }
  };

  const handleDisconnect = async () => {
    if (studioId === 'global') return;
    try {
      await api(`/api/v1/me/studios/${studioId}/payments/stripe`, {
        method: 'POST',
        json: {
          stripeAccountId: '',
          stripePublishableKey: '',
          stripeSecretKey: '',
        }
      });
      setStripeAccountId('');
      setStripeStatus('disconnected');
      setFormStripeAccountId('');
      setFormPublishableKey('');
      setFormSecretKey('');
    } catch (err) {
      console.error('Failed to disconnect Stripe account:', err);
    }
  };

  const handleUpdatePlan = async (newPlan: 'starter' | 'pro' | 'enterprise') => {
    if (studioId === 'global') {
      setPlan(newPlan);
      return;
    }
    try {
      await api(`/api/v1/me/studios/${studioId}/payments/plan`, {
        method: 'POST',
        json: { subscriptionTier: newPlan }
      });
      setPlan(newPlan);
    } catch (err) {
      console.error('Failed to update plan:', err);
    }
  };

  const stats = {
    monthlyFeeSGD: plan === 'starter' ? 149 : plan === 'pro' ? 299 : 599,
    monthlyFeeINR: plan === 'starter' ? 12900 : plan === 'pro' ? 25900 : 51900,
    outstandingSGD: 0,
    outstandingINR: 0,
    lifetimePaidSGD: 0,
    lifetimePaidINR: 0,
  };

  const invoices: Invoice[] = [];

  const formatAmount = (sgdVal: number, inrVal: number) => {
    if (currency === 'SGD') {
      return new Intl.NumberFormat('en-SG', { style: 'currency', currency: 'SGD' }).format(sgdVal);
    }
    return new Intl.NumberFormat('en-IN', { style: 'currency', currency: 'INR', maximumFractionDigits: 0 }).format(inrVal);
  };

  if (loading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <div className="h-6 w-6 animate-spin rounded-full border-2 border-brand-500 border-t-transparent" />
      </div>
    );
  }

  return (
    <div className="space-y-6 animate-in fade-in duration-300">
      {/* Top Bar with Currency Selector */}
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 border-b border-white/10 pb-4">
        <div>
          <h2 className="text-base font-black text-zinc-900 dark:text-white">Billing & Payments Plan</h2>
          <p className="text-[11px] text-zinc-400">Configure Stripe gateway, change subscription, or select billing currency</p>
        </div>
        <div className="flex items-center gap-3">
          <label className="text-xs font-bold text-zinc-400 uppercase tracking-wider">Billing Currency</label>
          <select
            value={currency}
            onChange={(e) => setCurrency(e.target.value as 'SGD' | 'INR')}
            className="rounded-xl border border-white/20 bg-white/10 px-3 py-1.5 text-xs font-bold text-zinc-800 dark:bg-neutral-800 dark:text-white focus:outline-none"
          >
            <option value="SGD">SGD (S$)</option>
            <option value="INR">INR (₹)</option>
          </select>
        </div>
      </div>

      {/* Stats row */}
      <div className="grid gap-6 sm:grid-cols-3">
        <Card className="border-white/30 bg-white/20 dark:border-white/5 dark:bg-neutral-900/30 backdrop-blur-2xl p-6">
          <span className="text-[10px] font-black uppercase tracking-wider text-zinc-400 block">Subscription Fee</span>
          <div className="mt-2 flex items-baseline gap-2">
            <span className="text-3xl font-black text-zinc-950 dark:text-white">
              {formatAmount(stats.monthlyFeeSGD, stats.monthlyFeeINR)}
            </span>
            <span className="text-xs text-zinc-400">/ month</span>
          </div>
          <span className="text-[10px] text-brand-500 font-bold uppercase tracking-wider mt-2 block">
            Plan: {plan.toUpperCase()}
          </span>
        </Card>

        <Card className="border-white/30 bg-white/20 dark:border-white/5 dark:bg-neutral-900/30 backdrop-blur-2xl p-6">
          <span className="text-[10px] font-black uppercase tracking-wider text-zinc-400 block">Outstanding Balance</span>
          <div className="mt-2 flex items-baseline gap-2">
            <span className="text-3xl font-black text-zinc-950 dark:text-white">
              {formatAmount(stats.outstandingSGD, stats.outstandingINR)}
            </span>
          </div>
          <span className="text-[10px] text-emerald-500 font-bold uppercase tracking-wider mt-2 block flex items-center gap-1">
            <CheckCircle className="h-3 w-3" /> Settled
          </span>
        </Card>

        <Card className="border-white/30 bg-white/20 dark:border-white/5 dark:bg-neutral-900/30 backdrop-blur-2xl p-6">
          <span className="text-[10px] font-black uppercase tracking-wider text-zinc-400 block">Lifetime Payments</span>
          <div className="mt-2 flex items-baseline gap-2">
            <span className="text-3xl font-black text-zinc-950 dark:text-white">
              {formatAmount(stats.lifetimePaidSGD, stats.lifetimePaidINR)}
            </span>
          </div>
          <span className="text-[10px] text-zinc-400 font-bold uppercase tracking-wider mt-2 block">
            Last Paid: {invoices[0]?.date || 'N/A'}
          </span>
        </Card>
      </div>

      {/* Main Sections */}
      <div className="grid gap-6 lg:grid-cols-3">
        {/* Plan Upgrade / Stripe Connect */}
        <div className="lg:col-span-1 space-y-6">
          {/* Stripe Connect Card */}
          <Card className="border-white/30 bg-white/20 dark:border-white/5 dark:bg-neutral-900/30 backdrop-blur-2xl">
            <h3 className="text-sm font-black text-zinc-950 dark:text-white mb-3">Stripe Gateway</h3>
            <p className="text-xs text-zinc-400 mb-4 leading-relaxed">
              Connect your studio Stripe account to handle client billing, memberships, and automated invoices.
            </p>
            
            {studioId === 'global' ? (
              <div className="rounded-xl border border-white/10 bg-white/10 p-4 text-center dark:bg-neutral-800/20">
                <AlertCircle className="h-5 w-5 mx-auto mb-2 text-zinc-400" />
                <span className="text-xs font-bold text-zinc-500 dark:text-zinc-400 block">
                  Stripe connections are managed per studio location.
                </span>
              </div>
            ) : stripeStatus === 'connected' ? (
              <div className="space-y-4">
                <div className="flex items-center gap-3 rounded-xl bg-emerald-500/10 p-3 text-emerald-600 dark:text-emerald-400">
                  <ShieldCheck className="h-5 w-5 shrink-0" />
                  <div className="min-w-0">
                    <span className="text-xs font-bold block">Connected to Stripe</span>
                    <span className="text-[10px] text-emerald-500/80 block truncate">Account: {stripeAccountId}</span>
                  </div>
                </div>
                <div className="rounded-xl border border-white/10 bg-white/10 p-3 dark:bg-neutral-800/20">
                  <span className="text-[10px] font-bold text-zinc-400 uppercase block">Active Card</span>
                  <div className="flex items-center gap-2 mt-1">
                    <CreditCard className="h-4 w-4 text-brand-500" />
                    <span className="text-xs font-semibold text-zinc-800 dark:text-zinc-200">Visa ending in 4242</span>
                  </div>
                </div>
                <Button variant="ghost" className="w-full text-xs text-red-500 border border-red-500/20 hover:bg-red-500/10" onClick={handleDisconnect}>
                  Disconnect Account
                </Button>
              </div>
            ) : showForm ? (
              <form onSubmit={handleLinkStripe} className="space-y-3.5 animate-in fade-in slide-in-from-top-2 duration-300">
                <div>
                  <label className="text-[10px] font-bold text-zinc-400 uppercase tracking-wider block mb-1">Stripe Account ID</label>
                  <input
                    type="text"
                    required
                    placeholder="acct_..."
                    value={formStripeAccountId}
                    onChange={(e) => setFormStripeAccountId(e.target.value)}
                    className="w-full rounded-xl border border-white/20 bg-white/10 px-3 py-2 text-xs font-bold text-zinc-800 dark:bg-neutral-800 dark:text-white focus:outline-none"
                  />
                </div>
                <div>
                  <label className="text-[10px] font-bold text-zinc-400 uppercase tracking-wider block mb-1">Publishable Key</label>
                  <input
                    type="text"
                    required
                    placeholder="pk_test_..."
                    value={formPublishableKey}
                    onChange={(e) => setFormPublishableKey(e.target.value)}
                    className="w-full rounded-xl border border-white/20 bg-white/10 px-3 py-2 text-xs font-bold text-zinc-800 dark:bg-neutral-800 dark:text-white focus:outline-none"
                  />
                </div>
                <div>
                  <label className="text-[10px] font-bold text-zinc-400 uppercase tracking-wider block mb-1">Secret Key</label>
                  <input
                    type="password"
                    required
                    placeholder="sk_test_..."
                    value={formSecretKey}
                    onChange={(e) => setFormSecretKey(e.target.value)}
                    className="w-full rounded-xl border border-white/20 bg-white/10 px-3 py-2 text-xs font-bold text-zinc-800 dark:bg-neutral-800 dark:text-white focus:outline-none"
                  />
                </div>
                {formError && (
                  <p className="text-[10px] text-red-500 font-bold">{formError}</p>
                )}
                <div className="flex gap-2 pt-1">
                  <Button type="button" variant="ghost" className="flex-1 text-xs" onClick={() => setShowForm(false)}>
                    Cancel
                  </Button>
                  <Button type="submit" className="flex-1 text-xs" loading={submitting}>
                    Connect
                  </Button>
                </div>
              </form>
            ) : (
              <Button className="w-full shadow-lg shadow-brand-500/15" onClick={() => setShowForm(true)}>
                Link Stripe Account
              </Button>
            )}
          </Card>

          {/* Pricing tier selector */}
          <Card className="border-white/30 bg-white/20 dark:border-white/5 dark:bg-neutral-900/30 backdrop-blur-2xl">
            <h3 className="text-sm font-black text-zinc-950 dark:text-white mb-3">Change Tier Plan</h3>
            <div className="space-y-3">
              {[
                { key: 'starter', label: 'Starter Tier', desc: '1 Active Location, basic leads capturing' },
                { key: 'pro', label: 'Pro Scale Tier', desc: 'Multi-location capabilities, unlimited CRM' },
                { key: 'enterprise', label: 'Enterprise Tier', desc: 'Full custom integration + developer API support' },
              ].map(p => (
                <button
                  key={p.key}
                  onClick={() => handleUpdatePlan(p.key as any)}
                  className={`w-full text-left p-3 rounded-xl border transition-all ${
                    plan === p.key 
                      ? 'border-brand-500 bg-brand-500/5' 
                      : 'border-white/10 bg-white/5 hover:bg-white/10'
                  }`}
                >
                  <span className="text-xs font-bold block text-zinc-900 dark:text-zinc-100">{p.label}</span>
                  <span className="text-[10px] text-zinc-400 block mt-0.5">{p.desc}</span>
                </button>
              ))}
            </div>
          </Card>
        </div>

        {/* Invoice List */}
        <div className="lg:col-span-2">
          <Card className="border-white/30 bg-white/20 dark:border-white/5 dark:bg-neutral-900/30 backdrop-blur-2xl">
            <h3 className="text-sm font-black text-zinc-950 dark:text-white mb-4">Billing History & Invoices</h3>
            {invoices.length === 0 ? (
              <div className="flex h-36 flex-col items-center justify-center text-center">
                <AlertCircle className="h-6 w-6 text-zinc-400 mb-2" />
                <span className="text-xs font-bold text-zinc-500">No active invoices. Connect Stripe to activate billing ledger.</span>
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-left">
                  <thead>
                    <tr className="border-b border-white/10 text-[9px] font-black uppercase tracking-wider text-zinc-400">
                      <th className="pb-3">Invoice ID</th>
                      <th className="pb-3">Date</th>
                      <th className="pb-3">Plan</th>
                      <th className="pb-3">Amount</th>
                      <th className="pb-3">Status</th>
                      <th className="pb-3 text-right">Actions</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-white/5">
                    {invoices.map(inv => (
                      <tr key={inv.id} className="text-xs text-zinc-700 dark:text-zinc-300">
                        <td className="py-3 font-semibold">{inv.id}</td>
                        <td className="py-3">{inv.date}</td>
                        <td className="py-3 font-medium">{inv.plan}</td>
                        <td className="py-3 font-bold text-zinc-950 dark:text-white">
                          {formatAmount(inv.amountSGD, inv.amountINR)}
                        </td>
                        <td className="py-3">
                          <Badge tone={inv.status === 'paid' ? 'success' : 'neutral'}>
                            {inv.status}
                          </Badge>
                        </td>
                        <td className="py-3 text-right">
                          <button className="p-1 hover:bg-white/15 rounded-lg text-zinc-400 hover:text-white transition-all">
                            <Download className="h-4 w-4" />
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </Card>
        </div>
      </div>
    </div>
  );
}
