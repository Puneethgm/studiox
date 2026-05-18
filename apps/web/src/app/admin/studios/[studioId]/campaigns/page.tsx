import Link from 'next/link';
import { Megaphone, Plus, Users, Link as LinkIcon, ExternalLink, Zap } from 'lucide-react';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { EmptyState } from '@/components/ui/EmptyState';
import { PageHeader } from '@/components/ui/PageHeader';
import { serverFetch } from '@/lib/auth';
import type { Campaign } from '@/lib/types';
import { CopyLink } from './CopyLink';
import { cn } from '@/lib/cn';

interface ListResp {
  campaigns: Campaign[];
}

export default async function CampaignsPage({
  params,
}: {
  params: Promise<{ studioId: string }>;
}) {
  const { studioId } = await params;
  const { campaigns } = await serverFetch<ListResp>(`/api/v1/studios/${studioId}/campaigns`);

  return (
    <div className="space-y-8">
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
              <Megaphone className="h-6 w-6" />
            </div>
            <div>
              <h1 className="text-2xl font-black tracking-tight text-zinc-900 dark:text-white">Campaigns</h1>
              <p className="mt-0.5 text-xs font-semibold text-zinc-500 dark:text-zinc-400">
                Each campaign generates a unique lead-capture link you can drop in an Instagram bio or ad.
              </p>
            </div>
          </div>
          <Link href={`/admin/studios/${studioId}/campaigns/new`}>
            <Button
              leftIcon={<Plus className="h-4 w-4" />}
              className="shadow-lg shadow-brand-500/20"
              suppressHydrationWarning
            >
              New Campaign
            </Button>
          </Link>
        </div>
      </div>

      {campaigns.length === 0 ? (
        <Card className="border-none bg-white/40 backdrop-blur-xl dark:bg-slate-900/40">
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
        <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
          {campaigns.map((c, i) => (
            <CampaignCard key={c.id} campaign={c} studioId={studioId} index={i} />
          ))}
          
          <Link 
            href={`/admin/studios/${studioId}/campaigns/new`}
            className="group flex flex-col items-center justify-center rounded-[32px] border-2 border-dashed border-white/20 bg-white/10 p-8 transition-all hover:border-brand-500/50 hover:bg-white/20 dark:border-white/5 dark:bg-white/5 dark:hover:bg-white/10"
          >
            <div className="mb-4 grid h-16 w-16 place-items-center rounded-2xl bg-white/80 shadow-lg backdrop-blur-md transition-transform group-hover:scale-110 dark:bg-white/10">
              <Plus className="h-8 w-8 text-brand-500" />
            </div>
            <span className="text-sm font-bold text-zinc-900 dark:text-zinc-100">Create New Campaign</span>
            <span className="mt-1 text-xs text-zinc-500">Add another lead magnet</span>
          </Link>
        </div>
      )}
    </div>
  );
}

function CampaignCard({ campaign, studioId, index }: { campaign: Campaign, studioId: string, index: number }) {
  const delay = `${index * 0.1}s`;
  
  return (
    <div 
      className="group relative animate-in"
      style={{ animationDelay: delay }}
    >
      <div className="absolute -inset-1 rounded-[36px] bg-gradient-to-br from-brand-500/20 to-sky-500/20 opacity-0 blur-xl transition duration-500 group-hover:opacity-100" />
      
      <Card className="relative h-full border-none bg-white/40 shadow-xl backdrop-blur-2xl dark:bg-neutral-900/40" noPadding elevated>
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
            <Link 
              href={`/admin/studios/${studioId}/campaigns/${campaign.id}`}
              className="text-xl font-black tracking-tight text-slate-900 hover:text-brand-500 dark:text-white dark:hover:text-brand-400"
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
            <div className="rounded-3xl bg-white/40 p-4 backdrop-blur-md dark:bg-white/5">
              <div className="flex items-center gap-2 text-[10px] font-black uppercase tracking-wider text-zinc-400">
                <Users className="h-3.5 w-3.5" />
                Leads
              </div>
              <div className="mt-1 text-2xl font-black text-zinc-900 dark:text-white">
                {campaign.leadCount ?? 0}
              </div>
            </div>
            <div className="rounded-3xl bg-white/40 p-4 backdrop-blur-md dark:bg-white/5">
              <div className="flex items-center gap-2 text-[10px] font-black uppercase tracking-wider text-zinc-400">
                <Zap className="h-3.5 w-3.5" />
                Plans
              </div>
              <div className="mt-1 text-2xl font-black text-zinc-900 dark:text-white">
                {campaign.fitnessPlans.length}
              </div>
            </div>
          </div>

          <div className="mt-auto pt-6 border-t border-slate-100 dark:border-slate-800/60">
            <div className="flex items-center gap-3">
              <div className="min-w-0 flex-1">
                <CopyLink url={campaign.shareUrl} />
              </div>
              <Link 
                href={campaign.shareUrl} 
                target="_blank"
                className="grid h-9 w-9 shrink-0 place-items-center rounded-xl bg-slate-100 text-slate-500 transition-all hover:bg-slate-200 hover:text-slate-900 dark:bg-slate-800 dark:text-slate-400 dark:hover:bg-slate-700 dark:hover:text-white"
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
