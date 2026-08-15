import { TrialPaymentPage } from './TrialPaymentPage';

export default async function TrialDetailsPage({
  params,
}: {
  params: Promise<{ leadId: string }>;
}) {
  const { leadId } = await params;
  return <TrialPaymentPage leadId={leadId} />;
}
