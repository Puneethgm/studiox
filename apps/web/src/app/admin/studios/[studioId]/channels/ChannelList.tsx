'use client';

const STATUS_DESCRIPTIONS: Record<ChannelStatus, string> = {
  active: '✓ Ready to send & receive messages',
  paused: '⏸ Paused — no messages in or out',
  disconnected: '✕ Disconnected — reconnect to resume messaging',
  error: '⚠️ Needs attention — check error below',
};

import { useRouter } from 'next/navigation';
import { useState } from 'react';
import { Trash2, Pencil, X, Plug } from 'lucide-react';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { Input } from '@/components/ui/Input';
import { FieldError, FieldHint, Label } from '@/components/ui/Label';
import { ApiError, api } from '@/lib/api';
import { formatDate } from '@/lib/datetime';
import type { ChannelAccount, ChannelKind, ChannelStatus } from '@/lib/types';

const KIND_LABELS: Record<ChannelKind, string> = {
  whatsapp_meta: 'WhatsApp',
  instagram_meta: 'Instagram DMs',
  messenger_meta: 'Facebook Messenger',
  x_dm: 'X DMs',
  sms: 'SMS',
  google_ads: 'Google Ads',
};

const STATUS_TONE: Record<ChannelStatus, 'success' | 'warning' | 'danger' | 'neutral'> = {
  active: 'success',
  paused: 'warning',
  disconnected: 'neutral',
  error: 'danger',
};

