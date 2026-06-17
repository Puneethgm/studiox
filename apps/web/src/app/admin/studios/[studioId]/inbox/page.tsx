import { serverFetch, requireSession } from '@/lib/auth';
import type { Conversation, Studio } from '@/lib/types';
import { InboxLive } from './InboxLive';
import { Inbox, Zap } from 'lucide-react';

interface ListResp {
  conversations: Conversation[];
  total: number;
}

export default async function InboxPage({
  params,
  searchParams,
}: {
  params: Promise<{ studioId: string }>;
  searchParams: Promise<{ [key: string]: string | string[] | undefined }>;
}) {
  const { studioId } = await params;
  const { unresponded } = (await searchParams) || {};
  const me = await requireSession();

  const studioEndpoint = me.role === 'super_admin'
    ? `/api/v1/admin/studios/${studioId}`
    : `/api/v1/me/studios/${studioId}`;

  const [data, studio] = await Promise.all([
    serverFetch<ListResp>(`/api/v1/studios/${studioId}/messaging/conversations?limit=50`),
    serverFetch<Studio>(studioEndpoint),
  ]);

  return (
    <div className="h-full">
      <InboxLive studioId={studioId} initialConversations={data.conversations} studio={studio} initialUnresponded={unresponded === 'true'} />
    </div>
  );
}
