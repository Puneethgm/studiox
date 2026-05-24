import { serverFetch } from '@/lib/auth';
import type { Campaign, Lead, Studio, LeadStatus } from '@/lib/types';
import DashboardClient from './DashboardClient';

interface LeadStats {
  total: number;
  byStatus: Record<LeadStatus, number>;
}

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

  // Map to the shape expected by DashboardClient
  const mappedStats = {
    total: stats.total,
    byStatus: {
      new: stats.byStatus.new ?? 0,
      contacted: stats.byStatus.contacted ?? 0,
      trial_booked: stats.byStatus.trial_booked ?? 0,
      member: stats.byStatus.member ?? 0,
      dropped: stats.byStatus.dropped ?? 0,
    }
  };

  return (
    <DashboardClient
      studio={studio}
      campaigns={campResp.campaigns}
      initialLeads={leadsResp.leads}
      initialLeadsTotal={leadsResp.total}
      initialStats={mappedStats}
    />
  );
}


