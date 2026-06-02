import PaymentsClient from '@/components/PaymentsClient';

export default async function StudioPaymentsPage({
  params,
}: {
  params: Promise<{ studioId: string }>;
}) {
  const { studioId } = await params;
  return (
    <div className="space-y-4">
      <div className="text-[11px] font-semibold text-zinc-400 dark:text-zinc-500 px-2 leading-relaxed">
        Manage subscription invoices and link Stripe for your studio location. Payments are securely routed to your connected Stripe account. Subscriptions are billed automatically.
      </div>
      <PaymentsClient studioId={studioId} />
    </div>
  );
}
