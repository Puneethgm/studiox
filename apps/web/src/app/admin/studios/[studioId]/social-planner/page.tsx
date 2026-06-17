import SocialPlannerClient from '@/components/SocialPlannerClient';
import { requireSession, serverFetch } from '@/lib/auth';
import type { Studio } from '@/lib/types';
import { Megaphone } from 'lucide-react';

export default async function StudioSocialPlannerPage({
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

  const studio = await serverFetch<Studio>(studioEndpoint);

  return (
    <div className="space-y-4">
      <div className="text-[11px] font-semibold text-zinc-400 dark:text-zinc-500 px-2">
        AI-driven ad creation and schedule management for your studio location.
      </div>
      <SocialPlannerClient studioId={studioId} studio={studio} isSuperAdmin={me.role === 'super_admin'} />
    </div>
  );
}
