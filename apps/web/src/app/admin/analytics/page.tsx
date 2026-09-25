import { serverFetch } from '@/lib/auth';
import type { Campaign, Lead, Studio, LeadStatus } from '@/lib/types';
import DashboardClient from '../studios/[studioId]/DashboardClient';
import { StudioSelector } from './StudioSelector';

interface LeadStats {
  total: number;
  byStatus: Record<LeadStatus, number>;
}

export default async function GlobalAnalyticsPage({
  searchParams,
}: {
  searchParams: Promise<{ studioId?: string }>;
}) {
  const { studioId } = await searchParams;

  const [{ studios }, campResp, leadsResp, stats, studio] = await Promise.all([
    serverFetch<{ studios: Studio[] }>(`/api/v1/admin/studios`),
    studioId
      ? serverFetch<{ campaigns: Campaign[] }>(`/api/v1/studios/${studioId}/campaigns`)
      : serverFetch<{ campaigns: Campaign[] }>(`/api/v1/admin/campaigns`),
    studioId
      ? serverFetch<{ leads: Lead[]; total: number }>(`/api/v1/studios/${studioId}/leads?limit=5`)
      : serverFetch<{ leads: Lead[]; total: number }>(`/api/v1/admin/leads?limit=5`),
    studioId
      ? serverFetch<LeadStats>(`/api/v1/studios/${studioId}/leads/stats`)
      : serverFetch<LeadStats>(`/api/v1/admin/leads/stats`),
    studioId ? serverFetch<Studio>(`/api/v1/admin/studios/${studioId}`) : Promise.resolve(null),
  ]);

  const mockGlobalStudio: Studio = {
    id: 'global',
    name: 'All Locations (Global)',
    slug: 'global',
    brandColor: '#6366f1', // sleek Indigo primary
    contactEmail: 'admin@studiox.com',
    active: true,
    logoUrl: '',
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  };

  const mappedStats = {
    total: stats.total,
    byStatus: {
      new: stats.byStatus?.new ?? 0,
      contacted: stats.byStatus?.contacted ?? 0,
      trial_booked: stats.byStatus?.trial_booked ?? 0,
      member: stats.byStatus?.member ?? 0,
      dropped: stats.byStatus?.dropped ?? 0,
      paused: stats.byStatus?.paused ?? 0,
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold tracking-tight text-white">Global Platform Analytics</h1>
        <StudioSelector studios={studios} selectedId={studioId ?? ''} />
      </div>
      <DashboardClient
        studio={studio ?? mockGlobalStudio}
        campaigns={campResp.campaigns}
        initialLeads={leadsResp.leads}
        initialLeadsTotal={leadsResp.total}
        initialStats={mappedStats}
        defaultTab="analytics"
      />
    </div>
  );
}
