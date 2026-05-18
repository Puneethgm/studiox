import { Zap } from 'lucide-react';
import { Badge } from '@/components/ui/Badge';
import { Card } from '@/components/ui/Card';
import { EmptyState } from '@/components/ui/EmptyState';
import { serverFetch } from '@/lib/auth';
import { formatDate, formatDateTime } from '@/lib/datetime';
import type { Campaign, Lead } from '@/lib/types';
import { CopyLink } from '../CopyLink';
import { CampaignActions } from './actions';

const statusTone = {
  new: 'info',
  contacted: 'brand',
  trial_booked: 'warning',
  member: 'success',
  dropped: 'neutral',
} as const;

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

  return (
    <div className="space-y-6">
      {/* Premium Glass Header */}
      <div
        className="relative overflow-hidden rounded-[26px] border border-white/30 p-6 backdrop-blur-2xl dark:border-white/5"
        style={{
          background: 'linear-gradient(135deg, rgba(255,255,255,0.30) 0%, rgba(237,233,254,0.22) 60%, rgba(219,234,254,0.20) 100%)',
          boxShadow: 'inset 0 0 0 1px rgba(255,255,255,0.2), 0 8px 32px rgba(139,92,246,0.07)',
        }}
      >
        <div className="pointer-events-none absolute -right-16 -top-16 h-48 w-48 rounded-full bg-brand-500/10 blur-[70px]" />
        
        <div className="relative flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex items-center gap-4">
            <div className="grid h-12 w-12 place-items-center rounded-2xl bg-gradient-to-br from-brand-500 to-violet-600 text-white shadow-lg shadow-brand-500/25">
              <Zap className="h-6 w-6" />
            </div>
            <div>
              <div className="flex items-center gap-3">
                <h1 className="text-2xl font-black tracking-tight text-zinc-900 dark:text-white">{c.name}</h1>
                <Badge tone={c.active ? 'success' : 'neutral'} className="rounded-xl px-2.5 py-0.5 text-[10px] font-black uppercase tracking-widest shadow-sm">
                  {c.active ? 'Active' : 'Inactive'}
                </Badge>
              </div>
              <p className="mt-1 text-sm font-medium text-zinc-500 dark:text-zinc-400">
                <span className="font-mono text-xs">/{c.slug}</span> · Created {formatDate(c.createdAt)}
              </p>
            </div>
          </div>
          <CampaignActions studioId={studioId} id={c.id} active={c.active} />
        </div>
      </div>

      <div className="space-y-6">
        <Card title="Share link" subtitle="Drop this URL in your Instagram bio, story, or ad.">
          <CopyLink url={c.shareUrl} />
        </Card>

        <div className="grid gap-6 md:grid-cols-2">
          <Card title="Fitness plans offered">
            <ul className="flex flex-wrap gap-2">
              {c.fitnessPlans.map((p) => (
                <li
                  key={p}
                  className="rounded-full px-3 py-1 text-xs font-medium"
                  style={{ background: 'var(--brand-soft, rgba(124,58,237,0.08))', color: 'var(--brand, #7c3aed)' }}
                >
                  {p}
                </li>
              ))}
            </ul>
          </Card>

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
          </Card>
        </div>

        <Card
          title={`Recent leads (${leadsResp.total})`}
          action={
            <Link
              href={`/admin/studios/${studioId}/leads?campaignId=${c.id}`}
              className="text-sm font-medium text-[color:var(--brand,#7c3aed)] hover:underline"
            >
              View all →
            </Link>
          }
          noPadding
        >
          {leadsResp.leads.length === 0 ? (
            <EmptyState title="No submissions yet" description="Share the link above to start collecting leads." />
          ) : (
            <div className="overflow-x-auto">
            <table className="w-full min-w-[640px] text-sm">
              <thead className="border-b border-slate-200 bg-slate-50/50 text-left text-xs uppercase tracking-wider text-slate-500 dark:border-slate-800 dark:bg-slate-900/40 dark:text-slate-400">
                <tr>
                  <th className="px-6 py-3 font-medium">Name</th>
                  <th className="px-6 py-3 font-medium">Email</th>
                  <th className="px-6 py-3 font-medium">Plan</th>
                  <th className="px-6 py-3 font-medium">Status</th>
                  <th className="px-6 py-3 font-medium">Submitted</th>
                </tr>
              </thead>
              <tbody>
                {leadsResp.leads.map((l) => (
                  <tr
                    key={l.id}
                    className="border-b border-slate-100 last:border-0 hover:bg-slate-50/60 dark:border-slate-800/60 dark:hover:bg-slate-800/30"
                  >
                    <td className="px-6 py-3">
                      <Link
                        href={`/admin/studios/${studioId}/leads/${l.id}`}
                        className="font-medium text-slate-900 hover:text-[color:var(--brand,#7c3aed)] dark:text-slate-100"
                      >
                        {l.name}
                      </Link>
                    </td>
                    <td className="px-6 py-3 text-slate-600 dark:text-slate-300">{l.email}</td>
                    <td className="px-6 py-3 text-slate-600 dark:text-slate-300">{l.fitnessPlan}</td>
                    <td className="px-6 py-3">
                      <Badge tone={statusTone[l.status]}>{l.status}</Badge>
                    </td>
                    <td className="px-6 py-3 text-slate-500 dark:text-slate-400">
                      {formatDateTime(l.createdAt)}
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
  );
}
