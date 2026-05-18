
'use client';

const STATUS_DESCRIPTIONS: Record<ChannelStatus, string> = {
  active: '✓ Ready to send & receive messages',
  paused: '⏸ Paused — no messages in or out',
  error: '⚠️ Needs attention — check error below',
};

import { useRouter } from 'next/navigation';
import { useState } from 'react';
import { Trash2 } from 'lucide-react';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { api } from '@/lib/api';
import { formatDate } from '@/lib/datetime';
import type { ChannelAccount, ChannelKind, ChannelStatus } from '@/lib/types';

const KIND_LABELS: Record<ChannelKind, string> = {
  whatsapp_meta: 'WhatsApp',
  instagram_meta: 'Instagram DMs',
  messenger_meta: 'Facebook Messenger',
  x_dm: 'X DMs',
};

const STATUS_TONE: Record<ChannelStatus, 'success' | 'warning' | 'danger' | 'neutral'> = {
  active: 'success',
  paused: 'warning',
  error: 'danger',
};

export function ChannelList({ studioId, channels }: { studioId: string; channels: ChannelAccount[] }) {
  const router = useRouter();
  const [pendingId, setPendingId] = useState<string | null>(null);

  async function disconnect(id: string) {
    const confirmed = confirm(
      '⚠️ Disconnect this channel?\n\n' +
      'After disconnecting:\n' +
      '• Incoming messages will STOP arriving\n' +
      '• You won\'t be able to send messages\n' +
      '• Conversations stay in history\n\n' +
      'You can reconnect anytime with a fresh token.'
    );
    if (!confirmed) return;
    setPendingId(id);
    try {
      await api(`/api/v1/studios/${studioId}/messaging/channels/${id}`, { method: 'DELETE' });
      router.refresh();
    } finally {
      setPendingId(null);
    }
  }

  // Filter out disconnected channels (should not occur as API filters them, but defensive)
  const activeChannels = channels.filter(c => c.status !== 'disconnected');

  return (
    <Card title="Connected channels" noPadding>
      <ul className="divide-y divide-slate-100 dark:divide-slate-800/60">
        {activeChannels.map((c) => (
          <li key={c.id} className="flex items-center justify-between gap-4 px-6 py-4">
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <span className="text-sm font-semibold text-slate-900 dark:text-slate-100">
                  {KIND_LABELS[c.kind]}
                </span>
                <Badge 
                  tone={STATUS_TONE[c.status]}
                  title={STATUS_DESCRIPTIONS[c.status]}
                  className="cursor-help"
                >
                  {c.status}
                </Badge>
              </div>
              <div className="mt-1 truncate text-xs text-slate-500 dark:text-slate-400">
                {c.displayHandle} · connected {formatDate(c.connectedAt)}
              </div>
              <div className="mt-1 text-xs text-slate-400 dark:text-slate-500">
                {STATUS_DESCRIPTIONS[c.status]}
              </div>
              {c.status === 'error' && c.lastError && (
                <div className="mt-2 rounded bg-red-50 p-2 text-xs text-red-700 dark:bg-red-900/20 dark:text-red-300">
                  <strong>Error:</strong> {c.lastError}
                  {c.lastError.includes('token') && (
                    <> — <a href="/docs/meta-whatsapp#troubleshooting" className="underline">Renew token →</a></>
                  )}
                </div>
              )}
            </div>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => disconnect(c.id)}
              loading={pendingId === c.id}
              leftIcon={<Trash2 className="h-4 w-4" />}
              aria-label={`Disconnect ${KIND_LABELS[c.kind]} channel (${c.displayHandle})`}
              title="Disconnect this channel and stop messaging"
            >
              <span className="hidden sm:inline">Disconnect</span>
            </Button>
          </li>
        ))}
      </ul>
    </Card>
  );
}
