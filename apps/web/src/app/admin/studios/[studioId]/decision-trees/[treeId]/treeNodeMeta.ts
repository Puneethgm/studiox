import {
  MessageSquare, Headset, Calendar, Link, ArrowRightLeft,
  type LucideIcon,
} from 'lucide-react';
import type { ConditionType, NodeAction } from '@/lib/types';

export const CONDITION_LABELS: Record<ConditionType, string> = {
  keyword: 'Keyword match',
  intent: 'AI intent',
  sentiment: 'Sentiment',
  default: 'Default (catch-all)',
  lead_status: 'Lead status',
};

export const ACTION_LABELS: Record<NodeAction, string> = {
  reply: 'Send reply',
  escalate_human: 'Escalate to human',
  book_trial: 'Book trial',
  send_link: 'Send link',
  change_status: 'Change lead status',
};

export const ACTION_COLORS: Record<NodeAction, string> = {
  reply: 'bg-blue-50 text-blue-700 border-blue-200',
  escalate_human: 'bg-amber-50 text-amber-700 border-amber-200',
  book_trial: 'bg-green-50 text-green-700 border-green-200',
  send_link: 'bg-purple-50 text-purple-700 border-purple-200',
  change_status: 'bg-orange-50 text-orange-700 border-orange-200',
};

export const ACTION_ICONS: Record<NodeAction, LucideIcon> = {
  reply: MessageSquare,
  escalate_human: Headset,
  book_trial: Calendar,
  send_link: Link,
  change_status: ArrowRightLeft,
};

export const LEAD_STATUSES = [
  { value: 'new', label: 'New' },
  { value: 'contacted', label: 'Contacted' },
  { value: 'trial_booked', label: 'Trial booked' },
  { value: 'member', label: 'Member' },
  { value: 'dropped', label: 'Dropped' },
  { value: 'paused', label: 'Paused' },
];
