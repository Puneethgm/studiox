import { serverFetch } from '@/lib/auth';
import type { StudioRole, StudioUser } from '@/lib/types';
import { UsersClient } from './UsersClient';

export const dynamic = 'force-dynamic';

export default async function UsersPage({
  params,
}: {
  params: Promise<{ studioId: string }>;
}) {
  const { studioId } = await params;
  const [{ users }, { roles }] = await Promise.all([
    serverFetch<{ users: StudioUser[] }>(`/api/v1/studios/${studioId}/users`),
    serverFetch<{ roles: StudioRole[] }>(`/api/v1/studios/${studioId}/roles`),
  ]);

  return (
    <div className="space-y-4">
      <div className="text-[11px] font-semibold text-zinc-400 dark:text-zinc-500 px-2 leading-relaxed">
        Add teammates and assign them a role. New teammates start with a default password and must set
        their own on first login.
      </div>
      <UsersClient studioId={studioId} users={users ?? []} roles={roles ?? []} />
    </div>
  );
}
