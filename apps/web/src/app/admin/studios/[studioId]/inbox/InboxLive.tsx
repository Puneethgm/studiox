'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { useRouter, useSearchParams, usePathname } from 'next/navigation';
import {
  Check,
  MessagesSquare,
  Send,
  Sparkles,
  Link as LinkIcon,
  Plus,
  Trash2,
  Clock,
  Play,
  FileText,
  X,
  Paperclip,
  Image as ImageIcon,
  RotateCw,
  Loader2,
  Pencil,
  Calendar,
  Star,
  Inbox,
  Mail,
  Video,
  Tag,
  Archive,
  ListFilter,
  ArrowUpDown,
  Phone,
  Bot,
} from 'lucide-react';
import { cn } from '@/lib/cn';
import { brandInitials } from '@/lib/color';
import { ApiError, api } from '@/lib/api';
import { formatTime, relativeTime } from '@/lib/datetime';
import type {
  ChannelKind,
  Conversation,
  Direction,
  Message,
  SourceKind,
  Attachment,
  Studio,
  Lead,
} from '@/lib/types';
import { ContactDetailsPanel } from './ContactDetailsPanel';
import { HeaderActions } from '@/components/HeaderActions';

// Strip @c.us / @lid / @s.whatsapp.net suffixes from WA chat IDs for display
function displayContact(value: string): string {
  return value ? value.replace(/@[^@]+$/, '') : value;
}

function getLeadStatusStyles(status: string, active: boolean): string {
  if (active) {
    return 'bg-white/20 text-white';
  }
  switch (status) {
    case 'new':
      return 'bg-blue-500/15 text-blue-600 dark:bg-blue-500/25 dark:text-blue-400';
    case 'contacted':
      return 'bg-violet-500/15 text-violet-600 dark:bg-violet-500/25 dark:text-violet-400';
    case 'trial_booked':
      return 'bg-emerald-500/15 text-emerald-600 dark:bg-emerald-500/25 dark:text-emerald-400';
    case 'member':
      return 'bg-amber-500/15 text-amber-600 dark:bg-amber-500/25 dark:text-amber-400';
    case 'dropped':
      return 'bg-rose-500/15 text-rose-600 dark:bg-rose-500/25 dark:text-rose-400';
    case 'paused':
      return 'bg-zinc-500/15 text-zinc-600 dark:bg-zinc-500/25 dark:text-zinc-400';
    default:
      return 'bg-zinc-100 text-zinc-600 dark:bg-white/5 dark:text-zinc-400';
  }
}

function getLeadStatusLabel(status: string): string {
  switch (status) {
    case 'new': return 'New';
    case 'contacted': return 'Connected';
    case 'trial_booked': return 'Trial';
    case 'member': return 'Member';
    case 'dropped': return 'Dropped';
    case 'paused': return 'Paused';
    default: return status;
  }
}


const CHANNEL_BADGE: Record<ChannelKind, { label: string; color: string }> = {
  whatsapp_meta:  { label: 'WhatsApp (Cloud)',  color: '#25D366' },
  whatsapp_web:   { label: 'WhatsApp (QR)',     color: '#128C7E' },
  instagram_meta: { label: 'Instagram',         color: '#E1306C' },
  messenger_meta: { label: 'Messenger',         color: '#0084FF' },
  x_dm:           { label: 'X / Twitter',       color: '#000000' },
  sms:            { label: 'SMS',               color: '#3b82f6' },
  google_ads:     { label: 'Google Ads',        color: '#4285F4' },
  telegram:       { label: 'Telegram (Bot)',    color: '#26A5E4' },
  telegram_mtproto: { label: 'Telegram',        color: '#26A5E4' },
};

interface SSEEvent {
  kind: 'message.received' | 'message.sent' | 'conversation.updated';
  studioId: string;
  conversationId: string;
  messageId?: string;
}

interface Template {
  id: string;
  name: string;
  body: string;
  channelKinds: string[];
  attachments?: Attachment[];
  createdAt: string;
}

interface TriggerLink {
  id: string;
  name: string;
  url: string;
  clicks: number;
  createdAt: string;
}

interface PendingJob {
  id: number;
  studioId: string;
  conversationId: string;
  contactDisplayName: string;
  contactValue: string;
  channelKind: ChannelKind;
  body: string;
  attachments?: Attachment[];
  scheduledFor: string;
  attempts: number;
  status: string;
}

type InboxTab = 'conversations' | 'automated_messages' | 'snippets' | 'trigger_links';

