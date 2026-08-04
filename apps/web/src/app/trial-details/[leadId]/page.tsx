import { TrialDetailsForm } from './TrialDetailsForm';

export default async function TrialDetailsPage({
  params,
}: {
  params: Promise<{ leadId: string }>;
}) {
  const { leadId } = await params;
  return <TrialDetailsForm leadId={leadId} />;
}
