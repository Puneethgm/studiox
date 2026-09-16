import { Link2 } from 'lucide-react';
import { serverFetch } from '@/lib/auth';
import type { Studio } from '@/lib/types';
import { PageHeader } from '@/components/ui/PageHeader';
import { CrmIntegrationsClient } from './CrmIntegrationsClient';
import type { AiTaskConfig, CrmProvider } from './types';
import { CRM_DOC_PARSING_PURPOSE } from './types';

export const metadata = { title: 'CRM Integrations | 1herosocial.ai' };

export default async function CrmIntegrationsPage() {
  const [{ providers }, { studios }, aiConfig] = await Promise.all([
    serverFetch<{ providers: CrmProvider[] }>('/api/v1/admin/crm-providers'),
    serverFetch<{ studios: Studio[] }>('/api/v1/admin/studios'),
    serverFetch<AiTaskConfig>(`/api/v1/admin/ai-task-configs/${CRM_DOC_PARSING_PURPOSE}`),
  ]);

  return (
    <div className="space-y-8 pb-12">
      <PageHeader
        title={
          <span className="flex items-center gap-3">
            <span className="grid h-10 w-10 place-items-center rounded-2xl bg-gradient-to-br from-brand-500 to-violet-600 text-white shadow-lg shadow-brand-500/25">
              <Link2 className="h-5 w-5" />
            </span>
            CRM Integrations
          </span>
        }
        description="Onboard a new CRM by uploading its API docs, review what it can do, then connect it to a studio. No code changes needed to add a new CRM."
      />
      <CrmIntegrationsClient
        initialProviders={providers ?? []}
        studios={(studios ?? []).filter((s) => s.active)}
        initialAiConfig={aiConfig}
      />
    </div>
  );
}
