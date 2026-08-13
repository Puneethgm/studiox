'use client';

import '@xyflow/react/dist/style.css';
import { useMemo, useEffect, useRef } from 'react';
import {
  ReactFlow, Background, Controls, MiniMap,
  type NodeMouseHandler, type OnNodeDrag,
} from '@xyflow/react';
import type { TreeNode, NodeAction } from '@/lib/types';
import { buildTreeGraph, type TreeNodeCardData } from './treeLayout';
import { TreeNodeCard } from './TreeNodeCard';
import { StartAnchorNode } from './StartAnchorNode';

const nodeTypes = { treeNode: TreeNodeCard, startNode: StartAnchorNode };

interface Props {
  nodes: TreeNode[];
  selectedId?: string;
  deletingId: string | null;
  onSelect: (node: TreeNode) => void;
  onAddChild: (parentId: string, parentLabel: string, initialAction?: NodeAction) => void;
  onDelete: (nodeId: string) => void;
  onNodePositionChange: (nodeId: string, x: number, y: number) => void;
  onHealPositions: (positions: { id: string; x: number; y: number }[]) => void;
  onOpenTriggerConfig: () => void;
}

export function TreeCanvas({
  nodes, selectedId, deletingId, onSelect, onAddChild, onDelete,
  onNodePositionChange, onHealPositions, onOpenTriggerConfig,
}: Props) {
  const { nodes: flowNodes, edges: flowEdges, needsPersist, computedPositions } = useMemo(
    () => buildTreeGraph(nodes, { selectedId, deletingId, onAddChild, onDelete }),
    [nodes, selectedId, deletingId, onAddChild, onDelete],
  );

  const healedRef = useRef(false);
  useEffect(() => {
    if (needsPersist && computedPositions?.length && !healedRef.current) {
      healedRef.current = true;
      onHealPositions(computedPositions);
    }
    if (!needsPersist) {
      healedRef.current = false;
    }
  }, [needsPersist, computedPositions, onHealPositions]);

  const handleNodeClick: NodeMouseHandler = (_evt, flowNode) => {
    if (flowNode.type === 'startNode') {
      onOpenTriggerConfig();
      return;
    }
    if (flowNode.type !== 'treeNode') return;
    onSelect((flowNode.data as TreeNodeCardData).node);
  };

  const handleNodeDragStop: OnNodeDrag = (_evt, flowNode) => {
    if (flowNode.type !== 'treeNode') return;
    onNodePositionChange(flowNode.id, flowNode.position.x, flowNode.position.y);
  };

  return (
    <div className="h-[600px] w-full overflow-visible rounded-lg border border-gray-100 bg-gray-50/50 dark:border-zinc-800 dark:bg-zinc-950/30">
      <ReactFlow
        nodes={flowNodes}
        edges={flowEdges}
        nodeTypes={nodeTypes}
        onNodeClick={handleNodeClick}
        onNodeDragStop={handleNodeDragStop}
        nodesDraggable
        nodesConnectable={false}
        elementsSelectable
        fitView
        proOptions={{ hideAttribution: true }}
      >
        <Background gap={20} color="#e5e7eb" />
        <Controls showInteractive={false} position="bottom-left" />
        {/* Not pannable/zoomable — purely a visual overview. A node laid out
            near the bottom-right corner can end up underneath this fixed
            panel; making it non-interactive means it can never block that
            node's "+"/delete buttons. Zoom/pan are already covered by
            Controls and canvas-drag, so nothing is lost. */}
        <MiniMap position="bottom-right" className="!bg-white" style={{ pointerEvents: 'none' }} />
      </ReactFlow>
    </div>
  );
}
