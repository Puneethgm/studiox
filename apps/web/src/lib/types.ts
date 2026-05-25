export type Role = 'super_admin' | 'studio_admin';

export interface StudioBrand {
  slug: string;
  name: string;
  brandColor: string;
  logoUrl: string;
  active: boolean;
}

export interface Me {
  id: string;
  email: string;
  role: Role;
  studioId?: string;
  studio?: StudioBrand; // present for studio_admin
}

export interface Studio {
  id: string;
  slug: string;
  name: string;
  brandColor: string;
  logoUrl: string;
  contactEmail: string;
  active: boolean;
  createdAt: string;
  updatedAt: string;
  availabilitySlots?: { day: string; times: string[] }[];
  availabilityTimezone?: string;
  geminiApiKey?: string;
  metaAppId?: string;
  metaAppSecret?: string;
  campaignCount?: number;
  leadCount?: number;
}

export interface Campaign {
  id: string;
  studioId: string;
  studioSlug?: string;
  studioName?: string;
  slug: string;
  name: string;
  description: string;
  fitnessPlans: string[];
  active: boolean;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
  leadCount?: number;
  shareUrl: string;
}

export type LeadStatus = 'new' | 'contacted' | 'trial_booked' | 'member' | 'dropped';

export const LEAD_STATUSES: LeadStatus[] = [
  'new',
  'contacted',
  'trial_booked',
  'member',
  'dropped',
];

export const LEAD_STATUS_LABELS: Record<LeadStatus, string> = {
  new: 'New',
  contacted: 'Contacted',
  trial_booked: 'Trial booked',
  member: 'Member',
  dropped: 'Dropped',
};

export interface Lead {
  id: string;
  studioId: string;
  studioName?: string;
  studioSlug?: string;
  campaignId: string;
  campaignName?: string;
  campaignSlug?: string;
  name: string;
  firstName: string;
  lastName: string;
  email: string;
  phone: string;
  fitnessPlan: string;
  goals: string;
  source: string;
  status: LeadStatus;
  notes: string;
  contactAttempts: number;
  lastContactedAt?: string;
  contactMade: boolean;
  hotLead: boolean;
  trialPurchased: boolean;
  assignedTo?: string;
  trialAttended: boolean;
  memberSold: boolean;
  monthlyFee: number;
  offer: string;
  furtherNotes: string;
  createdAt: string;
  updatedAt: string;
}

export interface StudioSheetsSettings {
  id?: string;
  studioId: string;
  spreadsheetId: string;
  tabName: string;
  active: boolean;
  createdAt?: string;
  updatedAt?: string;
}

// ===== Messaging =====

export type ChannelKind = 'whatsapp_meta' | 'instagram_meta' | 'messenger_meta' | 'x_dm' | 'sms';

export type ChannelStatus = 'active' | 'paused' | 'disconnected' | 'error';

export interface ChannelAccount {
  id: string;
  studioId: string;
  kind: ChannelKind;
  bsp: string;
  externalId: string;
  parentId: string;
  displayHandle: string;
  status: ChannelStatus;
  lastError?: string;
  connectedAt: string;
  disconnectedAt?: string;
  createdAt: string;
  updatedAt: string;
}

export type ConversationStatus = 'open' | 'snoozed' | 'closed';

export type Direction = 'inbound' | 'outbound';
export type SourceKind = 'customer' | 'studio_user' | 'automation' | 'ai';
export type MessageStatus = 'pending' | 'sent' | 'delivered' | 'read' | 'failed';

export interface Conversation {
  id: string;
  studioId: string;
  channelAccountId: string;
  channelKind: ChannelKind;
  channelHandle?: string;
  contactIdentityId: string;
  contactDisplayName: string;
  contactValue: string;
  externalThreadId: string;
  leadId?: string;
  status: ConversationStatus;
  assignedTo?: string;
  unreadCount: number;
  lastMessageAt: string;
  lastMessagePreview: string;
  lastMessageDirection?: Direction;
  createdAt: string;
  updatedAt: string;
}

export interface Attachment {
  type: string;
  url?: string;
  mime?: string;
  name?: string;
}

export interface Message {
  id: string;
  conversationId: string;
  studioId: string;
  direction: Direction;
  sourceKind: SourceKind;
  sourceUserId?: string;
  sourceRef?: string;
  body: string;
  attachments?: Attachment[];
  externalId?: string;
  inReplyTo?: string;
  status: MessageStatus;
  failureReason?: string;
  sentAt: string;
  deliveredAt?: string;
  readAt?: string;
  createdAt: string;
}

export interface CampaignAnalytics {
  id: string;
  name: string;
  slug: string;
  totalLeads: number;
  convertedLeads: number;
  conversionRate: number;
}

export interface PlatformAnalytics {
  platform: string;
  totalLeads: number;
  convertedLeads: number;
  conversionRate: number;
}

export interface AnalyticsSummary {
  totalLeads: number;
  newLeads: number;
  trialBookedLeads: number;
  memberLeads: number;
  droppedLeads: number;
  trialToMemberRate: number;
  followupsRequired: number;
  unrespondedMessages: number;
  avgResponseTimeLapseSecs: number;
  leadToTrialTimeLapseSecs: number;
  trialToMemberTimeLapseSecs: number;
  byCampaign: CampaignAnalytics[];
  byPlatform: PlatformAnalytics[];
}

