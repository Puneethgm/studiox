import { serverFetch } from '@/lib/auth';
import type { Campaign, Lead, Studio, LeadStatus } from '@/lib/types';
import DashboardClient from '../studios/[studioId]/DashboardClient';

interface LeadStats {
  total: number;
  byStatus: Record<LeadStatus, number>;
}

export default async function GlobalAnalyticsPage() {
  const [campResp, leadsResp, stats] = await Promise.all([
    serverFetch<{ campaigns: Campaign[] }>(`/api/v1/admin/campaigns`),
    serverFetch<{ leads: Lead[]; total: number }>(`/api/v1/admin/leads?limit=5`),
    serverFetch<LeadStats>(`/api/v1/admin/leads/stats`),
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
      </div>
      <DashboardClient
        studio={mockGlobalStudio}
        campaigns={campResp.campaigns}
        initialLeads={leadsResp.leads}
        initialLeadsTotal={leadsResp.total}
        initialStats={mappedStats}
      />
    </div>
  );
}
