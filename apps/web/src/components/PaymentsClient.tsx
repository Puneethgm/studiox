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
  number: string;
  amount_due: number;
  amount_paid: number;
  currency: string;
  status: string;
  created: number;
  hosted_invoice_url: string;
  invoice_pdf: string;
  description?: string;
  metadata?: Record<string, string>;
}

export default function PaymentsClient({ studioId }: { studioId: string }) {
  const [currency, setCurrency] = useState<'SGD' | 'INR' | 'USD'>('SGD');
  const [stripeStatus, setStripeStatus] = useState<'connected' | 'disconnected'>('disconnected');
  const [stripeAccountId, setStripeAccountId] = useState('');
  const [loading, setLoading] = useState(true);
  
  const [billingStats, setBillingStats] = useState({ outstandingSGD: 0, outstandingINR: 0, lifetimePaidSGD: 0, lifetimePaidINR: 0, lifetimePaidUSD: 0, lifetimePaid: 0 });
  const [invoices, setInvoices] = useState<Invoice[]>([]);
  
  // Link Stripe Form state
  const [showForm, setShowForm] = useState(false);
  const [formStripeAccountId, setFormStripeAccountId] = useState('');
  const [formPublishableKey, setFormPublishableKey] = useState('');
  const [formSecretKey, setFormSecretKey] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  useEffect(() => {
    if (studioId === 'global') {
      setStripeStatus('disconnected');
      setLoading(false);
      return;
    }

    setLoading(true);
    void (async () => {
      try {
        const res = await api<{ stripeAccountId: string; stripePublishableKey: string; hasStripeSecretKey: boolean; subscriptionTier: string }>(
          `/api/v1/me/studios/${studioId}/payments`
        );
        if (res.stripeAccountId && res.hasStripeSecretKey) {
          setStripeStatus('connected');
          setStripeAccountId(res.stripeAccountId);
          setFormStripeAccountId(res.stripeAccountId);
          setFormPublishableKey(res.stripePublishableKey || '');
          setFormSecretKey(''); // Do not pre-fill secret key
          
          try {
            const historyRes = await api<{ invoices: Invoice[], stats: any }>(`/api/v1/me/studios/${studioId}/billing/history`);
            if (historyRes.invoices) setInvoices(historyRes.invoices);
            if (historyRes.stats) setBillingStats(historyRes.stats);
          } catch (e) {
            console.error('failed to fetch billing history', e);
          }
        } else {
          setStripeStatus('disconnected');
        }
      } catch {
        // Gracefully fall back — studio may not have Stripe configured yet
        setStripeStatus('disconnected');
      } finally {
        setLoading(false);
      }
    })();
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

  const stats = {
    outstandingSGD: billingStats.outstandingSGD / 100,
    outstandingINR: billingStats.outstandingINR / 100,
    lifetimePaidSGD: billingStats.lifetimePaidSGD / 100,
    lifetimePaidINR: billingStats.lifetimePaidINR / 100,
    lifetimePaidUSD: (billingStats.lifetimePaidUSD ?? 0) / 100,
    lifetimePaidTotal: (billingStats.lifetimePaid ?? 0) / 100,
  };

  const formatAmount = (sgdVal: number, inrVal: number, usdVal: number) => {
    if (currency === 'INR') return new Intl.NumberFormat('en-IN', { style: 'currency', currency: 'INR', maximumFractionDigits: 0 }).format(inrVal);
    if (currency === 'USD') return new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' }).format(usdVal);
    return new Intl.NumberFormat('en-SG', { style: 'currency', currency: 'SGD' }).format(sgdVal);
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
      <div className="flex justify-end border-b border-white/10 pb-4">
        <div className="flex items-center gap-3">
          <label className="text-xs font-bold text-zinc-400 uppercase tracking-wider">Billing Currency</label>
          <select
            value={currency}
            onChange={(e) => setCurrency(e.target.value as 'SGD' | 'INR' | 'USD')}
            className="rounded-xl border border-white/20 bg-white/10 px-3 py-1.5 text-xs font-bold text-zinc-800 dark:bg-neutral-800 dark:text-white focus:outline-none"
          >
            <option value="SGD">SGD (S$)</option>
            <option value="INR">INR (₹)</option>
            <option value="USD">USD ($)</option>
          </select>
        </div>
      </div>

      {/* Stats row */}
      <div className="grid gap-6 sm:grid-cols-2">
        <Card className="border-white/30 bg-white/20 dark:border-white/5 dark:bg-neutral-900/30 backdrop-blur-2xl p-6">
          <span className="text-[10px] font-black uppercase tracking-wider text-zinc-400 block">Outstanding Balance</span>
          <div className="mt-2 flex items-baseline gap-2">
            <span className="text-3xl font-black text-zinc-950 dark:text-white">
              {formatAmount(stats.outstandingSGD, stats.outstandingINR, 0)}
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
              {formatAmount(stats.lifetimePaidSGD, stats.lifetimePaidINR, stats.lifetimePaidUSD)}
            </span>
          </div>
          <span className="text-[10px] text-zinc-400 font-bold uppercase tracking-wider mt-2 block">
            Last Paid: {invoices[0] ? new Date(invoices[0].created * 1000).toLocaleDateString() : 'N/A'}
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
              <div className="space-y-3">
                <Button className="w-full shadow-lg shadow-brand-500/15" onClick={() => window.location.href = `/api/v1/studios/${studioId}/stripe-oauth/login`}>
                  Connect with Stripe Connect (Recommended)
                </Button>
                <button type="button" onClick={() => setShowForm(true)} className="w-full text-xs text-zinc-400 hover:text-white transition-colors">
                  Or enter API keys manually
                </button>
              </div>
            )}
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
                      <th className="pb-3">Ref</th>
                      <th className="pb-3">Date</th>
                      <th className="pb-3">Description</th>
                      <th className="pb-3">Amount</th>
                      <th className="pb-3">Status</th>
                      <th className="pb-3 text-right">Receipt</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-white/5">
                    {invoices.map(inv => (
                      <tr key={inv.id} className="text-xs text-zinc-700 dark:text-zinc-300">
                        <td className="py-3 font-semibold font-mono text-zinc-500">{inv.number || inv.id.slice(0,12)}</td>
                        <td className="py-3">{new Date(inv.created * 1000).toLocaleDateString()}</td>
                        <td className="py-3 text-zinc-400 max-w-[160px] truncate">{inv.description || inv.metadata?.customer_name || 'Trial Session'}</td>
                        <td className="py-3 font-bold text-zinc-950 dark:text-white">
                          {new Intl.NumberFormat('en-US', { style: 'currency', currency: inv.currency.toUpperCase() }).format(inv.amount_paid / 100)}
                        </td>
                        <td className="py-3">
                          <Badge tone={inv.status === 'paid' ? 'success' : 'neutral'}>
                            {inv.status}
                          </Badge>
                        </td>
                        <td className="py-3 text-right">
                          {inv.hosted_invoice_url ? (
                            <a href={inv.hosted_invoice_url} target="_blank" rel="noreferrer" className="inline-block p-1 hover:bg-white/15 rounded-lg text-zinc-400 hover:text-white transition-all" title="View Receipt">
                              <ArrowUpRight className="h-4 w-4" />
                            </a>
                          ) : <span className="text-zinc-600 text-[10px]">—</span>}
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
