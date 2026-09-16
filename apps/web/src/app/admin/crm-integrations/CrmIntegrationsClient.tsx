'use client';

import { useState } from 'react';
import { Plus, Boxes } from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Badge } from '@/components/ui/Badge';
import { EmptyState } from '@/components/ui/EmptyState';
import type { Studio } from '@/lib/types';
import { AiSettingsCard } from './AiSettingsCard';
import { AddCrmForm } from './AddCrmForm';
import { OperationReviewTable } from './OperationReviewTable';
import { StudioConnectionCard } from './StudioConnectionCard';
import { getProviderOperations } from './actions';
import type { AiTaskConfig, CrmOperation, CrmProvider } from './types';

export function CrmIntegrationsClient({
  initialProviders,
  studios,
  initialAiConfig,
}: {
  initialProviders: CrmProvider[];
  studios: Studio[];
  initialAiConfig: AiTaskConfig;
}) {
  const [providers, setProviders] = useState(initialProviders);
  const [aiConfig, setAiConfig] = useState(initialAiConfig);
  const [adding, setAdding] = useState(false);
  const [reviewing, setReviewing] = useState<{ provider: CrmProvider; operations: CrmOperation[] } | null>(null);

  const activeProviders = providers.filter((p) => p.status === 'active');

  return (
    <div className="space-y-8">
      <AiSettingsCard initialConfig={aiConfig} />

      <Card
        title={<span className="flex items-center gap-2"><Boxes className="h-4 w-4 text-brand-500" />Your CRMs</span>}
        subtitle="Every CRM your studios can connect to."
        action={
          !adding && !reviewing && (
            <Button size="sm" leftIcon={<Plus className="h-3.5 w-3.5" />} onClick={() => setAdding(true)}>
              Add CRM
            </Button>
          )
        }
      >
        {adding && (
          <AddCrmForm
            aiConfigured={aiConfig.configured}
            onCancel={() => setAdding(false)}
            onAnalyzed={(provider, operations) => {
              setAdding(false);
              setProviders((prev) => [provider, ...prev]);
              setReviewing({ provider, operations });
            }}
          />
        )}

        {!adding && providers.length === 0 && (
          <EmptyState
            icon={<Boxes className="h-6 w-6" />}
            title="No CRMs yet"
            description='Click "Add CRM" and paste in the CRM’s API documentation to get started.'
          />
        )}

        {!adding && providers.length > 0 && (
          <div className="space-y-2">
            {providers.map((p) => (
              <button
                key={p.id}
                type="button"
                onClick={async () => {
                  const res = await getProviderOperations(p.id);
                  setReviewing({ provider: p, operations: res.ok ? res.data : [] });
                }}
                className="flex w-full items-center justify-between gap-3 rounded-lg border border-zinc-200 px-4 py-3 text-left transition-colors hover:border-brand-300 hover:bg-brand-50/30 dark:border-zinc-800 dark:hover:border-brand-700 dark:hover:bg-brand-500/5"
              >
                <div>
                  <div className="text-sm font-bold text-zinc-800 dark:text-zinc-100">{p.name}</div>
                  <div className="text-xs text-zinc-500">{p.description || p.baseUrl || 'No description'}</div>
                </div>
                <Badge tone={p.status === 'active' ? 'success' : 'warning'}>{p.status}</Badge>
              </button>
            ))}
          </div>
        )}
      </Card>

      {reviewing && (
        <OperationReviewTable
          provider={reviewing.provider}
          operations={reviewing.operations}
          onClose={() => setReviewing(null)}
          onChanged={(updated) => {
            setReviewing((prev) => (prev ? { ...prev, provider: updated } : prev));
            setProviders((prev) => prev.map((p) => (p.id === updated.id ? updated : p)));
          }}
        />
      )}

      <StudioConnectionCard studios={studios} activeProviders={activeProviders} />
    </div>
  );
}
