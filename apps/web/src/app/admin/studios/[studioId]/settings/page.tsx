import { Settings } from 'lucide-react';
import { serverFetch } from '@/lib/auth';
import type { Campaign, Studio, Plan } from '@/lib/types';
import { SettingsForm } from './SettingsForm';

export const dynamic = 'force-dynamic';

export default async function SettingsPage({
  params,
}: {
  params: Promise<{ studioId: string }>;
}) {
  const { studioId } = await params;
  // Use the /me/studios/{id} endpoint so studio_admins can also load it.
  const studio = await serverFetch<Studio>(`/api/v1/me/studios/${studioId}`);
  const campaignsResp = await serverFetch<{ campaigns: Campaign[] }>(`/api/v1/studios/${studioId}/campaigns`);
  const previewCampaign = campaignsResp.campaigns.find((campaign) => campaign.active) ?? campaignsResp.campaigns[0] ?? null;
  const previewHref = previewCampaign ? `/l/${studio.slug}/${previewCampaign.slug}` : null;
  
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