export function InboxLive({
  studioId,
  initialConversations,
  studio,
  initialUnresponded = false,
}: {
  studioId: string;
  initialConversations: Conversation[];
  studio?: Studio;
  initialUnresponded?: boolean;
}) {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const [mounted, setMounted] = useState(false);

  const VALID_TABS: InboxTab[] = ['conversations', 'automated_messages', 'snippets', 'trigger_links'];
  const VALID_CHANNELS: ChannelKind[] = ['whatsapp_web', 'whatsapp_meta', 'instagram_meta', 'messenger_meta', 'sms', 'telegram', 'telegram_mtproto'];

  const initialTab = (VALID_TABS.includes(searchParams.get('tab') as InboxTab) ? searchParams.get('tab') : 'conversations') as InboxTab;
  const initialChannel = (VALID_CHANNELS.includes(searchParams.get('channel') as ChannelKind) ? searchParams.get('channel') : 'whatsapp_web') as ChannelKind;

  const [currentTab, _setCurrentTab] = useState<InboxTab>(initialTab);
  const setCurrentTab = useCallback((tab: InboxTab) => {
    currentTabRef.current = tab;
    _setCurrentTab(tab);
    const p = new URLSearchParams(searchParams.toString());
    p.set('tab', tab);
    router.replace(`${pathname}?${p.toString()}`, { scroll: false });
  }, [pathname, router, searchParams]);

  const [activeChannel, setActiveChannel] = useState<ChannelKind>(initialChannel);
  const [unrespondedOnly, setUnrespondedOnly] = useState(initialUnresponded);
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [inboxTab, setInboxTab] = useState<'all' | 'unread' | 'recents' | 'starred'>('all');
  const [starredIds, setStarredIds] = useState<string[]>([]);

  useEffect(() => {
    try {
      const stored = localStorage.getItem('projectx_starred_conversations');
      if (stored) {
        setStarredIds(JSON.parse(stored));
      }
    } catch (e) {
      console.error('Failed to load starred conversations', e);
    }
  }, []);

  const toggleStar = useCallback((e: React.MouseEvent, id: string) => {
    e.stopPropagation();
    setStarredIds((prev) => {
      const next = prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id];
      try {
        localStorage.setItem('projectx_starred_conversations', JSON.stringify(next));
      } catch (err) {
        console.error('Failed to save starred conversations', err);
      }
      return next;
    });
  }, []);
  const [selectedId, _setSelectedId] = useState<string | null>(null);
  const [showDetailsPanel, setShowDetailsPanel] = useState(true);
  const [users, setUsers] = useState<{ id: string; email: string; role: string }[]>([]);
  const [selectedConvIds, setSelectedConvIds] = useState<string[]>([]);
  const [globalAI, setGlobalAI] = useState(false);
  const [globalAISaving, setGlobalAISaving] = useState(false);

  useEffect(() => {
    if (!studioId) return;
    api<{ users: { id: string; email: string; role: string }[] }>(`/api/v1/studios/${studioId}/users`)
      .then((res) => setUsers(res.users))
      .catch((err) => console.error('Failed to load studio users:', err));
  }, [studioId]);

  const handleLeadUpdated = useCallback((updatedLead: Lead) => {
    setConversations((prev) =>
      prev.map((c) => {
        if (c.leadId === updatedLead.id) {
          return {
            ...c,
            leadStatus: updatedLead.status,
            assignedTo: updatedLead.assignedTo,
          };
        }
        return c;
      })
    );
  }, []);

  const handleAssigneeChange = useCallback(async (newAssignee: string) => {
    if (!selectedId) return;
    const currentConv = conversations.find(c => c.id === selectedId);
    if (!currentConv || !currentConv.leadId) return;
    const leadId = currentConv.leadId;
    
    // Optimistic update
    setConversations((prev) =>
      prev.map((c) => {
        if (c.leadId === leadId) {
          return {
            ...c,
            assignedTo: newAssignee,
          };
        }
        return c;
      })
    );

    try {
      await api(`/api/v1/studios/${studioId}/leads/${leadId}`, {
        method: 'PATCH',
        json: { assignedTo: newAssignee },
      });
    } catch (err) {
      console.error('Failed to assign conversation owner:', err);
    }
  }, [selectedId, conversations, studioId]);

  const setSelectedId = useCallback((id: string | null) => { 
    selectedIdRef.current = id; 
    _setSelectedId(id); 
    setShowDetailsPanel(true);
  }, []);
  const [messages, setMessages] = useState<Message[]>([]);
  const [loadingMessages, setLoadingMessages] = useState(false);
  const [draft, setDraft] = useState('');
  const [sending, setSending] = useState(false);
  const [newReceiverValue, setNewReceiverValue] = useState('');
  const [creatingConversation, setCreatingConversation] = useState(false);
  const [authError, setAuthError] = useState(false);
  const messagesEndRef = useRef<HTMLLIElement>(null);
  const selectedIdRef = useRef<string | null>(null);
  const currentTabRef = useRef<InboxTab>(initialTab);

  // templates, links, jobs state
  const [templates, setTemplates] = useState<Template[]>([]);
  const [loadingTemplates, setLoadingTemplates] = useState(false);
  const [triggerLinks, setTriggerLinks] = useState<TriggerLink[]>([]);
  const [loadingLinks, setLoadingLinks] = useState(false);
  const [jobs, setJobs] = useState<PendingJob[]>([]);
  const [loadingJobs, setLoadingJobs] = useState(false);
  const [tabVisibleJobsCount, setTabVisibleJobsCount] = useState(5);
  const [tabVisibleTemplatesCount, setTabVisibleTemplatesCount] = useState(5);
  const [tabVisibleLinksCount, setTabVisibleLinksCount] = useState(5);

  // Template Form State
  const [newTplName, setNewTplName] = useState('');
  const [newTplBody, setNewTplBody] = useState('');
  const [newTplChannels, setNewTplChannels] = useState<string[]>(['whatsapp_meta']);
  const [newTplMediaUrl, setNewTplMediaUrl] = useState('');
  const [newTplMediaType, setNewTplMediaType] = useState<'image'|'video'|'document'>('image');
  const [newTplFileName, setNewTplFileName] = useState('');
  const [uploadingTplMedia, setUploadingTplMedia] = useState(false);
  const tplFileInputRef = useRef<HTMLInputElement>(null);
  const [aiPrompt, setAiPrompt] = useState('');
  const [generatingAi, setGeneratingAi] = useState(false);
  const [showAiModal, setShowAiModal] = useState(false);
  const [editingTemplateId, setEditingTemplateId] = useState<string | null>(null);

  // Trigger Link Form State
  const [newLinkName, setNewLinkName] = useState('');
  const [newLinkUrl, setNewLinkUrl] = useState('');
  const [editingLinkId, setEditingLinkId] = useState<string | null>(null);

  // Automated Messages Form State
  const [newJobConvId, setNewJobConvId] = useState('');
  const [selectedJobConvIds, setSelectedJobConvIds] = useState<string[]>([]);
  const [searchRecipientQuery, setSearchRecipientQuery] = useState('');
  const [schedulingInProgress, setSchedulingInProgress] = useState(false);
  const [newJobBody, setNewJobBody] = useState('');
  const [newJobSchedTime, setNewJobSchedTime] = useState('');
  const [editingJobId, setEditingJobId] = useState<string | null>(null);
  const [newJobMediaUrl, setNewJobMediaUrl] = useState('');
  const [newJobMediaType, setNewJobMediaType] = useState<'image'|'video'|'document'>('image');
  const [newJobFileName, setNewJobFileName] = useState('');
  const [uploadingJobMedia, setUploadingJobMedia] = useState(false);
  const jobFileInputRef = useRef<HTMLInputElement>(null);

  // Compose Area Attachments & Popovers
  const [attachedMediaUrl, setAttachedMediaUrl] = useState('');
  const [attachedMediaType, setAttachedMediaType] = useState<'image'|'video'|'document'>('image');
  const [attachedFileName, setAttachedFileName] = useState('');
  const [uploadingMedia, setUploadingMedia] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [showTemplatesPopover, setShowTemplatesPopover] = useState(false);
  const [showLinksPopover, setShowLinksPopover] = useState(false);
  const [showAttachmentMenu, setShowAttachmentMenu] = useState(false);
  const [templateSearchQuery, setTemplateSearchQuery] = useState('');
  const [linkSearchQuery, setLinkSearchQuery] = useState('');
  const [visibleTemplatesCount, setVisibleTemplatesCount] = useState(4);
  const [visibleLinksCount, setVisibleLinksCount] = useState(4);

  const handleAuthError = useCallback(() => {
    setAuthError(true);
    router.replace('/login');
  }, [router]);

  useEffect(() => {
    setMounted(true);
  }, []);

  useEffect(() => {
    if (mounted) {
      let filtered = initialConversations;

      // Awaiting Reply intentionally searches across all channels; otherwise
      // scope to the active channel tab.
      if (!unrespondedOnly) {
        filtered = filtered.filter(c => c.channelKind === activeChannel);
      }

      // Filter by inbox tab
      if (inboxTab === 'unread') {
        filtered = filtered.filter(c => c.unreadCount > 0);
      } else if (inboxTab === 'recents') {
        const sevenDaysAgo = new Date();
        sevenDaysAgo.setDate(sevenDaysAgo.getDate() - 7);
        filtered = filtered.filter(c => new Date(c.lastMessageAt) >= sevenDaysAgo);
      } else if (inboxTab === 'starred') {
        filtered = filtered.filter(c => starredIds.includes(c.id));
      }

      // Filter by awaiting reply (if checked)
      if (unrespondedOnly) {
        filtered = filtered.filter(c => c.status === 'open' && c.lastMessageDirection === 'inbound');
      }

      setConversations(filtered);
    }
  }, [mounted, initialConversations, activeChannel, unrespondedOnly, inboxTab, starredIds]);

  useEffect(() => {
    if (mounted && !selectedId && conversations.length > 0 && conversations[0]) {
      setSelectedId(conversations[0].id);
    }
  }, [mounted, conversations, selectedId]);

  useEffect(() => {
    setTabVisibleJobsCount(5);
    setTabVisibleTemplatesCount(5);
    setTabVisibleLinksCount(5);
  }, [currentTab]);

  const selected = conversations.find((c) => c.id === selectedId);

  const refreshConversations = useCallback(async () => {
    try {
      let url = `/api/v1/studios/${studioId}/messaging/conversations?limit=50`;
      // Awaiting Reply intentionally searches across all channels; otherwise
      // scope to the active channel tab.
      if (!unrespondedOnly) {
        url += `&channelKind=${activeChannel}`;
      }
      url += `&status=open`;
      const res = await api<{ conversations: Conversation[] }>(url, { cache: 'no-store' });
      let filtered = res.conversations;

      // Filter by inbox tab
      if (inboxTab === 'unread') {
        filtered = filtered.filter(c => c.unreadCount > 0);
      } else if (inboxTab === 'recents') {
        const sevenDaysAgo = new Date();
        sevenDaysAgo.setDate(sevenDaysAgo.getDate() - 7);
        filtered = filtered.filter(c => new Date(c.lastMessageAt) >= sevenDaysAgo);
      } else if (inboxTab === 'starred') {
        filtered = filtered.filter(c => starredIds.includes(c.id));
      }

      if (unrespondedOnly) {
        filtered = filtered.filter(c => c.status === 'open' && c.lastMessageDirection === 'inbound');
      }
      setConversations(filtered);
    } catch (error) {
      if (error instanceof ApiError && error.status === 401) {
        handleAuthError();
        return;
      }
      throw error;
    }
  }, [studioId, activeChannel, unrespondedOnly, inboxTab, starredIds, handleAuthError]);

  const refreshMessages = useCallback(
    async (convId: string) => {
      setLoadingMessages(true);
      try {
        const res = await api<{ messages: Message[] }>(
          `/api/v1/studios/${studioId}/messaging/conversations/${convId}/messages?limit=200`,
          { cache: 'no-store' }
        );
        setMessages(res.messages);
      } catch (error) {
        if (error instanceof ApiError && error.status === 401) {
          handleAuthError();
          return;
        }
        throw error;
      } finally {
        setLoadingMessages(false);
      }
    },
    [studioId, handleAuthError],
  );

  // Fetch functions for tabs
  const fetchTemplates = useCallback(async () => {
    setLoadingTemplates(true);
    try {
      const res = await api<{ templates: Template[] }>(`/api/v1/studios/${studioId}/messaging/templates`, { cache: 'no-store' });
      setTemplates(res.templates);
    } catch (err) {
      console.error(err);
    } finally {
      setLoadingTemplates(false);
    }
  }, [studioId]);

  const fetchTriggerLinks = useCallback(async () => {
    setLoadingLinks(true);
    try {
      const res = await api<{ triggerLinks: TriggerLink[] }>(`/api/v1/studios/${studioId}/messaging/trigger-links`, { cache: 'no-store' });
      setTriggerLinks(res.triggerLinks);
    } catch (err) {
      console.error(err);
    } finally {
      setLoadingLinks(false);
    }
  }, [studioId]);

  const fetchJobs = useCallback(async () => {
    setLoadingJobs(true);
    try {
      const res = await api<{ jobs: PendingJob[] }>(`/api/v1/studios/${studioId}/messaging/jobs`, { cache: 'no-store' });
      setJobs(res.jobs);
    } catch (err) {
      console.error(err);
    } finally {
      setLoadingJobs(false);
    }
  }, [studioId]);

  // Load resources based on tab
  useEffect(() => {
    if (mounted) {
      if (currentTab === 'snippets') fetchTemplates();
      if (currentTab === 'trigger_links') fetchTriggerLinks();
      if (currentTab === 'automated_messages') fetchJobs();
      if (currentTab === 'conversations') {
        fetchTemplates();
        fetchTriggerLinks();
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [mounted, currentTab]);

  useEffect(() => {
    if (selectedId) {
      refreshMessages(selectedId);
      api(`/api/v1/studios/${studioId}/messaging/conversations/${selectedId}/read`, {
        method: 'POST',
      }).catch((error) => {
        if (error instanceof ApiError && error.status === 401) {
          handleAuthError();
        }
      });
    } else {
      setMessages([]);
    }
  }, [selectedId, studioId, refreshMessages, handleAuthError]);

  useEffect(() => {
    const url = `/api/v1/studios/${studioId}/messaging/stream`;
    const es = new EventSource(url, { withCredentials: true });

    function onEvent(e: MessageEvent) {
      try {
        const evt: SSEEvent = JSON.parse(e.data);
        if (evt.studioId !== studioId) return;
        refreshConversations();
        if (evt.conversationId === selectedIdRef.current) {
          refreshMessages(evt.conversationId);
        }
        if (currentTabRef.current === 'automated_messages') {
          fetchJobs();
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
  }, [studioId, refreshConversations, refreshMessages, fetchJobs]);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  function makeOptimisticOutboundMessage(body: string, attachments?: { type: string; url: string }[]): Message {
    const now = new Date().toISOString();
    return {
      id: `temp-${Date.now()}`,
      conversationId: selectedId ?? '',
      studioId,
      direction: 'outbound',
      sourceKind: 'studio_user',
      body,
      attachments,
      status: 'pending',
      sentAt: now,
      createdAt: now,
    };
  }

  async function handleFileSelect(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    // reset so same file can be re-selected
    e.target.value = '';

    setUploadingMedia(true);
    try {
      const form = new FormData();
      form.append('file', file);
      const res = await fetch(
        `/api/v1/studios/${studioId}/messaging/upload`,
        { method: 'POST', body: form, credentials: 'include' }
      );
      if (!res.ok) throw new Error('upload failed');
      const data = await res.json() as { url: string; filename: string };
      // Use a relative URL so the Next.js proxy at /uploads/* serves it
      setAttachedMediaUrl(data.url);  // e.g. "/uploads/abc.jpg"
      setAttachedFileName(data.filename || file.name);
      // Derive type
      if (file.type.startsWith('video/')) setAttachedMediaType('video');
      else if (file.type.startsWith('image/')) setAttachedMediaType('image');
      else setAttachedMediaType('document');
    } catch {
      alert('Upload failed. Please try again.');
    } finally {
      setUploadingMedia(false);
    }
  }  async function handleTplFileSelect(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    e.target.value = '';

    setUploadingTplMedia(true);
    try {
      const form = new FormData();
      form.append('file', file);
      const res = await fetch(
        `/api/v1/studios/${studioId}/messaging/upload`,
        { method: 'POST', body: form, credentials: 'include' }
      );
      if (!res.ok) throw new Error('upload failed');
      const data = await res.json() as { url: string; filename: string };
      setNewTplMediaUrl(data.url);
      setNewTplFileName(data.filename || file.name);
      if (file.type.startsWith('video/')) setNewTplMediaType('video');
      else if (file.type.startsWith('image/')) setNewTplMediaType('image');
      else setNewTplMediaType('document');
    } catch {
      alert('Upload failed. Please try again.');
    } finally {
      setUploadingTplMedia(false);
    }
  }

  async function handleJobFileSelect(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    e.target.value = '';

    setUploadingJobMedia(true);
    try {
      const form = new FormData();
      form.append('file', file);
      const res = await fetch(
        `/api/v1/studios/${studioId}/messaging/upload`,
        { method: 'POST', body: form, credentials: 'include' }
      );
      if (!res.ok) throw new Error('upload failed');
      const data = await res.json() as { url: string; filename: string };
      setNewJobMediaUrl(data.url);
      setNewJobFileName(data.filename || file.name);
      if (file.type.startsWith('video/')) setNewJobMediaType('video');
      else if (file.type.startsWith('image/')) setNewJobMediaType('image');
      else setNewJobMediaType('document');
    } catch {
      alert('Upload failed. Please try again.');
    } finally {
      setUploadingJobMedia(false);
    }
  }

  async function send() {
    if (!selectedId || (!draft.trim() && !attachedMediaUrl.trim())) return;
    const body = draft.trim();
    const atts = attachedMediaUrl.trim()
      ? [{ type: attachedMediaType, url: attachedMediaUrl.trim() }]
      : [];
    const optimistic = makeOptimisticOutboundMessage(body, atts);
    setSending(true);
    setMessages((current) => [...current, optimistic]);
    setDraft('');
    setAttachedMediaUrl('');
    setAttachedFileName('');
    try {
      await api(`/api/v1/studios/${studioId}/messaging/conversations/${selectedId}/messages`, {
        method: 'POST',
        json: { body, attachments: atts },
      });
      refreshConversations();
    } catch (error) {
      if (error instanceof ApiError && error.status === 401) {
        handleAuthError();
        return;
      }
      setMessages((current) => current.filter((msg) => msg.id !== optimistic.id));
      setDraft(body);
      if (atts.length > 0 && atts[0]) {
        setAttachedMediaUrl(atts[0].url || '');
      }
    } finally {
      setSending(false);
    }
  }

  function insertAvailability() {
    if (!studio?.availabilitySlots || studio.availabilitySlots.length === 0) {
      alert('No availability slots configured in studio settings. Please configure them in Settings.');
      return;
    }
    const upcoming: string[] = [];
    const today = new Date();
    for (let i = 0; i < 7; i++) {
      const d = new Date(today);
      d.setDate(today.getDate() + i);
      const dayName = d.toLocaleDateString('en-US', { weekday: 'long' });
      const matchSlots = studio.availabilitySlots.filter(s => s.day === dayName);
      for (const slot of matchSlots) {
         if (!slot.times) continue;
         const dateStr = d.toLocaleDateString('en-US', { weekday: 'short', month: 'short', day: 'numeric' });
         for (const t of slot.times) {
           upcoming.push(`${dateStr} at ${t}`);
         }
      }
    }
    if (upcoming.length === 0) {
      alert('No availability slots found for the next 7 days.');
      return;
    }
    const text = "Please select a time slot:\n\n" + upcoming.map(s => `- ${s}`).join('\n');
    setDraft(prev => prev + (prev ? '\n\n' : '') + text);
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
      if (error instanceof ApiError && error.status === 401) {
        handleAuthError();
        return;
      }
      console.error('Failed to create conversation:', error);
    } finally {
      setCreatingConversation(false);
    }
  }

  const handleChannelSwitch = (kind: ChannelKind) => {
    setActiveChannel(kind);
    setSelectedId(null);
    setMessages([]);
    const p = new URLSearchParams(searchParams.toString());
    p.set('channel', kind);
    router.replace(`${pathname}?${p.toString()}`, { scroll: false });
  };

  useEffect(() => {
    refreshConversations();
  }, [activeChannel, unrespondedOnly, refreshConversations]);

  // Handler calls for Templates
  async function handleCreateTemplate(e: React.FormEvent) {
    e.preventDefault();
    if (!newTplName.trim() || !newTplBody.trim()) return;
    try {
      const atts = newTplMediaUrl.trim() ? [{ type: newTplMediaType, url: newTplMediaUrl.trim() }] : [];
      if (editingTemplateId) {
        await api(`/api/v1/studios/${studioId}/messaging/templates/${editingTemplateId}`, {
          method: 'PUT',
          json: {
            name: newTplName.trim(),
            body: newTplBody.trim(),
            channelKinds: newTplChannels,
            attachments: atts,
          },
        });
        setEditingTemplateId(null);
      } else {
        await api(`/api/v1/studios/${studioId}/messaging/templates`, {
          method: 'POST',
          json: {
            name: newTplName.trim(),
            body: newTplBody.trim(),
            channelKinds: newTplChannels,
            attachments: atts,
          },
        });
      }
      setNewTplName('');
      setNewTplBody('');
      setNewTplMediaUrl('');
      setNewTplFileName('');
      fetchTemplates();
    } catch (err) {
      console.error('Failed to save template:', err);
    }
  }

  const handleConversationAIToggle = useCallback((conversationId: string, enabled: boolean) => {
    setConversations((prev) => prev.map((c) => (c.id === conversationId ? { ...c, aiEnabled: enabled } : c)));
  }, []);

  async function toggleGlobalAI() {
    if (globalAISaving) return;
    const next = !globalAI;
    setGlobalAISaving(true);
    setGlobalAI(next);
    try {
      await api(`/api/v1/studios/${studioId}/messaging/conversations/ai/bulk`, {
        method: 'POST',
        json: { enabled: next },
      });
      // Reflect change on all loaded conversations
      setConversations((prev) => prev.map((c) => ({ ...c, aiEnabled: next })));
    } catch {
      setGlobalAI(!next);
    } finally {
      setGlobalAISaving(false);
    }
  }

  async function handleDeleteSelectedConversations() {
    if (selectedConvIds.length === 0) return;
    if (!confirm(`Delete ${selectedConvIds.length} selected conversation(s)?`)) return;
    const ids = selectedConvIds;
    const prevConversations = conversations;
    setConversations((prev) => prev.filter((c) => !ids.includes(c.id)));
    setSelectedConvIds([]);
    try {
      await Promise.all(
        ids.map((id) =>
          api(`/api/v1/studios/${studioId}/messaging/conversations/${id}`, { method: 'DELETE' })
        )
      );
    } catch (err) {
      console.error('Failed to delete conversations:', err);
      setConversations(prevConversations);
    }
  }

  async function handleDeleteTemplate(id: string) {
    if (!confirm('Are you sure you want to delete this template?')) return;
    try {
      await api(`/api/v1/studios/${studioId}/messaging/templates/${id}`, {
        method: 'DELETE',
      });
      fetchTemplates();
    } catch (err) {
      console.error('Failed to delete template:', err);
    }
  }

  async function handleAiGenerate() {
    if (!aiPrompt.trim()) return;
    setGeneratingAi(true);
    try {
      const res = await api<{ body: string }>(`/api/v1/studios/${studioId}/messaging/ai/generate`, {
        method: 'POST',
        json: { prompt: aiPrompt.trim() },
      });
      setNewTplBody(res.body);
      setShowAiModal(false);
      setAiPrompt('');
    } catch (err) {
      console.error('Failed AI generation:', err);
    } finally {
      setGeneratingAi(false);
    }
  }

  // Handler calls for Trigger Links
  async function handleCreateTriggerLink(e: React.FormEvent) {
    e.preventDefault();
    if (!newLinkName.trim() || !newLinkUrl.trim()) return;
    try {
      if (editingLinkId) {
        await api(`/api/v1/studios/${studioId}/messaging/trigger-links/${editingLinkId}`, {
          method: 'PUT',
          json: {
            name: newLinkName.trim(),
            url: newLinkUrl.trim(),
          },
        });
        setEditingLinkId(null);
      } else {
        await api(`/api/v1/studios/${studioId}/messaging/trigger-links`, {
          method: 'POST',
          json: {
            name: newLinkName.trim(),
            url: newLinkUrl.trim(),
          },
        });
      }
      setNewLinkName('');
      setNewLinkUrl('');
      fetchTriggerLinks();
    } catch (err) {
      console.error('Failed to save trigger link:', err);
    }
  }

  async function handleDeleteTriggerLink(id: string) {
    if (!confirm('Are you sure you want to delete this trigger link?')) return;
    try {
      await api(`/api/v1/studios/${studioId}/messaging/trigger-links/${id}`, {
        method: 'DELETE',
      });
      fetchTriggerLinks();
    } catch (err) {
      console.error('Failed to delete trigger link:', err);
    }
  }

  // Handler calls for jobs
  async function handleSaveJob(e: React.FormEvent) {
    e.preventDefault();
    if (!newJobBody.trim() && !newJobMediaUrl.trim()) return;
    try {
      const scheduledForISO = newJobSchedTime ? new Date(newJobSchedTime).toISOString() : '';
      const atts = newJobMediaUrl.trim()
        ? [{ type: newJobMediaType, url: newJobMediaUrl.trim(), name: newJobFileName }]
        : [];
      if (editingJobId) {
        await api(`/api/v1/studios/${studioId}/messaging/jobs/${editingJobId}`, {
          method: 'PUT',
          json: {
            body: newJobBody.trim(),
            scheduledFor: scheduledForISO,
            attachments: atts,
          },
        });
        setEditingJobId(null);
      } else {
        if (selectedJobConvIds.length === 0) {
          alert('Please select at least one recipient');
          return;
        }
        setSchedulingInProgress(true);
        try {
          for (const convId of selectedJobConvIds) {
            await api(`/api/v1/studios/${studioId}/messaging/jobs`, {
              method: 'POST',
              json: {
                conversationId: convId,
                body: newJobBody.trim(),
                scheduledFor: scheduledForISO,
                attachments: atts,
              },
            });
          }
          setSelectedJobConvIds([]);
        } finally {
          setSchedulingInProgress(false);
        }
      }
      setNewJobBody('');
      setNewJobSchedTime('');
      setNewJobMediaUrl('');
      setNewJobFileName('');
      fetchJobs();
    } catch (err) {
      console.error('Failed to save scheduled message:', err);
    }
  }

  async function handleTriggerJob(id: number) {
    try {
      await api(`/api/v1/studios/${studioId}/messaging/jobs/${id}/trigger`, {
        method: 'POST',
      });
      setTimeout(fetchJobs, 300);
    } catch (err) {
      console.error('Failed to trigger job:', err);
    }
  }

  async function handleCancelJob(id: number) {
    if (!confirm('Are you sure you want to cancel this scheduled message?')) return;
    try {
      await api(`/api/v1/studios/${studioId}/messaging/jobs/${id}`, {
        method: 'DELETE',
      });
      fetchJobs();
    } catch (err) {
      console.error('Failed to cancel job:', err);
    }
  }

  if (authError) {
    return null;
  }

  return (
    <div
      className="flex flex-col h-[calc(100vh-3.5rem)] overflow-hidden bg-white dark:bg-zinc-950 border border-zinc-200 dark:border-zinc-950"
    >
      {mounted && (
        <HeaderActions>
          <select
            value={activeChannel}
            onChange={(e) => handleChannelSwitch(e.target.value as ChannelKind)}
            className="rounded border border-zinc-200 bg-white px-2 py-1 text-[10px] font-black uppercase tracking-wider text-zinc-700 focus:outline-none dark:border-zinc-800 dark:bg-zinc-900 dark:text-zinc-200 cursor-pointer shadow-sm"
            suppressHydrationWarning
          >
            <option value="whatsapp_web">WhatsApp (QR / Web)</option>
            <option value="whatsapp_meta">WhatsApp (Cloud API)</option>
            <option value="instagram_meta">Instagram</option>
            <option value="messenger_meta">Messenger</option>
            <option value="sms">SMS</option>
            <option value="telegram_mtproto">Telegram (QR)</option>
            <option value="telegram">Telegram (Bot)</option>
          </select>
        </HeaderActions>
      )}
      {/* ── Top Navigation Tabs ─────────────────── */}
      <div className="flex items-center justify-between border-b border-zinc-200 bg-white px-6 dark:border-zinc-800 dark:bg-zinc-950 shrink-0 z-20">
        <div className="flex gap-6">
          {(
            [
              { id: 'conversations', label: 'Conversations' },
              { id: 'automated_messages', label: 'Manual Actions' },
              { id: 'snippets', label: 'Snippets' },
              { id: 'trigger_links', label: 'Trigger Links' },
            ] as const
          ).map((t) => (
            <button
              key={t.id}
              type="button"
              onClick={() => setCurrentTab(t.id)}
              className={cn(
                "py-3 text-xs font-black uppercase tracking-wider transition-all border-b-2",
                currentTab === t.id
                  ? "border-brand-500 text-brand-500 dark:text-brand-400"
                  : "border-transparent text-zinc-500 hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200"
              )}
            >
              {t.label}
            </button>
          ))}
        </div>
      </div>

      {/* ── Tab Content Renderer ─────────────────── */}
      <div className="flex flex-1 min-h-0">
        {currentTab === 'conversations' && (
          <>
            {/* Sidebar */}
            <aside
              className="hidden w-80 shrink-0 flex-col border-r border-zinc-200 sm:flex bg-[#f8f9fa] dark:border-zinc-800 dark:bg-zinc-900"
            >


              {/* Global AI toggle */}
              <div className="flex items-center justify-between px-3 py-2 border-b border-zinc-200 dark:border-zinc-800">
                <div className="flex items-center gap-2">
                  <Bot className={cn('h-3.5 w-3.5', globalAI ? 'text-emerald-500' : 'text-zinc-400')} />
                  <span className="text-[10px] font-black uppercase tracking-wider text-zinc-500 dark:text-zinc-400">
                    AI Auto-Reply
                  </span>
                </div>
                <button
                  onClick={toggleGlobalAI}
                  disabled={globalAISaving}
                  title={globalAI ? 'Disable AI for all chats' : 'Enable AI for all chats'}
                  className={cn(
                    'relative inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full transition-colors duration-300 focus:outline-none disabled:opacity-50',
                    globalAI ? 'bg-emerald-500' : 'bg-zinc-300 dark:bg-zinc-700',
                  )}
                >
                  <span className={cn(
                    'pointer-events-none flex h-4 w-4 items-center justify-center rounded-full bg-white shadow transition-transform duration-300',
                    globalAI ? 'translate-x-4' : 'translate-x-0.5',
                  )}>
                    {globalAISaving
                      ? <Loader2 className="h-2.5 w-2.5 animate-spin text-zinc-400" />
                      : <Bot className={cn('h-2 w-2', globalAI ? 'text-emerald-600' : 'text-zinc-400')} />
                    }
                  </span>
                </button>
              </div>

              {/* Inbox Tabs (Unread, All, Recents, Starred) */}
              <div className="px-3 pt-2 pb-0 border-b border-zinc-250 dark:border-zinc-800 bg-[#f8f9fa] dark:bg-zinc-900">
                <div className="flex items-center justify-between gap-1">
                  <button
                    type="button"
                    onClick={() => setInboxTab('unread')}
                    className={cn(
                      "flex flex-col items-center gap-1.5 pb-2 flex-1 text-[10px] font-black uppercase tracking-wider transition-all border-b-2",
                      inboxTab === 'unread'
                        ? "border-brand-500 text-brand-500 dark:text-brand-400"
                        : "border-transparent text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-200"
                    )}
                  >
                    <div className="relative">
                      <Mail className="h-4 w-4" />
                      {conversations.filter(c => c.unreadCount > 0).length > 0 && (
                        <span className="absolute -right-2.5 -top-1.5 grid h-4 min-w-[16px] place-items-center rounded-full bg-brand-500 px-1 text-[8px] font-black text-white leading-none shadow-sm">
                          {conversations.filter(c => c.unreadCount > 0).length}
                        </span>
                      )}
                    </div>
                    Unread
                  </button>
                  <button
                    type="button"
                    onClick={() => setInboxTab('all')}
                    className={cn(
                      "flex flex-col items-center gap-1.5 pb-2 flex-1 text-[10px] font-black uppercase tracking-wider transition-all border-b-2",
                      inboxTab === 'all'
                        ? "border-brand-500 text-brand-500 dark:text-brand-400"
                        : "border-transparent text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-200"
                    )}
                  >
                    <Inbox className="h-4 w-4" />
                    All
                  </button>
                  <button
                    type="button"
                    onClick={() => setInboxTab('recents')}
                    className={cn(
                      "flex flex-col items-center gap-1.5 pb-2 flex-1 text-[10px] font-black uppercase tracking-wider transition-all border-b-2",
                      inboxTab === 'recents'
                        ? "border-brand-500 text-brand-500 dark:text-brand-400"
                        : "border-transparent text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-200"
                    )}
                  >
                    <Clock className="h-4 w-4" />
                    Recents
                  </button>
                  <button
                    type="button"
                    onClick={() => setInboxTab('starred')}
                    className={cn(
                      "flex flex-col items-center gap-1.5 pb-2 flex-1 text-[10px] font-black uppercase tracking-wider transition-all border-b-2",
                      inboxTab === 'starred'
                        ? "border-brand-500 text-brand-500 dark:text-brand-400"
                        : "border-transparent text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-200"
                    )}
                  >
                    <Star className="h-4 w-4" />
                    Starred
                  </button>
                </div>
              </div>

              {/* Select All and Filter Row */}
              <div className="px-4 py-2 flex items-center justify-between border-b border-zinc-200 dark:border-zinc-800 bg-[#f8f9fa] dark:bg-zinc-900 shrink-0 z-10">
                <div className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    checked={conversations.length > 0 && selectedConvIds.length === conversations.length}
                    onChange={(e) => {
                      if (e.target.checked) {
                        setSelectedConvIds(conversations.map((c) => c.id));
                      } else {
                        setSelectedConvIds([]);
                      }
                    }}
                    className="rounded border-zinc-300 dark:border-zinc-700 bg-transparent text-brand-500 focus:ring-brand-500 h-3.5 w-3.5 cursor-pointer"
                  />
                  <span className="text-[10px] font-black uppercase tracking-wider text-zinc-400">
                    {selectedConvIds.length > 0 ? `${selectedConvIds.length} Selected` : 'Select all'}
                  </span>
                </div>
                {selectedConvIds.length > 0 ? (
                  <div className="flex items-center gap-1.5 animate-in fade-in duration-200">
                    <button
                      type="button"
                      onClick={() => {
                        setStarredIds((prev) => {
                          const toStar = selectedConvIds.filter(id => !prev.includes(id));
                          const next = toStar.length > 0
                            ? [...prev, ...toStar]
                            : prev.filter(id => !selectedConvIds.includes(id));
                          try {
                            localStorage.setItem('projectx_starred_conversations', JSON.stringify(next));
                          } catch (err) {
                            console.error('Failed to save starred conversations', err);
                          }
                          return next;
                        });
                        // Don't clear selection — user may want to unstar immediately
                      }}
                      className={cn(
                        "p-1 rounded transition-colors",
                        selectedConvIds.every(id => starredIds.includes(id))
                          ? "text-amber-500 hover:text-zinc-400"
                          : "text-zinc-400 hover:text-amber-500"
                      )}
                      title={selectedConvIds.every(id => starredIds.includes(id)) ? "Unstar selected" : "Star selected"}
                    >
                      <Star className={cn("h-3.5 w-3.5", selectedConvIds.every(id => starredIds.includes(id)) ? "fill-current" : "fill-none")} />
                    </button>
                    <button
                      type="button"
                      onClick={() => handleDeleteSelectedConversations()}
                      className="p-1 text-zinc-400 hover:text-rose-500 rounded transition-colors"
                      title="Delete selected"
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </button>
                    <button
                      type="button"
                      onClick={() => setSelectedConvIds([])}
                      className="p-1 text-zinc-400 hover:text-zinc-600 rounded transition-colors ml-1"
                      title="Clear selection"
                    >
                      <X className="h-3.5 w-3.5" />
                    </button>
                  </div>
                ) : (
                  <button
                    type="button"
                    onClick={() => setUnrespondedOnly(!unrespondedOnly)}
                    className={cn(
                      "flex items-center gap-1 px-2.5 py-0.5 rounded text-[9px] font-extrabold uppercase tracking-wider transition-all border",
                      unrespondedOnly
                        ? "bg-rose-500 border-rose-500 text-white shadow-sm"
                        : "bg-transparent border-zinc-200 text-zinc-500 hover:border-zinc-300 dark:border-zinc-700 dark:text-zinc-400"
                    )}
                  >
                    <span className={cn("h-1 w-1 rounded-full", unrespondedOnly ? "bg-white animate-pulse" : "bg-rose-500")} />
                    Awaiting Reply
                  </button>
                )}
                {unrespondedOnly && !selectedConvIds.length && (
                  <span className="text-[9px] font-semibold uppercase tracking-wider text-zinc-400">
                    Showing all channels
                  </span>
                )}
              </div>

              {/* New Conversation Input */}
              <div className="px-4 py-3 border-b border-zinc-200 dark:border-zinc-800 bg-[#f8f9fa] dark:bg-zinc-900">
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
                    placeholder={
                      activeChannel === 'whatsapp_meta' || activeChannel === 'whatsapp_web' || activeChannel === 'sms'
                        ? 'Search name/number, or start new...'
                        : activeChannel === 'telegram' || activeChannel === 'telegram_mtproto'
                        ? 'Search, or Telegram chat ID...'
                        : 'Search, or Messenger ID...'
                    }
                    className="w-full rounded border border-zinc-200 bg-white py-1.5 pl-3 pr-10 text-xs font-semibold text-zinc-900 placeholder:text-zinc-400 focus:border-brand-500 focus:outline-none dark:border-zinc-800 dark:bg-zinc-900 dark:text-zinc-100 dark:placeholder:text-zinc-500"
                    suppressHydrationWarning
                  />
                  <button
                    type="submit"
                    disabled={!newReceiverValue.trim() || creatingConversation}
                    className="absolute right-1 top-1/2 -translate-y-1/2 grid h-6.5 w-6.5 place-items-center rounded bg-brand-500 text-white transition-all hover:bg-brand-600 disabled:opacity-0"
                    suppressHydrationWarning
                  >
                    <Send className="h-3 w-3" />
                  </button>
                </form>
              </div>

              {/* Conversation List — filtered live by the box above, matching
                  against contact name and number (both the raw JID/value and
                  its display-cleaned form), so typing a saved contact's name
                  or a phone number narrows the list the same way it would in
                  a normal chat app's search bar. Falls through to the New
                  Conversation Input's existing "start a chat with this
                  number" behavior on submit when nothing matches. */}
              <div className="flex-1 overflow-y-auto no-scrollbar">
                {mounted ? (
                  <ul className="space-y-0">
                    {conversations
                      .filter((c) => {
                        const query = newReceiverValue.trim().toLowerCase();
                        if (!query) return true;
                        const name = (c.contactDisplayName || '').toLowerCase();
                        const val = (c.contactValue || '').toLowerCase();
                        const cleanVal = displayContact(c.contactValue || '').toLowerCase();
                        return name.includes(query) || val.includes(query) || cleanVal.includes(query);
                      })
                      .map((c) => (
                      <li key={c.id}>
                        <div
                          role="button"
                          tabIndex={0}
                          onClick={() => setSelectedId(c.id)}
                          onKeyDown={(e) => {
                            if (e.key === 'Enter' || e.key === ' ') {
                              e.preventDefault();
                              setSelectedId(c.id);
                            }
                          }}
                          className={cn(
                            'group relative flex w-full items-start gap-2.5 px-3 py-3 text-left transition-all duration-350 cursor-pointer border-l-4 focus:outline-none border-b border-zinc-200 dark:border-zinc-800',
                            selectedId === c.id
                              ? 'border-brand-500 bg-sky-50 dark:bg-brand-950/20'
                              : selectedConvIds.includes(c.id)
                              ? 'border-teal-400 bg-teal-50/60 dark:bg-teal-900/20'
                              : 'border-transparent bg-white dark:bg-zinc-950 hover:bg-zinc-50 dark:hover:bg-white/5',
                          )}
                          suppressHydrationWarning
                        >
                          <input
                            type="checkbox"
                            checked={selectedConvIds.includes(c.id)}
                            onClick={(e) => e.stopPropagation()}
                            onChange={(e) => {
                              if (e.target.checked) {
                                setSelectedConvIds((prev) => [...prev, c.id]);
                              } else {
                                setSelectedConvIds((prev) => prev.filter((id) => id !== c.id));
                              }
                            }}
                            className="mt-3 rounded border-zinc-300 dark:border-zinc-700 bg-transparent text-brand-500 focus:ring-brand-500 h-3.5 w-3.5 cursor-pointer shrink-0"
                          />
                          <ChannelAvatar kind={c.channelKind} name={c.contactDisplayName || c.contactValue} active={selectedId === c.id} />
                          <div className="min-w-0 flex-1">
                            <div className="flex items-center justify-between gap-1.5">
                              <span className="truncate text-xs font-black text-zinc-800 dark:text-zinc-100">
                                {c.contactDisplayName || displayContact(c.contactValue)}
                              </span>
                              <span
                                className="shrink-0 text-[9px] font-semibold text-zinc-400"
                                suppressHydrationWarning
                              >
                                {relativeTime(c.lastMessageAt)}
                              </span>
                            </div>
                            
                            <p className="mt-0.5 truncate text-[11px] font-medium text-zinc-500 dark:text-zinc-400">
                              {c.lastMessageDirection === 'outbound' && <span className="text-zinc-400">You: </span>}
                              {c.lastMessagePreview}
                            </p>

                            <div className="mt-1.5 flex items-center justify-between">
                              <div className="flex items-center gap-1.5 shrink-0">
                                {c.leadStatus && (
                                  <span className={cn(
                                    "px-1.5 py-0.5 rounded text-[8px] font-extrabold uppercase tracking-wider shrink-0",
                                    getLeadStatusStyles(c.leadStatus, selectedId === c.id)
                                  )}>
                                    {getLeadStatusLabel(c.leadStatus)}
                                  </span>
                                )}
                                {c.assignedTo && (
                                  <span className="px-1.5 py-0.5 rounded text-[8px] font-bold uppercase bg-zinc-100 text-zinc-600 dark:bg-white/5 dark:text-zinc-400 shrink-0">
                                    {c.assignedTo.split('@')[0]}
                                  </span>
                                )}
                              </div>
                              
                              <div className="flex items-center gap-1.5">
                                {c.unreadCount > 0 && (
                                  <span className="grid h-4 min-w-[16px] place-items-center rounded-full bg-brand-500 px-1 text-[8px] font-black text-white">
                                    {c.unreadCount}
                                  </span>
                                )}
                                <button
                                  type="button"
                                  onClick={(e) => toggleStar(e, c.id)}
                                  className={cn(
                                    "p-0.5 rounded transition-all hover:scale-110 active:scale-90",
                                    starredIds.includes(c.id)
                                      ? "text-amber-500"
                                      : "text-zinc-300 hover:text-zinc-500 dark:text-zinc-600 dark:hover:text-zinc-400"
                                  )}
                                >
                                  <Star className={cn("h-3.5 w-3.5", starredIds.includes(c.id) ? "fill-current" : "fill-none")} />
                                </button>
                              </div>
                            </div>
                          </div>
                        </div>
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

            {/* Main Chat Pane */}
            <section className="flex min-w-0 flex-1 flex-col relative">
              {selected ? (
                <>
                  <header className="z-10 flex h-14 items-center justify-between border-b border-zinc-200 bg-white px-5 dark:border-zinc-800 dark:bg-zinc-950 shrink-0">
                    <div
                      className="flex items-center gap-3 cursor-pointer select-none"
                      onClick={() => setShowDetailsPanel(!showDetailsPanel)}
                      title="Toggle details panel"
                    >
                      <ChannelAvatar
                        kind={selected.channelKind}
                        name={selected.contactDisplayName || displayContact(selected.contactValue)}
                      />
                      <div className="min-w-0">
                        <div className="truncate text-xs font-black text-zinc-900 dark:text-zinc-100">
                          {selected.contactDisplayName || displayContact(selected.contactValue)}
                        </div>
                        <div className="flex items-center gap-1.5 truncate text-[9px] font-bold uppercase tracking-[0.15em] text-zinc-400">
                          <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
                          {channelLabel(selected.channelKind)}{selected.contactDisplayName && selected.contactDisplayName !== displayContact(selected.contactValue) ? ` · ${displayContact(selected.contactValue)}` : ''}
                        </div>
                      </div>
                    </div>
                    <div className="flex items-center gap-1">
                      {selected.leadId && (
                        <div className="relative mr-2">
                          <select
                            value={selected.assignedTo || ''}
                            onChange={(e) => handleAssigneeChange(e.target.value)}
                            className="appearance-none pl-2.5 pr-6 py-1 rounded border border-zinc-200 dark:border-zinc-800 text-[10px] font-bold text-zinc-600 dark:text-zinc-400 bg-white/5 hover:bg-white/10 transition-colors cursor-pointer"
                            title="Assign Owner"
                          >
                            <option value="">Unassigned</option>
                            {users.map((u) => (
                              <option key={u.id} value={u.email}>
                                {u.email.split('@')[0]}
                              </option>
                            ))}
                            {selected.assignedTo && !users.some(u => u.email === selected.assignedTo) && (
                              <option value={selected.assignedTo}>{selected.assignedTo.split('@')[0]}</option>
                            )}
                          </select>
                          <span className="absolute right-2 top-1/2 -translate-y-1/2 text-[6px] text-zinc-400 pointer-events-none">▼</span>
                        </div>
                      )}
                      <button className="p-2 text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-200 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800 transition-colors" title="Call contact">
                        <Phone className="h-4 w-4" />
                      </button>
                      <button className="p-2 text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-200 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800 transition-colors" title="Video call">
                        <Video className="h-4 w-4" />
                      </button>
                      <button
                        onClick={(e) => toggleStar(e, selected.id)}
                        className={cn(
                          "p-2 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800 transition-all",
                          starredIds.includes(selected.id) ? "text-amber-500" : "text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-200"
                        )}
                        title={starredIds.includes(selected.id) ? "Unstar conversation" : "Star conversation"}
                      >
                        <Star className={cn("h-4 w-4", starredIds.includes(selected.id) ? "fill-current" : "fill-none")} />
                      </button>
                      <button className="p-2 text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-200 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800 transition-colors" title="Tags">
                        <Tag className="h-4 w-4" />
                      </button>
                      <button className="p-2 text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-200 rounded hover:bg-zinc-100 dark:hover:bg-zinc-800 transition-colors" title="Archive">
                        <Archive className="h-4 w-4" />
                      </button>
                    </div>
                  </header>

                  <div
                    className="relative flex-1 overflow-y-auto no-scrollbar px-5 py-6 bg-[#f4f5f6] dark:bg-zinc-900/40"
                  >
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

                  <footer
                    className="z-10 border-t border-zinc-200 p-4 bg-white dark:border-zinc-800 dark:bg-zinc-950 shrink-0"
                  >
                    {/* Hidden file input */}
                    <input
                      ref={fileInputRef}
                      type="file"
                      accept="image/*,video/*,.pdf,.doc,.docx"
                      className="hidden"
                      onChange={handleFileSelect}
                    />

                    {/* Attached file preview — like WhatsApp */}
                    {attachedMediaUrl && (
                      <div className="mb-2 relative w-max max-w-[200px] rounded-2xl overflow-hidden border-2 border-violet-400/40 shadow-lg">
                        {attachedMediaType === 'image' ? (
                          <img
                            src={attachedMediaUrl}
                            alt="Attachment preview"
                            className="w-full h-32 object-cover"
                          />
                        ) : attachedMediaType === 'video' ? (
                          <video src={attachedMediaUrl} className="w-full h-32 object-cover" muted />
                        ) : (
                          <div className="flex items-center gap-2 px-3 py-4 bg-violet-50 dark:bg-neutral-800">
                            <FileText className="h-8 w-8 text-violet-500 shrink-0" />
                            <span className="text-xs font-semibold text-zinc-700 dark:text-zinc-300 truncate">{attachedFileName}</span>
                          </div>
                        )}
                        {/* Remove button */}
                        <button
                          type="button"
                          onClick={() => { setAttachedMediaUrl(''); setAttachedFileName(''); }}
                          className="absolute top-1 right-1 p-1 rounded-full bg-black/50 hover:bg-black/70 text-white transition-all"
                        >
                          <X className="h-3 w-3" />
                        </button>
                        {attachedFileName && (
                          <div className="absolute bottom-0 left-0 right-0 bg-black/40 px-2 py-0.5 text-[9px] text-white font-semibold truncate">
                            {attachedFileName}
                          </div>
                        )}
                      </div>
                    )}

                    {/* Upload progress */}
                    {uploadingMedia && (
                      <div className="mb-2 flex items-center gap-2 text-xs text-violet-500 font-semibold">
                        <Loader2 className="h-4 w-4 animate-spin" /> Uploading...
                      </div>
                    )}

                    <form
                      onSubmit={(e) => {
                        e.preventDefault();
                        send();
                      }}
                      className="flex items-end gap-3"
                    >
                      <div className="relative shrink-0">
                        <button
                          type="button"
                          onClick={() => {
                            setShowAttachmentMenu(!showAttachmentMenu);
                            setShowTemplatesPopover(false);
                            setShowLinksPopover(false);
                            setTemplateSearchQuery('');
                            setLinkSearchQuery('');
                            setVisibleTemplatesCount(4);
                            setVisibleLinksCount(4);
                          }}
                          className={cn(
                            "flex h-10 w-10 items-center justify-center rounded border border-zinc-200 bg-zinc-50 text-zinc-650 hover:bg-zinc-100 hover:text-zinc-700 transition-all dark:border-zinc-800 dark:bg-zinc-900 dark:text-zinc-400 dark:hover:bg-zinc-800",
                            (showAttachmentMenu || showTemplatesPopover || showLinksPopover || attachedMediaUrl) && "bg-brand-500 border-brand-500 text-white hover:bg-brand-600 dark:bg-brand-500 dark:text-white"
                          )}
                          title="Attach templates, links, or files"
                        >
                          {uploadingMedia ? (
                            <Loader2 className="h-5 w-5 animate-spin" />
                          ) : (
                            <Paperclip className="h-5 w-5" />
                          )}
                        </button>

                        {/* Dropdowns relative to this Paperclip button container */}
                        {/* 1. Main Menu */}
                        {showAttachmentMenu && (
                          <div className="absolute bottom-12 left-0 z-35 w-48 p-1.5 rounded border border-zinc-200 bg-white shadow-lg dark:border-zinc-800 dark:bg-zinc-900 space-y-0.5">
                            <button
                              type="button"
                              onClick={() => {
                                setShowTemplatesPopover(true);
                                setShowAttachmentMenu(false);
                                setTemplateSearchQuery('');
                                setVisibleTemplatesCount(4);
                              }}
                              className="flex items-center gap-2.5 w-full text-left px-3 py-2 rounded text-xs font-bold text-zinc-700 hover:bg-zinc-50 dark:text-zinc-300 dark:hover:bg-zinc-800 transition-colors"
                            >
                              <span className="font-semibold text-xs tracking-wider">{`{...}`}</span> Templates
                            </button>
                            <button
                              type="button"
                              onClick={() => {
                                setShowLinksPopover(true);
                                setShowAttachmentMenu(false);
                                setLinkSearchQuery('');
                                setVisibleLinksCount(4);
                              }}
                              className="flex items-center gap-2.5 w-full text-left px-3 py-2 rounded text-xs font-bold text-zinc-700 hover:bg-zinc-50 dark:text-zinc-300 dark:hover:bg-zinc-800 transition-colors"
                            >
                              <LinkIcon className="h-3.5 w-3.5" /> Tracked Link
                            </button>
                            <button
                              type="button"
                              onClick={() => {
                                fileInputRef.current?.click();
                                setShowAttachmentMenu(false);
                              }}
                              className="flex items-center gap-2.5 w-full text-left px-3 py-2 rounded text-xs font-bold text-zinc-700 hover:bg-zinc-50 dark:text-zinc-300 dark:hover:bg-zinc-800 transition-colors"
                            >
                              <ImageIcon className="h-3.5 w-3.5" /> Photo / File
                            </button>
                            <button
                              type="button"
                              onClick={() => {
                                insertAvailability();
                                setShowAttachmentMenu(false);
                              }}
                              className="flex items-center gap-2.5 w-full text-left px-3 py-2 rounded text-xs font-bold text-zinc-700 hover:bg-zinc-50 dark:text-zinc-300 dark:hover:bg-zinc-800 transition-colors"
                            >
                              <Calendar className="h-3.5 w-3.5" /> Availability
                            </button>
                          </div>
                        )}

                        {/* 2. Templates popover */}
                        {showTemplatesPopover && (
                          <div 
                            onScroll={(e) => {
                              const target = e.currentTarget;
                              if (target.scrollHeight - target.scrollTop <= target.clientHeight + 10) {
                                setVisibleTemplatesCount((prev) => prev + 4);
                              }
                            }}
                            className="absolute bottom-12 left-0 z-35 w-64 max-h-48 overflow-y-auto p-1.5 rounded border border-zinc-200 bg-white shadow-lg dark:border-zinc-800 dark:bg-zinc-900 space-y-0.5"
                          >
                            <div className="flex items-center gap-2 px-2 py-1 border-b border-zinc-150 dark:border-zinc-800 mb-1">
                              <input
                                type="text"
                                placeholder="Search templates..."
                                value={templateSearchQuery}
                                onChange={(e) => {
                                  setTemplateSearchQuery(e.target.value);
                                  setVisibleTemplatesCount(4);
                                }}
                                className="flex-1 bg-zinc-50 dark:bg-zinc-800 border border-zinc-200 dark:border-zinc-700 rounded px-2 py-0.5 text-[11px] text-zinc-800 dark:text-zinc-200 placeholder-zinc-400 focus:outline-none focus:ring-1 focus:ring-brand-500"
                                autoFocus
                              />
                              <button 
                                type="button" 
                                onClick={() => {
                                  setShowTemplatesPopover(false);
                                  setShowAttachmentMenu(true);
                                  setTemplateSearchQuery('');
                                }}
                                className="text-[10px] font-bold text-brand-500 hover:underline shrink-0"
                              >
                                Back
                              </button>
                            </div>
                            {(() => {
                              const filteredTemplates = templates.filter(t => 
                                t.name.toLowerCase().includes(templateSearchQuery.toLowerCase()) ||
                                t.body.toLowerCase().includes(templateSearchQuery.toLowerCase())
                              );
                              const displayedTemplates = filteredTemplates.slice(0, visibleTemplatesCount);

                              if (filteredTemplates.length === 0) {
                                return <div className="text-[10px] font-semibold text-zinc-400 p-2 text-center">No templates found</div>;
                              }
                              return displayedTemplates.map((t) => (
                                <button
                                  key={t.id}
                                  type="button"
                                  onClick={() => {
                                    setDraft((d) => d + (d ? '\n' : '') + t.body);
                                    if (t.attachments && t.attachments.length > 0) {
                                      const att = t.attachments[0];
                                      if (att) {
                                        const url = att.url || '';
                                        setAttachedMediaUrl(url);
                                        setAttachedMediaType(att.type as 'image'|'video'|'document' || 'image');
                                        setAttachedFileName(url.split('/').pop() || '');
                                      }
                                    }
                                    setShowTemplatesPopover(false);
                                  }}
                                  className="w-full text-left p-2 rounded text-xs font-semibold text-zinc-700 hover:bg-zinc-50 dark:text-zinc-300 dark:hover:bg-zinc-800 truncate"
                                >
                                  {t.name}
                                </button>
                              ));
                            })()}
                          </div>
                        )}

                        {/* 3. Tracked Link popover */}
                        {showLinksPopover && (
                          <div 
                            onScroll={(e) => {
                              const target = e.currentTarget;
                              if (target.scrollHeight - target.scrollTop <= target.clientHeight + 10) {
                                setVisibleLinksCount((prev) => prev + 4);
                              }
                            }}
                            className="absolute bottom-12 left-0 z-35 w-64 max-h-48 overflow-y-auto p-1.5 rounded border border-zinc-200 bg-white shadow-lg dark:border-zinc-800 dark:bg-zinc-900 space-y-0.5"
                          >
                            <div className="flex items-center gap-2 px-2 py-1 border-b border-zinc-150 dark:border-zinc-800 mb-1">
                              <input
                                type="text"
                                placeholder="Search links..."
                                value={linkSearchQuery}
                                onChange={(e) => {
                                  setLinkSearchQuery(e.target.value);
                                  setVisibleLinksCount(4);
                                }}
                                className="flex-1 bg-zinc-50 dark:bg-zinc-800 border border-zinc-200 dark:border-zinc-700 rounded px-2 py-0.5 text-[11px] text-zinc-800 dark:text-zinc-200 placeholder-zinc-400 focus:outline-none focus:ring-1 focus:ring-brand-500"
                                autoFocus
                              />
                              <button 
                                type="button" 
                                onClick={() => {
                                  setShowLinksPopover(false);
                                  setShowAttachmentMenu(true);
                                  setLinkSearchQuery('');
                                }}
                                className="text-[10px] font-bold text-brand-500 hover:underline shrink-0"
                              >
                                Back
                              </button>
                            </div>
                            {(() => {
                              const filteredLinks = triggerLinks.filter(l => 
                                l.name.toLowerCase().includes(linkSearchQuery.toLowerCase()) ||
                                l.url.toLowerCase().includes(linkSearchQuery.toLowerCase())
                              );
                              const displayedLinks = filteredLinks.slice(0, visibleLinksCount);

                              if (filteredLinks.length === 0) {
                                return <div className="text-[10px] font-semibold text-zinc-400 p-2 text-center">No links found</div>;
                              }
                              return displayedLinks.map((l) => {
                                const trackedUrl = `http://localhost:8080/api/v1/links/${l.id}?leadId=${selected.leadId || ''}`;
                                return (
                                  <button
                                    key={l.id}
                                    type="button"
                                    onClick={() => {
                                      setDraft((d) => d + (d ? ' ' : '') + trackedUrl);
                                      setShowLinksPopover(false);
                                    }}
                                    className="w-full text-left p-2 rounded text-xs font-semibold text-zinc-700 hover:bg-zinc-50 dark:text-zinc-300 dark:hover:bg-zinc-800 truncate"
                                  >
                                    {l.name}
                                  </button>
                                );
                              });
                            })()}
                          </div>
                        )}
                      </div>

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
                          className="block min-h-[40px] max-h-32 w-full resize-none rounded border border-zinc-200 bg-white px-4 py-2 text-sm font-semibold text-zinc-900 placeholder:text-zinc-400 focus:border-brand-500 focus:outline-none dark:border-zinc-800 dark:bg-zinc-900 dark:text-zinc-100 dark:placeholder:text-zinc-500"
                          suppressHydrationWarning
                        />
                      </div>
                      <button
                        type="submit"
                        disabled={(!draft.trim() && !attachedMediaUrl.trim()) || sending || uploadingMedia}
                        className="grid h-10 w-10 shrink-0 place-items-center rounded bg-brand-500 text-white transition-all hover:bg-brand-600 disabled:opacity-40"
                        suppressHydrationWarning
                      >
                        <Send className="h-4.5 w-4.5" />
                      </button>
                    </form>
                    <p className="mt-1.5 text-center text-[10px] font-semibold text-zinc-400">Enter to send · Shift+Enter for new line</p>
                  </footer>
                </>
              ) : (
                /* Empty state */
                <div className="grid flex-1 place-items-center px-6 text-center bg-transparent">
                  <div className="max-w-sm border border-zinc-200 dark:border-zinc-800 bg-zinc-50 dark:bg-zinc-900 p-10 shadow-none">
                    <div className="relative mx-auto mb-6 grid h-16 w-16 place-items-center">
                      <div className="relative grid h-16 w-16 place-items-center rounded bg-brand-50 text-brand-500 dark:bg-brand-500/10 dark:text-brand-400">
                        <MessagesSquare className="h-8 w-8" />
                      </div>
                    </div>
                    <h3 className="text-base font-black text-zinc-800 dark:text-zinc-100">Select a Conversation</h3>
                    <p className="mt-2.5 text-xs font-semibold leading-relaxed text-zinc-500 dark:text-zinc-500">
                      Choose a chat from the sidebar, or use the input box to start a new conversation.
                    </p>
                    <div className="mt-6 flex flex-wrap justify-center gap-2">
                      <div className="flex items-center gap-1.5 px-3 py-1 text-[10px] font-black uppercase border border-violet-200 dark:border-violet-800 text-violet-600 dark:text-violet-400">
                        <span className="h-1.5 w-1.5 rounded-full bg-violet-500" />WhatsApp
                      </div>
                      <div className="flex items-center gap-1.5 px-3 py-1 text-[10px] font-black uppercase border border-pink-200 dark:border-pink-800 text-pink-600 dark:text-pink-400">
                        <span className="h-1.5 w-1.5 rounded-full bg-pink-500" />Instagram
                      </div>
                      <div className="flex items-center gap-1.5 px-3 py-1 text-[10px] font-black uppercase border border-blue-200 dark:border-blue-800 text-blue-600 dark:text-blue-400">
                        <span className="h-1.5 w-1.5 rounded-full bg-blue-500" />Messenger
                      </div>
                      <div className="flex items-center gap-1.5 px-3 py-1 text-[10px] font-black uppercase border border-sky-200 dark:border-sky-800 text-sky-600 dark:text-sky-400">
                        <span className="h-1.5 w-1.5 rounded-full bg-sky-500" />SMS
                      </div>
                    </div>
                  </div>
                </div>
              )}
            </section>

            {selected && showDetailsPanel && (
              <ContactDetailsPanel
                studioId={studioId}
                conversation={selected}
                onClose={() => setShowDetailsPanel(false)}
                onLeadUpdated={handleLeadUpdated}
                onAIToggle={handleConversationAIToggle}
              />
            )}
          </>
        )}

        {currentTab === 'automated_messages' && renderAutomatedMessages()}
        {currentTab === 'snippets' && renderSnippets()}
        {currentTab === 'trigger_links' && renderTriggerLinks()}
      </div>
    </div>
  );

  // Render layouts
  function renderAutomatedMessages() {
    return (
      <div className="flex-1 overflow-hidden flex flex-col p-6 gap-6">
        <div className="flex items-center justify-between shrink-0">
          <div>
            <h3 className="text-lg font-black text-zinc-900 dark:text-white">Scheduled &amp; Automated Messages</h3>
            <p className="text-xs font-semibold text-zinc-400 dark:text-zinc-500 uppercase tracking-widest mt-1">Scheduled follow-up contact pipeline</p>
          </div>
          <button
            onClick={fetchJobs}
            className="flex items-center gap-1.5 px-4 py-2 rounded-xl bg-white/30 text-xs font-bold text-violet-600 border border-violet-200/30 hover:bg-white/50 transition-all dark:bg-white/5 dark:text-violet-400 dark:border-violet-500/10"
          >
            <RotateCw className="h-3.5 w-3.5" /> Refresh
          </button>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 flex-1 min-h-0">
          {/* Creator/Edit Form */}
          <div className="lg:col-span-1 p-4 border border-zinc-200 dark:border-zinc-800 bg-white dark:bg-zinc-900 space-y-4 overflow-y-auto no-scrollbar" style={{ maxHeight: 'calc(100vh - 10rem)' }}>
            <h4 className="text-xs font-black uppercase tracking-wider text-violet-600 dark:text-violet-400 border-b border-violet-200/20 pb-3 dark:border-white/5">
              {editingJobId ? "Edit Scheduled Message" : "Schedule Message"}
            </h4>

            <form onSubmit={handleSaveJob} className="space-y-4">
              {!editingJobId && (
                <div className="space-y-2">
                  <div className="flex items-center justify-between">
                    <label className="block text-[9px] font-black text-zinc-400 uppercase tracking-widest">Recipients ({selectedJobConvIds.length} selected)</label>
                    <button
                      type="button"
                      onClick={() => {
                        if (selectedJobConvIds.length === conversations.length) {
                          setSelectedJobConvIds([]);
                        } else {
                          setSelectedJobConvIds(conversations.map(c => c.id));
                        }
                      }}
                      className="text-[9px] font-bold text-violet-600 dark:text-violet-400 hover:underline"
                    >
                      {selectedJobConvIds.length === conversations.length ? 'Deselect All' : 'Select All'}
                    </button>
                  </div>
                  <input
                    type="text"
                    placeholder="Search recipients..."
                    value={searchRecipientQuery}
                    onChange={(e) => setSearchRecipientQuery(e.target.value)}
                    className="w-full rounded-xl border border-white/20 bg-white/30 px-3 py-1.5 text-xs font-semibold text-zinc-900 placeholder:text-zinc-400 dark:border-white/5 dark:bg-zinc-800 dark:text-zinc-100 focus:outline-none"
                  />
                  <div className="h-36 overflow-y-auto no-scrollbar border border-violet-200/20 rounded-xl bg-white/10 dark:border-white/5 dark:bg-white/5 p-2 space-y-1.5">
                    {conversations
                      .filter(c => {
                        const query = searchRecipientQuery.toLowerCase();
                        const name = (c.contactDisplayName || '').toLowerCase();
                        const val = (c.contactValue || '').toLowerCase();
                        return name.includes(query) || val.includes(query);
                      })
                      .map(c => {
                        const isChecked = selectedJobConvIds.includes(c.id);
                        return (
                          <label key={c.id} className="flex items-center gap-2 px-2 py-1 rounded-lg hover:bg-white/20 dark:hover:bg-white/5 cursor-pointer text-xs font-semibold text-zinc-800 dark:text-zinc-200">
                            <input
                              type="checkbox"
                              checked={isChecked}
                              onChange={() => {
                                if (isChecked) {
                                  setSelectedJobConvIds(selectedJobConvIds.filter(id => id !== c.id));
                                } else {
                                  setSelectedJobConvIds([...selectedJobConvIds, c.id]);
                                }
                              }}
                              className="rounded border-zinc-300 text-brand-600 focus:ring-brand-500"
                            />
                            <div className="flex-1 truncate">
                              <div>{c.contactDisplayName || displayContact(c.contactValue)}</div>
                              <div className="text-[9px] text-zinc-400">{c.contactDisplayName && c.contactDisplayName !== displayContact(c.contactValue) ? displayContact(c.contactValue) : ''}</div>
                            </div>
                            <span className="text-[8px] font-bold px-1.5 py-0.5 rounded text-white" style={{ backgroundColor: CHANNEL_BADGE[c.channelKind]?.color || '#999' }}>
                              {CHANNEL_BADGE[c.channelKind]?.label || c.channelKind}
                            </span>
                          </label>
                        );
                      })}
                  </div>
                </div>
              )}

              <div>
                <div className="flex items-center gap-2 mb-1.5">
                  <label className="block text-[9px] font-black text-zinc-400 uppercase tracking-widest">Message Content</label>
                  <div className="ml-auto flex items-center gap-1.5">
                    {/* Template Quick Select */}
                    <select
                      onChange={(e) => {
                        const tpl = templates.find(t => t.id === e.target.value);
                        if (tpl) {
                          setNewJobBody(tpl.body);
                          if (tpl.attachments && tpl.attachments[0]) {
                            setNewJobMediaUrl(tpl.attachments[0].url || '');
                            setNewJobMediaType(tpl.attachments[0].type as any);
                            setNewJobFileName(tpl.attachments[0].name || '');
                          } else {
                            setNewJobMediaUrl('');
                            setNewJobFileName('');
                          }
                        }
                        e.target.value = '';
                      }}
                      className="rounded-lg border border-violet-200/20 bg-white/20 px-2 py-0.5 text-[10px] font-bold text-violet-600 dark:text-violet-400 dark:border-white/5 dark:bg-zinc-800 dark:text-zinc-100 focus:outline-none cursor-pointer"
                    >
                      <option value="">Insert Template...</option>
                      {templates.map(t => (
                        <option key={t.id} value={t.id}>{t.name}</option>
                      ))}
                    </select>

                    {/* Trigger Link Quick Select */}
                    <select
                      onChange={(e) => {
                        const link = triggerLinks.find(l => l.id === e.target.value);
                        if (link) {
                          const separator = newJobBody ? (newJobBody.endsWith(' ') ? '' : ' ') : '';
                          setNewJobBody(prev => prev + separator + link.url);
                        }
                        e.target.value = '';
                      }}
                      className="rounded-lg border border-violet-200/20 bg-white/20 px-2 py-0.5 text-[10px] font-bold text-violet-600 dark:text-violet-400 dark:border-white/5 dark:bg-zinc-800 dark:text-zinc-100 focus:outline-none cursor-pointer"
                    >
                      <option value="">Insert Link...</option>
                      {triggerLinks.map(l => (
                        <option key={l.id} value={l.id}>{l.name}</option>
                      ))}
                    </select>
                  </div>
                </div>
                <textarea
                  required={!newJobMediaUrl}
                  rows={4}
                  value={newJobBody}
                  onChange={(e) => setNewJobBody(e.target.value)}
                  placeholder="Type your automated message..."
                  className="w-full rounded-xl border border-white/20 bg-white/30 px-3.5 py-2.5 text-xs font-semibold text-zinc-900 placeholder:text-zinc-400 dark:border-white/5 dark:bg-white/5 dark:text-zinc-100 focus:outline-none resize-none"
                />
              </div>

              {/* Media Attachment Upload/URL */}
              <div className="space-y-2">
                <label className="block text-[9px] font-black text-zinc-400 uppercase tracking-widest">Media Attachment (Optional)</label>
                <input
                  type="file"
                  ref={jobFileInputRef}
                  className="hidden"
                  onChange={handleJobFileSelect}
                />
                
                {/* Upload Button & Preview */}
                {newJobMediaUrl ? (
                  <div className="relative rounded-xl overflow-hidden border border-violet-400/40 p-2 bg-white/40 dark:bg-white/5 flex items-center justify-between gap-3">
                    {newJobMediaType === 'image' ? (
                      <img src={newJobMediaUrl} alt="Scheduled preview" className="w-12 h-12 object-cover rounded-lg" />
                    ) : newJobMediaType === 'video' ? (
                      <video src={newJobMediaUrl} className="w-12 h-12 object-cover rounded-lg" muted />
                    ) : (
                      <FileText className="w-8 h-8 text-violet-500 shrink-0" />
                    )}
                    <div className="min-w-0 flex-1">
                      <p className="text-[10px] font-bold text-zinc-700 dark:text-zinc-300 truncate">
                        {newJobFileName || "Attached Media"}
                      </p>
                      <p className="text-[9px] text-zinc-400 capitalize font-medium">{newJobMediaType}</p>
                    </div>
                    <button
                      type="button"
                      onClick={() => { setNewJobMediaUrl(''); setNewJobFileName(''); }}
                      className="p-1 rounded-full bg-red-500/15 hover:bg-red-500/25 text-red-500 transition-all"
                    >
                      <X className="h-3 w-3" />
                    </button>
                  </div>
                ) : (
                  <div className="flex gap-2">
                    <button
                      type="button"
                      onClick={() => jobFileInputRef.current?.click()}
                      disabled={uploadingJobMedia}
                      className="flex-1 flex items-center justify-center gap-1.5 py-2 px-3 rounded-xl border border-dashed border-violet-300 hover:bg-white/40 text-violet-600 text-xs font-bold transition-all dark:border-violet-500/30 dark:text-violet-400"
                    >
                      {uploadingJobMedia ? (
                        <>
                          <Loader2 className="h-3.5 w-3.5 animate-spin" /> Uploading...
                        </>
                      ) : (
                        <>
                          <Paperclip className="h-3.5 w-3.5" /> Upload File
                        </>
                      )}
                    </button>
                    
                    <input
                      type="text"
                      value={newJobMediaUrl}
                      onChange={(e) => {
                        setNewJobMediaUrl(e.target.value);
                        setNewJobFileName(e.target.value.split('/').pop() || '');
                        if (e.target.value.match(/\.(mp4|webm|ogg|mov)$/i)) setNewJobMediaType('video');
                        else if (e.target.value.match(/\.(jpg|jpeg|png|gif|webp)$/i)) setNewJobMediaType('image');
                        else setNewJobMediaType('document');
                      }}
                      placeholder="Or paste media URL..."
                      className="flex-[2] rounded-xl border border-white/20 bg-white/30 px-3 py-2 text-xs font-semibold text-zinc-900 placeholder:text-zinc-400 dark:border-white/5 dark:bg-white/5 dark:text-zinc-100 focus:outline-none"
                    />
                  </div>
                )}
              </div>

              <div>
                <label className="block text-[9px] font-black text-zinc-400 uppercase tracking-widest mb-1.5">Scheduled For (Optional)</label>
                <input
                  type="datetime-local"
                  value={newJobSchedTime}
                  onChange={(e) => setNewJobSchedTime(e.target.value)}
                  className="w-full rounded-xl border border-white/20 bg-white/30 px-3.5 py-2 text-xs font-semibold text-zinc-900 dark:border-white/5 dark:bg-zinc-800 dark:text-zinc-100 focus:outline-none"
                />
                <span className="text-[8px] text-zinc-400 block mt-1">Leave empty to dispatch as soon as possible.</span>
              </div>

              <button
                type="submit"
                disabled={schedulingInProgress || uploadingJobMedia}
                className="w-full py-2.5 rounded-xl bg-gradient-to-r from-brand-500 to-violet-500 text-[10px] font-black uppercase tracking-widest text-white shadow-lg hover:shadow-brand-500/30 transition-all hover:-translate-y-0.5 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {schedulingInProgress ? "Scheduling..." : (editingJobId ? "Update Message" : "Schedule Message")}
              </button>
              {editingJobId && (
                <button
                  type="button"
                  onClick={() => {
                    setEditingJobId(null);
                    setNewJobBody('');
                    setNewJobSchedTime('');
                    setNewJobMediaUrl('');
                    setNewJobFileName('');
                  }}
                  className="w-full mt-2 py-2.5 rounded-xl border border-zinc-200 bg-white/20 text-[10px] font-black uppercase tracking-widest text-zinc-600 dark:text-zinc-400 hover:bg-white/40 dark:border-white/5 dark:bg-white/5 transition-all"
                >
                  Cancel Edit
                </button>
              )}
            </form>
          </div>

          {/* List of pending automated messages */}
          <div
            onScroll={(e) => {
              const t = e.currentTarget;
              if (t.scrollHeight - t.scrollTop <= t.clientHeight + 20) {
                setTabVisibleJobsCount(prev => prev + 5);
              }
            }}
            className="lg:col-span-2 space-y-3 lg:overflow-y-auto no-scrollbar lg:h-full lg:pr-2"
          >
            {loadingJobs ? (
              <div className="flex justify-center py-12">
                <div className="h-8 w-8 animate-spin rounded-full border-2 border-brand-500 border-t-transparent" />
              </div>
            ) : jobs.length === 0 ? (
              <div className="text-center py-16 bg-white/20 rounded-[22px] border border-violet-200/20 backdrop-blur-md p-8 dark:bg-white/5 dark:border-white/5">
                <Clock className="h-10 w-10 mx-auto text-violet-400/60 mb-3" />
                <p className="text-sm font-bold text-zinc-800 dark:text-zinc-200">No scheduled messages</p>
                <p className="text-xs text-zinc-400 mt-1">There are no pending outbound follow-up messages in the queue.</p>
              </div>
            ) : (
              <>
              <div className="overflow-hidden rounded-[22px] border border-violet-200/20 bg-white/20 backdrop-blur-md dark:border-white/5 dark:bg-white/5 shadow-xl">
                <table className="w-full text-left border-collapse">
                  <thead>
                    <tr className="border-b border-violet-200/20 bg-violet-100/10 dark:border-white/5 text-[10px] font-black uppercase tracking-widest text-zinc-400">
                      <th className="p-4">Recipient</th>
                      <th className="p-4">Channel</th>
                      <th className="p-4">Message Preview</th>
                      <th className="p-4">Scheduled For</th>
                      <th className="p-4">Attempts</th>
                      <th className="p-4 text-right">Actions</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-violet-200/10 dark:divide-white/5 text-xs">
                    {jobs.slice(0, tabVisibleJobsCount).map((job) => (
                      <tr key={job.id} className="hover:bg-white/30 dark:hover:bg-white/5 transition-colors">
                        <td className="p-4 font-bold text-zinc-800 dark:text-zinc-200">
                          <div>{job.contactDisplayName}</div>
                          <div className="text-[10px] text-zinc-400 font-medium">{job.contactValue}</div>
                        </td>
                        <td className="p-4">
                          <span className="inline-block px-2 py-0.5 rounded text-[10px] font-bold text-white" style={{ backgroundColor: CHANNEL_BADGE[job.channelKind]?.color || '#999' }}>
                            {CHANNEL_BADGE[job.channelKind]?.label || job.channelKind}
                          </span>
                        </td>
                        <td className="p-4 text-zinc-600 dark:text-zinc-400 font-medium max-w-xs">
                          <div className="whitespace-pre-wrap leading-relaxed">{job.body}</div>
                          {job.attachments && job.attachments.length > 0 && job.attachments[0] && (() => {
                            const att = job.attachments[0];
                            const filename = (att.url || '').split('/').pop() || '';
                            return (
                              <div className="mt-2 flex flex-col gap-1.5">
                                <div className="flex items-center gap-1 text-[9px] text-violet-500 font-bold bg-violet-500/10 px-2 py-1 rounded-lg w-max">
                                  <Paperclip className="h-3 w-3 shrink-0" /> Media ({att.type}): {filename}
                                </div>
                                {att.type === 'image' && (
                                  <img src={att.url} alt="Attachment" className="max-w-[120px] max-h-[80px] object-cover rounded-lg border border-violet-200/20" />
                                )}
                                {att.type === 'video' && (
                                  <video src={att.url} className="max-w-[120px] max-h-[80px] object-cover rounded-lg border border-violet-200/20" muted controls />
                                )}
                              </div>
                            );
                          })()}
                        </td>
                        <td className="p-4 font-semibold text-zinc-500 dark:text-zinc-400">{new Date(job.scheduledFor).toLocaleString()}</td>
                        <td className="p-4 font-bold text-zinc-500">{job.attempts}</td>
                        <td className="p-4 text-right space-x-2 whitespace-nowrap">
                          <button
                            onClick={() => {
                              setEditingJobId(job.id.toString());
                              setNewJobBody(job.body);
                              if (job.scheduledFor) {
                                const d = new Date(job.scheduledFor);
                                const tzoffset = d.getTimezoneOffset() * 60000;
                                const localISOTime = (new Date(d.getTime() - tzoffset)).toISOString().slice(0, 16);
                                setNewJobSchedTime(localISOTime);
                              } else {
                                setNewJobSchedTime('');
                              }
                              setNewJobConvId(job.conversationId);
                              if (job.attachments && job.attachments.length > 0 && job.attachments[0]) {
                                setNewJobMediaUrl(job.attachments[0].url || '');
                                setNewJobMediaType(job.attachments[0].type as any);
                                setNewJobFileName(job.attachments[0].name || '');
                              } else {
                                setNewJobMediaUrl('');
                                setNewJobFileName('');
                              }
                            }}
                            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-violet-500 text-[10px] font-bold text-white hover:bg-violet-600 shadow-md hover:scale-105 transition-all"
                          >
                            <Pencil className="h-3 w-3" /> Edit
                          </button>
                          <button
                            onClick={() => handleTriggerJob(job.id)}
                            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-emerald-500 text-[10px] font-bold text-white hover:bg-emerald-600 shadow-md hover:scale-105 transition-all"
                          >
                            <Play className="h-3 w-3" /> Send Now
                          </button>
                          <button
                            onClick={() => handleCancelJob(job.id)}
                            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-red-500 text-[10px] font-bold text-white hover:bg-red-600 shadow-md hover:scale-105 transition-all"
                          >
                            <Trash2 className="h-3 w-3" /> Cancel
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              {jobs.length > tabVisibleJobsCount && (
                <div className="text-center text-xs font-bold text-violet-400/70 py-4">
                  Scroll down to load more scheduled messages (Showing {tabVisibleJobsCount} of {jobs.length})
                </div>
              )}
              </>
            )}
          </div>
        </div>
      </div>
    );
  }

  function renderSnippets() {
    return (
      <div className="flex-1 overflow-hidden flex flex-col p-6 gap-6">
        <div className="flex items-center justify-between shrink-0">
          <div>
            <h3 className="text-lg font-black text-zinc-900 dark:text-white">Message Snippets &amp; Templates</h3>
            <p className="text-xs font-semibold text-zinc-400 dark:text-zinc-500 uppercase tracking-widest mt-1">Predefined responses and AI builders</p>
          </div>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 flex-1 min-h-0">
          {/* Creator Form */}
          <div className="lg:col-span-1 p-5 rounded-[22px] border border-violet-200/20 bg-white/20 backdrop-blur-md dark:border-white/5 dark:bg-white/5 space-y-4 shadow-xl lg:overflow-y-auto no-scrollbar h-fit">
            <div className="flex items-center justify-between border-b border-violet-200/20 pb-3 dark:border-white/5">
              <h4 className="text-xs font-black uppercase tracking-wider text-violet-600 dark:text-violet-400">
                {editingTemplateId ? "Edit Template" : "Create Template"}
              </h4>
              <button
                type="button"
                onClick={() => setShowAiModal(true)}
                className="inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-xl bg-gradient-to-r from-violet-500 to-brand-500 text-[9px] font-black text-white uppercase tracking-widest shadow-md hover:scale-105 transition-all"
              >
                <Sparkles className="h-3 w-3" /> Write with AI
              </button>
            </div>

            <form onSubmit={handleCreateTemplate} className="space-y-4">
              <div>
                <label className="block text-[9px] font-black text-zinc-400 uppercase tracking-widest mb-1.5">Template Name</label>
                <input
                  type="text"
                  required
                  value={newTplName}
                  onChange={(e) => setNewTplName(e.target.value)}
                  placeholder="e.g. Trial Invite"
                  className="w-full rounded-xl border border-white/20 bg-white/30 px-3.5 py-2 text-xs font-semibold text-zinc-900 placeholder:text-zinc-400 dark:border-white/5 dark:bg-white/5 dark:text-zinc-100 focus:outline-none"
                />
              </div>

              <div>
                <label className="block text-[9px] font-black text-zinc-400 uppercase tracking-widest mb-1.5">Template Body (Use {"{{contact.first_name}}"})</label>
                <textarea
                  required
                  rows={4}
                  value={newTplBody}
                  onChange={(e) => setNewTplBody(e.target.value)}
                  placeholder="Hi {{contact.first_name}}, we'd love to invite you..."
                  className="w-full rounded-xl border border-white/20 bg-white/30 px-3.5 py-2 text-xs font-semibold text-zinc-900 placeholder:text-zinc-400 dark:border-white/5 dark:bg-white/5 dark:text-zinc-100 resize-none focus:outline-none"
                />
              </div>

              <div>
                <label className="block text-[9px] font-black text-zinc-400 uppercase tracking-widest mb-1.5">Template Media / File (Optional)</label>
                <input
                  ref={tplFileInputRef}
                  type="file"
                  accept="image/*,video/*,.pdf,.doc,.docx"
                  className="hidden"
                  onChange={handleTplFileSelect}
                />
                
                {/* Upload Button & Preview */}
                {newTplMediaUrl ? (
                  <div className="relative rounded-xl overflow-hidden border border-violet-400/40 p-2 bg-white/40 dark:bg-white/5 flex items-center justify-between gap-3">
                    {newTplMediaType === 'image' ? (
                      <img src={newTplMediaUrl} alt="Template preview" className="w-12 h-12 object-cover rounded-lg" />
                    ) : newTplMediaType === 'video' ? (
                      <video src={newTplMediaUrl} className="w-12 h-12 object-cover rounded-lg" muted />
                    ) : (
                      <FileText className="w-8 h-8 text-violet-500 shrink-0" />
                    )}
                    <div className="min-w-0 flex-1">
                      <p className="text-[10px] font-bold text-zinc-700 dark:text-zinc-300 truncate">
                        {newTplFileName || "Attached Media"}
                      </p>
                      <p className="text-[9px] text-zinc-400 capitalize font-medium">{newTplMediaType}</p>
                    </div>
                    <button
                      type="button"
                      onClick={() => { setNewTplMediaUrl(''); setNewTplFileName(''); }}
                      className="p-1 rounded-full bg-red-500/15 hover:bg-red-500/25 text-red-500 transition-all"
                    >
                      <X className="h-3 w-3" />
                    </button>
                  </div>
                ) : (
                  <div className="flex gap-2">
                    <button
                      type="button"
                      onClick={() => tplFileInputRef.current?.click()}
                      disabled={uploadingTplMedia}
                      className="flex-1 flex items-center justify-center gap-1.5 py-2 px-3 rounded-xl border border-dashed border-violet-300 hover:bg-white/40 text-violet-600 text-xs font-bold transition-all dark:border-violet-500/30 dark:text-violet-400"
                    >
                      {uploadingTplMedia ? (
                        <>
                          <Loader2 className="h-3.5 w-3.5 animate-spin" /> Uploading...
                        </>
                      ) : (
                        <>
                          <Paperclip className="h-3.5 w-3.5" /> Upload File
                        </>
                      )}
                    </button>
                    
                    <input
                      type="text"
                      value={newTplMediaUrl}
                      onChange={(e) => {
                        setNewTplMediaUrl(e.target.value);
                        setNewTplFileName(e.target.value.split('/').pop() || '');
                        if (e.target.value.match(/\.(mp4|webm|ogg|mov)$/i)) setNewTplMediaType('video');
                        else if (e.target.value.match(/\.(jpg|jpeg|png|gif|webp)$/i)) setNewTplMediaType('image');
                        else setNewTplMediaType('document');
                      }}
                      placeholder="Or paste media URL..."
                      className="flex-[2] rounded-xl border border-white/20 bg-white/30 px-3 py-2 text-xs font-semibold text-zinc-900 placeholder:text-zinc-400 dark:border-white/5 dark:bg-white/5 dark:text-zinc-100 focus:outline-none"
                    />
                  </div>
                )}
              </div>

              <div>
                <label className="block text-[9px] font-black text-zinc-400 uppercase tracking-widest mb-1.5">Available Channels</label>
                <div className="flex flex-wrap gap-2 mt-1">
                  {(['whatsapp_meta', 'instagram_meta', 'messenger_meta', 'sms'] as const).map((kind) => {
                    const isSelected = newTplChannels.includes(kind);
                    return (
                      <button
                        key={kind}
                        type="button"
                        onClick={() => {
                          if (isSelected) {
                            setNewTplChannels(newTplChannels.filter(c => c !== kind));
                          } else {
                            setNewTplChannels([...newTplChannels, kind]);
                          }
                        }}
                        className={cn(
                          "px-2.5 py-1 rounded-lg text-[9px] font-bold uppercase tracking-wider transition-all",
                          isSelected
                            ? "bg-violet-500 text-white shadow-sm"
                            : "bg-white/30 text-zinc-500 border border-violet-200/20 dark:bg-white/5 dark:text-zinc-400 dark:border-white/5"
                        )}
                      >
                        {CHANNEL_BADGE[kind].label}
                      </button>
                    );
                  })}
                </div>
              </div>

              <button
                type="submit"
                className="w-full py-2.5 rounded-xl bg-gradient-to-r from-brand-500 to-violet-500 text-[10px] font-black uppercase tracking-widest text-white shadow-lg hover:shadow-brand-500/30 transition-all hover:-translate-y-0.5"
              >
                {editingTemplateId ? "Update Template" : "Save Template"}
              </button>
              {editingTemplateId && (
                <button
                  type="button"
                  onClick={() => {
                    setEditingTemplateId(null);
                    setNewTplName('');
                    setNewTplBody('');
                    setNewTplMediaUrl('');
                    setNewTplFileName('');
                  }}
                  className="w-full mt-2 py-2.5 rounded-xl border border-zinc-200 bg-white/20 text-[10px] font-black uppercase tracking-widest text-zinc-600 dark:text-zinc-400 hover:bg-white/40 dark:border-white/5 dark:bg-white/5 transition-all"
                >
                  Cancel Edit
                </button>
              )}
            </form>
          </div>

          {/* List of Templates */}
          <div
            onScroll={(e) => {
              const t = e.currentTarget;
              if (t.scrollHeight - t.scrollTop <= t.clientHeight + 20) {
                setTabVisibleTemplatesCount(prev => prev + 5);
              }
            }}
            className="lg:col-span-2 space-y-3 lg:overflow-y-auto no-scrollbar lg:h-full lg:pr-2"
          >
            {loadingTemplates ? (
              <div className="flex justify-center py-12">
                <div className="h-8 w-8 animate-spin rounded-full border-2 border-brand-500 border-t-transparent" />
              </div>
            ) : templates.length === 0 ? (
              <div className="text-center py-16 bg-white/20 rounded-[22px] border border-violet-200/20 backdrop-blur-md p-8 dark:bg-white/5 dark:border-white/5">
                <FileText className="h-10 w-10 mx-auto text-violet-400/60 mb-3" />
                <p className="text-sm font-bold text-zinc-800 dark:text-zinc-200">No message templates</p>
                <p className="text-xs text-zinc-400 mt-1">Create your first template manually or using the AI assistant.</p>
              </div>
            ) : (
              <>
                {templates.slice(0, tabVisibleTemplatesCount).map((tpl) => (
                  <div key={tpl.id} className="p-4 rounded-[22px] border border-violet-200/20 bg-white/20 backdrop-blur-md dark:border-white/5 dark:bg-white/5 shadow-lg flex flex-col justify-between md:flex-row md:items-start gap-4">
                    <div className="space-y-1.5 flex-1 min-w-0">
                      <div className="flex items-center gap-2 flex-wrap">
                        <h4 className="text-xs font-black text-zinc-900 dark:text-white">{tpl.name}</h4>
                        <div className="flex gap-1">
                          {tpl.channelKinds.map((k) => (
                            <span key={k} className="px-1.5 py-0.5 rounded text-[8px] font-black uppercase text-white" style={{ backgroundColor: CHANNEL_BADGE[k as ChannelKind]?.color || '#999' }}>
                              {CHANNEL_BADGE[k as ChannelKind]?.label}
                            </span>
                          ))}
                        </div>
                      </div>
                      <p className="text-xs text-zinc-600 dark:text-zinc-400 font-medium whitespace-pre-wrap leading-relaxed">{tpl.body}</p>
                      {tpl.attachments && tpl.attachments.length > 0 && tpl.attachments[0] && (() => {
                        const att = tpl.attachments[0];
                        const filename = (att.url || '').split('/').pop() || '';
                        return (
                          <div className="mt-2 flex flex-col gap-1.5">
                            <div className="flex items-center gap-1 text-[9px] text-violet-500 font-bold bg-violet-500/10 px-2 py-1 rounded-lg w-max">
                              <Paperclip className="h-3 w-3 shrink-0" /> Media ({att.type}): {filename}
                            </div>
                            {att.type === 'image' && (
                              <img src={att.url} alt="Attachment" className="max-w-[120px] max-h-[80px] object-cover rounded-lg border border-violet-200/20" />
                            )}
                            {att.type === 'video' && (
                              <video src={att.url} className="max-w-[120px] max-h-[80px] object-cover rounded-lg border border-violet-200/20" muted controls />
                            )}
                          </div>
                        );
                      })()}
                    </div>
                    <div className="flex gap-2 self-end md:self-start">
                      <button
                        onClick={() => {
                          setEditingTemplateId(tpl.id);
                          setNewTplName(tpl.name);
                          setNewTplBody(tpl.body);
                          setNewTplChannels(tpl.channelKinds);
                          if (tpl.attachments && tpl.attachments.length > 0 && tpl.attachments[0]) {
                            setNewTplMediaUrl(tpl.attachments[0].url || '');
                            setNewTplMediaType(tpl.attachments[0].type as 'image'|'video'|'document' || 'image');
                            setNewTplFileName((tpl.attachments[0].url || '').split('/').pop() || '');
                          } else {
                            setNewTplMediaUrl('');
                            setNewTplFileName('');
                          }
                        }}
                        className="p-2 rounded-xl bg-violet-500/10 hover:bg-violet-500 text-violet-500 hover:text-white transition-all scale-95 hover:scale-100"
                      >
                        <Pencil className="h-4 w-4" />
                      </button>
                      <button
                        onClick={() => handleDeleteTemplate(tpl.id)}
                        className="p-2 rounded-xl bg-red-500/10 hover:bg-red-500 text-red-500 hover:text-white transition-all scale-95 hover:scale-100"
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </div>
                  </div>
                ))}
                {templates.length > tabVisibleTemplatesCount && (
                  <div className="text-center text-xs font-bold text-violet-400/70 py-4">
                    Scroll down to load more templates (Showing {tabVisibleTemplatesCount} of {templates.length})
                  </div>
                )}
              </>
            )}
          </div>
        </div>

        {/* AI Helper Modal */}
        {showAiModal && (
          <div className="fixed inset-0 z-50 grid place-items-center bg-black/40 backdrop-blur-sm p-4">
            <div className="w-full max-w-md p-6 rounded-[28px] border border-violet-200/30 bg-white shadow-2xl dark:border-white/5 dark:bg-neutral-900 space-y-4">
              <div className="flex items-center justify-between border-b border-violet-200/20 pb-3 dark:border-white/5">
                <div className="flex items-center gap-2">
                  <Sparkles className="h-5 w-5 text-brand-500" />
                  <h3 className="text-xs font-black uppercase tracking-wider text-zinc-900 dark:text-white">AI Content Generator</h3>
                </div>
                <button onClick={() => setShowAiModal(false)} className="p-1 rounded-lg hover:bg-zinc-100 dark:hover:bg-neutral-800 text-zinc-400">
                  <X className="h-5 w-5" />
                </button>
              </div>

              <div className="space-y-4">
                <div>
                  <label className="block text-[9px] font-black text-zinc-400 uppercase tracking-widest mb-1.5">What is this message about?</label>
                  <textarea
                    rows={3}
                    value={aiPrompt}
                    onChange={(e) => setAiPrompt(e.target.value)}
                    placeholder="e.g. Write a friendly follow-up reminding the customer of a pricing discount on membership plan"
                    className="w-full rounded-xl border border-zinc-200 bg-transparent px-3.5 py-2.5 text-xs font-medium text-zinc-900 placeholder:text-zinc-400 dark:border-neutral-800 dark:text-zinc-100 focus:outline-none focus:ring-2 focus:ring-brand-500/15"
                  />
                </div>

                <div className="flex gap-2">
                  <button
                    onClick={() => setShowAiModal(false)}
                    className="flex-1 py-2.5 rounded-xl border border-zinc-200 text-[10px] font-black uppercase tracking-wider text-zinc-500 hover:bg-zinc-50 transition-all dark:border-neutral-800 dark:text-zinc-400 dark:hover:bg-neutral-800"
                  >
                    Cancel
                  </button>
                  <button
                    onClick={handleAiGenerate}
                    disabled={!aiPrompt.trim() || generatingAi}
                    className="flex-1 py-2.5 rounded-xl bg-gradient-to-r from-brand-500 to-violet-500 text-[10px] font-black uppercase tracking-wider text-white shadow-lg shadow-brand-500/25 hover:scale-[1.02] transition-all disabled:opacity-40"
                  >
                    {generatingAi ? "Generating..." : "Generate Draft"}
                  </button>
                </div>
              </div>
            </div>
          </div>
        )}
      </div>
    );
  }

  function renderTriggerLinks() {
    return (
      <div className="flex-1 overflow-hidden flex flex-col p-6 gap-6">
        <div className="flex items-center justify-between shrink-0">
          <div>
            <h3 className="text-lg font-black text-zinc-900 dark:text-white">Trigger Links</h3>
            <p className="text-xs font-semibold text-zinc-400 dark:text-zinc-500 uppercase tracking-widest mt-1">Short urls that track user clicks</p>
          </div>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 flex-1 min-h-0">
          {/* Creator Form */}
          <div className="lg:col-span-1 p-5 rounded-[22px] border border-violet-200/20 bg-white/20 backdrop-blur-md dark:border-white/5 dark:bg-white/5 space-y-4 shadow-xl lg:overflow-y-auto no-scrollbar h-fit">
            <h4 className="text-xs font-black uppercase tracking-wider text-violet-600 dark:text-violet-400 border-b border-violet-200/20 pb-3 dark:border-white/5">
              {editingLinkId ? "Edit Link" : "Create Link"}
            </h4>

            <form onSubmit={handleCreateTriggerLink} className="space-y-4">
              <div>
                <label className="block text-[9px] font-black text-zinc-400 uppercase tracking-widest mb-1.5">Link Name</label>
                <input
                  type="text"
                  required
                  value={newLinkName}
                  onChange={(e) => setNewLinkName(e.target.value)}
                  placeholder="e.g. Booking Portal"
                  className="w-full rounded-xl border border-white/20 bg-white/30 px-3.5 py-2 text-xs font-semibold text-zinc-900 placeholder:text-zinc-400 dark:border-white/5 dark:bg-white/5 dark:text-zinc-100 focus:outline-none"
                />
              </div>

              <div>
                <label className="block text-[9px] font-black text-zinc-400 uppercase tracking-widest mb-1.5">Destination URL</label>
                <input
                  type="url"
                  required
                  value={newLinkUrl}
                  onChange={(e) => setNewLinkUrl(e.target.value)}
                  placeholder="https://calendly.com/your-studio"
                  className="w-full rounded-xl border border-white/20 bg-white/30 px-3.5 py-2 text-xs font-semibold text-zinc-900 placeholder:text-zinc-400 dark:border-white/5 dark:bg-white/5 dark:text-zinc-100 focus:outline-none"
                />
              </div>

              <button
                type="submit"
                className="w-full py-2.5 rounded-xl bg-gradient-to-r from-brand-500 to-violet-500 text-[10px] font-black uppercase tracking-widest text-white shadow-lg hover:shadow-brand-500/30 transition-all hover:-translate-y-0.5"
              >
                {editingLinkId ? "Update Link" : "Create Link"}
              </button>
              {editingLinkId && (
                <button
                  type="button"
                  onClick={() => {
                    setEditingLinkId(null);
                    setNewLinkName('');
                    setNewLinkUrl('');
                  }}
                  className="w-full mt-2 py-2.5 rounded-xl border border-zinc-200 bg-white/20 text-[10px] font-black uppercase tracking-widest text-zinc-600 dark:text-zinc-400 hover:bg-white/40 dark:border-white/5 dark:bg-white/5 transition-all"
                >
                  Cancel Edit
                </button>
              )}
            </form>
          </div>

          {/* List of Links */}
          <div
            onScroll={(e) => {
              const t = e.currentTarget;
              if (t.scrollHeight - t.scrollTop <= t.clientHeight + 20) {
                setTabVisibleLinksCount(prev => prev + 5);
              }
            }}
            className="lg:col-span-2 space-y-3 lg:overflow-y-auto no-scrollbar lg:h-full lg:pr-2"
          >
            {loadingLinks ? (
              <div className="flex justify-center py-12">
                <div className="h-8 w-8 animate-spin rounded-full border-2 border-brand-500 border-t-transparent" />
              </div>
            ) : triggerLinks.length === 0 ? (
              <div className="text-center py-16 bg-white/20 rounded-[22px] border border-violet-200/20 backdrop-blur-md p-8 dark:bg-white/5 dark:border-white/5">
                <LinkIcon className="h-10 w-10 mx-auto text-violet-400/60 mb-3" />
                <p className="text-sm font-bold text-zinc-800 dark:text-zinc-200">No trigger links</p>
                <p className="text-xs text-zinc-400 mt-1">Short URLs let you track clicks when they are clicked by customers.</p>
              </div>
            ) : (
              <>
                {triggerLinks.slice(0, tabVisibleLinksCount).map((link) => {
                  const shortLink = `http://localhost:8080/api/v1/links/${link.id}`;
                  return (
                    <div key={link.id} className="p-4 rounded-[22px] border border-violet-200/20 bg-white/20 backdrop-blur-md dark:border-white/5 dark:bg-white/5 shadow-lg flex flex-col justify-between md:flex-row md:items-start gap-4">
                      <div className="space-y-1.5 flex-1 min-w-0">
                        <div className="flex items-center justify-between gap-4">
                          <h4 className="text-xs font-black text-zinc-900 dark:text-white truncate">{link.name}</h4>
                          <div className="flex items-center gap-1.5 bg-violet-500/10 px-2.5 py-1 rounded-full text-violet-600 dark:text-violet-400 text-[9px] font-black uppercase tracking-wider">
                            Clicks: {link.clicks}
                          </div>
                        </div>
                        <div className="text-xs space-y-1">
                          <div className="text-zinc-400 truncate">
                            Destination: <a href={link.url} target="_blank" rel="noreferrer" className="text-violet-500 font-semibold hover:underline">{link.url}</a>
                          </div>
                          <div className="text-zinc-400 truncate flex items-center gap-1.5 flex-wrap">
                            Tracked Link: <span className="font-semibold text-zinc-700 dark:text-zinc-200">{shortLink}</span>
                            <button
                              onClick={() => {
                                navigator.clipboard.writeText(shortLink);
                                alert('Link copied to clipboard!');
                              }}
                              className="text-[9px] font-black uppercase tracking-widest text-violet-500 hover:text-violet-600 ml-1.5 bg-white/40 dark:bg-white/5 px-2 py-0.5 rounded border border-violet-200/10"
                            >
                              Copy
                            </button>
                          </div>
                        </div>
                      </div>
                      <div className="flex gap-2 self-end md:self-start">
                        <button
                          onClick={() => {
                            setEditingLinkId(link.id);
                            setNewLinkName(link.name);
                            setNewLinkUrl(link.url);
                          }}
                          className="p-2 rounded-xl bg-violet-500/10 hover:bg-violet-500 text-violet-500 hover:text-white transition-all scale-95 hover:scale-100"
                        >
                          <Pencil className="h-4 w-4" />
                        </button>
                        <button
                          onClick={() => handleDeleteTriggerLink(link.id)}
                          className="p-2 rounded-xl bg-red-500/10 hover:bg-red-500 text-red-500 hover:text-white transition-all scale-95 hover:scale-100"
                        >
                          <Trash2 className="h-4 w-4" />
                        </button>
                      </div>
                    </div>
                  );
                })}
                {triggerLinks.length > tabVisibleLinksCount && (
                  <div className="text-center text-xs font-bold text-violet-400/70 py-4">
                    Scroll down to load more links (Showing {tabVisibleLinksCount} of {triggerLinks.length})
                  </div>
                )}
              </>
            )}
          </div>
        </div>
      </div>
    );
  }
}

// ────────────────────────────────────────────────────────────────

function ChannelAvatar({ kind, name, active }: { kind: ChannelKind; name: string; active?: boolean }) {
  const ch = CHANNEL_BADGE[kind];
  return (
    <span className="relative shrink-0">
      <span
        className={cn(
          "grid h-11 w-11 place-items-center rounded-full text-sm font-black text-white shadow-lg transition-transform group-hover:scale-105",
          active ? "bg-white/25 backdrop-blur-md" : "ring-3 ring-white/30 dark:ring-white/10"
        )}
        style={!active ? { background: avatarColor(name) } : undefined}
        aria-hidden
      >
        {brandInitials(name)}
      </span>
      <span
        className={cn(
          "absolute -bottom-0.5 -right-0.5 grid h-5 w-5 place-items-center rounded-full text-[9px] font-black text-white shadow-md",
          active ? "ring-2 ring-brand-500/50" : "ring-2 ring-white/50 dark:ring-neutral-900/80"
        )}
        style={{ background: ch?.color || '#999' }}
        aria-label={ch?.label || kind}
      >
        {ch?.label[0] || '?'}
      </span>
    </span>
  );
}

function MessageBubble({ msg }: { msg: Message }) {
  const isOutbound = msg.direction === 'outbound';
  const sourceTag = sourceTagFor(msg.sourceKind);
  return (
    <li className={cn('flex flex-col gap-0.5 animate-in', isOutbound ? 'items-end' : 'items-start')}>
      {/* Sender label */}
      <span className="px-1 text-[9px] font-black uppercase tracking-wider text-zinc-400">
        {isOutbound ? 'You' : 'Contact'}
      </span>
      <div
        className={cn(
          'relative max-w-[85%] px-3 py-2 text-xs shadow-sm transition-all duration-200 sm:max-w-[70%] rounded-lg border',
          isOutbound
            ? 'bg-slate-100 border-slate-300 text-slate-800 dark:bg-slate-800 dark:border-slate-600 dark:text-slate-100'
            : 'bg-white border-zinc-200 text-zinc-800 dark:bg-zinc-900 dark:border-zinc-700 dark:text-zinc-100',
        )}
      >
        {sourceTag && (
          <div className="mb-1 text-[9px] font-black uppercase tracking-widest opacity-50">
            {sourceTag}
          </div>
        )}
        <div className="whitespace-pre-wrap font-medium leading-relaxed">{msg.body}</div>

        {msg.attachments && msg.attachments.length > 0 && msg.attachments[0] && (() => {
          const att = msg.attachments[0]!;
          const type = att.type || 'image';
          return (
            <div className="mt-2 rounded overflow-hidden border border-zinc-200 dark:border-zinc-700">
              {type === 'video' ? (
                <video src={att.url} controls className="max-h-52 w-full object-cover rounded" />
              ) : type === 'document' ? (
                <a
                  href={att.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="flex items-center gap-2 px-3 py-3 bg-zinc-100 dark:bg-zinc-800 rounded text-[10px] font-bold hover:bg-zinc-200 dark:hover:bg-zinc-700 transition-all"
                >
                  <span className="text-xl">📎</span>
                  <span className="truncate">{att.url?.split('/').pop()}</span>
                </a>
              ) : (
                <img
                  src={att.url}
                  alt="Attached media"
                  className="max-h-52 w-full object-cover cursor-pointer rounded"
                  onClick={() => window.open(att.url, '_blank')}
                />
              )}
            </div>
          );
        })()}

        <div className={cn(
          'mt-1.5 flex items-center justify-end gap-1.5 text-[9px] font-bold',
          isOutbound ? 'text-slate-400 dark:text-slate-400' : 'text-zinc-400',
        )}>
          <span suppressHydrationWarning>{formatTime(msg.sentAt)}</span>
          {isOutbound && <StatusTick status={msg.status} />}
        </div>
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
      return null;
  }
}

function channelLabel(k: ChannelKind): string {
  switch (k) {
    case 'whatsapp_meta':  return 'WhatsApp';
    case 'whatsapp_web':   return 'WhatsApp';
    case 'instagram_meta': return 'Instagram';
    case 'messenger_meta': return 'Messenger';
    case 'x_dm':           return 'X DM';
    case 'sms':            return 'SMS';
    case 'google_ads':     return 'Google Ads';
    case 'telegram':       return 'Telegram';
    case 'telegram_mtproto': return 'Telegram';
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

export type _DirectionAlias = Direction;
