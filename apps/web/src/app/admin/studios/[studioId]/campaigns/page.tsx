import React from 'react';
import Link from 'next/link';
import { Megaphone, Plus, Users, Link as LinkIcon, ExternalLink, Zap, Calendar, Facebook, Instagram, Twitter, Globe } from 'lucide-react';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { EmptyState } from '@/components/ui/EmptyState';
import { PageHeader } from '@/components/ui/PageHeader';
import { serverFetch } from '@/lib/auth';
import type { Campaign } from '@/lib/types';
import { CopyLink } from './CopyLink';
import { cn } from '@/lib/cn';
import { HeaderActions } from '@/components/HeaderActions';

interface ListResp {
  campaigns: Campaign[];
  total: number;
}

interface SocialPost {
  id: string;
  campaignName: string;
  platform: 'Facebook' | 'Instagram' | 'Google Ads' | 'X (Twitter)';
  status: 'published' | 'scheduled' | 'draft' | 'failed';
  scheduledTime: string;
}

export default async function CampaignsPage({
  params,
  searchParams,
}: {
  params: Promise<{ studioId: string }>;
  searchParams: Promise<{ page?: string }>;
}) {
  const { studioId } = await params;
  const sParams = await searchParams;
  const currentPage = Math.max(1, parseInt(sParams.page ?? '1', 10));
  const limit = 5; // 5 campaigns + 1 create card = 6 slots on page 1
  const offset = (currentPage - 1) * limit;

  const [{ campaigns, total = 0 }, socialPosts] = await Promise.all([
    serverFetch<ListResp>(`/api/v1/studios/${studioId}/campaigns?limit=${limit}&offset=${offset}`),
    serverFetch<SocialPost[]>(`/api/v1/studios/${studioId}/social-posts`).catch(() => [] as SocialPost[]),
  ]);

  // Group social posts by campaign name for quick lookup
  const postsByCampaign = (socialPosts ?? []).reduce<Record<string, SocialPost[]>>((acc, p) => {
    const key = p.campaignName ?? '';
    if (!acc[key]) acc[key] = [];
    acc[key].push(p);
    return acc;
  }, {});

  return (
    <div className="space-y-6">
      <HeaderActions>
        <Link href={`/admin/studios/${studioId}/campaigns/new`}>
          <Button
            leftIcon={<Plus className="h-4 w-4" />}
            className="bg-gradient-to-r from-teal-500 to-cyan-600 border-none text-white hover:from-teal-600 hover:to-cyan-700 shadow-lg shadow-teal-550/20"
            suppressHydrationWarning
          >
            New Campaign
          </Button>
        </Link>
      </HeaderActions>

      <div className="text-[11px] font-semibold text-zinc-400 dark:text-zinc-500 px-2">
        Each campaign generates a unique lead-capture link you can drop in an Instagram bio or ad.
      </div>

      {campaigns.length === 0 ? (
        <Card className="border border-white/30 bg-white/30 backdrop-blur-2xl dark:border-white/5 dark:bg-neutral-900/30">
          <EmptyState
            icon={<Megaphone className="h-8 w-8 text-slate-400" />}
            title="No campaigns yet"
            description="Create your first campaign to start collecting leads from a shareable link."
            action={
              <Link href={`/admin/studios/${studioId}/campaigns/new`}>
                <Button leftIcon={<Plus className="h-4 w-4" />} suppressHydrationWarning>New campaign</Button>
              </Link>
            }
          />
        </Card>
      ) : (
        <div className="space-y-6">
          <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
            {campaigns.map((c, i) => (
              <CampaignCard key={c.id} campaign={c} studioId={studioId} index={i} posts={postsByCampaign[c.name] ?? []} />
            ))}
            
            {currentPage === 1 && (
              <Link 
                href={`/admin/studios/${studioId}/campaigns/new`}
                className="group flex flex-col items-center justify-center rounded-[32px] border-2 border-dashed border-white/20 bg-white/10 p-8 transition-all hover:border-brand-500/50 hover:bg-white/20 dark:border-white/5 dark:bg-neutral-900/20 dark:hover:bg-neutral-900/30"
              >
                <div className="mb-4 grid h-16 w-16 place-items-center rounded-2xl bg-white/80 shadow-lg backdrop-blur-md transition-transform group-hover:scale-110 dark:bg-neutral-800">
                  <Plus className="h-8 w-8 text-brand-500" />
                </div>
                <span className="text-sm font-bold text-zinc-900 dark:text-zinc-100">Create New Campaign</span>
                <span className="mt-1 text-xs text-zinc-500">Add another lead magnet</span>
              </Link>
            )}
          </div>

          {total > limit && (
            <div className="flex items-center justify-between border-t border-zinc-200/50 pt-6 dark:border-zinc-800/50">
              <span className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">
                Showing {offset + 1} to {Math.min(offset + campaigns.length, total)} of {total} campaigns
              </span>
              <div className="flex gap-2">
                <Link
                  href={currentPage > 1 ? `?page=${currentPage - 1}` : '#'}
                  className={cn(
                    "inline-flex h-9 items-center justify-center rounded-xl bg-white/80 px-4 text-xs font-black shadow-sm backdrop-blur-md border border-zinc-200/50 transition-all dark:bg-zinc-900/80 dark:border-zinc-800/50",
                    currentPage === 1 ? "pointer-events-none opacity-50" : "hover:bg-zinc-100 dark:hover:bg-zinc-800"
                  )}
                >
                  Previous
                </Link>
                <Link
                  href={offset + campaigns.length < total ? `?page=${currentPage + 1}` : '#'}
                  className={cn(
                    "inline-flex h-9 items-center justify-center rounded-xl bg-white/80 px-4 text-xs font-black shadow-sm backdrop-blur-md border border-zinc-200/50 transition-all dark:bg-zinc-900/80 dark:border-zinc-800/50",
                    offset + campaigns.length >= total ? "pointer-events-none opacity-50" : "hover:bg-zinc-100 dark:hover:bg-zinc-800"
                  )}
                >
                  Next
                </Link>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

const PLATFORM_ICON: Record<string, React.ReactNode> = {
  'Facebook': <Facebook className="h-3 w-3" />,
  'Instagram': <Instagram className="h-3 w-3" />,
  'X (Twitter)': <Twitter className="h-3 w-3" />,
  'Google Ads': <Globe className="h-3 w-3" />,
};

const PLATFORM_COLOR: Record<string, string> = {
  'Facebook': 'bg-blue-500/10 text-blue-600 dark:text-blue-400',
  'Instagram': 'bg-pink-500/10 text-pink-600 dark:text-pink-400',
  'X (Twitter)': 'bg-zinc-500/10 text-zinc-600 dark:text-zinc-400',
  'Google Ads': 'bg-amber-500/10 text-amber-600 dark:text-amber-400',
};

function CampaignCard({ campaign, studioId, index, posts }: { campaign: Campaign, studioId: string, index: number, posts: SocialPost[] }) {
  const delay = `${index * 0.1}s`;
  const detailHref = `/admin/studios/${studioId}/campaigns/${campaign.id}`;

  // Unique platforms across all posts
  const platforms = [...new Set(posts.map(p => p.platform))];

  // Nearest upcoming scheduled or most recent published date
  const relevantPosts = posts.filter(p => p.status === 'scheduled' || p.status === 'published');
  const scheduledPosts = relevantPosts.filter(p => p.status === 'scheduled').sort((a, b) => new Date(a.scheduledTime).getTime() - new Date(b.scheduledTime).getTime());
  const publishedPosts = relevantPosts.filter(p => p.status === 'published').sort((a, b) => new Date(b.scheduledTime).getTime() - new Date(a.scheduledTime).getTime());
  const featuredPost = scheduledPosts[0] ?? publishedPosts[0] ?? null;

  return (
    <div
      className="group relative animate-in"
      style={{ animationDelay: delay }}
    >
      {/* Glow on hover */}
      <div className="absolute -inset-1 rounded-[36px] bg-gradient-to-br from-brand-500/20 to-sky-500/20 opacity-0 blur-xl transition duration-500 group-hover:opacity-100" />

      <Card className="relative h-full border border-white/30 bg-white/30 shadow-xl backdrop-blur-2xl dark:border-white/5 dark:bg-neutral-900/30" noPadding elevated>
        {/* ── Full-card clickable overlay ── */}
        <Link
          href={detailHref}
          className="absolute inset-0 z-10 rounded-[inherit]"
          aria-label={`Open campaign: ${campaign.name}`}
        />

        <div className="flex h-full flex-col p-8">
          <div className="mb-6 flex items-start justify-between">
            <div className="grid h-12 w-12 place-items-center rounded-2xl bg-gradient-to-br from-brand-500 to-brand-600 text-white shadow-lg shadow-brand-500/20">
              <Zap className="h-6 w-6" />
            </div>
            <Badge tone={campaign.active ? 'success' : 'neutral'} className="rounded-xl px-3 py-1 text-[10px] font-black uppercase tracking-widest shadow-sm">
              {campaign.active ? 'Active' : 'Draft'}
            </Badge>
          </div>

          <div className="mb-2">
            {/* Title — sits above overlay via z-20, still navigates (same href) */}
            <Link
              href={detailHref}
              className="relative z-20 text-xl font-black tracking-tight text-slate-900 hover:text-brand-500 dark:text-white dark:hover:text-brand-400"
            >
              {campaign.name}
            </Link>
            <div className="mt-1 flex items-center gap-1.5 font-mono text-[10px] font-bold uppercase tracking-wider text-slate-400">
              <LinkIcon className="h-3 w-3" />
              /{campaign.slug}
            </div>
          </div>

          <p className="mb-6 line-clamp-2 text-xs font-medium leading-relaxed text-slate-500 dark:text-slate-400">
            {campaign.description || "Start collecting leads with this premium capture form."}
          </p>

          <div className="mb-8 grid grid-cols-2 gap-4">
            <div className="rounded-3xl bg-white/40 p-4 backdrop-blur-md dark:bg-neutral-950/40">
              <div className="flex items-center gap-2 text-[10px] font-black uppercase tracking-wider text-zinc-400">
                <Users className="h-3.5 w-3.5" />
                Leads
              </div>
              <div className="mt-1 text-2xl font-black text-zinc-900 dark:text-white">
                {campaign.leadCount ?? 0}
              </div>
            </div>
            <div className="rounded-3xl bg-white/40 p-4 backdrop-blur-md dark:bg-neutral-950/40">
              <div className="flex items-center gap-2 text-[10px] font-black uppercase tracking-wider text-zinc-400">
                <Zap className="h-3.5 w-3.5" />
                Plans
              </div>
              <div className="mt-1 text-2xl font-black text-zinc-900 dark:text-white">
                {campaign.fitnessPlans.length}
              </div>
            </div>
          </div>

          {/* ── Social planner info ── */}
          {(platforms.length > 0 || featuredPost) && (
            <div className="mb-6 rounded-2xl bg-white/30 px-4 py-3 backdrop-blur-md dark:bg-neutral-950/30 space-y-2">
              {platforms.length > 0 && (
                <div className="flex flex-wrap gap-1.5">
                  {platforms.map(pl => (
                    <span key={pl} className={cn('inline-flex items-center gap-1 rounded-lg px-2 py-0.5 text-[10px] font-bold', PLATFORM_COLOR[pl] ?? 'bg-zinc-500/10 text-zinc-500')}>
                      {PLATFORM_ICON[pl]}
                      {pl}
                    </span>
                  ))}
                </div>
              )}
              {featuredPost && (
                <div className="flex items-center gap-1.5 text-[10px] font-semibold text-zinc-500 dark:text-zinc-400">
                  <Calendar className="h-3 w-3 shrink-0" />
                  <span className="capitalize">{featuredPost.status}</span>
                  <span>·</span>
                  <span>{new Date(featuredPost.scheduledTime).toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })}</span>
                </div>
              )}
            </div>
          )}

          {/* ── Footer: raised above overlay so buttons still work ── */}
          <div className="relative z-20 mt-auto border-t border-slate-100 pt-6 dark:border-white/5">
            <div className="flex items-center gap-3">
              <div className="min-w-0 flex-1">
                <CopyLink url={campaign.shareUrl} />
              </div>
              <Link
                href={campaign.shareUrl}
                target="_blank"
                className="grid h-9 w-9 shrink-0 place-items-center rounded-xl bg-slate-100 text-slate-500 transition-all hover:bg-slate-200 hover:text-slate-900 dark:bg-neutral-800 dark:text-zinc-400 dark:hover:bg-neutral-700 dark:hover:text-white"
                title="Preview live page"
              >
                <ExternalLink className="h-4 w-4" />
              </Link>
            </div>
          </div>
        </div>
      </Card>
    </div>
  );
}

