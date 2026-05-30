import Link from 'next/link';
import { Zap, Users, Target, ArrowRight, BarChart3, Calendar } from 'lucide-react';
import { Badge } from '@/components/ui/Badge';
import { Card } from '@/components/ui/Card';
import { EmptyState } from '@/components/ui/EmptyState';
import { serverFetch } from '@/lib/auth';
import { formatDate, formatDateTime } from '@/lib/datetime';
import type { Campaign, Lead } from '@/lib/types';
import { CopyLink } from '../CopyLink';
import { CampaignActions } from './actions';
import { CampaignPlansEditor } from './CampaignPlansEditor';

const statusTone = {
  new: 'info',
  contacted: 'brand',
  trial_booked: 'warning',
  member: 'success',
  dropped: 'neutral',
  paused: 'brand',
} as const;

const statusLabel: Record<string, string> = {
  new: 'New',
  contacted: 'Contacted',
  trial_booked: 'Trial Booked',
  member: 'Member',
  dropped: 'Dropped',
  paused: 'Paused',
};

export default async function CampaignDetailPage({
  params,
}: {
  params: Promise<{ studioId: string; id: string }>;
}) {
  const { studioId, id } = await params;
  const [c, leadsResp] = await Promise.all([
    serverFetch<Campaign>(`/api/v1/studios/${studioId}/campaigns/${id}`),
    serverFetch<{ leads: Lead[]; total: number }>(
      `/api/v1/studios/${studioId}/leads?campaignId=${id}&limit=10`,
    ),
  ]);

  const totalLeads = leadsResp.total ?? 0;
  const plans = c.fitnessPlans ?? [];

  // Compute a quick status breakdown from the first 10 leads shown
  const statusCounts: Record<string, number> = {};
  for (const l of leadsResp.leads) {
    statusCounts[l.status] = (statusCounts[l.status] ?? 0) + 1;
  }

  return (
    <div className="space-y-6">
      {/* ── Premium Glass Header ── */}
      <div
        className="relative overflow-hidden rounded-[26px] border border-white/30 p-6 backdrop-blur-2xl dark:border-white/5 bg-white/30 dark:bg-neutral-900/30"
        style={{
          boxShadow: 'inset 0 0 0 1px rgba(255,255,255,0.2), 0 8px 32px rgba(139,92,246,0.07)',
        }}
      >
        <div className="pointer-events-none absolute -right-16 -top-16 h-48 w-48 rounded-full bg-brand-500/10 blur-[70px]" />

        <div className="relative flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div className="flex items-start gap-4">
            <div className="grid h-14 w-14 shrink-0 place-items-center rounded-2xl bg-gradient-to-br from-brand-500 to-violet-600 text-white shadow-lg shadow-brand-500/25">
              <Zap className="h-7 w-7" />
            </div>
            <div>
              <div className="flex flex-wrap items-center gap-3">
                <h1 className="text-2xl font-black tracking-tight text-zinc-900 dark:text-white">{c.name}</h1>
                <Badge tone={c.active ? 'success' : 'neutral'} className="rounded-xl px-2.5 py-0.5 text-[10px] font-black uppercase tracking-widest shadow-sm">
                  {c.active ? 'Active' : 'Inactive'}
                </Badge>
              </div>
              <p className="mt-1 text-sm font-medium text-zinc-500 dark:text-zinc-400">
                <span className="font-mono text-xs">/{c.slug}</span>
                {c.description && <span> · {c.description}</span>}
                {' · '}Created {formatDate(c.createdAt)}
              </p>

              {/* ── Inline Stats Chips ── */}
              <div className="mt-3 flex flex-wrap gap-3">
                <div className="flex items-center gap-1.5 rounded-full bg-violet-50 px-3 py-1 text-xs font-bold text-violet-700 dark:bg-violet-900/30 dark:text-violet-300">
                  <Users className="h-3.5 w-3.5" />
                  {totalLeads} lead{totalLeads !== 1 ? 's' : ''}
                </div>
                <div className="flex items-center gap-1.5 rounded-full bg-sky-50 px-3 py-1 text-xs font-bold text-sky-700 dark:bg-sky-900/30 dark:text-sky-300">
                  <Target className="h-3.5 w-3.5" />
                  {plans.length} plan{plans.length !== 1 ? 's' : ''}
                </div>
                <div className="flex items-center gap-1.5 rounded-full bg-zinc-100 px-3 py-1 text-xs font-bold text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300">
                  <Calendar className="h-3.5 w-3.5" />
                  Created {formatDate(c.createdAt)}
                </div>
              </div>
            </div>
          </div>
          <CampaignActions studioId={studioId} id={c.id} active={c.active} />
        </div>
      </div>

      <div className="space-y-6">

        {/* ── Share Link ── */}
        <Card title="Share link" subtitle="Drop this URL in your Instagram bio, story, or ad.">
          <CopyLink url={c.shareUrl} />
        </Card>

        <div className="grid gap-6 md:grid-cols-2">
          {/* ── Fitness Plans ── */}
          <Card title="Fitness plans offered" subtitle="Edit the options shown in the public lead form.">
            <CampaignPlansEditor
              studioId={studioId}
              campaignId={c.id}
              studioSlug={c.studioSlug || ''}
              campaignSlug={c.slug}
              initialPlans={plans}
            />
            {/* Plan chips preview */}
            {plans.length > 0 && (
              <div className="mt-4 flex flex-wrap gap-2 border-t border-slate-100 pt-4 dark:border-slate-800/60">
                <p className="w-full text-[11px] font-semibold uppercase tracking-wider text-slate-400">Plans offered</p>
                {plans.map((p) => (
                  <span
                    key={p}
                    className="inline-flex items-center gap-1 rounded-xl bg-violet-50 px-3 py-1 text-xs font-semibold text-violet-700 dark:bg-violet-900/30 dark:text-violet-300"
                  >
                    <Zap className="h-3 w-3" />
                    {p}
                  </span>
                ))}
              </div>
            )}
          </Card>

          {/* ── Campaign Details ── */}
          <Card title="Details">
            <dl className="space-y-3 text-sm">
              <div className="flex items-center justify-between">
                <dt className="text-slate-500 dark:text-slate-400">Status</dt>
                <dd>
                  <Badge tone={c.active ? 'success' : 'neutral'}>
                    {c.active ? 'Active' : 'Inactive'}
                  </Badge>
                </dd>
              </div>
              <div className="flex items-center justify-between">
                <dt className="text-slate-500 dark:text-slate-400">Total leads</dt>
                <dd className="flex items-center gap-1.5 font-black text-zinc-900 dark:text-white">
                  <Users className="h-4 w-4 text-violet-500" />
                  {totalLeads}
                </dd>
              </div>
              <div className="flex items-center justify-between">
                <dt className="text-slate-500 dark:text-slate-400">Plans</dt>
                <dd className="font-black text-zinc-900 dark:text-white">{plans.length}</dd>
              </div>
              <div className="flex items-center justify-between">
                <dt className="text-slate-500 dark:text-slate-400">Slug</dt>
                <dd className="font-mono text-xs text-slate-700 dark:text-slate-300">/{c.slug}</dd>
              </div>
              <div className="flex items-center justify-between">
                <dt className="text-slate-500 dark:text-slate-400">Created</dt>
                <dd className="text-slate-700 dark:text-slate-300">
                  {formatDateTime(c.createdAt)}
                </dd>
              </div>
              {c.description && (
                <div className="border-t border-slate-100 pt-3 text-slate-600 dark:border-slate-800/60 dark:text-slate-300">
                  {c.description}
                </div>
              )}
            </dl>

            {/* ── Quick lead status breakdown ── */}
            {totalLeads > 0 && (
              <div className="mt-5 border-t border-slate-100 pt-4 dark:border-slate-800/60">
                <p className="mb-3 text-[11px] font-semibold uppercase tracking-wider text-slate-400">Lead status (latest 10)</p>
                <div className="flex flex-wrap gap-2">
                  {Object.entries(statusCounts).map(([status, count]) => (
                    <Link
                      key={status}
                      href={`/admin/studios/${studioId}/leads?campaignId=${c.id}&status=${status}`}
                      className="flex items-center gap-1.5 rounded-xl bg-slate-50 px-3 py-1 text-xs font-bold text-slate-600 transition-all hover:bg-violet-50 hover:text-violet-700 dark:bg-zinc-800 dark:text-zinc-300 dark:hover:bg-violet-900/30 dark:hover:text-violet-300"
                    >
                      <Badge tone={statusTone[status as keyof typeof statusTone] ?? 'neutral'} className="!py-0 !px-1.5 text-[10px]">
                        {count}
                      </Badge>
                      {statusLabel[status] ?? status}
                    </Link>
                  ))}
                </div>
              </div>
            )}
          </Card>
        </div>

        {/* ── Leads Table ── */}
        <Card
          title={`Recent leads${totalLeads > 0 ? ` (${totalLeads} total)` : ''}`}
          action={
            totalLeads > 0 ? (
              <Link
                href={`/admin/studios/${studioId}/leads?campaignId=${c.id}`}
                className="flex items-center gap-1 text-sm font-semibold text-[color:var(--brand,#7c3aed)] hover:underline"
              >
                View all <ArrowRight className="h-3.5 w-3.5" />
              </Link>
            ) : undefined
          }
          noPadding
        >
          {leadsResp.leads.length === 0 ? (
            <EmptyState
              title="No submissions yet"
              description="Share the link above to start collecting leads."
            />
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full min-w-[640px] text-sm">
                <thead className="border-b border-slate-200 bg-slate-50/50 text-left text-xs uppercase tracking-wider text-slate-500 dark:border-slate-800 dark:bg-slate-900/40 dark:text-slate-400">
                  <tr>
                    <th className="px-6 py-3 font-semibold">Name</th>
                    <th className="px-6 py-3 font-semibold">Email</th>
                    <th className="px-6 py-3 font-semibold">Phone</th>
                    <th className="px-6 py-3 font-semibold">Plan</th>
                    <th className="px-6 py-3 font-semibold">Status</th>
                    <th className="px-6 py-3 font-semibold">Submitted</th>
                    <th className="px-6 py-3 font-semibold" />
                  </tr>
                </thead>
                <tbody>
                  {leadsResp.leads.map((l) => (
                    <tr
                      key={l.id}
                      className="border-b border-slate-100 last:border-0 hover:bg-violet-50/30 dark:border-slate-800/60 dark:hover:bg-violet-900/10 transition-colors"
                    >
                      <td className="px-6 py-3">
                        <Link
                          href={`/admin/studios/${studioId}/leads/${l.id}`}
                          className="font-semibold text-slate-900 hover:text-violet-600 dark:text-slate-100 dark:hover:text-violet-400"
                        >
                          {l.name || `${l.firstName} ${l.lastName}`.trim() || '—'}
                        </Link>
                      </td>
                      <td className="px-6 py-3 text-slate-600 dark:text-slate-300">
                        {l.email || '—'}
                      </td>
                      <td className="px-6 py-3 text-slate-600 dark:text-slate-300">
                        {l.phone || '—'}
                      </td>
                      <td className="px-6 py-3">
                        {l.fitnessPlan ? (
                          <span className="inline-flex items-center gap-1 rounded-xl bg-violet-50 px-2 py-0.5 text-xs font-semibold text-violet-700 dark:bg-violet-900/30 dark:text-violet-300">
                            <Zap className="h-3 w-3" />
                            {l.fitnessPlan}
                          </span>
                        ) : '—'}
                      </td>
                      <td className="px-6 py-3">
                        <Badge tone={statusTone[l.status] ?? 'neutral'}>
                          {statusLabel[l.status] ?? l.status}
                        </Badge>
                      </td>
                      <td className="px-6 py-3 text-slate-500 dark:text-slate-400">
                        {formatDateTime(l.createdAt)}
                      </td>
                      <td className="px-6 py-3">
                        <Link
                          href={`/admin/studios/${studioId}/leads/${l.id}`}
                          className="flex items-center gap-1 text-xs font-semibold text-violet-600 hover:underline dark:text-violet-400"
                        >
                          View <ArrowRight className="h-3 w-3" />
                        </Link>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>

              {/* Show More footer */}
              {totalLeads > 10 && (
                <div className="border-t border-slate-100 px-6 py-4 dark:border-slate-800/60">
                  <Link
                    href={`/admin/studios/${studioId}/leads?campaignId=${c.id}`}
                    className="flex items-center gap-2 text-sm font-semibold text-violet-600 hover:underline dark:text-violet-400"
                  >
                    <BarChart3 className="h-4 w-4" />
                    See all {totalLeads} leads for this campaign
                  </Link>
                </div>
              )}
            </div>
          )}
        </Card>
      </div>
    </div>
  );
}
