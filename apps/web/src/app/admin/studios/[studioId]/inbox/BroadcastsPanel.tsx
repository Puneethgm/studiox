'use client';

import { useEffect, useState, useCallback } from 'react';
import { Upload, Send, Trash2, X, RotateCw, Image as ImageIcon, Mail, MessageCircle } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { Input } from '@/components/ui/Input';
import { Label } from '@/components/ui/Label';
import { Badge } from '@/components/ui/Badge';
import { ApiError, api } from '@/lib/api';
import { formatDateTime } from '@/lib/datetime';
import { jobFailureReason } from '@/lib/jobFailureReason';

interface BroadcastList {
  id: string;
  name: string;
  contactCount: number;
  createdAt: string;
}

interface BroadcastContact {
  id: string;
  name: string;
  phone: string;
  email: string;
  createdAt: string;
}

interface BroadcastRecipientDetail {
  name: string;
  phone: string;
  email: string;
  status: 'pending' | 'sent' | 'failed' | 'dead';
  sentAt: string | null;
  reason?: string;
}

interface BroadcastCampaign {
  id: string;
  broadcastListId: string;
  listName: string;
  channel: 'whatsapp' | 'email';
  subject?: string;
  body: string;
  attachments: { type: string; url: string; name?: string }[];
  scheduledFor: string;
  status: 'scheduled' | 'sending' | 'completed' | 'canceled';
  totalCount: number;
  enqueuedCount: number;
  createdAt: string;
}

interface ChannelAccount {
  kind: string;
  status: string;
}

interface ImportPreview {
  importId: string;
  totalRows: number;
  missingCountryCode: number;
  sampleMissingNumbers: string[];
}

const STATUS_TONE: Record<BroadcastCampaign['status'], 'neutral' | 'warning' | 'success' | 'danger'> = {
  scheduled: 'neutral',
  sending: 'warning',
  completed: 'success',
  canceled: 'danger',
};

const RECIPIENT_STATUS_TONE: Record<BroadcastRecipientDetail['status'], 'neutral' | 'success' | 'danger'> = {
  pending: 'neutral',
  sent: 'success',
  failed: 'danger',
  dead: 'danger',
};
const RECIPIENT_STATUS_LABEL: Record<BroadcastRecipientDetail['status'], string> = {
  pending: 'Not sent yet',
  sent: 'Sent',
  failed: 'Failed, retrying',
  dead: 'Failed',
};

