// Mirrors apps/api/internal/integrations/crm/{domain.go,operations.go} and
// internal/integrations/llm/domain.go JSON shapes exactly.

export type AuthType = 'bearer' | 'api_key' | 'basic' | 'token_exchange';
export type ProviderStatus = 'draft' | 'active';

export interface AuthFieldDef {
  key: string;
  label: string;
  secret: boolean;
}

export interface CrmProvider {
  id: string;
  name: string;
  description: string;
  baseUrl: string;
  authType: AuthType;
  authFieldDefs: AuthFieldDef[];
  // Only meaningful when authType === 'token_exchange' — see the login flow
  // this drives in apps/api/internal/integrations/crm/executor.go.
  tokenLoginPath?: string;
  tokenLoginMethod?: string;
  tokenLoginBodyMapping?: Record<string, string>;
  tokenResponsePath?: string;
  tokenExpiryPath?: string;
  tokenExpirySeconds?: number;
  specSource?: string;
  status: ProviderStatus;
  createdBy?: string;
  createdAt: string;
  updatedAt: string;
}

export type OperationKey =
  | 'create_lead'
  | 'register_user'
  | 'purchase_membership'
  | 'find_membership_plan'
  | 'get_member'
  | 'list_bookings';

export const OPERATION_CATALOG: { key: OperationKey; label: string; description: string }[] = [
  { key: 'create_lead', label: 'Create lead', description: 'Register a new prospect in the CRM.' },
  { key: 'register_user', label: 'Register user', description: 'Create a full member record with login credentials.' },
  { key: 'purchase_membership', label: 'Purchase membership', description: 'Charge a member for a trial or membership plan.' },
  { key: 'find_membership_plan', label: 'Find membership plan', description: 'Look up a plan by price.' },
  { key: 'get_member', label: 'Get member', description: "Fetch one member's profile by CRM user id." },
  { key: 'list_bookings', label: 'List bookings', description: 'Read a location’s class/session bookings.' },
];

export interface CrmOperation {
  id: string;
  crmProviderId: string;
  operationKey: OperationKey;
  httpMethod: string;
  pathTemplate: string;
  requestMapping: Record<string, unknown>;
  responseMapping: Record<string, unknown>;
  reviewed: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface CrmConnection {
  id: string;
  studioId: string;
  crmProviderId: string;
  status: 'active' | 'disconnected';
  connectedAt: string;
  updatedAt: string;
}

export type LlmProviderName = 'claude' | 'gemini' | 'groq';

export interface AiTaskConfig {
  purpose: string;
  configured: boolean;
  provider?: LlmProviderName;
  model?: string;
  hasApiKey?: boolean;
}

export const LLM_MODELS: Record<LlmProviderName, string[]> = {
  claude: ['claude-sonnet-5', 'claude-opus-5', 'claude-haiku-4-5-20251001'],
  gemini: ['gemini-2.5-flash', 'gemini-2.0-flash', 'gemini-2.0-flash-lite'],
  groq: ['llama-3.3-70b-versatile', 'llama-3.1-8b-instant'],
};

export const CRM_DOC_PARSING_PURPOSE = 'crm_doc_parsing';
