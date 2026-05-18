'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { Check, MessagesSquare, Send } from 'lucide-react';
import { cn } from '@/lib/cn';
import { brandInitials } from '@/lib/color';
import { api } from '@/lib/api';
import { formatTime, relativeTime } from '@/lib/datetime';
import type {
  ChannelKind,
  Conversation,
  Direction,
  Message,
  SourceKind,
} from '@/lib/types';

const CHANNEL_BADGE: Record<ChannelKind, { label: string; color: string }> = {
  whatsapp_meta:  { label: 'WA',  color: '#25D366' },
  instagram_meta: { label: 'IG',  color: '#E1306C' },
  messenger_meta: { label: 'FB',  color: '#0084FF' },
  x_dm:           { label: 'X',   color: '#000000' },
};

interface SSEEvent {
  kind: 'message.received' | 'message.sent' | 'conversation.updated';
  studioId: string;
  conversationId: string;
  messageId?: string;
}

export function InboxLive({
  studioId,
  initialConversations,
}: {
  studioId: string;
  initialConversations: Conversation[];
}) {
  const [mounted, setMounted] = useState(false);
  const [activeChannel, setActiveChannel] = useState<ChannelKind>('whatsapp_meta');
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [loadingMessages, setLoadingMessages] = useState(false);
  const [draft, setDraft] = useState('');
  const [sending, setSending] = useState(false);
  const [newReceiverValue, setNewReceiverValue] = useState('');
  const [creatingConversation, setCreatingConversation] = useState(false);
  const messagesEndRef = useRef<HTMLLIElement>(null);

  useEffect(() => {
    setMounted(true);
  }, []);

  useEffect(() => {
    if (mounted) {
      setConversations(initialConversations.filter(c => c.channelKind === activeChannel));
    }
  }, [mounted, initialConversations, activeChannel]);

  useEffect(() => {
    if (mounted && !selectedId && conversations.length > 0) {
      setSelectedId(conversations[0].id);
    }
  }, [mounted, conversations, selectedId]);

  const selected = conversations.find((c) => c.id === selectedId);

  const refreshConversations = useCallback(async () => {
    const res = await api<{ conversations: Conversation[] }>(
      `/api/v1/studios/${studioId}/messaging/conversations?limit=50&channelKind=${activeChannel}`,
    );
    setConversations(res.conversations);
  }, [studioId, activeChannel]);

  const refreshMessages = useCallback(
    async (convId: string) => {
      setLoadingMessages(true);
      try {
        const res = await api<{ messages: Message[] }>(
          `/api/v1/studios/${studioId}/messaging/conversations/${convId}/messages?limit=200`,
        );
        setMessages(res.messages);
      } finally {
        setLoadingMessages(false);
      }
    },
    [studioId],
  );

  useEffect(() => {
    if (selectedId) {
      refreshMessages(selectedId);
      api(`/api/v1/studios/${studioId}/messaging/conversations/${selectedId}/read`, {
        method: 'POST',
      }).catch(() => {});
    } else {
      setMessages([]);
    }
  }, [selectedId, studioId, refreshMessages]);

  useEffect(() => {
    const url = `/api/v1/studios/${studioId}/messaging/stream`;
    const es = new EventSource(url, { withCredentials: true });

    function onEvent(e: MessageEvent) {
      try {
        const evt: SSEEvent = JSON.parse(e.data);
        if (evt.studioId !== studioId) return;
        refreshConversations();
        if (evt.conversationId === selectedId) {
          refreshMessages(evt.conversationId);
        }
      } catch {
        // Ignore malformed.
      }
    }
    es.addEventListener('message.received', onEvent);
    es.addEventListener('message.sent', onEvent);
    es.addEventListener('conversation.updated', onEvent);

    return () => {
      es.close();
    };
  }, [studioId, selectedId, refreshConversations, refreshMessages]);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  function makeOptimisticOutboundMessage(body: string): Message {
    const now = new Date().toISOString();
    return {
      id: `temp-${Date.now()}`,
      conversationId: selectedId ?? '',
      studioId,
      direction: 'outbound',
      sourceKind: 'studio_user',
      body,
      status: 'pending',
      sentAt: now,
      createdAt: now,
    };
  }

  async function send() {
    if (!selectedId || !draft.trim()) return;
    const body = draft.trim();
    const optimistic = makeOptimisticOutboundMessage(body);
    setSending(true);
    setMessages((current) => [...current, optimistic]);
    setDraft('');
    try {
      await api(`/api/v1/studios/${studioId}/messaging/conversations/${selectedId}/messages`, {
        method: 'POST',
        json: { body },
      });
      refreshConversations();
    } catch {
      setMessages((current) => current.filter((msg) => msg.id !== optimistic.id));
      setDraft(body);
    } finally {
      setSending(false);
    }
  }

  async function createConversation() {
    if (!newReceiverValue.trim()) return;
    setCreatingConversation(true);
    try {
      const conv = await api<Conversation>(
        `/api/v1/studios/${studioId}/messaging/conversations`,
        {
          method: 'POST',
          json: {
            channelKind: activeChannel,
            contactValue: newReceiverValue.trim()
          },
        },
      );
      setNewReceiverValue('');
      setSelectedId(conv.id);
      await refreshConversations();
    } catch (error) {
      console.error('Failed to create conversation:', error);
    } finally {
      setCreatingConversation(false);
    }
  }

  const handleChannelSwitch = (kind: ChannelKind) => {
    setActiveChannel(kind);
    setSelectedId(null);
    setMessages([]);
  };

  useEffect(() => {
    refreshConversations();
  }, [activeChannel, refreshConversations]);

  return (
    <div
      className="flex h-[calc(100vh-11rem)] overflow-hidden rounded-[22px] border border-violet-200/30 backdrop-blur-2xl dark:border-violet-500/10"
      style={{
        background: 'linear-gradient(135deg, rgba(255,255,255,0.22) 0%, rgba(237,233,254,0.18) 50%, rgba(219,234,254,0.18) 100%)',
        boxShadow: 'inset 0 0 0 1px rgba(255,255,255,0.2), 0 8px 40px rgba(139,92,246,0.08)',
      }}
    >
      {/* ── Sidebar ─────────────────────────── */}
      <aside
        className="hidden w-80 shrink-0 flex-col border-r border-violet-200/25 dark:border-violet-500/10 sm:flex"
        style={{ background: 'linear-gradient(180deg, rgba(237,233,254,0.25) 0%, rgba(219,234,254,0.15) 100%)' }}
      >

        {/* Sidebar header */}
        <div className="flex h-14 items-center justify-between border-b border-violet-200/20 px-5 dark:border-violet-500/10">
          <div className="flex items-center gap-2">
            <h2 className="text-sm font-black uppercase tracking-[0.15em] text-violet-600 dark:text-violet-400">Messages</h2>
          </div>
          <div className="flex items-center gap-1.5 rounded-full bg-emerald-500/15 px-2.5 py-1 text-[10px] font-black uppercase tracking-widest text-emerald-600 dark:text-emerald-400">
            <span className="relative flex h-1.5 w-1.5">
              <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-75" />
              <span className="relative inline-flex h-1.5 w-1.5 rounded-full bg-emerald-500" />
            </span>
            Live
          </div>
        </div>

        {/* Channel tabs */}
        <div className="flex gap-1.5 px-4 py-3">
          {mounted ? (
            (['whatsapp_meta', 'messenger_meta'] as const).map((kind) => (
              <button
                key={kind}
                type="button"
                onClick={() => handleChannelSwitch(kind)}
                className={cn(
                  "flex-1 py-2 px-3 rounded-xl text-[10px] font-black uppercase tracking-wider transition-all duration-300",
                  activeChannel === kind
                    ? "bg-gradient-to-r from-brand-500 to-violet-500 text-white shadow-lg shadow-brand-500/25"
                    : "bg-white/30 text-zinc-500 hover:bg-white/50 hover:text-zinc-700 dark:bg-white/5 dark:text-zinc-400 dark:hover:bg-white/10"
                )}
                suppressHydrationWarning
              >
                {CHANNEL_BADGE[kind].label}
              </button>
            ))
          ) : (
            <div className="flex-1 h-8 bg-white/20 animate-pulse rounded-xl dark:bg-white/5" />
          )}
        </div>

        {/* New conversation input */}
        <div className="px-4 pb-3">
          <form
            onSubmit={(e) => {
              e.preventDefault();
              createConversation();
            }}
            className="group relative"
          >
            <input
              type="text"
              value={newReceiverValue}
              onChange={(e) => setNewReceiverValue(e.target.value)}
              placeholder={activeChannel === 'whatsapp_meta' ? "Phone number..." : "Messenger ID..."}
              className="w-full rounded-2xl border border-white/20 bg-white/30 py-2.5 pl-4 pr-12 text-sm font-medium text-zinc-900 placeholder:text-zinc-400 backdrop-blur-md focus:border-brand-500/40 focus:outline-none focus:ring-2 focus:ring-brand-500/15 dark:border-white/5 dark:bg-white/5 dark:text-zinc-100 dark:placeholder:text-zinc-500"
              suppressHydrationWarning
            />
            <button
              type="submit"
              disabled={!newReceiverValue.trim() || creatingConversation}
              className="absolute right-1.5 top-1/2 -translate-y-1/2 grid h-8 w-8 place-items-center rounded-xl bg-gradient-to-br from-brand-500 to-violet-500 text-white shadow-md transition-all hover:scale-105 active:scale-95 disabled:opacity-0"
              suppressHydrationWarning
            >
              <Send className="h-3.5 w-3.5" />
            </button>
          </form>
        </div>

        {/* Conversation list */}
        <div className="flex-1 overflow-y-auto px-2 pb-2">
          {mounted ? (
            <ul className="space-y-0.5">
              {conversations.map((c) => (
                <li key={c.id}>
                  <button
                    type="button"
                    onClick={() => setSelectedId(c.id)}
                    className={cn(
                      'group relative flex w-full items-center gap-3 rounded-2xl px-3 py-3 text-left transition-all duration-300',
                      selectedId === c.id
                        ? 'bg-gradient-to-r from-brand-500 to-violet-500 text-white shadow-lg shadow-brand-500/20'
                        : 'hover:bg-white/40 dark:hover:bg-white/5',
                    )}
                    suppressHydrationWarning
                  >
                    <ChannelAvatar kind={c.channelKind} name={c.contactDisplayName || c.contactValue} active={selectedId === c.id} />
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center justify-between gap-2">
                        <span className={cn('truncate text-sm font-bold', selectedId === c.id ? 'text-white' : 'text-zinc-900 dark:text-zinc-100')}>
                          {c.contactDisplayName || c.contactValue}
                        </span>
                        <span
                          className={cn('shrink-0 text-[10px] font-bold uppercase tracking-wider', selectedId === c.id ? 'text-white/60' : 'text-zinc-400')}
                          suppressHydrationWarning
                        >
                          {relativeTime(c.lastMessageAt)}
                        </span>
                      </div>
                      <div className="mt-0.5 flex items-center gap-2">
                        <p className={cn('min-w-0 flex-1 truncate text-xs font-medium', selectedId === c.id ? 'text-white/75' : 'text-zinc-500 dark:text-zinc-400')}>
                          {c.lastMessageDirection === 'outbound' && <span className={selectedId === c.id ? 'text-white/50' : 'text-zinc-400'}>You: </span>}
                          {c.lastMessagePreview}
                        </p>
                        {c.unreadCount > 0 && selectedId !== c.id && (
                          <span className="grid h-5 min-w-5 shrink-0 place-items-center rounded-full bg-gradient-to-r from-brand-500 to-violet-500 px-1.5 text-[10px] font-black text-white shadow-md shadow-brand-500/30">
                            {c.unreadCount}
                          </span>
                        )}
                      </div>
                    </div>
                  </button>
                </li>
              ))}
            </ul>
          ) : (
            <div className="flex h-full items-center justify-center p-8">
              <div className="h-6 w-6 animate-spin rounded-full border-2 border-white/20 border-t-brand-500" />
            </div>
          )}
        </div>
      </aside>

      {/* ── Main Chat Area ─────────────────── */}
      <section className="flex min-w-0 flex-1 flex-col">
        {selected ? (
          <>
            {/* Chat header */}
            <header className="z-10 flex h-14 items-center gap-3 border-b border-white/10 bg-white/20 px-5 backdrop-blur-xl dark:border-white/5 dark:bg-white/5">
              <ChannelAvatar
                kind={selected.channelKind}
                name={selected.contactDisplayName || selected.contactValue}
              />
              <div className="min-w-0">
                <div className="truncate text-sm font-bold text-zinc-900 dark:text-zinc-100">
                  {selected.contactDisplayName || selected.contactValue}
                </div>
                <div className="flex items-center gap-1.5 truncate text-[10px] font-bold uppercase tracking-[0.15em] text-zinc-400">
                  <span className="h-1.5 w-1.5 rounded-full bg-emerald-500 shadow-sm shadow-emerald-500/50" />
                  {channelLabel(selected.channelKind)} · {selected.contactValue}
                </div>
              </div>
            </header>

            {/* Messages */}
            <div
              className="relative flex-1 overflow-y-auto px-5 py-6"
              style={{
                background: 'linear-gradient(160deg, rgba(255,255,255,0.08) 0%, rgba(237,233,254,0.06) 50%, rgba(219,234,254,0.08) 100%)',
              }}
            >
              {/* Subtle dot pattern */}
              <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(rgba(139,92,246,0.04)_1px,transparent_1px)] [background-size:22px_22px] dark:bg-[radial-gradient(rgba(139,92,246,0.06)_1px,transparent_1px)]" />

              {/* Ambient glow blobs */}
              <div className="pointer-events-none absolute left-1/4 top-1/4 h-48 w-48 rounded-full bg-violet-400/10 blur-3xl" />
              <div className="pointer-events-none absolute bottom-1/4 right-1/4 h-40 w-40 rounded-full bg-sky-400/10 blur-3xl" />

              <div className="relative">
                {loadingMessages && messages.length === 0 ? (
                  <div className="grid h-full place-items-center py-20">
                    <div className="h-8 w-8 animate-spin rounded-full border-2 border-brand-500 border-t-transparent" />
                  </div>
                ) : (
                  <ul className="space-y-3">
                    {messages.map((m) => (
                      <MessageBubble key={m.id} msg={m} />
                    ))}
                    <li ref={messagesEndRef} className="h-2" />
                  </ul>
                )}
              </div>
            </div>

            {/* Compose */}
            <footer
              className="z-10 border-t border-violet-200/20 p-4 backdrop-blur-xl dark:border-violet-500/10"
              style={{
                background: 'linear-gradient(to top, rgba(237,233,254,0.35) 0%, rgba(219,234,254,0.25) 100%)',
              }}
            >
              <form
                onSubmit={(e) => {
                  e.preventDefault();
                  send();
                }}
                className="flex items-end gap-3"
              >
                <div className="relative flex-1">
                  <textarea
                    value={draft}
                    onChange={(e) => setDraft(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' && !e.shiftKey) {
                        e.preventDefault();
                        send();
                      }
                    }}
                    rows={1}
                    placeholder="Type a message... (Enter to send)"
                    className="block min-h-[48px] max-h-32 w-full resize-none rounded-2xl border border-violet-200/40 px-5 py-3 text-sm font-medium text-zinc-900 placeholder:text-violet-400/60 backdrop-blur-md focus:border-violet-400/50 focus:outline-none focus:ring-2 focus:ring-violet-400/20 dark:border-violet-500/20 dark:text-zinc-100 dark:placeholder:text-violet-300/30"
                    style={{
                      background: 'linear-gradient(135deg, rgba(255,255,255,0.7) 0%, rgba(245,243,255,0.65) 100%)',
                      boxShadow: 'inset 0 1px 3px rgba(139,92,246,0.08), 0 2px 12px rgba(139,92,246,0.06)',
                    }}
                    suppressHydrationWarning
                  />
                </div>
                <button
                  type="submit"
                  disabled={!draft.trim() || sending}
                  className="grid h-12 w-12 shrink-0 place-items-center rounded-2xl text-white shadow-xl shadow-brand-500/30 transition-all hover:scale-105 hover:shadow-brand-500/40 active:scale-95 disabled:opacity-40 disabled:hover:scale-100"
                  style={{ background: 'linear-gradient(135deg, #7c3aed 0%, #6d28d9 50%, #4f46e5 100%)' }}
                  suppressHydrationWarning
                >
                  <Send className="h-5 w-5" />
                </button>
              </form>
              <p className="mt-2 text-center text-[10px] font-semibold text-violet-400/50">Enter to send · Shift+Enter for new line</p>
            </footer>
          </>
        ) : (
          /* Empty state */
          <div
            className="grid flex-1 place-items-center px-6 text-center"
            style={{
              background: 'linear-gradient(160deg, rgba(255,255,255,0.08) 0%, rgba(237,233,254,0.06) 50%, rgba(219,234,254,0.08) 100%)',
            }}
          >
            <div
              className="max-w-sm rounded-[28px] border border-violet-200/30 p-10 backdrop-blur-2xl dark:border-violet-500/10"
              style={{
                background: 'linear-gradient(135deg, rgba(255,255,255,0.5) 0%, rgba(245,243,255,0.4) 100%)',
                boxShadow: 'inset 0 0 0 1px rgba(255,255,255,0.3), 0 16px 60px rgba(139,92,246,0.12)',
              }}
            >
              {/* Icon with ring glow */}
              <div className="relative mx-auto mb-6 grid h-20 w-20 place-items-center">
                <div className="absolute inset-0 rounded-3xl bg-gradient-to-br from-violet-400/30 to-blue-400/20 blur-lg" />
                <div className="relative grid h-20 w-20 place-items-center rounded-3xl bg-gradient-to-br from-brand-500 to-violet-600 text-white shadow-xl shadow-brand-500/25">
                  <MessagesSquare className="h-9 w-9" />
                </div>
              </div>
              <h3 className="text-xl font-black text-zinc-900 dark:text-white">Select a Conversation</h3>
              <p className="mt-2.5 text-sm font-medium leading-relaxed text-zinc-500 dark:text-zinc-400">
                Choose a chat from the sidebar, or use the search box to start a new conversation.
              </p>
              <div className="mt-6 flex justify-center gap-3">
                <div className="flex items-center gap-1.5 rounded-full bg-violet-50 px-4 py-2 text-[11px] font-bold text-violet-600 dark:bg-violet-500/10 dark:text-violet-400">
                  <span className="h-1.5 w-1.5 rounded-full bg-violet-500" />WhatsApp
                </div>
                <div className="flex items-center gap-1.5 rounded-full bg-blue-50 px-4 py-2 text-[11px] font-bold text-blue-600 dark:bg-blue-500/10 dark:text-blue-400">
                  <span className="h-1.5 w-1.5 rounded-full bg-blue-500" />Messenger
                </div>
              </div>
            </div>
          </div>
        )}
      </section>
    </div>
  );
}

