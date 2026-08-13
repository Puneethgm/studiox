import dagre from '@dagrejs/dagre';
import { MarkerType, type Node, type Edge } from '@xyflow/react';
import type { TreeNode, NodeAction } from '@/lib/types';

export const NODE_WIDTH = 220;
export const NODE_HEIGHT = 72;

const START_ID = '__start__';

export interface TreeNodeCardData {
  node: TreeNode;
  isSelected: boolean;
  isDeleting: boolean;
  hasChildren: boolean;
  onAddChild: (parentId: string, parentLabel: string, initialAction?: NodeAction) => void;
  onDelete: (nodeId: string) => void;
  [key: string]: unknown;
}

interface BuildOptions {
  selectedId?: string;
  deletingId: string | null;
  onAddChild: (parentId: string, parentLabel: string, initialAction?: NodeAction) => void;
  onDelete: (nodeId: string) => void;
}

interface FlatEntry {
  node: TreeNode;
  parentId: string | null;
}

function flatten(nodes: TreeNode[], parentId: string | null, out: FlatEntry[]) {
  for (const node of nodes) {
    out.push({ node, parentId });
    if (node.children?.length) {
      flatten(node.children, node.id, out);
    }
  }
}

function edgeStyle() {
  return { stroke: '#c4b5fd', strokeWidth: 1.5 };
}

function buildFlowNodesAndEdges(flat: FlatEntry[], opts: BuildOptions): { nodes: Node[]; edges: Edge[] } {
  const flowNodes: Node[] = [
    { id: START_ID, type: 'startNode', position: { x: 0, y: 0 }, draggable: false, data: {} },
    ...flat.map(({ node }) => ({
      id: node.id,
      type: 'treeNode',
      position: { x: 0, y: 0 },
      data: {
        node,
        isSelected: node.id === opts.selectedId,
        isDeleting: node.id === opts.deletingId,
        hasChildren: !!node.children?.length,
        onAddChild: opts.onAddChild,
        onDelete: opts.onDelete,
      } satisfies TreeNodeCardData,
    })),
  ];

  const flowEdges: Edge[] = flat.map(({ node, parentId }) => ({
    id: `${parentId ?? START_ID}-${node.id}`,
    source: parentId ?? START_ID,
    target: node.id,
    style: edgeStyle(),
    markerEnd: { type: MarkerType.ArrowClosed, color: '#c4b5fd' },
  }));

  return { nodes: flowNodes, edges: flowEdges };
}

function layoutWithDagre(nodes: Node[], edges: Edge[]): Node[] {
  const g = new dagre.graphlib.Graph();
  g.setGraph({ rankdir: 'TB', nodesep: 48, ranksep: 64 });
  g.setDefaultEdgeLabel(() => ({}));

  nodes.forEach((n) => g.setNode(n.id, { width: NODE_WIDTH, height: NODE_HEIGHT }));
  edges.forEach((e) => g.setEdge(e.source, e.target));

  dagre.layout(g);

  return nodes.map((n) => {
    const pos = g.node(n.id);
    return { ...n, position: { x: pos.x - NODE_WIDTH / 2, y: pos.y - NODE_HEIGHT / 2 } };
  });
}

export interface BuildTreeGraphResult {
  nodes: Node[];
  edges: Edge[];
  /** True when one or more nodes had no saved position, so a fallback
   * auto-layout ran — the caller should persist `computedPositions` once so
   * subsequent loads use saved positions instead of recomputing. */
  needsPersist: boolean;
  computedPositions?: { id: string; x: number; y: number }[];
}

export function buildTreeGraph(nodes: TreeNode[], opts: BuildOptions): BuildTreeGraphResult {
  const flat: FlatEntry[] = [];
  flatten(nodes, null, flat);

  const allPositioned = flat.length > 0 && flat.every(
    ({ node }) => typeof node.positionX === 'number' && typeof node.positionY === 'number',
  );

  const { nodes: flowNodes, edges: flowEdges } = buildFlowNodesAndEdges(flat, opts);

  if (allPositioned) {
    const positioned = flowNodes.map((n) => {
      if (n.id === START_ID) return n;
      const entry = flat.find((f) => f.node.id === n.id)!;
      return { ...n, position: { x: entry.node.positionX!, y: entry.node.positionY! } };
    });

    const roots = positioned.filter((n) => n.type === 'treeNode' && flat.find((f) => f.node.id === n.id)?.parentId === null);
    const startX = roots.length
      ? roots.reduce((sum, n) => sum + n.position.x, 0) / roots.length
      : 0;
    const startY = roots.length
      ? Math.min(...roots.map((n) => n.position.y)) - NODE_HEIGHT - 64
      : 0;
    const final = positioned.map((n) => (n.id === START_ID ? { ...n, position: { x: startX, y: startY } } : n));

    return { nodes: final, edges: flowEdges, needsPersist: false };
  }

  const laidOut = layoutWithDagre(flowNodes, flowEdges);
  const computedPositions = laidOut
    .filter((n) => n.id !== START_ID)
    .map((n) => ({ id: n.id, x: n.position.x, y: n.position.y }));

  return { nodes: laidOut, edges: flowEdges, needsPersist: true, computedPositions };
}

/**
 * Lays out every node in `nodes` in one single, mutually-consistent dagre
 * pass, ignoring any existing saved positions. Used after adding a node so
 * the new node AND its siblings all come from the same coherent layout —
 * computing a new node's position in isolation (a separate dagre call that
 * doesn't know about a sibling added a moment ago) can place it on top of
 * that already-saved sibling, since dagre isn't guaranteed to assign the
 * same coordinates to a node across two different calls with different
 * graphs. The tradeoff: any node the user has manually dragged elsewhere in
 * the tree gets reset to its auto-computed spot too, since this recomputes
 * the whole tree, not just the new branch. Worth it — a resettable layout
 * beats a canvas where sibling cards silently overlap.
 */
export function computeFullLayout(nodes: TreeNode[]): { id: string; x: number; y: number }[] {
  const flat: FlatEntry[] = [];
  flatten(nodes, null, flat);

  const { nodes: flowNodes, edges: flowEdges } = buildFlowNodesAndEdges(flat, {
    selectedId: undefined,
    deletingId: null,
    onAddChild: () => {},
    onDelete: () => {},
  });

  const laidOut = layoutWithDagre(flowNodes, flowEdges);
  return laidOut.filter((n) => n.id !== START_ID).map((n) => ({ id: n.id, x: n.position.x, y: n.position.y }));
}

/**
 * Suggests a rough initial position for a brand-new node under `parentId`
 * (or a new root when null), for the CREATE payload itself — the exact
 * final position gets reconciled immediately after via computeFullLayout,
 * so this only needs to be a reasonable placeholder, not perfectly final.
 */
export function computeNewNodePosition(existingNodes: TreeNode[], parentId: string | null): { x: number; y: number } {
  const flat: FlatEntry[] = [];
  flatten(existingNodes, null, flat);

  const placeholderId = '__new__';
  const placeholder: FlatEntry = {
    node: { id: placeholderId } as TreeNode,
    parentId,
  };

  const { nodes: flowNodes, edges: flowEdges } = buildFlowNodesAndEdges([...flat, placeholder], {
    selectedId: undefined,
    deletingId: null,
    onAddChild: () => {},
    onDelete: () => {},
  });

  const laidOut = layoutWithDagre(flowNodes, flowEdges);
  const found = laidOut.find((n) => n.id === placeholderId);
  return found ? { x: found.position.x, y: found.position.y } : { x: 0, y: 0 };
}
