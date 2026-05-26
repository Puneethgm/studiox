import PaymentsClient from '@/components/PaymentsClient';

export default function SuperAdminPaymentsPage() {
  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-black tracking-tight text-white">Global Billing & Payments</h1>
          <p className="text-xs text-zinc-400 font-semibold mt-0.5">Aggregate invoice monitoring and gateway integrations for all platform studios</p>
        </div>
      </div>
      <PaymentsClient studioId="global" />
    </div>
  );
}