// ────────────────────────────────────────────────────────────────

function ChannelAvatar({ kind, name, active }: { kind: ChannelKind; name: string; active?: boolean }) {
  const ch = CHANNEL_BADGE[kind];
  return (
    <span className="relative shrink-0">
      <span
        className={cn(
          "grid h-11 w-11 place-items-center rounded-2xl text-sm font-black text-white shadow-lg transition-transform group-hover:scale-105",
          active ? "bg-white/25 backdrop-blur-md" : "ring-3 ring-white/30 dark:ring-white/10"
        )}
        style={!active ? { background: avatarColor(name) } : undefined}
        aria-hidden
      >
        {brandInitials(name)}
      </span>
      <span
        className={cn(
          "absolute -bottom-0.5 -right-0.5 grid h-5 w-5 place-items-center rounded-lg text-[9px] font-black text-white shadow-md",
          active ? "ring-2 ring-brand-500/50" : "ring-2 ring-white/50 dark:ring-neutral-900/80"
        )}
        style={{ background: ch.color }}
        aria-label={ch.label}
      >
        {ch.label[0]}
      </span>
    </span>
  );
}

function MessageBubble({ msg }: { msg: Message }) {
  const isOutbound = msg.direction === 'outbound';
  const sourceTag = sourceTagFor(msg.sourceKind);
  return (
    <li className={cn('flex animate-in', isOutbound ? 'justify-end' : 'justify-start')}>
      <div
        className={cn(
          'relative max-w-[85%] px-4 py-2.5 text-sm shadow-lg transition-all duration-300 sm:max-w-[70%]',
          isOutbound
            ? 'rounded-[20px] rounded-br-none bg-gradient-to-br from-brand-500 to-violet-500 text-white shadow-brand-500/15'
            : 'rounded-[20px] rounded-bl-none border border-white/30 bg-white/50 text-zinc-900 backdrop-blur-xl dark:border-white/5 dark:bg-white/10 dark:text-zinc-100',
        )}
      >
        {sourceTag && (
          <div className={cn('mb-1 text-[10px] font-black uppercase tracking-widest opacity-60')}>
            {sourceTag}
          </div>
        )}
        <div className="whitespace-pre-wrap font-medium leading-relaxed">{msg.body}</div>
        <div
          className={cn(
            'mt-1.5 flex items-center justify-end gap-1.5 text-[10px] font-bold',
            isOutbound ? 'text-white/60' : 'text-zinc-400',
          )}
        >
          <span suppressHydrationWarning>{formatTime(msg.sentAt)}</span>
          {isOutbound && <StatusTick status={msg.status} />}
        </div>

        {/* Tail */}
        <div className={cn(
          "absolute bottom-0 h-3 w-3",
          isOutbound
            ? "-right-0.5 bg-violet-500 [clip-path:polygon(0_0,0%_100%,100%_100%)]"
            : "-left-0.5 bg-white/50 dark:bg-white/10 [clip-path:polygon(100%_0,0%_100%,100%_100%)]"
        )} />
      </div>
    </li>
  );
}

