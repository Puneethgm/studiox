import Link from 'next/link';
import { LayoutTemplate, ExternalLink } from 'lucide-react';
import { requireSession, serverFetch } from '@/lib/auth';
import type { Campaign, Studio, Plan } from '@/lib/types';
import { SettingsForm } from './SettingsForm';

export const dynamic = 'force-dynamic';

export default async function SettingsPage({
  params,
}: {
  params: Promise<{ studioId: string }>;
}) {
  const { studioId } = await params;
  const me = await requireSession();

  const studio = await serverFetch<Studio>(`/api/v1/me/studios/${studioId}`);

  let previewHref: string | null = null;
  try {
    const campaignsResp = await serverFetch<{ campaigns: Campaign[] }>(`/api/v1/studios/${studioId}/campaigns`);
    const previewCampaign = campaignsResp.campaigns.find((c) => c.active) ?? campaignsResp.campaigns[0] ?? null;
    previewHref = previewCampaign ? `/l/${studio.slug}/${previewCampaign.slug}` : null;
  } catch (e) {
    console.error('Failed to fetch preview campaign:', e);
  }

  const plansResp = await serverFetch<{ plans: Plan[] }>(`/api/v1/me/studios/${studioId}/plans`);
  const plans = plansResp.plans || [];

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 border-b border-zinc-200 pb-5 dark:border-zinc-800 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 className="text-lg font-black text-zinc-900 dark:text-white">{studio.name} Settings</h1>
          <p className="mt-1 text-xs font-medium text-zinc-400 dark:text-zinc-500">
            Studio identity, plans, integrations, AI Assistant, and everything the public pages read from.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {previewHref && (
            <a
              href={previewHref}
              target="_blank"
              rel="noreferrer"
              className="inline-flex items-center gap-2 rounded-xl border border-zinc-200 bg-white px-3.5 py-2 text-xs font-bold text-zinc-600 transition-colors hover:bg-zinc-50 dark:border-zinc-800 dark:bg-zinc-950 dark:text-zinc-300 dark:hover:bg-zinc-900"
            >
              <ExternalLink className="h-3.5 w-3.5" />
              Preview Public Page
            </a>
          )}
          <Link
            href={`/admin/studios/${studioId}/settings/trial-page`}
            className="inline-flex items-center gap-2 rounded-xl border border-zinc-200 bg-white px-3.5 py-2 text-xs font-bold text-zinc-600 transition-colors hover:bg-zinc-50 dark:border-zinc-800 dark:bg-zinc-950 dark:text-zinc-300 dark:hover:bg-zinc-900"
          >
            <LayoutTemplate className="h-3.5 w-3.5" />
            Customize Trial Payment Page
          </Link>
        </div>
      </div>
      <SettingsForm studio={studio} previewHref={previewHref} initialPlans={plans} />
    </div>
  );
}

