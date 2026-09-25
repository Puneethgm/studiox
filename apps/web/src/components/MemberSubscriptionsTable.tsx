'use client';

import { Fragment, useEffect, useState } from 'react';
import { Users, AlertCircle, ChevronDown, ChevronUp, ArrowUpRight, Copy, Check } from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { Badge, type BadgeTone } from '@/components/ui/Badge';
import { api } from '@/lib/api';
import { formatDateTime } from '@/lib/datetime';
import type { MemberSubscription } from '@/lib/types';
import type { Invoice } from './PaymentsClient';

const STATUS_LABEL: Record<string, string> = {
  active: 'Active',
  past_due: 'Pending',
  canceled: 'Canceled',
  superseded: 'Replaced',
  completed: 'Completed',
};

const STATUS_TONE: Record<string, BadgeTone> = {
  active: 'success',
  past_due: 'warning',
  canceled: 'danger',
  superseded: 'neutral',
  completed: 'info',
};

const UNIT_WORD: Record<string, string> = { day: 'Day', week: 'Week', month: 'Month', year: 'Year' };

function formatCadence(interval: string, count: number): string {
  const word = UNIT_WORD[interval] || interval || 'Month';
  const n = count > 0 ? count : 1;
  return `Every ${n} ${word}${n === 1 ? '' : 's'}`;
}

function formatAmount(amount: number, currency: string): string {
  try {
    return new Intl.NumberFormat('en-US', { style: 'currency', currency: currency.toUpperCase() || 'SGD' }).format(amount / 100);
  } catch {
    return `${currency} ${(amount / 100).toFixed(2)}`;
  }
}

function CopyableID({ value }: { value: string }) {
  const [copied, setCopied] = useState(false);
  if (!value) return <span className="text-zinc-400">—</span>;
  return (
    <button
      type="button"
      onClick={() => {
        navigator.clipboard.writeText(value).then(() => {
          setCopied(true);
          setTimeout(() => setCopied(false), 1500);
        });
      }}
      className="inline-flex items-center gap-1.5 font-mono text-[11px] text-zinc-500 hover:text-zinc-900 dark:text-zinc-400 dark:hover:text-white transition-colors"
      title="Copy"
    >
      {value}
      {copied ? <Check className="h-3 w-3 text-emerald-500" /> : <Copy className="h-3 w-3" />}
    </button>
  );
}

