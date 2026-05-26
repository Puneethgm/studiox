import Link from 'next/link';
import { Inbox, Users, TrendingUp } from 'lucide-react';
import { Badge } from '@/components/ui/Badge';
import { EmptyState } from '@/components/ui/EmptyState';
import { Pagination } from '@/components/ui/Pagination';
import { serverFetch } from '@/lib/auth';
import { formatDateTime } from '@/lib/datetime';
import type { Lead, LeadStatus } from '@/lib/types';
import { LEAD_STATUSES, LEAD_STATUS_LABELS } from '@/lib/types';
import { LeadFilters } from '../studios/[studioId]/leads/LeadFilters';

interface ListResp {
  leads: Lead[];
  total: number;
}

interface SearchParams {
  campaignId?: string;
  status?: string;
  page?: string;
  search?: string;
  source?: string;
  duration?: string;
  startDate?: string;
  endDate?: string;
}

const PAGE_SIZE = 25;

const statusTone: Record<LeadStatus, 'info' | 'brand' | 'warning' | 'success' | 'neutral'> = {
  new: 'info',
  contacted: 'brand',
  trial_booked: 'warning',
  member: 'success',
  dropped: 'neutral',
  paused: 'brand',
};

const statusDot: Record<LeadStatus, string> = {
  new: 'bg-sky-400',
  contacted: 'bg-violet-500',
  trial_booked: 'bg-amber-400',
  member: 'bg-emerald-400',
  dropped: 'bg-zinc-400',
  paused: 'bg-indigo-500',
};

const avatarGradients = [
  'from-violet-500 to-purple-600',
  'from-sky-500 to-blue-600',
  'from-emerald-500 to-teal-600',
  'from-amber-500 to-orange-600',
  'from-rose-500 to-pink-600',
];

function gradientForName(name: string) {
  const code = name ? name.charCodeAt(0) : 65;
  return avatarGradients[code % avatarGradients.length];
}

