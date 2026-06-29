// Server-side fetches for the public form (no cookie forwarding).
// Server-only address of the Go API. Never exposed to the browser.
const SERVER_BASE = process.env.API_BASE_URL ?? 'http://localhost:8080';

export interface PublicStudio {
  slug: string;
  name: string;
  brandColor: string;
  logoUrl: string;
  trialAmountSgd?: number;
  availabilitySlots?: Record<string, string[]>;
  availabilityTimezone?: string;
  stripePublishableKey?: string;
  bookingHeroImageUrl?: string;
  bookingHeroVideoUrl?: string;
}

export interface PublicPlan {
  id: string;
  planName: string;
  priceSgd: number;
  billingCycle: string;
  features: string[];
  isActive: boolean;
}

export interface PublicCampaign {
  studioSlug: string;
  studioName: string;
  slug: string;
  name: string;
  description: string;
  fitnessPlans: string[];
}

export async function fetchPublicStudio(slug: string): Promise<PublicStudio | null> {
  const res = await fetch(`${SERVER_BASE}/api/v1/public/studios/${encodeURIComponent(slug)}`, {
    cache: 'no-store',
  });
  if (res.status === 404) return null;
  if (!res.ok) throw new Error(`API ${res.status}`);
  return (await res.json()) as PublicStudio;
}

export async function fetchPublicPlans(slug: string): Promise<PublicPlan[]> {
  const res = await fetch(
    `${SERVER_BASE}/api/v1/public/studios/${encodeURIComponent(slug)}/plans`,
    { cache: 'no-store' },
  );
  if (!res.ok) return [];
  const data = await res.json();
  return (data.plans ?? []) as PublicPlan[];
}

export async function fetchPublicCampaign(
  studioSlug: string,
  campaignSlug: string,
): Promise<PublicCampaign | null> {
  const res = await fetch(
    `${SERVER_BASE}/api/v1/public/studios/${encodeURIComponent(studioSlug)}/campaigns/${encodeURIComponent(campaignSlug)}`,
    { cache: 'no-store' },
  );
  if (res.status === 404) return null;
  if (!res.ok) throw new Error(`API ${res.status}`);
  return (await res.json()) as PublicCampaign;
}