export function BroadcastsPanel({ studioId }: { studioId: string }) {
  const [lists, setLists] = useState<BroadcastList[]>([]);

  // Contact-list detail modal
  const [viewingList, setViewingList] = useState<BroadcastList | null>(null);
  const [viewingContacts, setViewingContacts] = useState<BroadcastContact[]>([]);
  const [loadingContacts, setLoadingContacts] = useState(false);

  // Campaign detail modal — per-recipient send status
  const [viewingCampaign, setViewingCampaign] = useState<BroadcastCampaign | null>(null);
  const [viewingRecipients, setViewingRecipients] = useState<BroadcastRecipientDetail[]>([]);
  const [loadingRecipients, setLoadingRecipients] = useState(false);
  const [campaigns, setCampaigns] = useState<BroadcastCampaign[]>([]);
  const [loading, setLoading] = useState(true);

  // Upload flow
  const [uploading, setUploading] = useState(false);
  const [preview, setPreview] = useState<ImportPreview | null>(null);
  const [listName, setListName] = useState('');
  const [countryCode, setCountryCode] = useState('65');
  const [confirming, setConfirming] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);

  // Which channels the studio actually has connected — gates which option
  // is selectable in the compose form below.
  const [channels, setChannels] = useState<ChannelAccount[]>([]);
  const hasActiveWhatsApp = channels.some((c) => (c.kind === 'whatsapp_meta' || c.kind === 'whatsapp_web') && c.status === 'active');
  const hasActiveEmail = channels.some((c) => c.kind === 'email_smtp' && c.status === 'active');

  // Compose flow
  const [composing, setComposing] = useState(false);
  const [selectedListId, setSelectedListId] = useState('');
  const [channel, setChannel] = useState<'whatsapp' | 'email'>('whatsapp');
  const [subject, setSubject] = useState('');
  const [body, setBody] = useState('');
  const [attachedUrl, setAttachedUrl] = useState<string | null>(null);
  const [attachedName, setAttachedName] = useState<string | null>(null);
  const [uploadingImage, setUploadingImage] = useState(false);
  const [scheduledFor, setScheduledFor] = useState('');
  const [scheduling, setScheduling] = useState(false);
  const [composeError, setComposeError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [listsRes, campaignsRes, channelsRes] = await Promise.all([
        api<{ lists: BroadcastList[] }>(`/api/v1/studios/${studioId}/messaging/broadcasts/lists`),
        api<{ campaigns: BroadcastCampaign[] }>(`/api/v1/studios/${studioId}/messaging/broadcasts/campaigns`),
        api<{ channels: ChannelAccount[] }>(`/api/v1/studios/${studioId}/messaging/channels`),
      ]);
      setLists(listsRes.lists || []);
      setCampaigns(campaignsRes.campaigns || []);
      setChannels(channelsRes.channels || []);
    } catch {
      // best-effort — surfaced implicitly by empty lists, no need for a toast here
    } finally {
      setLoading(false);
    }
  }, [studioId]);

  useEffect(() => {
    load();
  }, [load]);

  async function handleUpload(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    e.target.value = '';
    setUploadError(null);
    setUploading(true);
    try {
      const form = new FormData();
      form.append('file', file);
      const res = await fetch(`/api/v1/studios/${studioId}/messaging/broadcasts/import`, {
        method: 'POST',
        body: form,
        credentials: 'include',
      });
      if (!res.ok) {
        const body = await res.json().catch(() => null) as { error?: string } | null;
        throw new Error(body?.error || 'Upload failed');
      }
      const data = (await res.json()) as ImportPreview;
      setPreview(data);
      setListName(file.name.replace(/\.(csv|xlsx|xls)$/i, ''));
    } catch (err) {
      setUploadError(err instanceof Error ? err.message : 'Upload failed');
    } finally {
      setUploading(false);
    }
  }

  async function confirmImport() {
    if (!preview) return;
    if (!listName.trim()) {
      setUploadError('List name is required.');
      return;
    }
    setUploadError(null);
    setConfirming(true);
    try {
      await api(`/api/v1/studios/${studioId}/messaging/broadcasts/import/${preview.importId}/confirm`, {
        method: 'POST',
        json: { name: listName.trim(), defaultCountryCode: countryCode.trim() },
      });
      setPreview(null);
      setListName('');
      await load();
    } catch (err) {
      setUploadError(err instanceof ApiError ? err.message : 'Could not save the list.');
    } finally {
      setConfirming(false);
    }
  }

  async function deleteList(id: string) {
    if (!confirm('Delete this contact list? This cannot be undone.')) return;
    try {
      await api(`/api/v1/studios/${studioId}/messaging/broadcasts/lists/${id}`, { method: 'DELETE' });
      await load();
    } catch {
      alert('Could not delete the list.');
    }
  }

  async function openList(list: BroadcastList) {
    setViewingList(list);
    setViewingContacts([]);
    setLoadingContacts(true);
    try {
      const res = await api<{ contacts: BroadcastContact[] }>(
        `/api/v1/studios/${studioId}/messaging/broadcasts/lists/${list.id}`,
      );
      setViewingContacts(res.contacts || []);
    } catch {
      // modal just shows "no contacts" below — not worth a separate error state here
    } finally {
      setLoadingContacts(false);
    }
  }

  async function openCampaign(campaign: BroadcastCampaign) {
    setViewingCampaign(campaign);
    setViewingRecipients([]);
    setLoadingRecipients(true);
    try {
      const res = await api<{ recipients: BroadcastRecipientDetail[] }>(
        `/api/v1/studios/${studioId}/messaging/broadcasts/campaigns/${campaign.id}/recipients`,
      );
      setViewingRecipients(res.recipients || []);
    } catch {
      // modal just shows "no recipients" below — not worth a separate error state here
    } finally {
      setLoadingRecipients(false);
    }
  }

  async function handleAttachImage(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    e.target.value = '';
    setUploadingImage(true);
    try {
      const form = new FormData();
      form.append('file', file);
      const res = await fetch(`/api/v1/studios/${studioId}/messaging/upload`, {
        method: 'POST',
        body: form,
        credentials: 'include',
      });
      if (!res.ok) throw new Error('upload failed');
      const data = (await res.json()) as { url: string; filename: string };
      setAttachedUrl(data.url);
      setAttachedName(data.filename || file.name);
    } catch {
      alert('Image upload failed.');
    } finally {
      setUploadingImage(false);
    }
  }

  async function scheduleCampaign() {
    setComposeError(null);
    if (!selectedListId) {
      setComposeError('Pick a contact list.');
      return;
    }
    if (channel === 'email' && !subject.trim()) {
      setComposeError('Enter a subject line for the email.');
      return;
    }
    if (!body.trim() && !attachedUrl) {
      setComposeError('Enter a message or attach an image.');
      return;
    }
    setScheduling(true);
    try {
      await api(`/api/v1/studios/${studioId}/messaging/broadcasts/campaigns`, {
        method: 'POST',
        json: {
          broadcastListId: selectedListId,
          channel,
          subject: channel === 'email' ? subject.trim() : undefined,
          body: body.trim(),
          attachments: attachedUrl ? [{ type: 'image', url: attachedUrl, name: attachedName || undefined }] : [],
          scheduledFor: scheduledFor ? new Date(scheduledFor).toISOString() : undefined,
        },
      });
      setComposing(false);
      setSelectedListId('');
      setChannel('whatsapp');
      setSubject('');
      setBody('');
      setAttachedUrl(null);
      setAttachedName(null);
      setScheduledFor('');
      await load();
    } catch (err) {
      setComposeError(err instanceof ApiError ? err.message : 'Could not schedule the broadcast.');
    } finally {
      setScheduling(false);
    }
  }

  async function cancelCampaign(id: string) {
    if (!confirm('Cancel this broadcast? Recipients not yet enqueued will never be sent.')) return;
    try {
      await api(`/api/v1/studios/${studioId}/messaging/broadcasts/campaigns/${id}`, { method: 'DELETE' });
      await load();
    } catch {
      alert('Could not cancel.');
    }
  }

  return (
    <div className="flex-1 overflow-y-auto p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-lg font-black text-zinc-900 dark:text-white">Broadcasts</h3>
          <p className="text-xs font-semibold text-zinc-400 dark:text-zinc-500 uppercase tracking-widest mt-1">
            Upload a contact sheet, schedule one message to the whole list over WhatsApp or Email. Sends are paced automatically — a large list spreads over following days/ticks rather than firing all at once.
          </p>
        </div>
        <button
          onClick={load}
          className="flex items-center gap-1.5 px-4 py-2 rounded-xl bg-white/30 text-xs font-bold text-violet-600 border border-violet-200/30 hover:bg-white/50 transition-all dark:bg-white/5 dark:text-violet-400 dark:border-violet-500/10"
        >
          <RotateCw className="h-3.5 w-3.5" /> Refresh
        </button>
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        {/* Contact lists */}
        <Card>
          <div className="flex items-center justify-between mb-4">
            <h4 className="text-xs font-black uppercase tracking-wider text-violet-600 dark:text-violet-400">Contact Lists</h4>
            <label className="cursor-pointer">
              <input type="file" accept=".csv,.xlsx,.xls" className="hidden" onChange={handleUpload} disabled={uploading} />
              <span className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-violet-600 text-white text-xs font-bold hover:bg-violet-700">
                <Upload className="h-3.5 w-3.5" /> {uploading ? 'Uploading…' : 'Upload Contacts'}
              </span>
            </label>
          </div>

          {preview && (
            <div className="mb-4 rounded-xl border border-violet-200 bg-violet-50 dark:border-violet-900/40 dark:bg-violet-950/20 p-4 space-y-3">
              <p className="text-xs font-bold text-zinc-700 dark:text-zinc-200">
                Found {preview.totalRows} contact{preview.totalRows === 1 ? '' : 's'}.
                {preview.missingCountryCode > 0 && (
                  <> {preview.missingCountryCode} number{preview.missingCountryCode === 1 ? '' : 's'} missing a country code (e.g. {preview.sampleMissingNumbers.slice(0, 3).join(', ')}).</>
                )}
              </p>
              <div>
                <Label htmlFor="bc-list-name">List name</Label>
                <Input id="bc-list-name" value={listName} onChange={(e) => setListName(e.target.value)} />
              </div>
              {preview.missingCountryCode > 0 && (
                <div>
                  <Label htmlFor="bc-country-code">Country code to apply to those numbers</Label>
                  <Input id="bc-country-code" value={countryCode} onChange={(e) => setCountryCode(e.target.value)} placeholder="65" className="w-24" />
                </div>
              )}
              {uploadError && <p className="text-xs font-semibold text-red-500">{uploadError}</p>}
              <div className="flex gap-2">
                <Button size="sm" loading={confirming} onClick={confirmImport}>Save List</Button>
                <Button size="sm" variant="ghost" onClick={() => { setPreview(null); setUploadError(null); }}>Cancel</Button>
              </div>
            </div>
          )}
          {uploadError && !preview && <p className="mb-3 text-xs font-semibold text-red-500">{uploadError}</p>}

          {lists.length === 0 ? (
            <p className="text-xs text-zinc-400 py-6 text-center">No contact lists yet.</p>
          ) : (
            <div className="space-y-2">
              {lists.map((l) => (
                <div
                  key={l.id}
                  onClick={() => openList(l)}
                  className="flex items-center justify-between rounded-xl border border-zinc-200 dark:border-zinc-800 px-3 py-2 cursor-pointer hover:border-violet-300 hover:bg-violet-50/50 dark:hover:border-violet-800 dark:hover:bg-violet-950/20 transition-colors"
                >
                  <div>
                    <div className="text-sm font-bold text-zinc-800 dark:text-zinc-100">{l.name}</div>
                    <div className="text-[10px] text-zinc-400">{l.contactCount} contact{l.contactCount === 1 ? '' : 's'} &middot; click to view</div>
                  </div>
                  <button
                    onClick={(e) => { e.stopPropagation(); deleteList(l.id); }}
                    className="text-zinc-400 hover:text-red-500 shrink-0 ml-2"
                  >
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>
              ))}
            </div>
          )}
        </Card>

        {/* Compose / schedule */}
        <Card>
          <div className="flex items-center justify-between mb-4">
            <h4 className="text-xs font-black uppercase tracking-wider text-violet-600 dark:text-violet-400">Schedule a Broadcast</h4>
            {!composing && (
              <Button size="sm" leftIcon={<Send className="h-3.5 w-3.5" />} onClick={() => setComposing(true)} disabled={lists.length === 0}>
                New Broadcast
              </Button>
            )}
          </div>

          {!composing ? (
            <p className="text-xs text-zinc-400 py-6 text-center">
              {lists.length === 0 ? 'Upload a contact list first.' : 'Click "New Broadcast" to compose a message.'}
            </p>
          ) : (
            <div className="space-y-3">
              <div>
                <Label htmlFor="bc-select-list">Contact list</Label>
                <select
                  id="bc-select-list"
                  value={selectedListId}
                  onChange={(e) => setSelectedListId(e.target.value)}
                  className="w-full rounded-xl border border-zinc-200 dark:border-zinc-800 bg-white dark:bg-zinc-900 px-3 py-2 text-sm"
                >
                  <option value="">Select a list…</option>
                  {lists.map((l) => (
                    <option key={l.id} value={l.id}>{l.name} ({l.contactCount})</option>
                  ))}
                </select>
              </div>
              <div>
                <Label>Send over</Label>
                <div className="flex gap-2">
                  <button
                    type="button"
                    disabled={!hasActiveWhatsApp}
                    onClick={() => setChannel('whatsapp')}
                    title={!hasActiveWhatsApp ? 'No connected WhatsApp channel — connect one under Channels first' : undefined}
                    className={`flex-1 flex items-center justify-center gap-1.5 rounded-xl border px-3 py-2 text-xs font-bold transition-colors ${
                      channel === 'whatsapp'
                        ? 'border-violet-500 bg-violet-50 text-violet-700 dark:bg-violet-950/30 dark:text-violet-300'
                        : 'border-zinc-200 dark:border-zinc-800 text-zinc-500 dark:text-zinc-400'
                    } ${!hasActiveWhatsApp ? 'opacity-40 cursor-not-allowed' : 'hover:border-violet-300'}`}
                  >
                    <MessageCircle className="h-3.5 w-3.5" /> WhatsApp
                  </button>
                  <button
                    type="button"
                    disabled={!hasActiveEmail}
                    onClick={() => setChannel('email')}
                    title={!hasActiveEmail ? 'No connected email channel — connect one under Channels first' : undefined}
                    className={`flex-1 flex items-center justify-center gap-1.5 rounded-xl border px-3 py-2 text-xs font-bold transition-colors ${
                      channel === 'email'
                        ? 'border-violet-500 bg-violet-50 text-violet-700 dark:bg-violet-950/30 dark:text-violet-300'
                        : 'border-zinc-200 dark:border-zinc-800 text-zinc-500 dark:text-zinc-400'
                    } ${!hasActiveEmail ? 'opacity-40 cursor-not-allowed' : 'hover:border-violet-300'}`}
                  >
                    <Mail className="h-3.5 w-3.5" /> Email
                  </button>
                </div>
                {channel === 'email' && !hasActiveEmail && (
                  <p className="mt-1 text-[10px] font-semibold text-amber-500">No connected email channel yet — connect one under Channels before scheduling.</p>
                )}
                {channel === 'whatsapp' && !hasActiveWhatsApp && (
                  <p className="mt-1 text-[10px] font-semibold text-amber-500">No connected WhatsApp channel yet — connect one under Channels before scheduling.</p>
                )}
              </div>
              {channel === 'email' && (
                <div>
                  <Label htmlFor="bc-subject">Subject</Label>
                  <Input id="bc-subject" value={subject} onChange={(e) => setSubject(e.target.value)} placeholder="e.g. This week's class schedule" />
                </div>
              )}
              <div>
                <Label htmlFor="bc-body">Message</Label>
                <textarea
                  id="bc-body"
                  value={body}
                  onChange={(e) => setBody(e.target.value)}
                  rows={4}
                  className="w-full rounded-xl border border-zinc-200 dark:border-zinc-800 bg-white dark:bg-zinc-900 px-3 py-2 text-sm"
                  placeholder="Type the message every recipient will get…"
                />
              </div>
              <div>
                <Label>Image (optional)</Label>
                {attachedUrl ? (
                  <div className="flex items-center gap-2 text-xs">
                    <ImageIcon className="h-4 w-4 text-violet-500" />
                    <span className="truncate flex-1">{attachedName}</span>
                    <button onClick={() => { setAttachedUrl(null); setAttachedName(null); }}><X className="h-3.5 w-3.5 text-zinc-400" /></button>
                  </div>
                ) : (
                  <label className="cursor-pointer inline-flex items-center gap-1.5 text-xs font-bold text-violet-600 dark:text-violet-400">
                    <input type="file" accept="image/*" className="hidden" onChange={handleAttachImage} disabled={uploadingImage} />
                    <Upload className="h-3.5 w-3.5" /> {uploadingImage ? 'Uploading…' : 'Attach image'}
                  </label>
                )}
              </div>
              <div>
                <Label htmlFor="bc-schedule">Send time (leave blank for as-soon-as-possible)</Label>
                <Input id="bc-schedule" type="datetime-local" value={scheduledFor} onChange={(e) => setScheduledFor(e.target.value)} />
              </div>
              {composeError && <p className="text-xs font-semibold text-red-500">{composeError}</p>}
              <div className="flex gap-2">
                <Button
                  size="sm"
                  loading={scheduling}
                  disabled={channel === 'email' ? !hasActiveEmail : !hasActiveWhatsApp}
                  leftIcon={<Send className="h-3.5 w-3.5" />}
                  onClick={scheduleCampaign}
                >
                  Schedule
                </Button>
                <Button size="sm" variant="ghost" onClick={() => setComposing(false)}>Cancel</Button>
              </div>
            </div>
          )}
        </Card>
      </div>

      {/* Campaigns */}
      <Card>
        <h4 className="text-xs font-black uppercase tracking-wider text-violet-600 dark:text-violet-400 mb-4">Broadcasts</h4>
        {loading ? (
          <p className="text-xs text-zinc-400 py-6 text-center">Loading…</p>
        ) : campaigns.length === 0 ? (
          <p className="text-xs text-zinc-400 py-6 text-center">No broadcasts scheduled yet.</p>
        ) : (
          <div className="space-y-2">
            {campaigns.map((c) => (
              <div
                key={c.id}
                onClick={() => openCampaign(c)}
                className="flex items-center justify-between rounded-xl border border-zinc-200 dark:border-zinc-800 px-3 py-2.5 cursor-pointer hover:border-violet-300 hover:bg-violet-50/50 dark:hover:border-violet-800 dark:hover:bg-violet-950/20 transition-colors"
              >
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-bold text-zinc-800 dark:text-zinc-100">{c.listName}</span>
                    <Badge tone="neutral">
                      <span className="inline-flex items-center gap-1">
                        {c.channel === 'email' ? <Mail className="h-3 w-3" /> : <MessageCircle className="h-3 w-3" />}
                        {c.channel === 'email' ? 'Email' : 'WhatsApp'}
                      </span>
                    </Badge>
                    <Badge tone={STATUS_TONE[c.status]}>{c.status}</Badge>
                  </div>
                  {c.channel === 'email' && c.subject && (
                    <p className="text-xs font-bold text-zinc-700 dark:text-zinc-200 truncate mt-0.5">{c.subject}</p>
                  )}
                  <p className="text-xs text-zinc-500 dark:text-zinc-400 truncate mt-0.5">{c.body || '(image only)'}</p>
                  <p className="text-[10px] text-zinc-400 mt-0.5">
                    {c.enqueuedCount} of {c.totalCount} queued &middot; scheduled {formatDateTime(c.scheduledFor)} &middot; click for per-contact status
                  </p>
                </div>
                {(c.status === 'scheduled' || c.status === 'sending') && (
                  <button
                    onClick={(e) => { e.stopPropagation(); cancelCampaign(c.id); }}
                    className="text-zinc-400 hover:text-red-500 shrink-0 ml-3"
                  >
                    <Trash2 className="h-4 w-4" />
                  </button>
                )}
              </div>
            ))}
          </div>
        )}
      </Card>

      {viewingList && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
          onClick={() => setViewingList(null)}
        >
          <div
            className="w-full max-w-lg max-h-[80vh] overflow-hidden flex flex-col rounded-2xl bg-white dark:bg-zinc-900 shadow-xl"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between px-5 py-4 border-b border-zinc-200 dark:border-zinc-800 shrink-0">
              <div>
                <h4 className="text-sm font-black text-zinc-900 dark:text-white">{viewingList.name}</h4>
                <p className="text-[10px] text-zinc-400">{viewingList.contactCount} contact{viewingList.contactCount === 1 ? '' : 's'}</p>
              </div>
              <button onClick={() => setViewingList(null)} className="text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-200">
                <X className="h-4 w-4" />
              </button>
            </div>
            <div className="overflow-y-auto flex-1">
              {loadingContacts ? (
                <p className="text-xs text-zinc-400 py-8 text-center">Loading…</p>
              ) : viewingContacts.length === 0 ? (
                <p className="text-xs text-zinc-400 py-8 text-center">No contacts in this list.</p>
              ) : (
                <table className="w-full text-left text-xs">
                  <thead className="sticky top-0 bg-white dark:bg-zinc-900">
                    <tr className="border-b border-zinc-200 dark:border-zinc-800 text-[9px] font-black uppercase tracking-widest text-zinc-400">
                      <th className="px-5 py-2">Name</th>
                      <th className="px-5 py-2">Phone</th>
                      <th className="px-5 py-2">Email</th>
                    </tr>
                  </thead>
                  <tbody>
                    {viewingContacts.map((c) => (
                      <tr key={c.id} className="border-b border-zinc-100 last:border-0 dark:border-zinc-800/60">
                        <td className="px-5 py-2 font-semibold text-zinc-800 dark:text-zinc-100">{c.name || '—'}</td>
                        <td className="px-5 py-2 text-zinc-500 dark:text-zinc-400">{c.phone}</td>
                        <td className="px-5 py-2 text-zinc-500 dark:text-zinc-400">{c.email || '—'}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>
          </div>
        </div>
      )}

      {viewingCampaign && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
          onClick={() => setViewingCampaign(null)}
        >
          <div
            className="w-full max-w-lg max-h-[80vh] overflow-hidden flex flex-col rounded-2xl bg-white dark:bg-zinc-900 shadow-xl"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between px-5 py-4 border-b border-zinc-200 dark:border-zinc-800 shrink-0">
              <div>
                <h4 className="text-sm font-black text-zinc-900 dark:text-white">{viewingCampaign.listName}</h4>
                <p className="text-[10px] text-zinc-400">
                  {viewingRecipients.filter((r) => r.status === 'sent').length} of {viewingCampaign.totalCount} actually sent
                </p>
              </div>
              <button onClick={() => setViewingCampaign(null)} className="text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-200">
                <X className="h-4 w-4" />
              </button>
            </div>
            <div className="overflow-y-auto flex-1">
              {loadingRecipients ? (
                <p className="text-xs text-zinc-400 py-8 text-center">Loading…</p>
              ) : viewingRecipients.length === 0 ? (
                <p className="text-xs text-zinc-400 py-8 text-center">No recipients found.</p>
              ) : (
                <table className="w-full text-left text-xs">
                  <thead className="sticky top-0 bg-white dark:bg-zinc-900">
                    <tr className="border-b border-zinc-200 dark:border-zinc-800 text-[9px] font-black uppercase tracking-widest text-zinc-400">
                      <th className="px-5 py-2">Name</th>
                      <th className="px-5 py-2">{viewingCampaign.channel === 'email' ? 'Email' : 'Phone'}</th>
                      <th className="px-5 py-2">Status</th>
                      <th className="px-5 py-2">Sent at</th>
                    </tr>
                  </thead>
                  <tbody>
                    {viewingRecipients.map((r, i) => (
                      <tr key={i} className="border-b border-zinc-100 last:border-0 dark:border-zinc-800/60">
                        <td className="px-5 py-2 font-semibold text-zinc-800 dark:text-zinc-100">{r.name || '—'}</td>
                        <td className="px-5 py-2 text-zinc-500 dark:text-zinc-400">{viewingCampaign.channel === 'email' ? (r.email || '—') : r.phone}</td>
                        <td className="px-5 py-2">
                          <Badge tone={RECIPIENT_STATUS_TONE[r.status]}>{RECIPIENT_STATUS_LABEL[r.status]}</Badge>
                          {r.status !== 'sent' && r.reason && (
                            <div className="mt-1 text-[10px] font-semibold text-zinc-500 dark:text-zinc-400">
                              {jobFailureReason(r.reason)}
                            </div>
                          )}
                        </td>
                        <td className="px-5 py-2 text-zinc-500 dark:text-zinc-400 whitespace-nowrap">
                          {r.sentAt ? formatDateTime(r.sentAt) : '—'}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