export default async function GlobalLeadsPage({
  searchParams,
}: {
  searchParams: Promise<SearchParams>;
}) {
  const sp = await searchParams;
  const page = Math.max(1, Number(sp.page) || 1);
  const offset = (page - 1) * PAGE_SIZE;

  const qs = new URLSearchParams();
  if (sp.campaignId) qs.set('campaignId', sp.campaignId);
  if (sp.status && (LEAD_STATUSES as string[]).includes(sp.status)) qs.set('status', sp.status);
  if (sp.search) qs.set('search', sp.search);
  if (sp.source) qs.set('source', sp.source);
  if (sp.duration) qs.set('duration', sp.duration);
  if (sp.startDate) qs.set('startDate', sp.startDate);
  if (sp.endDate) qs.set('endDate', sp.endDate);
  qs.set('limit', String(PAGE_SIZE));
  qs.set('offset', String(offset));

  const data = await serverFetch<ListResp>(`/api/v1/admin/leads?${qs.toString()}`);
  const sources = await serverFetch<string[]>(`/api/v1/admin/leads/sources`).catch(() => []);

  return (
    <div className="space-y-5 pb-10">
      {/* Header */}
      <div
        className="relative overflow-hidden rounded-[24px] border border-white/30 bg-white/30 px-6 py-4 backdrop-blur-2xl dark:border-white/5 dark:bg-neutral-900/30"
        style={{ boxShadow: 'inset 0 0 0 1px rgba(255,255,255,0.15), 0 4px 16px rgba(0,0,0,0.05)' }}
      >
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
          <div className="flex items-center gap-4">
            <div className="grid h-10 w-10 place-items-center rounded-2xl bg-gradient-to-br from-violet-500/20 to-purple-600/10 text-violet-600 dark:text-violet-400">
              <Users className="h-5 w-5" />
            </div>
            <div>
              <h1 className="text-lg font-black tracking-tight text-zinc-900 dark:text-white">Global Leads</h1>
              <p className="text-[11px] font-semibold text-zinc-400">{data.total} submissions captured across all studios</p>
            </div>
          </div>
          
          <div className="flex items-center gap-3">
            <div className="flex items-center gap-1.5 rounded-full bg-emerald-500/10 px-3 py-1.5 text-[10px] font-black uppercase tracking-widest text-emerald-600 dark:text-emerald-400">
              <TrendingUp className="h-3 w-3" />
              Live
            </div>
          </div>
        </div>
      </div>

      {/* Filters */}
      <LeadFilters
        status={sp.status}
        search={sp.search}
        source={sp.source}
        duration={sp.duration}
        startDate={sp.startDate}
        endDate={sp.endDate}
        sources={sources}
      />

      {/* List */}
      {data.leads.length === 0 ? (
        <div
          className="overflow-hidden rounded-[24px] border border-white/30 bg-white/30 backdrop-blur-2xl dark:border-white/5 dark:bg-neutral-900/30"
          style={{ boxShadow: 'inset 0 0 0 1px rgba(255,255,255,0.15)' }}
        >
          <EmptyState
            icon={<Inbox className="h-6 w-6" />}
            title="No leads match these filters"
            description="Try changing the status filter or select a different time range."
          />
        </div>
      ) : (
        <>
          <div
            className="overflow-hidden rounded-[24px] border border-white/30 bg-white/30 backdrop-blur-2xl dark:border-white/5 dark:bg-neutral-900/30"
            style={{ boxShadow: 'inset 0 0 0 1px rgba(255,255,255,0.15), 0 4px 16px rgba(0,0,0,0.04)' }}
          >
            {/* Column headers */}
            <div className="grid grid-cols-[1.2fr,1.1fr,1fr,90px,80px,110px,100px,120px] items-center gap-4 border-b border-white/20 bg-white/20 px-5 py-3 dark:border-white/5 dark:bg-white/5">
              <span className="text-[10px] font-black uppercase tracking-[0.18em] text-zinc-400">Name / Studio</span>
              <span className="text-[10px] font-black uppercase tracking-[0.18em] text-zinc-400">Contact</span>
              <span className="text-[10px] font-black uppercase tracking-[0.18em] text-zinc-400">Campaign</span>
              <span className="text-[10px] font-black uppercase tracking-[0.18em] text-zinc-400">Source</span>
              <span className="text-[10px] font-black uppercase tracking-[0.18em] text-zinc-400">Attempts</span>
              <span className="text-[10px] font-black uppercase tracking-[0.18em] text-zinc-400">Flags</span>
              <span className="text-[10px] font-black uppercase tracking-[0.18em] text-zinc-400">Status</span>
              <span className="text-[10px] font-black uppercase tracking-[0.18em] text-zinc-400 text-right">Date</span>
            </div>

            <ul className="divide-y divide-white/10 dark:divide-white/5">
              {data.leads.map((l) => (
                <li key={l.id}>
                  <Link
                    href={`/admin/studios/${l.studioId}/leads/${l.id}`}
                    className="group grid grid-cols-[1.2fr,1.1fr,1fr,90px,80px,110px,100px,120px] items-center gap-4 px-5 py-3.5 transition-all hover:bg-white/30 dark:hover:bg-white/5"
                  >
                    {/* Name / Studio */}
                    <div className="flex min-w-0 items-center gap-3">
                      <div className={`grid h-9 w-9 shrink-0 place-items-center rounded-2xl bg-gradient-to-br ${gradientForName(l.name)} text-sm font-black text-white shadow-md`}>
                        {l.name ? l.name.charAt(0).toUpperCase() : 'L'}
                      </div>
                      <div className="min-w-0">
                        <span className="truncate block text-sm font-bold text-zinc-900 group-hover:text-brand-600 dark:text-zinc-100 dark:group-hover:text-brand-400">
                          {l.name || 'Anonymous Lead'}
                        </span>
                        <span className="text-[10px] font-semibold text-zinc-400 block truncate">
                          {l.studioName || 'Unknown Studio'}
                        </span>
                      </div>
                    </div>

                    {/* Contact */}
                    <div className="min-w-0">
                      <div className="truncate text-sm font-semibold text-zinc-700 dark:text-zinc-300">{l.email || '-'}</div>
                      <div className="truncate text-[11px] text-zinc-400">{l.phone || '-'}</div>
                    </div>

                    {/* Campaign */}
                    <div className="truncate text-xs font-semibold text-zinc-600 dark:text-zinc-400">
                      {l.campaignName || '-'}
                    </div>

                    {/* Source */}
                    <div className="truncate text-xs font-semibold text-zinc-600 dark:text-zinc-400">
                      {l.source || '-'}
                    </div>

                    {/* Attempts */}
                    <div className="text-sm font-medium text-zinc-600 dark:text-zinc-300 pl-4">{l.contactAttempts || 0}</div>

                    {/* Flags */}
                    <div className="flex flex-wrap gap-1">
                      {l.hotLead && (
                        <span className="inline-flex items-center gap-0.5 rounded-full bg-red-500/10 px-1.5 py-0.5 text-[9px] font-black uppercase tracking-wider text-red-600 dark:text-red-400">
                          🔥 Hot
                        </span>
                      )}
                      {l.trialPurchased && (
                        <span className="inline-flex items-center gap-0.5 rounded-full bg-emerald-500/10 px-1.5 py-0.5 text-[9px] font-black uppercase tracking-wider text-emerald-600 dark:text-emerald-400">
                          🎟️ Trial
                        </span>
                      )}
                      {l.contactMade && (
                        <span className="inline-flex items-center gap-0.5 rounded-full bg-sky-500/10 px-1.5 py-0.5 text-[9px] font-black uppercase tracking-wider text-sky-600 dark:text-sky-400">
                          💬 Made
                        </span>
                      )}
                      {!l.hotLead && !l.trialPurchased && !l.contactMade && (
                        <span className="text-xs text-zinc-400">-</span>
                      )}
                    </div>

                    {/* Status */}
                    <div className="flex items-center gap-2">
                      <span className={`h-2 w-2 shrink-0 rounded-full ${statusDot[l.status]}`} />
                      <Badge tone={statusTone[l.status]}>{LEAD_STATUS_LABELS[l.status]}</Badge>
                    </div>

                    {/* Date */}
                    <div className="text-right text-[11px] font-semibold text-zinc-400">
                      {formatDateTime(l.createdAt)}
                    </div>
                  </Link>
                </li>
              ))}
            </ul>

            {/* Footer row */}
            <div className="flex items-center justify-between border-t border-white/10 bg-white/10 px-5 py-2.5 dark:border-white/5 dark:bg-white/5">
              <span className="text-[11px] font-semibold text-zinc-400">
                Showing {offset + 1}–{Math.min(offset + PAGE_SIZE, data.total)} of {data.total} leads
              </span>
            </div>
          </div>

          <Pagination total={data.total} pageSize={PAGE_SIZE} page={page} />
        </>
      )}
    </div>
  );
}
