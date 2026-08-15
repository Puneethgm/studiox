import { Suspense } from 'react';
import { TrialPaymentPage } from '../../[leadId]/TrialPaymentPage';

// Static, no-lead-attached trial signup link — safe to share manually
// (WhatsApp broadcast, bio link, etc.). A new lead is created on submit,
// unlike the per-lead /trial-details/{leadId} link sent automatically after
// a specific conversation.
export default async function TrialDetailsStudioPage({
  params,
}: {
  params: Promise<{ studioSlug: string }>;
}) {
  const { studioSlug } = await params;
  return (
    <Suspense>
      <TrialPaymentPage standalone studioSlug={studioSlug} />
    </Suspense>
  );
}
