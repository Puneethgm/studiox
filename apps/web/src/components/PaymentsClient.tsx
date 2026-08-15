'use client';

import React, { useState, useEffect, useRef } from 'react';
import {
  CreditCard,
  ArrowUpRight,
  CheckCircle,
  ShieldCheck,
  AlertCircle,
  Calendar,
  Copy,
  Check,
  Link2,
} from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Badge } from '@/components/ui/Badge';
import { Select } from '@/components/ui/Select';
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
  buyer_name?: string;
  campaign_name?: string;
  campaign_slug?: string;
  metadata?: Record<string, string>;
}
// Module-level cache to prevent re-fetching on client-side tab navigation
const paymentsCache: Record<string, {
  stripeStatus: 'connected' | 'disconnected';
  stripeAccountId: string;
  formPublishableKey: string;
  billingStats: { outstandingSGD: number; lifetimePaidSGD: number; lifetimePaidTotal: number };
  invoices: Invoice[];
  fetchedAt: number;
}> = {};

export default function PaymentsClient({ studioId }: { studioId: string }) {
  const cached = paymentsCache[studioId];

  const [stripeStatus, setStripeStatus] = useState<'connected' | 'disconnected'>(
    cached ? cached.stripeStatus : 'disconnected'
  );
  const [stripeAccountId, setStripeAccountId] = useState(
    cached ? cached.stripeAccountId : ''
  );
  const [loading, setLoading] = useState(
    cached ? false : true
  );
  const [billingStats, setBillingStats] = useState(
    cached ? cached.billingStats : { outstandingSGD: 0, lifetimePaidSGD: 0, lifetimePaidTotal: 0 }
  );
  const [invoices, setInvoices] = useState<Invoice[]>(
    cached ? cached.invoices : []
  );
  const PAGE_SIZE = 10;
  const [visibleCount, setVisibleCount] = useState(PAGE_SIZE);
  const [invoicesLoading, setInvoicesLoading] = useState(false);
  
  // Link Stripe Form state
  const [showForm, setShowForm] = useState(false);
  const [formStripeAccountId, setFormStripeAccountId] = useState(
    cached ? cached.stripeAccountId : ''
  );
  const [formPublishableKey, setFormPublishableKey] = useState(
    cached ? cached.formPublishableKey : ''
  );
  const [formSecretKey, setFormSecretKey] = useState('');
  const [formWebhookSecret, setFormWebhookSecret] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  // Booking links
  const [campaigns, setCampaigns] = useState<{ slug: string; name: string; studioSlug: string; shareUrl: string }[]>([]);
  const [selectedCampaignSlug, setSelectedCampaignSlug] = useState('');
  const [bookingLinkCopied, setBookingLinkCopied] = useState(false);
  const [leadQuery, setLeadQuery] = useState('');
  const [leadResults, setLeadResults] = useState<{ id: string; name: string; phone: string }[]>([]);
  const [leadResultsOpen, setLeadResultsOpen] = useState(false);
  const [selectedLead, setSelectedLead] = useState<{ id: string; name: string; phone: string } | null>(null);

  // Filters
  const [duration, setDuration] = useState('');
  const [startDate, setStartDate] = useState('');
  const [endDate, setEndDate] = useState('');

  // Height tracking for dynamic table max-height matching
  const leftCardRef = useRef<HTMLDivElement>(null);
  const [leftCardHeight, setLeftCardHeight] = useState(280);

  useEffect(() => {
    if (!leftCardRef.current) return;
    const resizeObserver = new ResizeObserver((entries) => {
      for (const entry of entries) {
        setLeftCardHeight(entry.contentRect.height);
      }
    });
    resizeObserver.observe(leftCardRef.current);
    return () => resizeObserver.disconnect();
  }, [stripeStatus, showForm]);

  useEffect(() => {
    setVisibleCount(PAGE_SIZE);
  }, [invoices]);

  const handleInvoicesScroll = (e: React.UIEvent<HTMLDivElement>) => {
    const target = e.currentTarget;
    if (target.scrollHeight - target.scrollTop <= target.clientHeight + 20) {
      if (visibleCount < invoices.length) {
        setVisibleCount((prev) => prev + PAGE_SIZE);
      }
    }
  };

  // Fetch logic wrapped in a callback with background refresh support
  const fetchBillingHistory = async (studioIdStr: string, startUnix?: number, endUnix?: number, silent = false) => {
    if (!silent) {
      setInvoicesLoading(true);
    }
    try {
      let url = `/api/v1/me/studios/${studioIdStr}/billing/history`;
      const params = new URLSearchParams();
      if (startUnix) params.set('startDate', startUnix.toString());
      if (endUnix) params.set('endDate', endUnix.toString());
      if (params.toString()) {
        url += `?${params.toString()}`;
      }

      const historyRes = await api<{ invoices: Invoice[] | null, stats: any }>(url);
      setInvoices(historyRes.invoices || []);
      if (historyRes.stats) setBillingStats(historyRes.stats);

      // Populate or update local cache
      if (!paymentsCache[studioIdStr]) {
        paymentsCache[studioIdStr] = {
          stripeStatus: 'connected',
          stripeAccountId: stripeAccountId || (historyRes.stats ? stripeAccountId : ''),
          formPublishableKey: formPublishableKey,
          billingStats: historyRes.stats || { outstandingSGD: 0, lifetimePaidSGD: 0, lifetimePaidTotal: 0 },
          invoices: historyRes.invoices || [],
          fetchedAt: Date.now()
        };
      } else {
        paymentsCache[studioIdStr].invoices = historyRes.invoices || [];
        if (historyRes.stats) paymentsCache[studioIdStr].billingStats = historyRes.stats;
        paymentsCache[studioIdStr].fetchedAt = Date.now();
      }
    } catch (e) {
      console.error('failed to fetch billing history', e);
    } finally {
      if (!silent) {
        setInvoicesLoading(false);
      }
    }
  };

  useEffect(() => {
    const cachedInfo = paymentsCache[studioId];
    if (cachedInfo) {
      // Instant mount from cache
      setLoading(false);
    } else {
      setLoading(true);
    }

    void (async () => {
      try {
        const [res, campaignsRes] = await Promise.all([
          api<{ stripeAccountId: string; stripePublishableKey: string; hasStripeSecretKey: boolean; hasStripeWebhookSecret: boolean; subscriptionTier: string }>(
            `/api/v1/me/studios/${studioId}/payments`
          ),
          api<{ campaigns: { slug: string; name: string; studioSlug: string; shareUrl: string }[] }>(
            `/api/v1/studios/${studioId}/campaigns`
          ).catch(() => ({ campaigns: [] })),
        ]);
        if (campaignsRes.campaigns?.length) {
          setCampaigns(campaignsRes.campaigns);
          setSelectedCampaignSlug(campaignsRes.campaigns[0]?.slug ?? '');
        }
        const isConnected = !!(res.stripeAccountId && res.hasStripeSecretKey);
        const newStatus = isConnected ? 'connected' : 'disconnected';
        setStripeStatus(newStatus);

        if (isConnected) {
          setStripeAccountId(res.stripeAccountId);
          setFormStripeAccountId(res.stripeAccountId);
          setFormPublishableKey(res.stripePublishableKey || '');
          setFormWebhookSecret(res.hasStripeWebhookSecret ? '••••••••••••••••' : '');

          if (!paymentsCache[studioId]) {
            paymentsCache[studioId] = {
              stripeStatus: 'connected',
              stripeAccountId: res.stripeAccountId,
              formPublishableKey: res.stripePublishableKey || '',
              billingStats: { outstandingSGD: 0, lifetimePaidSGD: 0, lifetimePaidTotal: 0 },
              invoices: [],
              fetchedAt: Date.now()
            };
          } else {
            paymentsCache[studioId].stripeStatus = 'connected';
            paymentsCache[studioId].stripeAccountId = res.stripeAccountId;
            paymentsCache[studioId].formPublishableKey = res.stripePublishableKey || '';
          }

          // Fetch billing history. If cached, fetch silently in the background (no loading spinner)
          const hasCache = cachedInfo && cachedInfo.invoices.length > 0;
          void fetchBillingHistory(studioId, undefined, undefined, hasCache);
        } else {
          setStripeStatus('disconnected');
          delete paymentsCache[studioId];
        }
      } catch {
        setStripeStatus('disconnected');
      } finally {
        setLoading(false);
      }
    })();
  }, [studioId]);

  // Effect to refetch invoices when filter changes
  useEffect(() => {
    if (stripeStatus !== 'connected') return;

    let startUnix: number | undefined;
    let endUnix: number | undefined;

    if (duration && duration !== 'custom') {
      const now = new Date();
      if (duration === '1d') {
        const start = new Date(now.getFullYear(), now.getMonth(), now.getDate());
        startUnix = Math.floor(start.getTime() / 1000);
      } else if (duration === '7d') {
        const start = new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000);
        startUnix = Math.floor(start.getTime() / 1000);
      } else if (duration === '30d') {
        const start = new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000);
        startUnix = Math.floor(start.getTime() / 1000);
      }
    } else if (duration === 'custom') {
      if (startDate) {
        startUnix = Math.floor(new Date(startDate).getTime() / 1000);
      }
      if (endDate) {
        const end = new Date(endDate);
        end.setHours(23, 59, 59, 999);
        endUnix = Math.floor(end.getTime() / 1000);
      }
    }

    const hasActiveFilters = duration !== '' || startDate !== '' || endDate !== '';
    const cachedData = paymentsCache[studioId];

    // Avoid double fetching on mount when cache is already populated
    if (hasActiveFilters || !cachedData || cachedData.invoices.length === 0) {
      void fetchBillingHistory(studioId, startUnix, endUnix);
    }
  }, [duration, startDate, endDate, stripeStatus, studioId]);

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
          stripeWebhookSecret: formWebhookSecret,
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

  const copyBookingLink = async (url: string) => {
    await navigator.clipboard.writeText(url);
    setBookingLinkCopied(true);
    setTimeout(() => setBookingLinkCopied(false), 2000);
  };

  // Debounced lead search — powers the "pick a lead" dropdown below so the
  // copied booking link points at a real lead id instead of the inert
  // "LEAD_ID" placeholder (which 404s if opened directly).
  useEffect(() => {
    if (studioId === 'global' || leadQuery.trim().length < 2) {
      setLeadResults([]);
      return;
    }
    const handle = setTimeout(() => {
      api<{ leads: { id: string; name: string; phone: string }[] }>(
        `/api/v1/studios/${studioId}/leads?search=${encodeURIComponent(leadQuery.trim())}&limit=8`
      )
        .then((res) => setLeadResults(res.leads || []))
        .catch(() => setLeadResults([]));
    }, 300);
    return () => clearTimeout(handle);
  }, [leadQuery, studioId]);

  const handleDisconnect = async () => {
    if (studioId === 'global') return;
    try {
      await api(`/api/v1/me/studios/${studioId}/payments/stripe`, {
        method: 'POST',
        json: {
          stripeAccountId: '',
          stripePublishableKey: '',
          stripeSecretKey: '',
          stripeWebhookSecret: '',
        }
      });
      setStripeAccountId('');
      setStripeStatus('disconnected');
      setFormStripeAccountId('');
      setFormPublishableKey('');
      setFormSecretKey('');
      setFormWebhookSecret('');
    } catch (err) {
      console.error('Failed to disconnect Stripe account:', err);
    }
  };

  const stats = {
    outstandingSGD: billingStats.outstandingSGD / 100,
    lifetimePaidSGD: billingStats.lifetimePaidSGD / 100,
    lifetimePaidTotal: (billingStats.lifetimePaidTotal ?? 0) / 100,
  };

  const formatAmount = (val: number) => {
    return new Intl.NumberFormat('en-SG', { style: 'currency', currency: 'SGD' }).format(val);
  };

  if (loading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <div className="h-6 w-6 animate-spin rounded-full border-2 border-brand-500 border-t-transparent" />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4 animate-in fade-in duration-300 sm:gap-6 lg:grid lg:grid-cols-3 lg:gap-6">
        {/* Plan Upgrade / Stripe Connect — shown first on mobile so
            connecting Stripe doesn't require scrolling past everything else */}
        <div className="order-1 space-y-6 lg:order-2 lg:col-span-1">

          {/* Booking Links Card */}
          {studioId !== 'global' && campaigns.length > 0 && (() => {
            const selected = campaigns.find(c => c.slug === selectedCampaignSlug) ?? campaigns[0]!;
            // Points at the same customizable trial-details page the bot's
            // WhatsApp link uses (Settings → Trial Payment Page), not the
            // separate, uncustomizable /book campaign page — one page, one
            // design, whichever way the link reaches the lead. A real lead
            // must be picked below — "LEAD_ID" alone isn't a valid id, it's
            // only ever auto-filled when the bot sends the link itself.
            const leadIdPart = selectedLead ? selectedLead.id : 'LEAD_ID';
            const bookUrl = `${new URL(selected.shareUrl).origin}/trial-details/${leadIdPart}?studio=${selected.studioSlug}`;
            return (
              <Card className="border-white/30 bg-white/20 dark:border-white/5 dark:bg-neutral-900/30 backdrop-blur-2xl">
                <div className="flex items-center gap-2 mb-1">
                  <Link2 className="h-4 w-4 text-brand-500" />
                  <h3 className="text-sm font-black text-zinc-950 dark:text-white">Trial Booking Link</h3>
                  <span className="ml-auto text-[9px] font-bold uppercase tracking-widest text-zinc-400 bg-white/20 dark:bg-white/5 rounded-full px-2 py-0.5">
                    {campaigns.length} campaign{campaigns.length > 1 ? 's' : ''}
                  </span>
                </div>
                <p className="text-[11px] text-zinc-400 mb-3 leading-relaxed">
                  The bot fills this in automatically when it sends the link itself. To copy a working link and send it manually, search and pick the lead below first.
                </p>

                {/* Campaign picker — only shown when > 1 */}
                {campaigns.length > 1 && (
                  <div className="mb-3">
                    <label className="text-[10px] font-bold text-zinc-400 uppercase tracking-wider block mb-1">Campaign</label>
                    <select
                      value={selectedCampaignSlug}
                      onChange={e => { setSelectedCampaignSlug(e.target.value); setBookingLinkCopied(false); }}
                      className="w-full rounded-xl border border-white/20 bg-white/10 dark:bg-neutral-800 px-3 py-2 text-xs font-bold text-zinc-800 dark:text-white focus:outline-none focus:border-brand-500 appearance-none"
                    >
                      {campaigns.map(c => (
                        <option key={c.slug} value={c.slug}>{c.name}</option>
                      ))}
                    </select>
                  </div>
                )}

                {/* Lead picker — search by name/phone, pick a real lead so the link below is valid */}
                <div className="mb-3 relative">
                  <label className="text-[10px] font-bold text-zinc-400 uppercase tracking-wider block mb-1">Lead</label>
                  {selectedLead ? (
                    <div className="flex items-center justify-between rounded-xl border border-brand-500/30 bg-brand-500/5 px-3 py-2">
                      <div className="min-w-0">
                        <span className="text-xs font-bold text-zinc-800 dark:text-white block truncate">{selectedLead.name || 'Unnamed lead'}</span>
                        <span className="text-[10px] text-zinc-400 block truncate">{selectedLead.phone}</span>
                      </div>
                      <button
                        type="button"
                        onClick={() => { setSelectedLead(null); setLeadQuery(''); setBookingLinkCopied(false); }}
                        className="text-[10px] font-bold text-brand-500 hover:text-brand-600 shrink-0 ml-2"
                      >
                        Change
                      </button>
                    </div>
                  ) : (
                    <>
                      <input
                        type="text"
                        value={leadQuery}
                        onChange={(e) => { setLeadQuery(e.target.value); setLeadResultsOpen(true); }}
                        onFocus={() => setLeadResultsOpen(true)}
                        onBlur={() => setTimeout(() => setLeadResultsOpen(false), 150)}
                        placeholder="Search by name or phone…"
                        className="w-full rounded-xl border border-white/20 bg-white/10 dark:bg-neutral-800 px-3 py-2 text-xs font-bold text-zinc-800 dark:text-white focus:outline-none focus:border-brand-500"
                      />
                      {leadResultsOpen && leadQuery.trim().length >= 2 && (
                        <div className="absolute z-20 mt-1 w-full rounded-xl border border-white/10 bg-white dark:bg-neutral-900 shadow-lg max-h-48 overflow-y-auto">
                          {leadResults.length === 0 ? (
                            <div className="px-3 py-2 text-[10px] text-zinc-400">No matching leads</div>
                          ) : (
                            leadResults.map((l) => (
                              <button
                                key={l.id}
                                type="button"
                                onMouseDown={() => { setSelectedLead(l); setLeadResultsOpen(false); setBookingLinkCopied(false); }}
                                className="w-full text-left px-3 py-2 hover:bg-brand-500/10 border-b border-white/5 last:border-0"
                              >
                                <span className="text-xs font-bold text-zinc-800 dark:text-white block truncate">{l.name || 'Unnamed lead'}</span>
                                <span className="text-[10px] text-zinc-400 block truncate">{l.phone}</span>
                              </button>
                            ))
                          )}
                        </div>
                      )}
                    </>
                  )}
                </div>

                {/* URL display + copy */}
                <div className="rounded-xl border border-white/10 bg-white/10 dark:bg-neutral-900/40 px-3 py-2.5 mb-2">
                  <code className="block text-[10px] font-mono text-zinc-600 dark:text-zinc-300 break-all leading-relaxed">
                    {bookUrl}
                  </code>
                </div>
                <button
                  type="button"
                  disabled={!selectedLead}
                  onClick={() => copyBookingLink(bookUrl)}
                  className={`w-full flex items-center justify-center gap-2 rounded-xl py-2 text-xs font-bold text-white transition-all disabled:opacity-40 disabled:cursor-not-allowed ${bookingLinkCopied ? 'bg-emerald-500' : 'bg-brand-500 hover:bg-brand-600'}`}
                >
                  {bookingLinkCopied
                    ? <><Check className="h-3.5 w-3.5" />Copied to clipboard!</>
                    : <><Copy className="h-3.5 w-3.5" />Copy booking link</>
                  }
                </button>
              </Card>
            );
          })()}

          {/* Stripe Connect Card */}
          <div ref={leftCardRef}>
            <Card className="border-white/30 bg-white/20 dark:border-white/5 dark:bg-neutral-900/30 backdrop-blur-2xl">
              <h3 className="text-sm font-black text-zinc-950 dark:text-white mb-3">Stripe Gateway</h3>
              <p className="text-xs text-zinc-400 mb-4 leading-relaxed">
                Connect your studio Stripe account to handle client billing, memberships, and automated invoices.
              </p>
              
              {studioId === 'global' ? (
                showForm ? (
                  <form onSubmit={handleLinkStripe} className="space-y-3.5 animate-in fade-in slide-in-from-top-2 duration-300">
                    <div>
                      <label className="text-[10px] font-bold text-zinc-400 uppercase tracking-wider block mb-1">Platform Stripe Account ID</label>
                      <input
                        type="text"
                        required
                        placeholder="acct_..."
                        value={formStripeAccountId}
                        onChange={(e) => setFormStripeAccountId(e.target.value)}
                        className="w-full rounded-xl border border-brand-500/30 bg-white/10 px-3 py-2 text-xs font-bold text-zinc-800 dark:bg-neutral-800 dark:text-white focus:outline-none focus:border-brand-500"
                      />
                    </div>
                    <div>
                      <label className="text-[10px] font-bold text-zinc-400 uppercase tracking-wider block mb-1">Platform Publishable Key</label>
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
                      <label className="text-[10px] font-bold text-zinc-400 uppercase tracking-wider block mb-1">Platform Secret Key</label>
                      <input
                        type="password"
                        required
                        placeholder="sk_test_..."
                        value={formSecretKey}
                        onChange={(e) => setFormSecretKey(e.target.value)}
                        className="w-full rounded-xl border border-white/20 bg-white/10 px-3 py-2 text-xs font-bold text-zinc-800 dark:bg-neutral-800 dark:text-white focus:outline-none"
                      />
                    </div>
                    <div>
                      <label className="text-[10px] font-bold text-zinc-400 uppercase tracking-wider block mb-1">Platform Webhook Secret</label>
                      <input
                        type="password"
                        required
                        placeholder="whsec_..."
                        value={formWebhookSecret}
                        onChange={(e) => setFormWebhookSecret(e.target.value)}
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
                        Save Platform Gateway
                      </Button>
                    </div>
                  </form>
                ) : stripeStatus === 'connected' ? (
                  <div className="space-y-4">
                    <div className="flex items-center gap-3 rounded-xl bg-emerald-500/10 p-3 text-emerald-600 dark:text-emerald-400">
                      <ShieldCheck className="h-5 w-5 shrink-0" />
                      <div className="min-w-0">
                        <span className="text-xs font-bold block">Platform Gateway Connected</span>
                        <span className="text-[10px] text-emerald-500/80 block truncate">Routing all Studio subscriptions</span>
                      </div>
                    </div>
                    <Button variant="ghost" className="w-full text-xs text-brand-500 border border-brand-500/20 hover:bg-brand-500/10" onClick={() => setShowForm(true)}>
                      Edit Credentials
                    </Button>
                  </div>
                ) : (
                  <div className="rounded-xl border border-brand-500/20 bg-brand-500/5 p-4 text-center dark:bg-brand-500/10">
                    <ShieldCheck className="h-5 w-5 mx-auto mb-2 text-brand-500" />
                    <span className="text-xs font-bold text-zinc-700 dark:text-zinc-200 block mb-3">
                      Configure the Global Platform Gateway to collect monthly recurring subscriptions from Studios.
                    </span>
                    <Button className="w-full text-xs shadow-lg shadow-brand-500/20" onClick={() => setShowForm(true)}>
                      Configure Platform Gateway
                    </Button>
                  </div>
                )
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
                  {studioId !== 'global' && (
                    <div className="rounded-xl border border-white/10 bg-white/10 p-3 dark:bg-neutral-800/20 text-[10.5px]">
                      <span className="text-[10px] font-bold text-zinc-400 uppercase block mb-1">Webhook Endpoint URL</span>
                      <code className="block select-all rounded bg-white/40 dark:bg-neutral-900/40 p-1.5 font-mono break-all font-bold text-zinc-700 dark:text-zinc-300">
                        {`${typeof window !== 'undefined' ? window.location.origin : ''}/api/v1/webhooks/stripe/${studioId}`}
                      </code>
                    </div>
                  )}
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
                  <div>
                    <label className="text-[10px] font-bold text-zinc-400 uppercase tracking-wider block mb-1">Webhook Secret</label>
                    <input
                      type="password"
                      required
                      placeholder="whsec_..."
                      value={formWebhookSecret}
                      onChange={(e) => setFormWebhookSecret(e.target.value)}
                      className="w-full rounded-xl border border-white/20 bg-white/10 px-3 py-2 text-xs font-bold text-zinc-800 dark:bg-neutral-800 dark:text-white focus:outline-none"
                    />
                  </div>
                  {studioId !== 'global' && (
                    <div className="rounded-xl border border-brand-500/10 bg-brand-500/5 p-3 dark:bg-brand-500/10 text-[10.5px]">
                      <span className="font-bold text-zinc-700 dark:text-zinc-200 block mb-1">Your Isolated Webhook Endpoint URL:</span>
                      <code className="block select-all rounded bg-white/40 dark:bg-neutral-900/40 p-1.5 font-mono break-all font-black text-brand-600 dark:text-brand-400">
                        {`${typeof window !== 'undefined' ? window.location.origin : ''}/api/v1/webhooks/stripe/${studioId}`}
                      </code>
                      <span className="text-[9px] text-zinc-400 block mt-1 leading-normal">
                        Configure this URL in your Stripe Dashboard under Developers &gt; Webhooks to receive payment events.
                      </span>
                    </div>
                  )}
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
                <div className="rounded-xl border border-brand-500/20 bg-brand-500/5 p-4 text-center dark:bg-brand-500/10">
                  <ShieldCheck className="h-5 w-5 mx-auto mb-2 text-brand-500" />
                  <span className="text-xs font-bold text-zinc-700 dark:text-zinc-200 block mb-3">
                    Enter your Stripe API keys to enable payment processing.
                  </span>
                  <Button className="w-full text-xs shadow-lg shadow-brand-500/20" onClick={() => setShowForm(true)}>
                    Connect Stripe Account
                  </Button>
                </div>
              )}
            </Card>
          </div>
        </div>

        {/* Stats row — side by side even on mobile, sits between the
            Stripe card and billing history */}
        <div className="order-2 grid grid-cols-2 gap-3 lg:order-1 lg:col-span-3 lg:gap-6">
          <Card className="border-white/30 bg-white/20 dark:border-white/5 dark:bg-neutral-900/30 backdrop-blur-2xl p-4 sm:p-6">
            <span className="text-[10px] font-black uppercase tracking-wider text-zinc-400 block">Outstanding Balance</span>
            <div className="mt-2 flex items-baseline gap-2">
              <span className="text-2xl font-black text-zinc-950 dark:text-white sm:text-3xl">
                {formatAmount(stats.outstandingSGD)}
              </span>
            </div>
            <span className="text-[10px] text-emerald-500 font-bold uppercase tracking-wider mt-2 block flex items-center gap-1">
              <CheckCircle className="h-3 w-3" /> Settled
            </span>
          </Card>

          <Card className="border-white/30 bg-white/20 dark:border-white/5 dark:bg-neutral-900/30 backdrop-blur-2xl p-4 sm:p-6">
            <span className="text-[10px] font-black uppercase tracking-wider text-zinc-400 block">Lifetime Payments</span>
            <div className="mt-2 flex items-baseline gap-2">
              <span className="text-2xl font-black text-zinc-950 dark:text-white sm:text-3xl">
                {formatAmount(stats.lifetimePaidTotal || stats.lifetimePaidSGD)}
              </span>
            </div>
            <span className="text-[10px] text-zinc-400 font-bold uppercase tracking-wider mt-2 block">
              Last Paid: {invoices[0] ? new Date(invoices[0].created * 1000).toLocaleDateString() : 'N/A'}
            </span>
          </Card>
        </div>

        {/* Invoice List — shown last on mobile */}
        <div className="order-3 space-y-4 lg:order-3 lg:col-span-2">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <span className="text-xs font-black uppercase tracking-wider text-zinc-400 flex items-center gap-1">
                <Calendar className="h-3.5 w-3.5" />
                Filter by Date:
              </span>
              <Select
                className="w-36 rounded-2xl border border-zinc-200/50 bg-white/50 py-2 focus:border-brand-500 focus:outline-none dark:border-zinc-800/50 dark:bg-zinc-950/50 text-xs font-bold"
                value={duration}
                onChange={(e) => {
                  setDuration(e.target.value);
                  if (e.target.value !== 'custom') {
                    setStartDate('');
                    setEndDate('');
                  }
                }}
              >
                <option value="">All Time</option>
                <option value="1d">Today</option>
                <option value="7d">Last 7 Days</option>
                <option value="30d">Last 30 Days</option>
                <option value="custom">Custom Date</option>
              </Select>
            </div>

            {duration === 'custom' && (
              <div className="flex items-center gap-2 animate-in fade-in slide-in-from-right-4 duration-300">
                <input
                  type="date"
                  value={startDate}
                  onChange={(e) => setStartDate(e.target.value)}
                  className="rounded-xl border border-zinc-200 bg-white/50 px-2.5 py-1.5 text-xs font-bold text-zinc-800 shadow-sm focus:border-brand-500 focus:outline-none dark:border-zinc-700 dark:bg-zinc-900 dark:text-white"
                />
                <span className="text-xs font-bold text-zinc-400">to</span>
                <input
                  type="date"
                  value={endDate}
                  onChange={(e) => setEndDate(e.target.value)}
                  className="rounded-xl border border-zinc-200 bg-white/50 px-2.5 py-1.5 text-xs font-bold text-zinc-800 shadow-sm focus:border-brand-500 focus:outline-none dark:border-zinc-700 dark:bg-zinc-900 dark:text-white"
                />
              </div>
            )}
          </div>

          <Card className="border-white/30 bg-white/20 dark:border-white/5 dark:bg-neutral-900/30 backdrop-blur-2xl">
            <h3 className="text-sm font-black text-zinc-950 dark:text-white mb-4">Billing History & Invoices</h3>
            {invoicesLoading ? (
              <div className="flex h-36 items-center justify-center">
                <div className="h-5 w-5 animate-spin rounded-full border-2 border-brand-500 border-t-transparent" />
              </div>
            ) : invoices.length === 0 ? (
              <div className="flex h-36 flex-col items-center justify-center text-center">
                <AlertCircle className="h-6 w-6 text-zinc-400 mb-2" />
                <span className="text-xs font-bold text-zinc-500">No active invoices. Connect Stripe to activate billing ledger.</span>
              </div>
            ) : (
              <div className="space-y-0">
                <div 
                  className="overflow-y-auto overflow-x-auto pr-1 scrollbar-thin scrollbar-thumb-zinc-200 dark:scrollbar-thumb-neutral-800"
                  style={{ maxHeight: `${Math.max(400, leftCardHeight - 84)}px` }}
                  onScroll={handleInvoicesScroll}
                >
                  <table className="w-full text-left min-w-[500px]">
                    <thead>
                      <tr className="border-b border-zinc-200 dark:border-white/10 text-[9px] font-black uppercase tracking-wider text-zinc-400 sticky top-0 bg-white/95 dark:bg-neutral-900/95 backdrop-blur z-10">
                        <th className="pb-3">Ref</th>
                        <th className="pb-3">Date</th>
                        <th className="pb-3">Buyer</th>
                        <th className="pb-3">Description</th>
                        <th className="pb-3">Amount</th>
                        <th className="pb-3">Status</th>
                        <th className="pb-3 text-right">Receipt</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-zinc-200 dark:divide-white/5">
                      {invoices.slice(0, visibleCount).map(inv => (
                        <tr key={inv.id} className="text-xs text-zinc-700 dark:text-zinc-300">
                          <td className="py-3 font-semibold font-mono text-zinc-555 dark:text-zinc-500">{inv.number || inv.id.slice(0,12)}</td>
                          <td className="py-3">{new Date(inv.created * 1000).toLocaleDateString()}</td>
                          <td className="py-3 font-semibold text-zinc-900 dark:text-zinc-100">{inv.buyer_name || inv.metadata?.customer_name || 'Guest'}</td>
                          <td className="py-3 max-w-[180px]">
                            <p className="font-semibold text-zinc-900 dark:text-zinc-100 truncate">{inv.description || 'Payment'}</p>
                            {inv.campaign_name && (
                              <span className="inline-block mt-0.5 rounded-full bg-brand-100 dark:bg-brand-900/30 text-brand-700 dark:text-brand-300 text-[9px] font-bold px-1.5 py-0.5 uppercase tracking-wide truncate max-w-full">
                                {inv.campaign_name}
                              </span>
                            )}
                          </td>
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
                              <a href={inv.hosted_invoice_url} target="_blank" rel="noreferrer" className="inline-block p-1 hover:bg-zinc-100 dark:hover:bg-white/15 rounded-lg text-zinc-400 hover:text-zinc-900 dark:hover:text-white transition-all" title="View Receipt">
                                <ArrowUpRight className="h-4 w-4" />
                              </a>
                            ) : <span className="text-zinc-600 text-[10px]">—</span>}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
                
                {/* Fixed footer indicator showing current visible counts */}
                <div className="pt-3 flex items-center justify-between border-t border-zinc-200 dark:border-white/10 mt-2 text-[10px] font-semibold text-zinc-400 dark:text-zinc-500">
                  <span>
                    Showing {Math.min(visibleCount, invoices.length)} of {invoices.length} entries
                  </span>
                  {visibleCount < invoices.length && (
                    <span className="animate-pulse text-brand-500 dark:text-brand-400 uppercase tracking-widest font-black text-[9px]">
                      Scroll down to load more
                    </span>
                  )}
                </div>
              </div>
            )}
          </Card>
        </div>
    </div>
  );
}
