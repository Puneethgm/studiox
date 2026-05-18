import { Share2 } from 'lucide-react';
import { serverFetch } from '@/lib/auth';
import type { ChannelAccount } from '@/lib/types';
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
  const { channels } = await serverFetch<ListResp>(
    `/api/v1/studios/${studioId}/messaging/channels`,
  );

  return (
    <div className="space-y-6">
      {/* Premium Glass Header */}
      <div
        className="relative overflow-hidden rounded-[26px] border border-white/30 p-6 backdrop-blur-2xl dark:border-white/5"
        style={{
          background: 'linear-gradient(135deg, rgba(255,255,255,0.30) 0%, rgba(237,233,254,0.22) 60%, rgba(219,234,254,0.20) 100%)',
          boxShadow: 'inset 0 0 0 1px rgba(255,255,255,0.2), 0 8px 32px rgba(139,92,246,0.07)',
        }}
      >
        <div className="pointer-events-none absolute -right-16 -top-16 h-48 w-48 rounded-full bg-brand-500/10 blur-[70px]" />
        
        <div className="relative flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex items-center gap-4">
            <div className="grid h-12 w-12 place-items-center rounded-2xl bg-gradient-to-br from-brand-500 to-violet-600 text-white shadow-lg shadow-brand-500/25">
              <Share2 className="h-6 w-6" />
            </div>
            <div>
              <h1 className="text-2xl font-black tracking-tight text-zinc-900 dark:text-white">Channels</h1>
              <p className="mt-0.5 text-xs font-semibold text-zinc-500 dark:text-zinc-400">
                Connect social accounts to receive messages and automate leads.
              </p>
            </div>
          </div>
          <div className="hidden text-right sm:block">
            <div className="text-[10px] font-black uppercase tracking-[0.18em] text-zinc-400">Connectivity</div>
            <div className="mt-0.5 text-xs font-black text-zinc-700 dark:text-zinc-200">Multi-Channel API</div>
          </div>
        </div>
      </div>
      <ChannelTabs studioId={studioId} channels={channels} />
    </div>
  );
}
