import { User, ArrowLeft } from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { serverFetch } from '@/lib/auth';
import { formatDateTime } from '@/lib/datetime';
import type { Lead } from '@/lib/types';
import { LeadEditor } from './editor';

export default async function LeadDetailPage({
  params,
}: {
  params: Promise<{ studioId: string; id: string }>;
}) {
  const { studioId, id } = await params;
  const lead = await serverFetch<Lead>(`/api/v1/studios/${studioId}/leads/${id}`);

  return (
    <div className="space-y-6">
      {/* Premium Glass Header */}
      <div
        className="relative overflow-hidden rounded-[26px] border border-white/30 p-6 backdrop-blur-2xl dark:border-white/5"
        style={{
          background: 'linear-gradient(135deg, rgba(255,255,255,0.30) 0%, rgba(237,233,254,0.22) 60%, rgba(219,234,254,0.20) 100%)',
          boxShadow: 'inset 0 0 0 1px rgba(255,255,255,0.2), 0 8px 32px rgba(139,92,246,0.07)',
        }}
      >
        <div className="pointer-events-none absolute -right-16 -top-16 h-48 w-48 rounded-full bg-brand-500/10 blur-[70px]" />
        
        <div className="relative flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex items-center gap-4">
            <div className="grid h-12 w-12 place-items-center rounded-2xl bg-gradient-to-br from-brand-500 to-violet-600 text-white shadow-lg shadow-brand-500/25">
              <User className="h-6 w-6" />
            </div>
            <div>
              <h1 className="text-2xl font-black tracking-tight text-zinc-900 dark:text-white">{lead.name}</h1>
              <p className="mt-0.5 text-xs font-semibold text-zinc-500 dark:text-zinc-400">
                From <span className="font-bold text-brand-600 dark:text-brand-400">{lead.campaignName ?? lead.campaignId}</span> · {formatDateTime(lead.createdAt)}
              </p>
            </div>
          </div>
          <Link href={`/admin/studios/${studioId}/leads`}>
            <Button
              variant="secondary"
              leftIcon={<ArrowLeft className="h-4 w-4" />}
              suppressHydrationWarning
            >
              Back to Leads
            </Button>
          </Link>
        </div>
      </div>

      <div className="grid gap-6 md:grid-cols-3">
        <div className="space-y-6 md:col-span-2">
          <Card title="Contact details">
            <dl className="grid grid-cols-2 gap-x-6 gap-y-5 text-sm">
              <Field label="Email" value={lead.email} />
              <Field label="Phone" value={lead.phone} />
              <Field label="Fitness plan" value={lead.fitnessPlan} />
              <Field label="Source" value={lead.source} />
              {lead.goals && <Field label="Goals" value={lead.goals} className="col-span-2" />}
            </dl>
          </Card>
        </div>

        <div className="md:col-span-1">
          <LeadEditor studioId={studioId} lead={lead} />
        </div>
      </div>
    </div>
  );
}

function Field({ label, value, className }: { label: string; value: string; className?: string }) {
  return (
    <div className={className}>
      <dt className="text-[10px] font-black uppercase tracking-[0.18em] text-zinc-400">
        {label}
      </dt>
      <dd className="mt-1.5 break-words font-semibold text-zinc-900 dark:text-zinc-100">{value || '—'}</dd>
    </div>
  );
}
