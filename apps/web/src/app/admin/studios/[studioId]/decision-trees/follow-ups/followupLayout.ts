import { MarkerType, type Node, type Edge } from '@xyflow/react';

export const NODE_WIDTH = 220;
export const NODE_HEIGHT = 72;
const RANK_GAP = 64;
const START_ID = '__start__';

export interface StepDraft {
  key: number;
  delayValue: number;
  delayUnit: 'minutes' | 'hours' | 'days';
  messageTemplate: string;
}

export interface FollowupStepCardData {
  step: StepDraft;
  index: number;
  isSelected: boolean;
  isLast: boolean;
  onSelect: (key: number) => void;
  onAddNext: () => void;
  onDelete: (key: number) => void;
  [key: string]: unknown;
}

function edgeStyle() {
  return { stroke: '#c4b5fd', strokeWidth: 1.5 };
}

/**
 * Follow-up steps are always a single, strictly linear chain (no branching —
 * unlike a decision tree, there's nothing to match, just "wait, then send
 * the next one"), so positions are a plain vertical stack rather than a
 * dagre layout — dagre would be a no-op here anyway for a single-child chain.
 */
export function buildFollowupGraph(steps: StepDraft[], selectedKey: number | null, callbacks: {
  onSelect: (key: number) => void;
  onAddNext: () => void;
  onDelete: (key: number) => void;
}): { nodes: Node[]; edges: Edge[] } {
  const startY = 0;
  const nodes: Node[] = [
    { id: START_ID, type: 'startNode', position: { x: 0, y: startY }, draggable: false, data: {} },
    ...steps.map((step, i) => ({
      id: String(step.key),
      type: 'followupNode',
      position: { x: 0, y: startY + (i + 1) * (NODE_HEIGHT + RANK_GAP) },
      draggable: false,
      data: {
        step,
        index: i,
        isSelected: step.key === selectedKey,
        isLast: i === steps.length - 1,
        onSelect: callbacks.onSelect,
        onAddNext: callbacks.onAddNext,
        onDelete: callbacks.onDelete,
      } satisfies FollowupStepCardData,
    })),
  ];

  const edges: Edge[] = steps.map((step, i) => {
    const sourceId = i === 0 ? START_ID : String(steps[i - 1]?.key ?? START_ID);
    return {
      id: `${sourceId}-${step.key}`,
      source: sourceId,
      target: String(step.key),
      style: edgeStyle(),
      markerEnd: { type: MarkerType.ArrowClosed, color: '#c4b5fd' },
    };
  });

  return { nodes, edges };
}
