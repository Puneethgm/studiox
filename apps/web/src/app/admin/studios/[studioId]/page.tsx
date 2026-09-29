import { requireSession, serverFetch } from '@/lib/auth';
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
  const me = await requireSession();

  // Super admins fetch studio from /admin, others from /me
  const studioEndpoint = me.role === 'super_admin'
    ? `/api/v1/admin/studios/${studioId}`
    : `/api/v1/me/studios/${studioId}`;

  // A studio_staff teammate may not be granted every one of these sections
  // (identity/permissions) — campaigns/leads/stats degrade to empty instead
  // of taking down the whole dashboard on a 403. `studio` itself never
  // degrades: every role needs it, and its own endpoint isn't permission-gated.
  const [studio, campResp, leadsResp, stats] = await Promise.all([
    serverFetch<Studio>(studioEndpoint),
    serverFetch<{ campaigns: Campaign[] }>(`/api/v1/studios/${studioId}/campaigns`).catch(() => ({ campaigns: [] })),
    serverFetch<{ leads: Lead[]; total: number }>(`/api/v1/studios/${studioId}/leads?limit=5`).catch(() => ({ leads: [], total: 0 })),
    serverFetch<LeadStats>(`/api/v1/studios/${studioId}/leads/stats`).catch(() => ({ total: 0, byStatus: {} as LeadStats['byStatus'] })),
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
      paused: stats.byStatus.paused ?? 0,
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


