'use client';

import '@xyflow/react/dist/style.css';
import { useMemo } from 'react';
import { ReactFlow, Background, Controls, MiniMap, type NodeMouseHandler } from '@xyflow/react';
import { StartAnchorNode } from '../[treeId]/StartAnchorNode';
import { FollowupStepCard } from './FollowupStepCard';
import { buildFollowupGraph, type StepDraft } from './followupLayout';

const nodeTypes = { followupNode: FollowupStepCard, startNode: StartAnchorNode };

interface Props {
  steps: StepDraft[];
  selectedKey: number | null;
  onSelect: (key: number) => void;
  onAddNext: () => void;
  onDelete: (key: number) => void;
}

export function FollowupCanvas({ steps, selectedKey, onSelect, onAddNext, onDelete }: Props) {
  const { nodes, edges } = useMemo(
    () => buildFollowupGraph(steps, selectedKey, { onSelect, onAddNext, onDelete }),
    [steps, selectedKey, onSelect, onAddNext, onDelete],
  );

  const handleNodeClick: NodeMouseHandler = (_evt, flowNode) => {
    if (flowNode.type !== 'followupNode') return;
    onSelect(Number(flowNode.id));
  };

  return (
    <div className="h-[600px] w-full overflow-visible rounded-lg border border-gray-100 bg-gray-50/50 dark:border-zinc-800 dark:bg-zinc-950/30">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        onNodeClick={handleNodeClick}
        nodesDraggable={false}
        nodesConnectable={false}
        elementsSelectable
        fitView
        proOptions={{ hideAttribution: true }}
      >
        <Background gap={20} color="#e5e7eb" />
        <Controls showInteractive={false} position="bottom-left" />
        <MiniMap position="bottom-right" className="!bg-white" style={{ pointerEvents: 'none' }} />
      </ReactFlow>
    </div>
  );
}
