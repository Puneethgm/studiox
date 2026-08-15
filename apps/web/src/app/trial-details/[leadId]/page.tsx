import { Suspense } from 'react';
import { TrialPaymentPage } from './TrialPaymentPage';

export default async function TrialDetailsPage({
  params,
}: {
  params: Promise<{ leadId: string }>;
}) {
  const { leadId } = await params;
  return (
    <Suspense>
      <TrialPaymentPage leadId={leadId} />
    </Suspense>
  );
}
