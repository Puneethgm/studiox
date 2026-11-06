import { serverFetch } from '@/lib/auth';
import { PageHeader } from '@/components/ui/PageHeader';
import { OffersManagement } from './OffersManagement';

export interface Offer {
  id: string;
  studioId: string;
  name: string;
  description: string;
  terms: string;
  priceSgd: number;
  isActive: boolean;
  createdAt: string;
  updatedAt: string;
}

export default async function OffersPage({
  params,
}: {
  params: Promise<{ studioId: string }>;
}) {
  const { studioId } = await params;
  const resp = await serverFetch<{ offers: Offer[] }>(`/api/v1/me/studios/${studioId}/offers`);

  return (
    <div className="space-y-6">
      <PageHeader
        title="Offers"
        description="Structured facts the AI hands to customers directly — pricing and trial offers it's allowed to quote, instead of guessing or inventing them (ai-reply-flow-llm-design)."
      />
      <OffersManagement studioId={studioId} initialOffers={resp?.offers ?? []} />
    </div>
  );
}
