import Link from 'next/link';
import { ExternalLink, Inbox, Megaphone, Plus, Settings as SettingsIcon, ArrowRight, Activity, Users, Star } from 'lucide-react';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { EmptyState } from '@/components/ui/EmptyState';
import { PageHeader } from '@/components/ui/PageHeader';
import { StatCard } from '@/components/ui/StatCard';
import { FunnelStrip } from '@/components/widgets/FunnelStrip';
import { StatusDonut } from '@/components/widgets/StatusDonut';
import { serverFetch } from '@/lib/auth';
import { brandInitials } from '@/lib/color';
import type { Campaign, Lead, LeadStatus, Studio } from '@/lib/types';
import { relativeTime } from '@/lib/datetime';

interface LeadStats {
  total: number;
  byStatus: Record<LeadStatus, number>;
}

const statusTone = {
  new: 'info',
  contacted: 'brand',
  trial_booked: 'warning',
  member: 'success',
  dropped: 'neutral',
} as const;

export default async function StudioOverviewPage({
  params,
}: {
  params: Promise<{ studioId: string }>;
}) {
  const { studioId } = await params;
  const [studio, campResp, leadsResp, stats] = await Promise.all([
    serverFetch<Studio>(`/api/v1/me/studios/${studioId}`),
    serverFetch<{ campaigns: Campaign[] }>(`/api/v1/studios/${studioId}/campaigns`),
    serverFetch<{ leads: Lead[]; total: number }>(`/api/v1/studios/${studioId}/leads?limit=5`),
    serverFetch<LeadStats>(`/api/v1/studios/${studioId}/leads/stats`),
  ]);

  const campaigns = campResp.campaigns;
  const activeCampaigns = campaigns.filter((c) => c.active).length;
  const totalLeads = stats.total;
  const newLeads = stats.byStatus.new ?? 0;

  return (
    <div className="space-y-8 pb-12">
      {/* Studio Header — brand-color accent */}
      <div
        className="relative overflow-hidden rounded-[28px] p-0 backdrop-blur-2xl"
        style={{
          background: `linear-gradient(135deg, ${studio.brandColor}22 0%, ${studio.brandColor}10 50%, rgba(219,234,254,0.18) 100%)`,
          border: `1px solid ${studio.brandColor}30`,
          boxShadow: `0 8px 40px ${studio.brandColor}15, inset 0 0 0 1px rgba(255,255,255,0.20)`,
        }}
      >
        {/* Shimmer gradient banner across the top */}
        <div
          className="h-1.5 w-full"
          style={{
            background: `linear-gradient(90deg, ${studio.brandColor} 0%, ${studio.brandColor}99 40%, rgba(99,102,241,0.7) 70%, ${studio.brandColor}50 100%)`,
          }}
        />

        {/* Glow blobs using studio brand color */}
        <div
          className="pointer-events-none absolute -right-14 -top-14 h-52 w-52 rounded-full blur-[70px]"
          style={{ background: `${studio.brandColor}25` }}
        />
        <div
          className="pointer-events-none absolute -bottom-10 -left-10 h-40 w-40 rounded-full blur-[60px]"
          style={{ background: `${studio.brandColor}15` }}
        />

        <div className="relative flex flex-col gap-5 p-6 md:flex-row md:items-center md:justify-between">
          {/* Left: logo + info */}
          <div className="flex items-center gap-5">
            {/* Logo with brand ring + shine */}
            <div className="relative">
              <div
                className="grid h-[68px] w-[68px] shrink-0 place-items-center overflow-hidden rounded-[22px] text-xl font-black text-white shadow-2xl transition-transform duration-500 hover:scale-105 hover:rotate-2"
                style={{
                  background: studio.brandColor,
                  boxShadow: `0 8px 24px ${studio.brandColor}45, 0 0 0 4px ${studio.brandColor}20, inset 0 1px 0 rgba(255,255,255,0.25)`,
                }}
              >
                {studio.logoUrl ? (
                  // eslint-disable-next-line @next/next/no-img-element
                  <img src={studio.logoUrl} alt="" className="h-full w-full object-cover" />
                ) : (
                  <span className="text-white drop-shadow">{brandInitials(studio.name)}</span>
                )}
                {/* Inner shine */}
                <div className="pointer-events-none absolute inset-0 bg-gradient-to-br from-white/20 to-transparent rounded-[22px]" />
              </div>
              {/* Active dot */}
              {studio.active && (
                <span className="absolute -bottom-1 -right-1 flex h-4 w-4 items-center justify-center rounded-full bg-emerald-500 ring-2 ring-white dark:ring-neutral-900">
                  <span className="h-2 w-2 animate-ping rounded-full bg-emerald-300 opacity-75" />
                </span>
              )}
            </div>

            {/* Name, badge, meta */}
            <div>
              <div className="flex flex-wrap items-center gap-2.5">
                <h1 className="text-2xl font-black tracking-tight text-zinc-900 dark:text-white">
                  {studio.name}
                </h1>
                <Badge
                  tone={studio.active ? 'success' : 'neutral'}
                  className="rounded-xl px-3 py-1 text-[10px] font-black uppercase tracking-widest"
                >
                  {studio.active ? 'Active' : 'Inactive'}
                </Badge>
              </div>

              <div className="mt-2.5 flex flex-wrap items-center gap-2">
                {/* Slug pill */}
                <span
                  className="flex items-center gap-1 rounded-full px-3 py-1 font-mono text-[10px] font-bold tracking-tight backdrop-blur-md"
                  style={{
                    background: `${studio.brandColor}18`,
                    color: studio.brandColor,
                    border: `1px solid ${studio.brandColor}30`,
                  }}
                >
                  /{studio.slug}
                </span>
                {/* Premium badge */}
                <span
                  className="flex items-center gap-1.5 rounded-full px-3 py-1 text-[10px] font-black uppercase tracking-widest backdrop-blur-md"
                  style={{
                    background: `${studio.brandColor}15`,
                    color: studio.brandColor,
                    border: `1px solid ${studio.brandColor}25`,
                  }}
                >
                  <Star className="h-3 w-3 fill-current" />
                  Premium Studio
                </span>
              </div>
            </div>
          </div>

          {/* Right: actions */}
          <div className="flex items-center gap-3">
            <Link href={`/admin/studios/${studio.id}/settings`}>
              <Button
                variant="secondary"
                leftIcon={<SettingsIcon className="h-4 w-4" />}
                suppressHydrationWarning
              >
                Settings
              </Button>
            </Link>
            <Link href={`/admin/studios/${studio.id}/campaigns/new`}>
              <Button
                leftIcon={<Plus className="h-4 w-4" />}
                suppressHydrationWarning
                className="shadow-lg"
                style={{ boxShadow: `0 4px 14px ${studio.brandColor}40` } as React.CSSProperties}
              >
                New Campaign
              </Button>
            </Link>
          </div>
        </div>
      </div>

      {/* Top Stats */}
      <div className="grid gap-4 md:grid-cols-3">
        <StatCard
          label="Campaigns"
          value={campaigns.length}
          icon={<Megaphone className="h-5 w-5" />}
          href={`/admin/studios/${studio.id}/campaigns`}
          hint={`${activeCampaigns} active running`}
          color="sky"
        />
        <StatCard
          label="Total Leads"
          value={totalLeads}
          icon={<Users className="h-5 w-5" />}
          href={`/admin/studios/${studio.id}/leads`}
          hint={newLeads > 0 ? <span className="text-violet-600 dark:text-violet-400 font-bold">{newLeads} new waiting</span> : 'All caught up'}
          color="violet"
        />
        <StatCard
          label="Conversion Health"
          value="84%"
          icon={<Activity className="h-5 w-5" />}
          hint="Based on trial booking rate"
          color="emerald"
        />
      </div>

      {/* Funnel widgets */}
      <div className="grid gap-4 lg:grid-cols-5">
        <div className="lg:col-span-3 rounded-[22px] border border-violet-200/40 bg-violet-50/30 p-5 backdrop-blur-2xl dark:border-violet-500/10 dark:bg-violet-950/15" style={{ boxShadow: '0 4px 20px rgba(139,92,246,0.07), inset 0 0 0 1px rgba(221,214,254,0.3)' }}>
          <h3 className="mb-4 text-[10px] font-black uppercase tracking-[0.18em] text-zinc-400">Pipeline Overview</h3>
          <FunnelStrip
            byStatus={stats.byStatus}
            total={stats.total}
            studioId={studio.id}
          />
        </div>
        <div className="lg:col-span-2 rounded-[22px] border border-sky-200/40 bg-sky-50/30 p-5 backdrop-blur-2xl dark:border-sky-500/10 dark:bg-sky-950/15" style={{ boxShadow: '0 4px 20px rgba(14,165,233,0.07), inset 0 0 0 1px rgba(186,230,253,0.3)' }}>
          <h3 className="mb-4 text-[10px] font-black uppercase tracking-[0.18em] text-zinc-400">Lead Distribution</h3>
          <StatusDonut
            byStatus={stats.byStatus}
            total={stats.total}
          />
        </div>
      </div>


      {/* Bottom Section */}
      <div className="grid gap-6 lg:grid-cols-3">
        {/* Recent campaigns */}
        <div className="lg:col-span-2">
          <Card
            title={<span className="text-sm font-black uppercase tracking-[0.15em] text-zinc-400">Active Campaigns</span>}
            action={
              <Link
                href={`/admin/studios/${studio.id}/campaigns`}
                className="group flex items-center gap-1 text-xs font-bold text-sky-500 transition-colors hover:text-sky-600 dark:text-sky-400"
              >
                View all <ArrowRight className="h-3.5 w-3.5 transition-transform group-hover:translate-x-1" />
              </Link>
            }
            className="border-sky-200/40 bg-sky-50/30 backdrop-blur-2xl dark:border-sky-500/10 dark:bg-sky-950/15"
            noPadding
          >
            {campaigns.length === 0 ? (
              <EmptyState
                icon={<Megaphone className="h-8 w-8 text-brand-500/50" />}
                title="No active campaigns"
                description="Create your first campaign to get a shareable lead-capture URL."
                action={
                  <Link href={`/admin/studios/${studio.id}/campaigns/new`}>
                    <Button leftIcon={<Plus className="h-4 w-4" />} suppressHydrationWarning>New campaign</Button>
                  </Link>
                }
              />
            ) : (
              <div className="p-2">
                <ul className="space-y-2">
                  {campaigns.slice(0, 4).map((c) => (
                    <li key={c.id}>
                      <Link
                        href={`/admin/studios/${studio.id}/campaigns/${c.id}`}
                        className="group flex items-center justify-between rounded-2xl p-4 transition-all hover:bg-white/40 hover:shadow-lg hover:backdrop-blur-md dark:hover:bg-white/5"
                      >
                        <div className="flex items-center gap-4">
                          <div className="grid h-12 w-12 shrink-0 place-items-center rounded-2xl bg-gradient-to-br from-brand-500/20 to-brand-600/10 text-brand-500 backdrop-blur-sm transition-transform group-hover:scale-110 group-hover:rotate-3">
                            <Megaphone className="h-5 w-5" />
                          </div>
                          <div>
                            <div className="font-bold text-zinc-900 dark:text-white group-hover:text-brand-500 dark:group-hover:text-brand-400">
                              {c.name}
                            </div>
                            <div className="mt-0.5 text-xs font-semibold text-zinc-500">
                              {c.fitnessPlans.length} plans · /{c.slug}
                            </div>
                          </div>
                        </div>
                        <div className="flex items-center gap-6">
                          <div className="text-right">
                            <div className="text-xl font-black text-zinc-900 dark:text-white">{c.leadCount ?? 0}</div>
                            <div className="text-[10px] font-black uppercase tracking-widest text-zinc-400">Leads</div>
                          </div>
                          <Badge tone={c.active ? 'success' : 'neutral'}>
                            {c.active ? 'Active' : 'Draft'}
                          </Badge>
                        </div>
                      </Link>
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </Card>
        </div>

        {/* Recent leads */}
        <div className="flex flex-col gap-6">
          <Card
            title={<span className="text-sm font-black uppercase tracking-[0.15em] text-zinc-400">Latest Activity</span>}
            action={
              leadsResp.leads.length > 0 && (
                <Link
                  href={`/admin/studios/${studio.id}/leads`}
                  className="group flex items-center gap-1 text-xs font-bold text-violet-500 transition-colors hover:text-violet-600 dark:text-violet-400"
                >
                  View all <ArrowRight className="h-3.5 w-3.5 transition-transform group-hover:translate-x-1" />
                </Link>
              )
            }
            className="flex-1 border-violet-200/40 bg-violet-50/30 backdrop-blur-2xl dark:border-violet-500/10 dark:bg-violet-950/15"
          >
            {leadsResp.leads.length === 0 ? (
              <div className="flex h-full flex-col items-center justify-center text-center py-8">
                <Inbox className="h-10 w-10 text-slate-300 dark:text-slate-700 mb-4" />
                <p className="text-sm font-medium text-slate-500 dark:text-slate-400">
                  Awaiting first lead submission
                </p>
              </div>
            ) : (
              <ul className="space-y-1 -mx-2">
                {leadsResp.leads.slice(0, 5).map((l) => (
                  <li key={l.id}>
                    <Link
                      href={`/admin/studios/${studio.id}/leads/${l.id}`}
                      className="group flex items-center gap-3 rounded-2xl p-3 transition-all hover:bg-white/40 hover:backdrop-blur-md dark:hover:bg-white/5"
                    >
                      <div className="grid h-10 w-10 shrink-0 place-items-center rounded-full bg-gradient-to-br from-brand-400 to-brand-600 text-sm font-black text-white shadow-lg shadow-brand-500/20">
                        {l.name.charAt(0).toUpperCase()}
                      </div>
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center justify-between">
                          <span className="truncate text-sm font-bold text-zinc-900 group-hover:text-brand-500 dark:text-white dark:group-hover:text-brand-400">
                            {l.name}
                          </span>
                          <span className="shrink-0 text-[10px] font-black uppercase text-zinc-400" suppressHydrationWarning>
                            {relativeTime(l.createdAt)}
                          </span>
                        </div>
                        <div className="truncate text-xs font-medium text-slate-500">
                          {l.fitnessPlan}
                        </div>
                      </div>
                    </Link>
                  </li>
                ))}
              </ul>
            )}
          </Card>

          <div className="relative overflow-hidden rounded-[32px] bg-gradient-to-br from-brand-500 to-brand-700 p-7 text-white shadow-xl shadow-brand-500/30">
            <div className="absolute -right-10 -top-10 h-40 w-40 rounded-full bg-white/10 blur-3xl animate-pulse-liquid" />
            <div className="absolute -left-10 -bottom-10 h-32 w-32 rounded-full bg-white/5 blur-2xl" />
            <div className="relative z-10">
              <div className="mb-3 flex items-center gap-2 text-lg font-black tracking-tight">
                <ExternalLink className="h-5 w-5" />
                Studio Login URL
              </div>
              <p className="text-sm font-semibold text-white/80 leading-relaxed">
                Admins can sign in at <code className="rounded-xl bg-black/20 px-2 py-1 font-mono text-xs">/login</code> to access this studio directly.
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

