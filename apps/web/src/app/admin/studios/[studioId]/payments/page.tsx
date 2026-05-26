import PaymentsClient from '@/components/PaymentsClient';

export default async function StudioPaymentsPage({
  params,
}: {
  params: Promise<{ studioId: string }>;
}) {
  const { studioId } = await params;
  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-black tracking-tight text-zinc-900 dark:text-white">Billing & Payments</h1>
          <p className="text-xs text-zinc-500 dark:text-zinc-400 font-semibold mt-0.5">Manage subscription invoices and link Stripe for your studio location</p>
        </div>
      </div>
      <PaymentsClient studioId={studioId} />
    </div>
  );
}
