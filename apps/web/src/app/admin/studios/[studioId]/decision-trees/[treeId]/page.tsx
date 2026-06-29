import { serverFetch } from '@/lib/auth';
import type { DecisionTree } from '@/lib/types';
import { TreeEditor } from './TreeEditor';

export default async function TreeEditorPage({
  params,
}: {
  params: Promise<{ studioId: string; treeId: string }>;
}) {
  const { studioId, treeId } = await params;
  const tree = await serverFetch<DecisionTree>(
    `/api/v1/studios/${studioId}/decision-trees/${treeId}`,
  );

  return <TreeEditor studioId={studioId} initialTree={tree} />;
}
