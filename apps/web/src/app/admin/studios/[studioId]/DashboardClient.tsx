'use client';

import { useState, useEffect } from 'react';
import Link from 'next/link';
import {
  ExternalLink,
  Inbox,
  Megaphone,
  Plus,
  Settings as SettingsIcon,
  ArrowRight,
  Activity,
  Users,
  Star,
  Clock,
  TrendingUp,
  DollarSign,
  HelpCircle,
  TrendingDown,
  Info,
  Calendar,
} from 'lucide-react';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { EmptyState } from '@/components/ui/EmptyState';
import { StatCard } from '@/components/ui/StatCard';
import { FunnelStrip } from '@/components/widgets/FunnelStrip';
import { StatusDonut } from '@/components/widgets/StatusDonut';
import { brandInitials } from '@/lib/color';
import { relativeTime } from '@/lib/datetime';
import type { Campaign, Lead, LeadStatus, Studio, AnalyticsSummary } from '@/lib/types';
import { api } from '@/lib/api';

interface LeadStats {
  total: number;
  byStatus: Record<LeadStatus, number>;
}

interface DashboardClientProps {
  studio: Studio;
  campaigns: Campaign[];
  initialLeads: Lead[];
  initialLeadsTotal: number;
  initialStats: LeadStats;
}

const statusTone = {
  new: 'info',
  contacted: 'brand',
  trial_booked: 'warning',
  member: 'success',
  dropped: 'neutral',
} as const;

function formatConversionHealth(totalLeads: number, trialBookedLeads: number) {
  if (totalLeads <= 0) {
    return '0%';
  }
  return `${Math.round((trialBookedLeads / totalLeads) * 100)}%`;
}

function formatDuration(seconds: number): string {
  if (seconds <= 0) return '0m';
  const mins = Math.round(seconds / 60);
  if (mins < 60) return `${mins}m`;
  const hours = mins / 60;
  if (hours < 24) return `${hours.toFixed(1)}h`;
  const days = hours / 24;
  return `${days.toFixed(1)}d`;
}

