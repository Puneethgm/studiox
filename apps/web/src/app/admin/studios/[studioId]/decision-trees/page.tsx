import Link from 'next/link';
import { GitBranch, CheckCircle2, Circle } from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { EmptyState } from '@/components/ui/EmptyState';
import { PageHeader } from '@/components/ui/PageHeader';
import { Badge } from '@/components/ui/Badge';
import { serverFetch } from '@/lib/auth';
import type { DecisionTree } from '@/lib/types';
import { HeaderActions } from '@/components/HeaderActions';
import { NewTreeButton } from './NewTreeButton';
import { DecisionTreesTabs } from './DecisionTreesTabs';

interface ListResp {
  trees: DecisionTree[];
}

export default async function DecisionTreesPage({
  params,
}: {
  params: Promise<{ studioId: string }>;
}) {
  const { studioId } = await params;
  const resp = await serverFetch<ListResp>(
    `/api/v1/studios/${studioId}/decision-trees`,
  );
  const trees = resp?.trees ?? [];

  return (
    <div className="space-y-6">
      <HeaderActions>
        <NewTreeButton studioId={studioId} />
      </HeaderActions>

      <PageHeader
        title="Decision Trees"
        description="Configure branching reply flows for customer messages. The active tree is used automatically when a customer message arrives."
      />

      <DecisionTreesTabs studioId={studioId} />

      {trees.length === 0 ? (
        <EmptyState
          icon={<GitBranch className="h-8 w-8" />}
          title="No decision trees yet"
          description="Create a tree to define how the AI should respond based on what customers ask."
          action={<NewTreeButton studioId={studioId} />}
        />
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {trees.map((tree) => (
            <Link
              key={tree.id}
              href={`/admin/studios/${studioId}/decision-trees/${tree.id}`}
              className="block group"
            >
              <Card className="p-5 hover:border-brand-500/50 transition-colors cursor-pointer h-full">
                <div className="flex items-start justify-between gap-3">
                  <div className="flex items-center gap-2 min-w-0">
                    <GitBranch className="h-4 w-4 shrink-0 text-brand-500" />
                    <span className="font-medium text-sm truncate group-hover:text-brand-500 transition-colors">
                      {tree.name}
                    </span>
                  </div>
                  {tree.isActive ? (
                    <Badge tone="success" className="shrink-0 flex items-center gap-1">
                      <CheckCircle2 className="h-3 w-3" />
                      Active
                    </Badge>
                  ) : (
                    <Badge tone="neutral" className="shrink-0 flex items-center gap-1">
                      <Circle className="h-3 w-3" />
                      Draft
                    </Badge>
                  )}
                </div>
                <div className="mt-2 flex flex-wrap items-center gap-1.5">
                  {tree.targetStatuses && tree.targetStatuses.length > 0 ? (
                    tree.targetStatuses.map((s) => (
                      <span key={s} className="rounded bg-violet-50 text-violet-600 border border-violet-200 dark:bg-violet-500/10 dark:border-violet-500/20 dark:text-violet-400 px-1.5 py-0.5 text-[9px] font-black uppercase tracking-wider">
                        {s.replace('_', ' ')}
                      </span>
                    ))
                  ) : (
                    <span className="text-xs text-gray-400">All leads</span>
                  )}
                  <span className="text-xs text-gray-400 ml-auto">
                    {new Date(tree.createdAt).toLocaleDateString()}
                  </span>
                </div>
              </Card>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
