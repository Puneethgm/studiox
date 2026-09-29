import { serverFetch } from '@/lib/auth';
import type { Permission, StudioRole } from '@/lib/types';
import { RolesClient } from './RolesClient';

export const dynamic = 'force-dynamic';

export default async function RolesPage({
  params,
}: {
  params: Promise<{ studioId: string }>;
}) {
  const { studioId } = await params;
  const [{ roles }, { permissions }] = await Promise.all([
    serverFetch<{ roles: StudioRole[] }>(`/api/v1/studios/${studioId}/roles`),
    serverFetch<{ permissions: Permission[] }>(`/api/v1/permissions`),
  ]);

  return (
    <div className="space-y-4">
      <div className="text-[11px] font-semibold text-zinc-400 dark:text-zinc-500 px-2 leading-relaxed">
        Define roles for your teammates and choose which sections of the app each one can access.
      </div>
      <RolesClient studioId={studioId} roles={roles ?? []} permissions={permissions ?? []} />
    </div>
  );
}
