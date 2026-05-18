import { serverFetch } from '@/lib/auth';
import type { Conversation } from '@/lib/types';
import { InboxLive } from './InboxLive';
import { Inbox, Zap } from 'lucide-react';

interface ListResp {
  conversations: Conversation[];
  total: number;
}

export default async function InboxPage({
  params,
}: {
  params: Promise<{ studioId: string }>;
}) {
  const { studioId } = await params;

  const data = await serverFetch<ListResp>(
    `/api/v1/studios/${studioId}/messaging/conversations?limit=50`,
  );

  return (
    <div className="space-y-4">
      {/* Compact glass header */}
      <div
        className="relative overflow-hidden rounded-[22px] border border-white/30 bg-white/30 px-5 py-3.5 backdrop-blur-2xl dark:border-white/5 dark:bg-neutral-900/30"
        style={{ boxShadow: 'inset 0 0 0 1px rgba(255,255,255,0.15), 0 4px 16px rgba(0,0,0,0.05)' }}
      >
        <div className="flex items-center justify-between gap-4">
          <div className="flex items-center gap-3">
            <div className="grid h-9 w-9 place-items-center rounded-xl bg-gradient-to-br from-blue-500/20 to-violet-500/10 text-blue-600 dark:text-blue-400">
              <Inbox className="h-4.5 w-4.5" />
            </div>
            <div>
              <h1 className="text-lg font-black tracking-tight text-zinc-900 dark:text-white">Inbox</h1>
              <p className="text-[11px] font-semibold text-zinc-400">
                {data.total} conversation{data.total === 1 ? '' : 's'} · Multi-Channel
              </p>
            </div>
          </div>
          <div className="flex items-center gap-2">
            {/* Live pill */}
            <div className="flex items-center gap-1.5 rounded-full bg-emerald-500/10 px-3 py-1.5 text-[10px] font-black uppercase tracking-widest text-emerald-600 dark:text-emerald-400">
              <span className="relative flex h-2 w-2">
                <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-75" />
                <span className="relative inline-flex h-2 w-2 rounded-full bg-emerald-500" />
              </span>
              Live
            </div>
            {/* Streaming badge */}
            <div className="hidden items-center gap-1 rounded-full bg-violet-500/10 px-3 py-1.5 text-[10px] font-black uppercase tracking-widest text-violet-600 sm:flex dark:text-violet-400">
              <Zap className="h-3 w-3" />
              Stream
            </div>
          </div>
        </div>
      </div>

      <InboxLive studioId={studioId} initialConversations={data.conversations} />
    </div>
  );
}
