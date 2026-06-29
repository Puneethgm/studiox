import { notFound } from 'next/navigation';
import { fetchPublicCampaign, fetchPublicStudio, fetchPublicPlans } from '@/lib/public';
import { BookingPageClient } from './BookingPageClient';

export default async function BookingPage({
  params,
  searchParams,
}: {
  params: Promise<{ studioSlug: string; campaignSlug: string }>;
  searchParams: Promise<{ leadId?: string; paid?: string }>;
}) {
  const { studioSlug, campaignSlug } = await params;
  const { leadId, paid } = await searchParams;

  if (!leadId) notFound();

  const [studio, campaign, plans] = await Promise.all([
    fetchPublicStudio(studioSlug),
    fetchPublicCampaign(studioSlug, campaignSlug),
    fetchPublicPlans(studioSlug),
  ]);

  if (!studio || !campaign) notFound();

  return (
    <BookingPageClient
      studio={studio}
      plans={plans}
      leadId={leadId}
      studioSlug={studioSlug}
      campaignSlug={campaignSlug}
      availabilitySlots={studio.availabilitySlots || {}}
      startAtSlot={paid === '1'}
    />
  );
}
