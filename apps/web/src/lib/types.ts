export type Role = 'super_admin' | 'studio_admin';

export interface StudioBrand {
  slug: string;
  name: string;
  brandColor: string;
  logoUrl: string;
  active: boolean;
  socialPlannerEnabled?: boolean;
  subscriptionTier?: string;
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
  contactPhone?: string;
  active: boolean;
  managedBy1Hero?: boolean;
  createdAt: string;
  updatedAt: string;
  availabilitySlots?: { day: string; times: string[] }[];
  availabilityTimezone?: string;
  metaAppId?: string;
  googleClientId?: string;
  hasGeminiApiKey?: boolean;
  hasGroqApiKey?: boolean;
  hasMetaAppSecret?: boolean;
  hasGoogleClientSecret?: boolean;
  hasGoogleDeveloperToken?: boolean;
  hasStripeSecretKey?: boolean;
  hasStripeWebhookSecret?: boolean;
  campaignCount?: number;
  leadCount?: number;
  knowledgeBase?: string;
  knowledgeBaseFiles?: { name: string; url: string; text: string; platform?: string }[];
  greetingMessage?: string;
  bookingHeroImageUrl?: string;
  bookingHeroVideoUrl?: string;
  trialAmountSgd?: number;
  subscriptionTier?: string;
  trialConfirmationMessage?: string;
  membershipConfirmationMessage?: string;
  trialGlofoxMembershipId?: string;
  trialGlofoxPlanCode?: string;
  membershipGlofoxMembershipId?: string;
  membershipGlofoxPlanCode?: string;
}

export interface Plan {
  id: string;
  studioId: string;
  planName: string;
  priceSgd: number;
  billingCycle: string;
  features: string[];
  isActive: boolean;
  createdAt: string;
  updatedAt: string;
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

export type LeadStatus = 'new' | 'contacted' | 'trial_booked' | 'member' | 'dropped' | 'paused';

export const LEAD_STATUSES: LeadStatus[] = [
  'new',
  'contacted',
  'trial_booked',
  'member',
  'dropped',
  'paused',
];

export const LEAD_STATUS_LABELS: Record<LeadStatus, string> = {
  new: 'New',
  contacted: 'Contacted',
  trial_booked: 'Trial booked',
  member: 'Member',
  dropped: 'Dropped',
  paused: 'Paused',
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
  currency: string;
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
  dndEnabled: boolean;
  referrer?: string;
  autoContactStage?: string;
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

export type ChannelKind = 'whatsapp_meta' | 'whatsapp_web' | 'instagram_meta' | 'messenger_meta' | 'x_dm' | 'sms' | 'google_ads' | 'telegram' | 'telegram_mtproto';

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
  leadStatus?: LeadStatus;
  status: ConversationStatus;
  assignedTo?: string;
  unreadCount: number;
  lastMessageAt: string;
  lastMessagePreview: string;
  lastMessageDirection?: Direction;
  aiEnabled: boolean;
  dndEnabled: boolean;
  createdAt: string;
  updatedAt: string;
  escalatedAt?: string;
  escalatedReason?: string;
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

// ===== Decision Trees =====

export type ConditionType = 'keyword' | 'intent' | 'sentiment' | 'default' | 'lead_status';
export type NodeAction = 'reply' | 'escalate_human' | 'book_trial' | 'send_link' | 'change_status';

export interface TreeNode {
  id: string;
  treeId: string;
  parentId?: string;
  label: string;
  conditionType: ConditionType;
  conditionValue: Record<string, unknown>;
  replyTemplate: string;
  action: NodeAction;
  actionValue: Record<string, unknown>;
  sortOrder: number;
  positionX?: number | null;
  positionY?: number | null;
  children?: TreeNode[];
  createdAt: string;
  updatedAt: string;
}

export interface DecisionTree {
  id: string;
  studioId: string;
  name: string;
  isActive: boolean;
  targetStatuses: string[];
  nodes?: TreeNode[];
  createdAt: string;
  updatedAt: string;
}

export interface SimulateResult {
  matched: boolean;
  nodeId?: string;
  nodeLabel?: string;
  reply?: string;
  action?: NodeAction;
  targetStatus?: string;
  traversalPath: string[];
}

// ===== Campaign Analytics =====

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
  pausedLeads: number;
  trialToMemberRate: number;
  droppedRate: number;
  pausedRate: number;
  followupsRequired: number;
  unrespondedMessages: number;
  avgResponseTimeLapseSecs: number;
  leadToTrialTimeLapseSecs: number;
  trialToMemberTimeLapseSecs: number;
  byCampaign: CampaignAnalytics[];
  byPlatform: PlatformAnalytics[];
}

