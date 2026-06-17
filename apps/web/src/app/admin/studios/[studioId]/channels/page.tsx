import { Share2 } from 'lucide-react';
import { serverFetch, requireSession } from '@/lib/auth';
import type { ChannelAccount, Studio } from '@/lib/types';
import { ChannelTabs } from './ChannelTabs';

interface ListResp {
  channels: ChannelAccount[];
}

export default async function ChannelsPage({
  params,
}: {
  params: Promise<{ studioId: string }>;
}) {
  const { studioId } = await params;
  const me = await requireSession();

  const studioEndpoint = me.role === 'super_admin'
    ? `/api/v1/admin/studios/${studioId}`
    : `/api/v1/me/studios/${studioId}`;

  const { channels } = await serverFetch<ListResp>(
    `/api/v1/studios/${studioId}/messaging/channels`,
  );
  const studio = await serverFetch<Studio>(studioEndpoint);

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-4 px-2">
        <div className="text-[11px] font-semibold text-zinc-400 dark:text-zinc-500">
          Connect social accounts to receive messages and automate leads.
        </div>
        <div className="text-right">
          <div className="text-[9px] font-black uppercase tracking-[0.18em] text-zinc-400">Connectivity: <span className="text-zinc-700 dark:text-zinc-200">Multi-Channel API</span></div>
        </div>
      </div>
      <ChannelTabs studioId={studioId} channels={channels} studio={studio} />
    </div>
  );
}
