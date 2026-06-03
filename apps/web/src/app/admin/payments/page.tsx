import PaymentsClient from '@/components/PaymentsClient';

export const metadata = {
  title: 'Platform Payments | 1herosocial.ai',
};

export default function SuperAdminPaymentsPage() {
  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-black tracking-tight text-zinc-900 dark:text-white">Global Billing & Payments</h1>
          <p className="text-xs text-zinc-500 dark:text-zinc-400 font-semibold mt-0.5">Aggregate invoice monitoring and gateway integrations for all platform studios</p>
        </div>
      </div>
      <PaymentsClient studioId="global" />
    </div>
  );
}
