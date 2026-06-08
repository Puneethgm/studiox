import { redirect } from 'next/navigation';
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

  // Super admins have no business in individual studio settings
  if (me.role === 'super_admin') {
    redirect('/admin/studios');
  }

  // Studio-admin only from here on
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
    <div className="space-y-4">
      <div className="text-[11px] font-semibold text-zinc-400 dark:text-zinc-500 px-2">
        Update the studio&rsquo;s name, logo, and brand color configuration.
      </div>
      <SettingsForm studio={studio} previewHref={previewHref} initialPlans={plans} />
    </div>
  );
}

