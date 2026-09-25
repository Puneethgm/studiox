import PaymentsClient from '@/components/PaymentsClient';

export default async function StudioPaymentsPage({
  params,
}: {
  params: Promise<{ studioId: string }>;
}) {
  const { studioId } = await params;
  return (
    <div className="space-y-4">
      <PaymentsClient studioId={studioId} />
    </div>
  );
}