// Studio owners use this to answer two questions live billing history can't:
// "who is actually still on a live recurring charge" and "who's about to be
// charged, on what cadence, when" — sourced from user_subscriptions (the
// record the Stripe webhook maintains), not a live Stripe API call.
export default function MemberSubscriptionsTable({ studioId, invoices }: { studioId: string; invoices: Invoice[] }) {
  const [subs, setSubs] = useState<MemberSubscription[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [expandedId, setExpandedId] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    api<{ subscriptions: MemberSubscription[] }>(`/api/v1/me/studios/${studioId}/member-subscriptions`)
      .then((res) => {
        if (!cancelled) setSubs(res.subscriptions || []);
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof Error ? e.message : 'Failed to load member subscriptions');
      });
    return () => {
      cancelled = true;
    };
  }, [studioId]);

  return (
    <Card className="border-white/30 bg-white/20 dark:border-white/5 dark:bg-neutral-900/30 backdrop-blur-2xl">
      <div className="flex items-center gap-2 mb-1">
        <Users className="h-4 w-4 text-brand-500" />
        <h3 className="text-sm font-black text-zinc-950 dark:text-white">Member Subscriptions</h3>
      </div>
      <p className="text-[11px] text-zinc-400 mb-4 leading-relaxed">
        Who's paid, what cadence they're on, and when the next charge lands. A missed or failed renewal shows as{' '}
        <span className="font-bold text-amber-500">Pending</span> here until Stripe either collects it or gives up.
        Click a row for the Stripe subscription details and that member's full payment history.
      </p>

      {error ? (
        <div className="flex h-24 flex-col items-center justify-center text-center">
          <AlertCircle className="h-6 w-6 text-zinc-400 mb-2" />
          <span className="text-xs font-bold text-zinc-500">{error}</span>
        </div>
      ) : subs === null ? (
        <div className="flex h-24 items-center justify-center">
          <div className="h-5 w-5 animate-spin rounded-full border-2 border-brand-500 border-t-transparent" />
        </div>
      ) : subs.length === 0 ? (
        <div className="flex h-24 flex-col items-center justify-center text-center">
          <AlertCircle className="h-6 w-6 text-zinc-400 mb-2" />
          <span className="text-xs font-bold text-zinc-500">No member subscriptions yet.</span>
        </div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-left min-w-[720px]">
            <thead>
              <tr className="border-b border-zinc-200 dark:border-white/10 text-[9px] font-black uppercase tracking-wider text-zinc-400">
                <th className="pb-3">Member</th>
                <th className="pb-3">Plan</th>
                <th className="pb-3">Amount</th>
                <th className="pb-3">Cadence</th>
                <th className="pb-3">Status</th>
                <th className="pb-3">Next Charge</th>
                <th className="pb-3 text-right">Details</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-zinc-200 dark:divide-white/5">
              {subs.map((s) => {
                const isOpen = expandedId === s.id;
                const memberInvoices = invoices.filter((inv) => inv.metadata?.lead_id === s.leadId);
                return (
                  <Fragment key={s.id}>
                    <tr
                      className="text-xs text-zinc-700 dark:text-zinc-300 cursor-pointer hover:bg-zinc-50 dark:hover:bg-white/5"
                      onClick={() => setExpandedId(isOpen ? null : s.id)}
                    >
                      <td className="py-3">
                        <p className="font-semibold text-zinc-900 dark:text-zinc-100">{s.leadName || 'Unnamed lead'}</p>
                        <span className="text-[10px] text-zinc-400">{s.leadPhone}</span>
                      </td>
                      <td className="py-3 font-semibold">{s.planName}</td>
                      <td className="py-3 font-bold text-zinc-950 dark:text-white">{formatAmount(s.amountPaid, s.currency)}</td>
                      <td className="py-3">{formatCadence(s.billingInterval, s.billingIntervalCount)}</td>
                      <td className="py-3">
                        <Badge tone={STATUS_TONE[s.subscriptionStatus] || 'neutral'}>
                          {STATUS_LABEL[s.subscriptionStatus] || s.subscriptionStatus}
                        </Badge>
                      </td>
                      <td className="py-3">
                        {s.subscriptionStatus === 'active' && s.nextRenewalAt
                          ? formatDateTime(s.nextRenewalAt)
                          : <span className="text-zinc-400">—</span>}
                      </td>
                      <td className="py-3 text-right">
                        {isOpen ? <ChevronUp className="inline h-4 w-4 text-zinc-400" /> : <ChevronDown className="inline h-4 w-4 text-zinc-400" />}
                      </td>
                    </tr>
                    {isOpen && (
                      <tr>
                        <td colSpan={7} className="bg-zinc-50/70 dark:bg-white/[0.03] px-2 py-4">
                          <div className="grid gap-4 sm:grid-cols-2">
                            <div>
                              <span className="text-[9px] font-black uppercase tracking-wider text-zinc-400 block mb-1">Stripe Subscription ID</span>
                              <CopyableID value={s.stripeSubscriptionId} />
                            </div>
                            <div>
                              <span className="text-[9px] font-black uppercase tracking-wider text-zinc-400 block mb-1">Stripe Customer ID</span>
                              <CopyableID value={s.stripeCustomerId} />
                            </div>
                          </div>

                          <span className="text-[9px] font-black uppercase tracking-wider text-zinc-400 block mt-4 mb-2">
                            Payment History ({memberInvoices.length})
                          </span>
                          {memberInvoices.length === 0 ? (
                            <p className="text-xs text-zinc-400">No matching payments found in the billing history.</p>
                          ) : (
                            <div className="space-y-1.5">
                              {memberInvoices.map((inv) => (
                                <div key={inv.id} className="flex items-center justify-between gap-3 rounded-lg bg-white/60 dark:bg-white/5 px-3 py-2">
                                  <div className="min-w-0">
                                    <p className="text-xs font-semibold text-zinc-900 dark:text-zinc-100 truncate">
                                      {inv.description || 'Payment'}
                                    </p>
                                    <span className="text-[10px] text-zinc-400">{new Date(inv.created * 1000).toLocaleDateString()}</span>
                                  </div>
                                  <div className="flex items-center gap-3 shrink-0">
                                    <span className="text-xs font-bold text-zinc-950 dark:text-white">
                                      {formatAmount(inv.amount_paid, inv.currency)}
                                    </span>
                                    <Badge tone={inv.status === 'paid' ? 'success' : 'neutral'}>{inv.status}</Badge>
                                    {inv.hosted_invoice_url && (
                                      <a
                                        href={inv.hosted_invoice_url}
                                        target="_blank"
                                        rel="noreferrer"
                                        onClick={(e) => e.stopPropagation()}
                                        className="p-1 hover:bg-zinc-100 dark:hover:bg-white/15 rounded-lg text-zinc-400 hover:text-zinc-900 dark:hover:text-white transition-all"
                                        title="View Receipt"
                                      >
                                        <ArrowUpRight className="h-3.5 w-3.5" />
                                      </a>
                                    )}
                                  </div>
                                </div>
                              ))}
                            </div>
                          )}
                        </td>
                      </tr>
                    )}
                  </Fragment>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </Card>
  );
}
