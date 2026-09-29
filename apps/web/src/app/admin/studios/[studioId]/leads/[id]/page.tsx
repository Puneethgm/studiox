import Link from 'next/link';
import { User, ArrowLeft } from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Badge, type BadgeTone } from '@/components/ui/Badge';
import { serverFetch } from '@/lib/auth';
import { formatDateTime } from '@/lib/datetime';
import type { Lead, MemberSubscription } from '@/lib/types';
import { LeadEditor } from './editor';

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

const CADENCE_UNIT: Record<string, string> = { day: 'Day', week: 'Week', month: 'Month', year: 'Year' };

function formatCadence(interval: string, count: number): string {
  const word = CADENCE_UNIT[interval] || interval || 'Month';
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

export default async function LeadDetailPage({
  params,
}: {
  params: Promise<{ studioId: string; id: string }>;
}) {
  const { studioId, id } = await params;
  const [lead, subscriptionsRes] = await Promise.all([
    serverFetch<Lead>(`/api/v1/studios/${studioId}/leads/${id}`),
    serverFetch<{ subscriptions: MemberSubscription[] | null }>(`/api/v1/me/studios/${studioId}/leads/${id}/member-subscriptions`)
      .catch(() => ({ subscriptions: [] as MemberSubscription[] })),
  ]);
  // Defensive fallback even after the backend fix (Go marshals a nil slice
  // as JSON null, not []) — don't let a future endpoint with the same gap
  // crash this page again.
  const subscriptions = subscriptionsRes.subscriptions ?? [];

  return (
    <div className="space-y-6">
      {/* Premium Glass Header */}
      <div
        className="relative overflow-hidden rounded-[26px] border border-white/30 p-6 backdrop-blur-2xl dark:border-white/5 bg-white/30 dark:bg-neutral-900/30"
        style={{
          boxShadow: 'inset 0 0 0 1px rgba(255,255,255,0.2), 0 8px 32px rgba(139,92,246,0.07)',
        }}
      >
        <div className="pointer-events-none absolute -right-16 -top-16 h-48 w-48 rounded-full bg-brand-500/10 blur-[70px]" />
        
        <div className="relative flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex items-center gap-4">
            <div className="grid h-12 w-12 place-items-center rounded-2xl bg-gradient-to-br from-brand-500 to-violet-600 text-white shadow-lg shadow-brand-500/25">
              <User className="h-6 w-6" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h1 className="text-2xl font-black tracking-tight text-zinc-900 dark:text-white">{lead.name}</h1>
                {lead.needsManualFollowup && <Badge tone="warning">Needs Follow-up</Badge>}
              </div>
              <p className="mt-0.5 text-xs font-semibold text-zinc-500 dark:text-zinc-400">
                From <span className="font-bold text-brand-600 dark:text-brand-400">{lead.campaignName ?? lead.campaignId}</span> · {formatDateTime(lead.createdAt)}
              </p>
            </div>
          </div>
          <Link href={`/admin/studios/${studioId}/leads`}>
            <Button
              variant="secondary"
              leftIcon={<ArrowLeft className="h-4 w-4" />}
              suppressHydrationWarning
            >
              Back to Leads
            </Button>
          </Link>
        </div>
      </div>

      <div className="grid gap-6 md:grid-cols-3">
        <div className="space-y-6 md:col-span-2">
          <Card title="Contact details">
            <dl className="grid grid-cols-2 gap-x-6 gap-y-5 text-sm">
              <Field label="Email" value={lead.email} />
              <Field label="Phone" value={lead.phone} />
              <Field label="Fitness plan" value={lead.fitnessPlan} />
              <Field label="Source" value={lead.source} />
              <Field label="Assigned to" value={lead.assignedTo ?? ''} />
              <Field label="Offer" value={lead.offer ?? ''} />
              <Field label="Monthly membership fee" value={lead.monthlyFee ? `$${lead.monthlyFee}` : '—'} />
              <Field label="Predicted Revenue Won" value={lead.monthlyFee ? `$${lead.monthlyFee * 9}` : '—'} />
              {lead.goals && <Field label="Goals" value={lead.goals} className="col-span-2" />}
              {lead.furtherNotes && <Field label="Further notes on Contact" value={lead.furtherNotes ?? ''} className="col-span-2" />}
            </dl>
          </Card>

          <Card title="Payment">
            {subscriptions.length === 0 ? (
              <p className="text-sm text-zinc-500 dark:text-zinc-400">
                No payment recorded yet — this lead hasn&rsquo;t purchased a plan through the platform.
              </p>
            ) : (
              <div className="space-y-4">
                {subscriptions.map((s) => (
                  <div
                    key={s.id}
                    className="rounded-xl border border-zinc-200 p-4 dark:border-zinc-800"
                  >
                    <div className="flex items-start justify-between gap-4">
                      <div>
                        <p className="text-sm font-bold text-zinc-900 dark:text-zinc-100">{s.planName}</p>
                        <p className="text-xs text-zinc-500 dark:text-zinc-400">
                          {s.subscriptionStatus === 'completed' ? 'One-time payment' : formatCadence(s.billingInterval, s.billingIntervalCount)}
                        </p>
                      </div>
                      <div className="text-right">
                        <p className="text-lg font-black text-zinc-950 dark:text-white">{formatAmount(s.amountPaid, s.currency)}</p>
                        <Badge tone={STATUS_TONE[s.subscriptionStatus] || 'neutral'}>
                          {STATUS_LABEL[s.subscriptionStatus] || s.subscriptionStatus}
                        </Badge>
                      </div>
                    </div>
                    <dl className="mt-3 grid grid-cols-2 gap-x-6 gap-y-3 border-t border-zinc-100 pt-3 text-sm dark:border-zinc-800">
                      <Field label="Payment status" value={s.paymentStatus} />
                      <Field label="Started" value={formatDateTime(s.startDate)} />
                      {s.subscriptionStatus === 'active' && s.nextRenewalAt && (
                        <Field label="Next charge" value={formatDateTime(s.nextRenewalAt)} />
                      )}
                      {s.canceledAt && <Field label="Canceled" value={formatDateTime(s.canceledAt)} />}
                      {s.receiptUrl && (
                        <div>
                          <dt className="text-[10px] font-black uppercase tracking-[0.18em] text-zinc-400">Receipt</dt>
                          <dd className="mt-1.5">
                            <a
                              href={s.receiptUrl}
                              target="_blank"
                              rel="noopener noreferrer"
                              className="font-semibold text-brand-600 underline hover:text-brand-700 dark:text-brand-400"
                            >
                              View receipt
                            </a>
                          </dd>
                        </div>
                      )}
                    </dl>
                  </div>
                ))}
              </div>
            )}
          </Card>
        </div>

        <div className="md:col-span-1">
          <LeadEditor studioId={studioId} lead={lead} />
        </div>
      </div>
    </div>
  );
}

function Field({ label, value, className }: { label: string; value: string; className?: string }) {
  return (
    <div className={className}>
      <dt className="text-[10px] font-black uppercase tracking-[0.18em] text-zinc-400">
        {label}
      </dt>
      <dd className="mt-1.5 break-words font-semibold text-zinc-900 dark:text-zinc-100">{value || '—'}</dd>
    </div>
  );
}