export function ChannelList({ studioId, channels }: { studioId: string; channels: ChannelAccount[] }) {
  const router = useRouter();
  const [pendingId, setPendingId] = useState<string | null>(null);

  // Edit modal state
  const [editingChannel, setEditingChannel] = useState<ChannelAccount | null>(null);
  const [externalId, setExternalId] = useState('');
  const [parentId, setParentId] = useState('');
  const [displayHandle, setDisplayHandle] = useState('');
  const [accessToken, setAccessToken] = useState('');
  const [xConsumerSecret, setXConsumerSecret] = useState('');
  const [xAccessToken, setXAccessToken] = useState('');
  const [xAccessTokenSecret, setXAccessTokenSecret] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

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

  async function handleUpdate(e: React.FormEvent) {
    e.preventDefault();
    if (!editingChannel) return;
    setError(null);
    setSubmitting(true);
    try {
      let finalAccessToken = accessToken || undefined;
      let finalExternalId = externalId;
      let finalDisplayHandle = displayHandle;
      let finalParentId = parentId;

      if (editingChannel.kind === 'sms') {
        finalDisplayHandle = externalId;
        if (accessToken) {
          finalAccessToken = `${parentId}:${accessToken}`;
        }
      } else if (editingChannel.kind === 'x_dm') {
        finalDisplayHandle = externalId;
        if (parentId && xConsumerSecret && xAccessToken && xAccessTokenSecret) {
          finalAccessToken = JSON.stringify({
            consumer_key: parentId,
            consumer_secret: xConsumerSecret,
            access_token: xAccessToken,
            access_token_secret: xAccessTokenSecret,
          });
        }
      }

      await api(`/api/v1/studios/${studioId}/messaging/channels/${editingChannel.id}`, {
        method: 'PUT',
        json: {
          externalId: finalExternalId,
          parentId: (editingChannel.kind === 'whatsapp_meta' || editingChannel.kind === 'sms' || editingChannel.kind === 'x_dm') ? finalParentId : undefined,
          displayHandle: finalDisplayHandle,
          accessToken: finalAccessToken,
        },
      });
      setEditingChannel(null);
      router.refresh();
    } catch (err) {
      if (err instanceof ApiError) setError(err.message);
      else setError('Could not update channel.');
    } finally {
      setSubmitting(false);
    }
  }

  // Filter out disconnected channels (should not occur as API filters them, but defensive)
  const activeChannels = channels.filter(c => c.status !== 'disconnected');

  return (
    <>
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
              <div className="flex items-center gap-2">
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => {
                    setEditingChannel(c);
                    setExternalId(c.externalId);
                    setParentId(c.parentId || '');
                    setDisplayHandle(c.displayHandle);
                    setAccessToken('');
                    setXConsumerSecret('');
                    setXAccessToken('');
                    setXAccessTokenSecret('');
                    setError(null);
                  }}
                  leftIcon={<Pencil className="h-4 w-4" />}
                  aria-label={`Edit ${KIND_LABELS[c.kind]} channel (${c.displayHandle})`}
                  title="Edit this channel configuration"
                >
                  <span className="hidden sm:inline">Edit</span>
                </Button>
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
              </div>
            </li>
          ))}
        </ul>
      </Card>

      {/* Edit Modal */}
      {editingChannel && (
        <div className="fixed inset-0 z-50 grid place-items-center bg-black/40 backdrop-blur-sm p-4">
          <div className="w-full max-w-md p-6 rounded-[28px] border border-violet-200/30 bg-white shadow-2xl dark:border-white/5 dark:bg-neutral-900 space-y-4">
            <div className="flex items-center justify-between border-b border-violet-200/20 pb-3 dark:border-white/5">
              <div className="flex items-center gap-2">
                <Pencil className="h-5 w-5 text-brand-500" />
                <h3 className="text-xs font-black uppercase tracking-wider text-zinc-900 dark:text-white">
                  Edit {KIND_LABELS[editingChannel.kind]} Account
                </h3>
              </div>
              <button 
                onClick={() => setEditingChannel(null)} 
                className="p-1 rounded-lg hover:bg-zinc-100 dark:hover:bg-neutral-800 text-zinc-400"
              >
                <X className="h-5 w-5" />
              </button>
            </div>

            <form onSubmit={handleUpdate} className="space-y-4 text-left">
              {editingChannel.kind === 'whatsapp_meta' && (
                <>
                  <div>
                    <Label htmlFor="edit-wabaId">WhatsApp Business Account (WABA) ID</Label>
                    <Input
                      id="edit-wabaId"
                      placeholder="987654321098765 (from Meta API Setup)"
                      required
                      value={parentId}
                      onChange={(e) => setParentId(e.target.value)}
                      className="font-mono text-xs"
                    />
                    <FieldHint>Find this in Meta → WhatsApp → API Setup page</FieldHint>
                  </div>
                  <div>
                    <Label htmlFor="edit-phoneNumberId">Phone Number ID</Label>
                    <Input
                      id="edit-phoneNumberId"
                      placeholder="123456789012345 (from Meta API Setup)"
                      required
                      value={externalId}
                      onChange={(e) => setExternalId(e.target.value)}
                      className="font-mono text-xs"
                    />
                    <FieldHint>From the WhatsApp → API Setup page in your Meta App.</FieldHint>
                  </div>
                  <div>
                    <Label htmlFor="edit-displayPhone">Display phone number</Label>
                    <Input
                      id="edit-displayPhone"
                      placeholder="+1 555 645 5341 (e.g. your business number)"
                      required
                      value={displayHandle}
                      onChange={(e) => setDisplayHandle(e.target.value)}
                    />
                    <FieldHint>Shown in the inbox header. Just for humans.</FieldHint>
                  </div>
                </>
              )}

              {editingChannel.kind === 'instagram_meta' && (
                <>
                  <div>
                    <Label htmlFor="edit-igId">Instagram Business Account ID</Label>
                    <Input
                      id="edit-igId"
                      placeholder="17841401234567890"
                      required
                      value={externalId}
                      onChange={(e) => setExternalId(e.target.value)}
                      className="font-mono text-xs"
                    />
                    <FieldHint>Find this in Meta Business Suite or your Page Settings.</FieldHint>
                  </div>
                  <div>
                    <Label htmlFor="edit-igHandle">Instagram Handle</Label>
                    <Input
                      id="edit-igHandle"
                      placeholder="@yourstudio"
                      required
                      value={displayHandle}
                      onChange={(e) => setDisplayHandle(e.target.value)}
                    />
                    <FieldHint>Just for reference in your inbox header.</FieldHint>
                  </div>
                </>
              )}

              {editingChannel.kind === 'messenger_meta' && (
                <>
                  <div>
                    <Label htmlFor="edit-pageId">Facebook Page ID</Label>
                    <Input
                      id="edit-pageId"
                      placeholder="102938475610293"
                      required
                      value={externalId}
                      onChange={(e) => setExternalId(e.target.value)}
                      className="font-mono text-xs"
                    />
                    <FieldHint>Find this in Meta Business Suite or your Page Settings.</FieldHint>
                  </div>
                  <div>
                    <Label htmlFor="edit-pageName">Page Name</Label>
                    <Input
                      id="edit-pageName"
                      placeholder="Your Studio Name"
                      required
                      value={displayHandle}
                      onChange={(e) => setDisplayHandle(e.target.value)}
                    />
                    <FieldHint>Just for reference in your inbox header.</FieldHint>
                  </div>
                </>
              )}

              {editingChannel.kind === 'sms' && (
                <>
                  <div>
                    <Label htmlFor="edit-twilioSid">Twilio Account SID</Label>
                    <Input
                      id="edit-twilioSid"
                      placeholder="ACxxxxxxxxxxxxxxxxxxxxxxxx"
                      required
                      value={parentId}
                      onChange={(e) => setParentId(e.target.value)}
                      className="font-mono text-xs"
                    />
                    <FieldHint>Find this in your Twilio Console Dashboard</FieldHint>
                  </div>
                  <div>
                    <Label htmlFor="edit-twilioPhone">Twilio Phone Number</Label>
                    <Input
                      id="edit-twilioPhone"
                      placeholder="+15556455341"
                      required
                      value={externalId}
                      onChange={(e) => setExternalId(e.target.value)}
                      className="font-mono text-xs"
                    />
                    <FieldHint>The active phone number purchased in Twilio.</FieldHint>
                  </div>
                </>
              )}

              {editingChannel.kind === 'x_dm' && (
                <>
                  <div>
                    <Label htmlFor="edit-xHandle">X Handle (without @)</Label>
                    <Input
                      id="edit-xHandle"
                      placeholder="e.g. PuneethGMt2"
                      required
                      value={externalId}
                      onChange={(e) => setExternalId(e.target.value)}
                      className="font-mono text-xs"
                    />
                    <FieldHint>The handle associated with these API keys.</FieldHint>
                  </div>
                  <div>
                    <Label htmlFor="edit-consumerKey">Consumer Key (API Key)</Label>
                    <Input
                      id="edit-consumerKey"
                      placeholder="e.g. mfnwJ9..."
                      required
                      value={parentId}
                      onChange={(e) => setParentId(e.target.value)}
                      className="font-mono text-xs"
                    />
                  </div>
                  <div>
                    <Label htmlFor="edit-consumerSecret">Consumer Secret (API Secret) (leave blank to keep current)</Label>
                    <Input
                      id="edit-consumerSecret"
                      type="password"
                      placeholder="••••••••••••••••"
                      value={xConsumerSecret}
                      onChange={(e) => setXConsumerSecret(e.target.value)}
                      className="font-mono text-xs"
                    />
                  </div>
                  <div>
                    <Label htmlFor="edit-xAccessToken">Access Token (leave blank to keep current)</Label>
                    <Input
                      id="edit-xAccessToken"
                      type="password"
                      placeholder="••••••••••••••••"
                      value={xAccessToken}
                      onChange={(e) => setXAccessToken(e.target.value)}
                      className="font-mono text-xs"
                    />
                  </div>
                  <div>
                    <Label htmlFor="edit-xAccessTokenSecret">Access Token Secret (leave blank to keep current)</Label>
                    <Input
                      id="edit-xAccessTokenSecret"
                      type="password"
                      placeholder="••••••••••••••••"
                      value={xAccessTokenSecret}
                      onChange={(e) => setXAccessTokenSecret(e.target.value)}
                      className="font-mono text-xs"
                    />
                  </div>
                </>
              )}

              {editingChannel.kind === 'google_ads' && (
                <>
                  <div>
                    <Label htmlFor="edit-googleClientId">Google Ads Client ID</Label>
                    <Input
                      id="edit-googleClientId"
                      placeholder="e.g. 123456789-abc.apps.googleusercontent.com"
                      required
                      value={externalId}
                      onChange={(e) => setExternalId(e.target.value)}
                      className="font-mono text-xs"
                    />
                    <FieldHint>The OAuth 2.0 Web Client ID registered in Google Developer Console.</FieldHint>
                  </div>
                </>
              )}

              {editingChannel.kind !== 'x_dm' && (
                <div>
                  <Label htmlFor="edit-accessToken">
                    {editingChannel.kind === 'sms' 
                      ? 'Twilio Auth Token (leave blank to keep current)' 
                      : editingChannel.kind === 'google_ads'
                      ? 'Google Ads Refresh Token (leave blank to keep current)'
                      : 'Access token (leave blank to keep current)'
                    }
                  </Label>
                  <Input
                    id="edit-accessToken"
                    type="password"
                    placeholder="••••••••••••••••"
                    value={accessToken}
                    onChange={(e) => setAccessToken(e.target.value)}
                    className="font-mono text-xs"
                  />
                  <FieldHint>Only enter a value if you wish to update or renew the credential token.</FieldHint>
                </div>
              )}

              <FieldError message={error ?? undefined} />

              <div className="flex gap-2 pt-2">
                <Button
                  type="button"
                  variant="ghost"
                  onClick={() => setEditingChannel(null)}
                  className="flex-1"
                >
                  Cancel
                </Button>
                <Button
                  type="submit"
                  loading={submitting}
                  className="flex-1"
                  leftIcon={<Plug className="h-4 w-4" />}
                >
                  Save Changes
                </Button>
              </div>
            </form>
          </div>
        </div>
      )}
    </>
  );
}