export default function DashboardClient({
  studio,
  campaigns,
  initialLeads,
  initialLeadsTotal,
  initialStats,
}: DashboardClientProps) {
  const [activeTab, setActiveTab] = useState<'overview' | 'analytics'>('overview');
  const [duration, setDuration] = useState<'1d' | '7d' | '15d' | '30d' | '90d' | '365d' | 'all' | 'custom'>('30d');
  const [startDate, setStartDate] = useState<string>('');
  const [endDate, setEndDate] = useState<string>('');
  const [analytics, setAnalytics] = useState<AnalyticsSummary | null>(null);
  const [loadingAnalytics, setLoadingAnalytics] = useState(false);
  const [logoError, setLogoError] = useState(false);

  useEffect(() => {
    setLogoError(false);
  }, [studio.logoUrl]);

  // ROI Calculator inputs
  const [adSpend, setAdSpend] = useState<number>(500);
  const [memberLtv, setMemberLtv] = useState<number>(150);

  // Fetch analytics when tab, duration, or custom dates change
  useEffect(() => {
    if (activeTab !== 'analytics') return;
    if (duration === 'custom' && (!startDate || !endDate)) return;

    async function loadAnalytics() {
      setLoadingAnalytics(true);
      try {
        let url = `/api/v1/studios/${studio.id}/analytics?duration=${duration}`;
        if (duration === 'custom') {
          url = `/api/v1/studios/${studio.id}/analytics?startDate=${startDate}&endDate=${endDate}`;
        }
        const data = await api<AnalyticsSummary>(url);
        setAnalytics(data);
      } catch (err) {
        console.error('Failed to load analytics', err);
      } finally {
        setLoadingAnalytics(false);
      }
    }
    loadAnalytics();
  }, [activeTab, duration, startDate, endDate, studio.id]);

  const activeCampaigns = campaigns.filter((c) => c.active).length;
  const totalLeads = initialStats.total;
  const newLeads = initialStats.byStatus.new ?? 0;
  const trialBookedLeads = initialStats.byStatus.trial_booked ?? 0;
  const memberLeads = initialStats.byStatus.member ?? 0;
  const conversionHealth = formatConversionHealth(totalLeads, trialBookedLeads + memberLeads);

  // ROI calculation based on platform analytics
  const paidPlatformData = analytics?.byPlatform?.find(
    (p) => p.platform === 'Paid Ads',
  );
  const paidLeads = paidPlatformData?.totalLeads ?? 0;
  const paidConversions = paidPlatformData?.convertedLeads ?? 0;
  const costPerLead = paidLeads > 0 ? adSpend / paidLeads : 0;
  const costPerAcquisition = paidConversions > 0 ? adSpend / paidConversions : 0;
  const revenueGenerated = paidConversions * memberLtv;
  const netProfit = revenueGenerated - adSpend;
  const roi = adSpend > 0 ? (netProfit / adSpend) * 100 : 0;

  return (
    <div className="space-y-8 pb-12">
      {/* Studio Header */}
      <div
        className="relative overflow-hidden rounded-[28px] p-0 backdrop-blur-2xl"
        style={{
          background: `linear-gradient(135deg, ${studio.brandColor}22 0%, ${studio.brandColor}10 50%, rgba(219,234,254,0.18) 100%)`,
          border: `1px solid ${studio.brandColor}30`,
          boxShadow: `0 8px 40px ${studio.brandColor}15, inset 0 0 0 1px rgba(255,255,255,0.20)`,
        }}
      >
        <div
          className="h-1.5 w-full"
          style={{
            background: `linear-gradient(90deg, ${studio.brandColor} 0%, ${studio.brandColor}99 40%, rgba(99,102,241,0.7) 70%, ${studio.brandColor}50 100%)`,
          }}
        />

        <div
          className="pointer-events-none absolute -right-14 -top-14 h-52 w-52 rounded-full blur-[70px]"
          style={{ background: `${studio.brandColor}25` }}
        />
        <div
          className="pointer-events-none absolute -bottom-10 -left-10 h-40 w-40 rounded-full blur-[60px]"
          style={{ background: `${studio.brandColor}15` }}
        />

        <div className="relative flex flex-col gap-5 p-6 md:flex-row md:items-center md:justify-between">
          <div className="flex items-center gap-5">
            <div className="relative">
              <div
                className="grid h-[68px] w-[68px] shrink-0 place-items-center overflow-hidden rounded-[22px] text-xl font-black text-white shadow-2xl transition-transform duration-500 hover:scale-105 hover:rotate-2"
                style={{
                  background: studio.brandColor,
                  boxShadow: `0 8px 24px ${studio.brandColor}45, 0 0 0 4px ${studio.brandColor}20, inset 0 1px 0 rgba(255,255,255,0.25)`,
                }}
              >
                {studio.logoUrl && !logoError ? (
                  // eslint-disable-next-line @next/next/no-img-element
                  <img src={studio.logoUrl} alt="" className="h-full w-full object-cover" onError={() => setLogoError(true)} />
                ) : (
                  <span className="text-white drop-shadow">{brandInitials(studio.name)}</span>
                )}
                <div className="pointer-events-none absolute inset-0 bg-gradient-to-br from-white/20 to-transparent rounded-[22px]" />
              </div>
              {studio.active && (
                <span className="absolute -bottom-1 -right-1 flex h-4 w-4 items-center justify-center rounded-full bg-emerald-500 ring-2 ring-white dark:ring-neutral-900">
                  <span className="h-2 w-2 animate-ping rounded-full bg-emerald-300 opacity-75" />
                </span>
              )}
            </div>

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

      {/* Tabs Selector */}
      <div className="flex border-b border-zinc-200 dark:border-zinc-800">
        <button
          onClick={() => setActiveTab('overview')}
          className={`px-6 py-3 font-bold text-sm border-b-2 transition-all ${
            activeTab === 'overview'
              ? 'border-brand-500 text-brand-500 dark:text-brand-400'
              : 'border-transparent text-zinc-500 hover:text-zinc-800 dark:hover:text-zinc-200'
          }`}
          style={activeTab === 'overview' ? { borderColor: studio.brandColor, color: studio.brandColor } : {}}
        >
          Overview
        </button>
        <button
          onClick={() => setActiveTab('analytics')}
          className={`px-6 py-3 font-bold text-sm border-b-2 transition-all flex items-center gap-1.5 ${
            activeTab === 'analytics'
              ? 'border-brand-500 text-brand-500 dark:text-brand-400'
              : 'border-transparent text-zinc-500 hover:text-zinc-800 dark:hover:text-zinc-200'
          }`}
          style={activeTab === 'analytics' ? { borderColor: studio.brandColor, color: studio.brandColor } : {}}
        >
          <Activity className="h-4 w-4" />
          Detailed Analytics
        </button>
      </div>

      {/* Tab Contents: Overview */}
      {activeTab === 'overview' && (
        <>
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
              value={conversionHealth}
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
                byStatus={initialStats.byStatus}
                total={initialStats.total}
                studioId={studio.id}
              />
            </div>
            <div className="lg:col-span-2 rounded-[22px] border border-sky-200/40 bg-sky-50/30 p-5 backdrop-blur-2xl dark:border-sky-500/10 dark:bg-sky-950/15" style={{ boxShadow: '0 4px 20px rgba(14,165,233,0.07), inset 0 0 0 1px rgba(186,230,253,0.3)' }}>
              <h3 className="mb-4 text-[10px] font-black uppercase tracking-[0.18em] text-zinc-400">Lead Distribution</h3>
              <StatusDonut
                byStatus={initialStats.byStatus}
                total={initialStats.total}
              />
            </div>
          </div>

          {/* Bottom Section */}
          <div className="grid gap-6 lg:grid-cols-3">
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

            <div className="flex flex-col gap-6">
              <Card
                title={<span className="text-sm font-black uppercase tracking-[0.15em] text-zinc-400">Latest Activity</span>}
                action={
                  initialLeads.length > 0 && (
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
                {initialLeads.length === 0 ? (
                  <div className="flex h-full flex-col items-center justify-center text-center py-8">
                    <Inbox className="h-10 w-10 text-slate-300 dark:text-slate-700 mb-4" />
                    <p className="text-sm font-medium text-slate-500 dark:text-slate-400">
                      Awaiting first lead submission
                    </p>
                  </div>
                ) : (
                  <ul className="space-y-1 -mx-2">
                    {initialLeads.slice(0, 5).map((l) => (
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
        </>
      )}

      {/* Tab Contents: Detailed Analytics */}
      {activeTab === 'analytics' && (
        <div className="space-y-6">
          {/* Filters & Loading Indicator */}
          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
            <div className="flex items-center gap-2">
              <span className="text-xs font-black uppercase tracking-widest text-zinc-400 flex items-center gap-1">
                <Calendar className="h-4 w-4" />
                Select Period:
              </span>
              <div className="inline-flex rounded-xl bg-zinc-100 p-1 dark:bg-zinc-800 flex-wrap gap-1">
                {(['1d', '7d', '15d', '30d', '90d', '365d', 'all', 'custom'] as const).map((d) => (
                  <button
                    key={d}
                    onClick={() => setDuration(d)}
                    className={`rounded-lg px-3 py-1 text-xs font-bold transition-all ${
                      duration === d
                        ? 'bg-white text-zinc-900 shadow dark:bg-zinc-700 dark:text-white'
                        : 'text-zinc-500 hover:text-zinc-800 dark:hover:text-zinc-300'
                    }`}
                  >
                    {d === '1d' && 'Today'}
                    {d === '7d' && '1 Week'}
                    {d === '15d' && '15 Days'}
                    {d === '30d' && '1 Month'}
                    {d === '90d' && '1 Quarter'}
                    {d === '365d' && '1 Year'}
                    {d === 'all' && 'All Time'}
                    {d === 'custom' && 'Custom Date'}
                  </button>
                ))}
              </div>
            </div>
            {duration === 'custom' && (
              <div className="flex items-center gap-3 animate-in fade-in slide-in-from-left-2 duration-300">
                <div className="flex items-center gap-1.5">
                  <span className="text-xs font-bold text-zinc-400">From:</span>
                  <input
                    type="date"
                    value={startDate}
                    onChange={(e) => setStartDate(e.target.value)}
                    className="rounded-xl border border-zinc-200 bg-white px-3 py-1.5 text-xs font-bold text-zinc-800 shadow-sm focus:border-brand-500 focus:outline-none dark:border-zinc-700 dark:bg-zinc-800 dark:text-white"
                  />
                </div>
                <div className="flex items-center gap-1.5">
                  <span className="text-xs font-bold text-zinc-400">To:</span>
                  <input
                    type="date"
                    value={endDate}
                    onChange={(e) => setEndDate(e.target.value)}
                    className="rounded-xl border border-zinc-200 bg-white px-3 py-1.5 text-xs font-bold text-zinc-800 shadow-sm focus:border-brand-500 focus:outline-none dark:border-zinc-700 dark:bg-zinc-800 dark:text-white"
                  />
                </div>
              </div>
            )}
            {loadingAnalytics && (
              <span className="text-xs text-zinc-500 flex items-center gap-1">
                <span className="h-3 w-3 animate-spin rounded-full border-2 border-zinc-400 border-t-transparent" />
                Recalculating real-time stats...
              </span>
            )}
          </div>

          {/* Lead vs Pipeline Explanation Card */}
          <div className="rounded-[24px] border border-blue-200 bg-blue-50/50 p-5 dark:border-blue-900/30 dark:bg-blue-950/20">
            <div className="flex gap-3">
              <div className="grid h-10 w-10 shrink-0 place-items-center rounded-xl bg-blue-500/10 text-blue-500">
                <Info className="h-5 w-5" />
              </div>
              <div>
                <h4 className="text-sm font-black text-blue-950 dark:text-blue-200">
                  Understanding Your Dashboard: Lead vs Pipeline
                </h4>
                <p className="mt-1 text-xs text-blue-800 dark:text-blue-300 leading-relaxed font-semibold">
                  A <strong>Lead</strong> is any prospective student captured in the system. The <strong>Pipeline</strong> maps the stages they advance through (<em>New &rarr; Contacted &rarr; Trial Booked &rarr; Member</em>). This dashboard calculates how fast prospects transition through each stage, enabling you to optimize conversions.
                </p>
              </div>
            </div>
          </div>

          {analytics ? (
            <>
              {/* Detailed Metrics Grid */}
              <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                <Card
                  title={<span className="text-[10px] font-black uppercase tracking-[0.18em] text-zinc-400">Conversion (Trial &rarr; Member)</span>}
                  className="bg-emerald-50/20 border-emerald-100 dark:border-emerald-950/20 dark:bg-emerald-950/5"
                >
                  <div className="flex items-baseline gap-2">
                    <span className="text-3xl font-black text-zinc-900 dark:text-white">
                      {analytics.trialToMemberRate.toFixed(1)}%
                    </span>
                    <TrendingUp className="h-5 w-5 text-emerald-500" />
                  </div>
                  <p className="mt-2 text-xs font-semibold text-zinc-500 dark:text-zinc-400">
                    Percentage of booked trial leads that converted to full-paying members.
                  </p>
                </Card>

                <Card
                  title={<span className="text-[10px] font-black uppercase tracking-[0.18em] text-zinc-400">Open Follow-ups</span>}
                  className="bg-amber-50/20 border-amber-100 dark:border-amber-950/20 dark:bg-amber-950/5"
                >
                  <div className="flex items-baseline gap-2">
                    <span className="text-3xl font-black text-zinc-900 dark:text-white">
                      {analytics.followupsRequired}
                    </span>
                    <Badge tone={analytics.followupsRequired > 0 ? 'warning' : 'neutral'} className="font-bold">
                      {analytics.followupsRequired > 0 ? 'Action Needed' : 'Good'}
                    </Badge>
                  </div>
                  <p className="mt-2 text-xs font-semibold text-zinc-500 dark:text-zinc-400">
                    New or contacted leads with fewer than 3 contact attempts.
                  </p>
                </Card>

                <Card
                  title={<span className="text-[10px] font-black uppercase tracking-[0.18em] text-zinc-400">Unresponded Conversations</span>}
                  className="bg-rose-50/20 border-rose-100 dark:border-rose-950/20 dark:bg-rose-950/5"
                >
                  <div className="flex items-baseline gap-2">
                    <span className="text-3xl font-black text-zinc-900 dark:text-white">
                      {analytics.unrespondedMessages}
                    </span>
                    <Badge tone={analytics.unrespondedMessages > 0 ? 'danger' : 'neutral'} className="font-bold">
                      {analytics.unrespondedMessages > 0 ? 'Awaiting Reply' : 'All Clear'}
                    </Badge>
                  </div>
                  <p className="mt-2 text-xs font-semibold text-zinc-500 dark:text-zinc-400">
                    Open customer conversations where the last message was inbound from client.
                  </p>
                </Card>
                 <Card
                  title={<span className="text-[10px] font-black uppercase tracking-[0.18em] text-zinc-400">Avg Response Time</span>}
                  className="bg-blue-50/20 border-blue-100 dark:border-blue-950/20 dark:bg-blue-950/5"
                >
                  <div className="flex items-baseline gap-2">
                    <span className="text-3xl font-black text-zinc-900 dark:text-white">
                      {analytics.avgResponseTimeLapseSecs > 0
                        ? formatDuration(analytics.avgResponseTimeLapseSecs)
                        : '--'}
                    </span>
                    <Clock className="h-5 w-5 text-blue-500" />
                    {analytics.avgResponseTimeLapseSecs <= 0 && (
                      <span className="text-[10px] text-zinc-400 font-semibold">(Benchmark: 1.2h)</span>
                    )}
                  </div>
                  <p className="mt-2 text-xs font-semibold text-zinc-500 dark:text-zinc-400">
                    Average delay between customer message received and studio's reply.
                  </p>
                </Card>

                <Card
                  title={<span className="text-[10px] font-black uppercase tracking-[0.18em] text-zinc-400">Lead &rarr; Trial Duration</span>}
                  className="bg-violet-50/20 border-violet-100 dark:border-violet-950/20 dark:bg-violet-950/5"
                >
                  <div className="flex items-baseline gap-2">
                    <span className="text-3xl font-black text-zinc-900 dark:text-white">
                      {analytics.leadToTrialTimeLapseSecs > 0
                        ? formatDuration(analytics.leadToTrialTimeLapseSecs)
                        : '--'}
                    </span>
                    <Activity className="h-5 w-5 text-violet-500" />
                    {analytics.leadToTrialTimeLapseSecs <= 0 && (
                      <span className="text-[10px] text-zinc-400 font-semibold">(Benchmark: 2.1d)</span>
                    )}
                  </div>
                  <p className="mt-2 text-xs font-semibold text-zinc-500 dark:text-zinc-400">
                    Average time from lead submission to booking their first trial class.
                  </p>
                </Card>

                <Card
                  title={<span className="text-[10px] font-black uppercase tracking-[0.18em] text-zinc-400">Trial &rarr; Member Duration</span>}
                  className="bg-purple-50/20 border-purple-100 dark:border-purple-950/20 dark:bg-purple-950/5"
                >
                  <div className="flex items-baseline gap-2">
                    <span className="text-3xl font-black text-zinc-900 dark:text-white">
                      {analytics.trialToMemberTimeLapseSecs > 0
                        ? formatDuration(analytics.trialToMemberTimeLapseSecs)
                        : '--'}
                    </span>
                    <Star className="h-5 w-5 text-purple-500" />
                    {analytics.trialToMemberTimeLapseSecs <= 0 && (
                      <span className="text-[10px] text-zinc-400 font-semibold">(Benchmark: 6.5d)</span>
                    )}
                  </div>
                  <p className="mt-2 text-xs font-semibold text-zinc-500 dark:text-zinc-400">
                    Average time from trial booking to fully upgrading to a membership plan.
                  </p>
                </Card>
              </div>

              {/* Campaigns & Platform breakdowns */}
              <div className="grid gap-6 lg:grid-cols-2">
                {/* Campaigns Conversion table */}
                <Card
                  title={<span className="text-sm font-black uppercase tracking-[0.15em] text-zinc-400">Campaign Conversions</span>}
                  noPadding
                >
                  <div className="overflow-x-auto overflow-y-auto max-h-[300px] relative scrollbar-thin">
                    <table className="w-full text-left text-xs">
                      <thead className="sticky top-0 bg-zinc-50 text-[10px] font-black uppercase tracking-widest text-zinc-500 dark:bg-zinc-900 z-10">
                        <tr>
                          <th className="px-6 py-4">Campaign Name</th>
                          <th className="px-6 py-4">Total Leads</th>
                          <th className="px-6 py-4">Conversions</th>
                          <th className="px-6 py-4 text-right">Conv. Rate</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-zinc-100 dark:divide-zinc-800">
                        {analytics.byCampaign.length === 0 ? (
                          <tr>
                            <td colSpan={4} className="px-6 py-12 text-center text-zinc-400">
                              No active campaign data for this period
                            </td>
                          </tr>
                        ) : (
                          analytics.byCampaign.map((c) => (
                            <tr key={c.id} className="hover:bg-zinc-50/50 dark:hover:bg-zinc-800/30">
                              <td className="px-6 py-4">
                                <Link
                                  href={`/admin/studios/${studio.id}/campaigns/${c.id}`}
                                  className="font-bold text-zinc-900 hover:text-brand-500 dark:text-white"
                                >
                                  {c.name}
                                </Link>
                                <span className="block mt-0.5 text-[10px] font-mono text-zinc-400">
                                  /{c.slug}
                                </span>
                              </td>
                              <td className="px-6 py-4 font-bold text-zinc-700 dark:text-zinc-300">
                                {c.totalLeads}
                              </td>
                              <td className="px-6 py-4 font-bold text-zinc-700 dark:text-zinc-300">
                                {c.convertedLeads}
                              </td>
                              <td className="px-6 py-4 text-right font-black text-emerald-600 dark:text-emerald-400">
                                {c.conversionRate.toFixed(1)}%
                              </td>
                            </tr>
                          ))
                        )}
                      </tbody>
                    </table>
                  </div>
                </Card>

                {/* Platforms breakdown list */}
                <Card
                  title={<span className="text-sm font-black uppercase tracking-[0.15em] text-zinc-400">Acquisition Channels</span>}
                  noPadding
                >
                  <div className="overflow-x-auto overflow-y-auto max-h-[300px] relative scrollbar-thin">
                    <table className="w-full text-left text-xs">
                      <thead className="sticky top-0 bg-zinc-50 text-[10px] font-black uppercase tracking-widest text-zinc-500 dark:bg-zinc-900 z-10">
                        <tr>
                          <th className="px-6 py-4">Channel / Platform</th>
                          <th className="px-6 py-4">Total Leads</th>
                          <th className="px-6 py-4">Conversions</th>
                          <th className="px-6 py-4 text-right">Conv. Rate</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-zinc-100 dark:divide-zinc-800">
                        {analytics.byPlatform.length === 0 ? (
                          <tr>
                            <td colSpan={4} className="px-6 py-12 text-center text-zinc-400">
                              No platform referral data found
                            </td>
                          </tr>
                        ) : (
                          analytics.byPlatform.map((p) => (
                            <tr key={p.platform} className="hover:bg-zinc-50/50 dark:hover:bg-zinc-800/30">
                              <td className="px-6 py-4">
                                <span className="font-bold text-zinc-900 dark:text-white">
                                  {p.platform}
                                </span>
                              </td>
                              <td className="px-6 py-4 font-bold text-zinc-700 dark:text-zinc-300">
                                {p.totalLeads}
                              </td>
                              <td className="px-6 py-4 font-bold text-zinc-700 dark:text-zinc-300">
                                {p.convertedLeads}
                              </td>
                              <td className="px-6 py-4 text-right font-black text-indigo-600 dark:text-indigo-400">
                                {p.conversionRate.toFixed(1)}%
                              </td>
                            </tr>
                          ))
                        )}
                      </tbody>
                    </table>
                  </div>
                </Card>
              </div>

              {/* Paid Platforms ROI Calculator */}
              <Card
                title={
                  <div className="flex items-center gap-2">
                    <DollarSign className="h-5 w-5 text-zinc-400" />
                    <span>Paid Marketing ROI Analysis</span>
                  </div>
                }
                subtitle="Estimate campaign revenue yields and calculate ROI metrics for 'Paid Ads'."
                className="bg-indigo-50/10 border-indigo-200/50 dark:border-indigo-900/30 dark:bg-indigo-950/5"
              >
                <div className="grid gap-6 md:grid-cols-2">
                  {/* Left: inputs */}
                  <div className="space-y-5">
                    <div>
                      <div className="flex justify-between items-center mb-1">
                        <label className="text-xs font-bold text-zinc-700 dark:text-zinc-300">
                          Monthly Paid Ad Spend ($)
                        </label>
                        <span className="text-sm font-black text-indigo-600 dark:text-indigo-400 font-mono">
                          ${adSpend.toLocaleString()}
                        </span>
                      </div>
                      <input
                        type="range"
                        min="0"
                        max="5000"
                        step="100"
                        value={adSpend}
                        onChange={(e) => setAdSpend(parseInt(e.target.value, 10))}
                        className="w-full accent-indigo-600"
                      />
                      <div className="flex justify-between text-[10px] text-zinc-400 font-bold mt-0.5">
                        <span>$0</span>
                        <span>$2,500</span>
                        <span>$5,000</span>
                      </div>
                    </div>

                    <div>
                      <div className="flex justify-between items-center mb-1">
                        <label className="text-xs font-bold text-zinc-700 dark:text-zinc-300">
                          Average Member Lifetime Value ($)
                        </label>
                        <span className="text-sm font-black text-indigo-600 dark:text-indigo-400 font-mono">
                          ${memberLtv.toLocaleString()}
                        </span>
                      </div>
                      <input
                        type="range"
                        min="50"
                        max="1000"
                        step="10"
                        value={memberLtv}
                        onChange={(e) => setMemberLtv(parseInt(e.target.value, 10))}
                        className="w-full accent-indigo-600"
                      />
                      <div className="flex justify-between text-[10px] text-zinc-400 font-bold mt-0.5">
                        <span>$50</span>
                        <span>$500</span>
                        <span>$1,000</span>
                      </div>
                    </div>
                  </div>

                  {/* Right: outputs */}
                  <div className="rounded-2xl bg-zinc-50 p-5 dark:bg-zinc-800/40 border border-zinc-200/50 dark:border-zinc-800">
                    <div className="grid grid-cols-2 gap-4 text-center">
                      <div className="p-3 bg-white rounded-xl shadow-sm dark:bg-zinc-800">
                        <div className="text-[10px] font-black uppercase text-zinc-400 tracking-wider">
                          Paid Leads
                        </div>
                        <div className="text-xl font-black mt-1 text-zinc-900 dark:text-white">
                          {paidLeads}
                        </div>
                      </div>

                      <div className="p-3 bg-white rounded-xl shadow-sm dark:bg-zinc-800">
                        <div className="text-[10px] font-black uppercase text-zinc-400 tracking-wider">
                          Paid Converts
                        </div>
                        <div className="text-xl font-black mt-1 text-zinc-900 dark:text-white">
                          {paidConversions}
                        </div>
                      </div>

                      <div className="p-3 bg-white rounded-xl shadow-sm dark:bg-zinc-800">
                        <div className="text-[10px] font-black uppercase text-zinc-400 tracking-wider">
                          Cost Per Lead
                        </div>
                        <div className="text-xl font-black mt-1 text-zinc-900 dark:text-white font-mono">
                          ${costPerLead.toFixed(1)}
                        </div>
                      </div>

                      <div className="p-3 bg-white rounded-xl shadow-sm dark:bg-zinc-800">
                        <div className="text-[10px] font-black uppercase text-zinc-400 tracking-wider">
                          Cost Per Member
                        </div>
                        <div className="text-xl font-black mt-1 text-zinc-900 dark:text-white font-mono">
                          ${costPerAcquisition.toFixed(1)}
                        </div>
                      </div>
                    </div>

                    <div className="mt-5 pt-4 border-t border-zinc-200 dark:border-zinc-700 flex justify-between items-center">
                      <div>
                        <div className="text-xs font-bold text-zinc-500">
                          Estimated Return
                        </div>
                        <div className="text-2xl font-black text-indigo-600 dark:text-indigo-400 font-mono mt-0.5">
                          ${revenueGenerated.toLocaleString()}
                        </div>
                      </div>

                      <div className="text-right">
                        <div className="text-xs font-bold text-zinc-500">
                          Net Campaign ROI
                        </div>
                        <div className="flex items-center gap-1 justify-end mt-0.5">
                          {roi >= 0 ? (
                            <span className="text-2xl font-black text-emerald-600 dark:text-emerald-400 font-mono">
                              +{roi.toFixed(0)}%
                            </span>
                          ) : (
                            <span className="text-2xl font-black text-rose-600 dark:text-rose-400 font-mono">
                              {roi.toFixed(0)}%
                            </span>
                          )}
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </Card>
            </>
          ) : (
            <div className="py-12 text-center text-zinc-400">
              No analytics data compiled for this studio.
            </div>
          )}
        </div>
      )}
    </div>
  );
}