function StatusTick({ status }: { status: Message['status'] }) {
  switch (status) {
    case 'pending': return <span className="animate-pulse">···</span>;
    case 'sent': return <Check className="h-3 w-3" />;
    case 'delivered': return <div className="flex -space-x-1.5"><Check className="h-3 w-3" /><Check className="h-3 w-3" /></div>;
    case 'read': return <div className="flex -space-x-1.5 text-sky-300"><Check className="h-3 w-3" /><Check className="h-3 w-3" /></div>;
    case 'failed': return <span className="text-red-400 font-bold">!</span>;
    default: return null;
  }
}

function sourceTagFor(s: SourceKind): string | null {
  switch (s) {
    case 'automation':
      return 'Automation';
    case 'ai':
      return 'AI';
    default:
      return null; // 'customer' / 'studio_user' don't need a tag
  }
}

function channelLabel(k: ChannelKind): string {
  switch (k) {
    case 'whatsapp_meta':  return 'WhatsApp';
    case 'instagram_meta': return 'Instagram';
    case 'messenger_meta': return 'Messenger';
    case 'x_dm':           return 'X DM';
  }
}

const AVATAR_PALETTE = [
  '#0ea5e9', '#6366f1', '#7c3aed', '#a855f7', '#ec4899',
  '#f43f5e', '#f97316', '#eab308', '#22c55e', '#14b8a6',
];

function avatarColor(seed: string): string {
  let h = 0;
  for (let i = 0; i < seed.length; i++) h = ((h << 5) - h + seed.charCodeAt(i)) | 0;
  return AVATAR_PALETTE[Math.abs(h) % AVATAR_PALETTE.length]!;
}

// Direction is referenced indirectly (msg.direction === 'outbound'); silence
// the unused-import lint while still exporting a typed boundary.
export type _DirectionAlias = Direction;
