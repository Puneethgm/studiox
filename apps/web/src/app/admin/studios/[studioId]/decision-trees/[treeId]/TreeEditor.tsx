'use client';

import { useState, useCallback, useEffect, useRef } from 'react';
import { useRouter } from 'next/navigation';
import {
  GitBranch, Plus, Trash2, Sparkles, Loader2,
  CheckCircle2, Circle, Play, ArrowLeft, Save, CornerDownRight, X,
} from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { Badge } from '@/components/ui/Badge';
import { MultiSelect } from '@/components/ui/MultiSelect';
import { api, ApiError } from '@/lib/api';
import type { DecisionTree, TreeNode, ConditionType, NodeAction, SimulateResult } from '@/lib/types';
import { ImportNodesButton } from './ImportNodesButton';
import { TreeCanvas } from './TreeCanvas';
import { BlockTypePicker } from './BlockTypePicker';
import { CONDITION_LABELS, ACTION_LABELS, ACTION_COLORS, LEAD_STATUSES } from './treeNodeMeta';
import { computeNewNodePosition, computeFullLayout } from './treeLayout';

interface Props {
  studioId: string;
  initialTree: DecisionTree;
}

interface NodeFormState {
  label: string;
  conditionType: ConditionType;
  keywords: string;
  leadStatuses: string[]; // for lead_status condition
  replyTemplate: string;
  action: NodeAction;
  targetStatus: string; // for change_status action
  sortOrder: number; // auto-computed, not shown in UI
}

type PanelMode =
  | { kind: 'idle' }
  | { kind: 'addRoot' }
  | { kind: 'addChild'; parentId: string; parentLabel: string }
  | { kind: 'edit'; node: TreeNode };

interface Toast {
  id: number;
  message: string;
  type: 'success' | 'error';
}

interface ConfirmDialog {
  title: string;
  message: string;
  confirmLabel: string;
  onConfirm: () => void;
}

const defaultForm = (): NodeFormState => ({
  label: '',
  conditionType: 'keyword',
  keywords: '',
  leadStatuses: [],
  replyTemplate: '',
  action: 'reply',
  targetStatus: '',
  sortOrder: 0,
});

function countChildren(nodes: TreeNode[], parentId: string): number {
  for (const n of nodes) {
    if (n.id === parentId) return n.children?.length ?? 0;
    if (n.children?.length) {
      const found = countChildren(n.children, parentId);
      if (found !== -1) return found;
    }
  }
  return -1; // not found in this branch
}

export function TreeEditor({ studioId, initialTree }: Props) {
  const router = useRouter();
  const [tree, setTree] = useState<DecisionTree>(initialTree);
  const [panel, setPanel] = useState<PanelMode>({ kind: 'idle' });
  const [form, setForm] = useState<NodeFormState>(defaultForm());
  const [saving, setSaving] = useState(false);
  const [toggling, setToggling] = useState(false);
  const [deleting, setDeleting] = useState<string | null>(null);
  const [suggesting, setSuggesting] = useState(false);

  const [toasts, setToasts] = useState<Toast[]>([]);
  const [confirm, setConfirm] = useState<ConfirmDialog | null>(null);

  function showToast(message: string, type: 'success' | 'error' = 'success') {
    const id = Date.now();
    setToasts((prev) => [...prev, { id, message, type }]);
    setTimeout(() => setToasts((prev) => prev.filter((t) => t.id !== id)), 3000);
  }

  const [simMessage, setSimMessage] = useState('');
  const [simLeadStatus, setSimLeadStatus] = useState('');
  const [simResult, setSimResult] = useState<SimulateResult | null>(null);
  const [simLoading, setSimLoading] = useState(false);
  const [showSim, setShowSim] = useState(false);
  const [headerPickerOpen, setHeaderPickerOpen] = useState(false);
  const [emptyStatePickerOpen, setEmptyStatePickerOpen] = useState(false);
  const headerAddRef = useRef<HTMLDivElement>(null);
  const emptyStateAddRef = useRef<HTMLDivElement>(null);
  const [pipelineModalOpen, setPipelineModalOpen] = useState(false);
  // Tracks whether the current Keywords field content came from the AI
  // auto-suggest (safe to silently replace/clear as label/reply change) or
  // was typed by the admin themselves (never touched automatically again).
  const [keywordsIsAI, setKeywordsIsAI] = useState(false);
  // label+replyTemplate combo we last generated (or scheduled a generation)
  // for — prevents the effect from re-triggering itself just because
  // applying a suggestion changes form.keywords (which is one of its own
  // dependencies). Only an actual label/reply change should fire it again.
  const lastSuggestKeyRef = useRef<string | null>(null);

  const refreshTree = useCallback(async () => {
    const updated = await api<DecisionTree>(
      `/api/v1/studios/${studioId}/decision-trees/${tree.id}`,
    );
    setTree(updated);
  }, [studioId, tree.id]);

  // Fire-and-forget: persist a dragged node's new position. No toast/loading
  // state — this is a minor, frequent adjustment, not a tracked save action.
  const handleNodePositionChange = useCallback((nodeId: string, x: number, y: number) => {
    api(`/api/v1/studios/${studioId}/decision-trees/${tree.id}/nodes/${nodeId}`, {
      method: 'PATCH',
      json: { positionX: x, positionY: y },
    }).catch(() => {});
  }, [studioId, tree.id]);

  // Runs once whenever the canvas had to fall back to auto-layout (a node had
  // no saved position yet — first load after this feature shipped, or right
  // after a bulk XLSX import) so the computed layout is saved and won't jump
  // around on the next load. Silent, no toast. Refreshes `tree` afterward so
  // the just-healed positions show up in local state — otherwise the fallback
  // keeps re-triggering every render (state still shows null positions).
  const handleHealPositions = useCallback(async (positions: { id: string; x: number; y: number }[]) => {
    await Promise.all(positions.map(({ id, x, y }) =>
      api(`/api/v1/studios/${studioId}/decision-trees/${tree.id}/nodes/${id}`, {
        method: 'PATCH',
        json: { positionX: x, positionY: y },
      }).catch(() => {}),
    ));
    await refreshTree();
  }, [studioId, tree.id, refreshTree]);

  function openAddRoot(initialAction?: NodeAction) {
    setPanel({ kind: 'addRoot' });
    setForm({ ...defaultForm(), action: initialAction ?? defaultForm().action, sortOrder: tree.nodes?.length ?? 0 });
    setKeywordsIsAI(false);
    lastSuggestKeyRef.current = null;
  }

  function openAddChild(parentId: string, parentLabel: string, initialAction?: NodeAction) {
    setPanel({ kind: 'addChild', parentId, parentLabel });
    const childCount = countChildren(tree.nodes ?? [], parentId);
    setForm({ ...defaultForm(), action: initialAction ?? defaultForm().action, sortOrder: Math.max(0, childCount) });
    setKeywordsIsAI(false);
    lastSuggestKeyRef.current = null;
  }

  function openEdit(node: TreeNode) {
    const kws = (node.conditionValue?.keywords as string[] | undefined) ?? [];
    const statuses = (node.conditionValue?.statuses as string[] | undefined) ?? [];
    const targetStatus = (node.actionValue?.target_status as string | undefined) ?? '';
    setPanel({ kind: 'edit', node });
    setForm({
      label: node.label,
      conditionType: node.conditionType,
      keywords: kws.join(', '),
      leadStatuses: statuses,
      replyTemplate: node.replyTemplate,
      action: node.action,
      targetStatus,
      sortOrder: node.sortOrder,
    });
    setKeywordsIsAI(false);
    lastSuggestKeyRef.current = null;
  }

  function closePanel() {
    setPanel({ kind: 'idle' });
  }

  async function handleToggleActive() {
    setToggling(true);
    try {
      const updated = await api<DecisionTree>(
        `/api/v1/studios/${studioId}/decision-trees/${tree.id}`,
        { method: 'PATCH', json: { isActive: !tree.isActive } },
      );
      setTree(updated);
    } finally {
      setToggling(false);
    }
  }

  async function handleSaveTargetStatuses(statuses: string[]) {
    const updated = await api<DecisionTree>(
      `/api/v1/studios/${studioId}/decision-trees/${tree.id}`,
      { method: 'PATCH', json: { targetStatuses: statuses } },
    );
    setTree(updated);
    showToast('Pipeline group updated');
  }

  function handleDeleteTree() {
    setConfirm({
      title: 'Delete tree',
      message: `Delete "${tree.name}"? This will remove all nodes and cannot be undone.`,
      confirmLabel: 'Delete tree',
      onConfirm: async () => {
        await api(`/api/v1/studios/${studioId}/decision-trees/${tree.id}`, { method: 'DELETE' });
        showToast('Tree deleted successfully');
        router.push(`/admin/studios/${studioId}/decision-trees`);
      },
    });
  }

  const suggestKeywords = useCallback(async (label: string, replyTemplate: string, keywordsSnapshot: string) => {
    setSuggesting(true);
    try {
      const { keywords } = await api<{ keywords: string[] }>(
        `/api/v1/studios/${studioId}/decision-trees/suggest-keywords`,
        { method: 'POST', json: { label, replyTemplate } },
      );
      setForm((f) => {
        // If the keywords field changed since this request started (the
        // admin typed their own, or another suggestion already landed),
        // this result is stale — don't clobber whatever's there now.
        if (f.keywords !== keywordsSnapshot) return f;
        return { ...f, keywords: keywords.join(', ') };
      });
      setKeywordsIsAI(true);
    } catch (err) {
      const message = err instanceof ApiError ? err.message : 'Could not generate keyword suggestions';
      showToast(
        message.includes('no AI key configured')
          ? 'No AI key configured for this studio — add a Groq API key in Settings to enable keyword suggestions'
          : message,
        'error',
      );
    } finally {
      setSuggesting(false);
    }
  }, [studioId]);

  // Keeps Keywords in sync with Label/Reply Template as the admin edits them
  // — no manual trigger. Only ever touches AI-generated content: once the
  // admin types their own keywords, this stops adjusting them entirely.
  // Guarded by lastSuggestKeyRef so applying a suggestion (which changes
  // form.keywords, one of this effect's own dependencies) doesn't cause the
  // effect to immediately re-schedule itself — only an actual change to
  // label/replyTemplate counts as something new to generate for.
  useEffect(() => {
    if (panel.kind === 'idle') return;
    if (form.conditionType !== 'keyword') return;
    if (form.keywords.trim() && !keywordsIsAI) return; // admin's own — hands off

    if (!form.label.trim()) {
      if (form.keywords.trim()) {
        setForm((f) => ({ ...f, keywords: '' }));
        setKeywordsIsAI(false);
      }
      lastSuggestKeyRef.current = null;
      return;
    }

    const key = `${form.label} ${form.replyTemplate}`;
    if (key === lastSuggestKeyRef.current) return; // nothing actually changed — don't re-fire

    const timer = setTimeout(() => {
      lastSuggestKeyRef.current = key;
      suggestKeywords(form.label, form.replyTemplate, form.keywords);
    }, 900);
    return () => clearTimeout(timer);
  }, [panel.kind, form.conditionType, form.label, form.replyTemplate, form.keywords, keywordsIsAI, suggestKeywords]);

  async function handleSave() {
    if (!form.label.trim()) return;
    setSaving(true);
    try {
      const conditionValue =
        form.conditionType === 'keyword'
          ? { keywords: form.keywords.split(',').map((k) => k.trim()).filter(Boolean) }
          : form.conditionType === 'intent'
          ? { intent: form.keywords.trim() }
          : form.conditionType === 'sentiment'
          ? { sentiment: form.keywords.trim() }
          : form.conditionType === 'lead_status'
          ? { statuses: form.leadStatuses }
          : {};

      const actionValue =
        form.action === 'change_status' && form.targetStatus
          ? { target_status: form.targetStatus }
          : {};

      const nodePayload = {
        label: form.label,
        conditionType: form.conditionType,
        conditionValue,
        replyTemplate: form.replyTemplate,
        action: form.action,
        actionValue,
        sortOrder: form.sortOrder,
      };

      if (panel.kind === 'edit') {
        await api(
          `/api/v1/studios/${studioId}/decision-trees/${tree.id}/nodes/${panel.node.id}`,
          { method: 'PATCH', json: nodePayload },
        );
      } else {
        const parentId = panel.kind === 'addChild' ? panel.parentId : null;
        const position = computeNewNodePosition(tree.nodes ?? [], parentId);
        await api(`/api/v1/studios/${studioId}/decision-trees/${tree.id}/nodes`, {
          method: 'POST',
          json: { ...nodePayload, parentId, positionX: position.x, positionY: position.y },
        });
        // Re-layout the whole tree in one coherent pass now that a new node
        // exists, so it and its siblings never end up overlapping (see
        // computeFullLayout's doc comment for why a per-node calc isn't enough).
        const updated = await api<DecisionTree>(`/api/v1/studios/${studioId}/decision-trees/${tree.id}`);
        const layout = computeFullLayout(updated.nodes ?? []);
        await Promise.all(layout.map(({ id, x, y }) =>
          api(`/api/v1/studios/${studioId}/decision-trees/${tree.id}/nodes/${id}`, {
            method: 'PATCH',
            json: { positionX: x, positionY: y },
          }).catch(() => {}),
        ));
      }
      await refreshTree();
      showToast(panel.kind === 'edit' ? 'Node updated successfully' : 'Node added successfully');
      closePanel();
    } finally {
      setSaving(false);
    }
  }

  function handleDeleteNode(nodeId: string) {
    setConfirm({
      title: 'Delete node',
      message: 'Delete this node and all its children? This cannot be undone.',
      confirmLabel: 'Delete node',
      onConfirm: async () => {
        setDeleting(nodeId);
        try {
          await api(
            `/api/v1/studios/${studioId}/decision-trees/${tree.id}/nodes/${nodeId}`,
            { method: 'DELETE' },
          );
          await refreshTree();
          showToast('Node deleted successfully');
          if (panel.kind === 'edit' && panel.node.id === nodeId) closePanel();
        } finally {
          setDeleting(null);
        }
      },
    });
  }

  async function handleSimulate() {
    if (!simMessage.trim()) return;
    setSimLoading(true);
    setSimResult(null);
    try {
      const result = await api<SimulateResult>(
        `/api/v1/studios/${studioId}/decision-trees/${tree.id}/simulate`,
        { method: 'POST', json: { message: simMessage, leadStatus: simLeadStatus || undefined } },
      );
      setSimResult(result);
    } finally {
      setSimLoading(false);
    }
  }

  const selectedNodeId = panel.kind === 'edit' ? panel.node.id : undefined;
  const isPanelOpen = panel.kind !== 'idle';

  // Panel title + context banner
  let panelTitle = 'Add root node';
  let panelBanner: React.ReactNode = null;
  if (panel.kind === 'addChild') {
    panelTitle = 'Add child node';
    panelBanner = (
      <div className="flex items-center gap-2 rounded-lg bg-violet-50 border border-violet-200 px-3 py-2 text-xs text-violet-700">
        <CornerDownRight className="h-3.5 w-3.5 shrink-0" />
        <span>Child of: <strong>{panel.parentLabel}</strong></span>
      </div>
    );
  } else if (panel.kind === 'edit') {
    panelTitle = 'Edit node';
  }

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between gap-4 flex-wrap">
        <div className="flex items-center gap-3">
          <button
            onClick={() => router.push(`/admin/studios/${studioId}/decision-trees`)}
            className="text-gray-400 hover:text-gray-600 transition-colors"
          >
            <ArrowLeft className="h-5 w-5" />
          </button>
          <div className="flex items-center gap-2">
            <GitBranch className="h-5 w-5 text-brand-500" />
            <h1 className="text-lg font-semibold">{tree.name}</h1>
            {tree.isActive ? (
              <Badge tone="success" className="flex items-center gap-1">
                <CheckCircle2 className="h-3 w-3" /> Active
              </Badge>
            ) : (
              <Badge tone="neutral" className="flex items-center gap-1">
                <Circle className="h-3 w-3" /> Draft
              </Badge>
            )}
          </div>
        </div>
        <div className="flex items-center gap-2">
          <ImportNodesButton studioId={studioId} treeId={tree.id} onImported={refreshTree} />
          <Button
            variant="ghost"
            size="sm"
            leftIcon={<Play className="h-3.5 w-3.5" />}
            onClick={() => setShowSim((v) => !v)}
          >
            Test
          </Button>
          <Button
            variant={tree.isActive ? 'secondary' : 'primary'}
            size="sm"
            disabled={toggling}
            onClick={handleToggleActive}
          >
            {tree.isActive ? 'Deactivate' : 'Activate'}
          </Button>
          <Button variant="ghost" size="sm" onClick={handleDeleteTree} className="text-red-500 hover:text-red-700">
            <Trash2 className="h-4 w-4" />
          </Button>
        </div>
      </div>

      {/* Pipeline group banner */}
      <PipelineGroupBanner
        statuses={tree.targetStatuses ?? []}
        onSave={handleSaveTargetStatuses}
        open={pipelineModalOpen}
        onOpenChange={setPipelineModalOpen}
      />

      {/* Simulate panel */}
      {showSim && (
        <Card className="p-4 border-brand-500/30 bg-brand-500/5 space-y-3">
          <p className="text-sm font-medium text-gray-700">Test a customer message</p>
          <div className="flex gap-2">
            <input
              className="flex-1 border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/30"
              placeholder="e.g. How much does a monthly membership cost?"
              value={simMessage}
              onChange={(e) => setSimMessage(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleSimulate()}
            />
            <select
              className="border rounded-lg px-2 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/30 bg-white text-gray-600"
              value={simLeadStatus}
              onChange={(e) => setSimLeadStatus(e.target.value)}
              title="Simulate as lead with this status"
            >
              <option value="">Any status</option>
              {LEAD_STATUSES.map((s) => (
                <option key={s.value} value={s.value}>{s.label}</option>
              ))}
            </select>
            <Button size="sm" onClick={handleSimulate} disabled={simLoading || !simMessage.trim()}>
              Simulate
            </Button>
          </div>
          {simResult && (
            <div className={`rounded-lg border p-3 text-sm space-y-1 ${simResult.matched ? 'bg-green-50 border-green-200' : 'bg-gray-50 border-gray-200'}`}>
              {simResult.matched ? (
                <>
                  <p className="font-medium text-green-700">Matched: {simResult.nodeLabel}</p>
                  {simResult.traversalPath.length > 0 && (
                    <p className="text-gray-500 text-xs">Path: {simResult.traversalPath.join(' → ')}</p>
                  )}
                  <p className="text-gray-700">Action: <span className="font-medium">{ACTION_LABELS[simResult.action!]}</span></p>
                  {simResult.reply && <p className="text-gray-700 italic">&ldquo;{simResult.reply}&rdquo;</p>}
                  {simResult.targetStatus && (
                    <p className="text-orange-700">→ Lead will be moved to: <span className="font-semibold">{LEAD_STATUSES.find(s => s.value === simResult.targetStatus)?.label ?? simResult.targetStatus}</span></p>
                  )}
                </>
              ) : (
                <p className="text-gray-500">No node matched this message.</p>
              )}
            </div>
          )}
        </Card>
      )}

      <div className="flex gap-4 items-start">
        {/* Tree panel */}
        <div className="flex-1 min-w-0">
          <Card className="p-4">
            <div className="flex items-center justify-between mb-4">
              <p className="text-sm font-medium text-gray-600">Tree nodes</p>
              <div ref={headerAddRef} className="relative inline-block">
                <Button
                  size="sm"
                  variant="ghost"
                  leftIcon={<Plus className="h-3.5 w-3.5" />}
                  onClick={() => setHeaderPickerOpen(true)}
                >
                  Add root node
                </Button>
                {headerPickerOpen && headerAddRef.current && (
                  <BlockTypePicker
                    anchorRect={headerAddRef.current.getBoundingClientRect()}
                    onPick={(action) => { setHeaderPickerOpen(false); openAddRoot(action); }}
                    onClose={() => setHeaderPickerOpen(false)}
                  />
                )}
              </div>
            </div>
            {(!tree.nodes || tree.nodes.length === 0) ? (
              <div className="text-center py-10 space-y-3">
                <p className="text-gray-400 text-sm">No nodes yet.</p>
                <div ref={emptyStateAddRef} className="relative inline-block">
                  <Button size="sm" leftIcon={<Plus className="h-3.5 w-3.5" />} onClick={() => setEmptyStatePickerOpen(true)}>
                    Add first node
                  </Button>
                  {emptyStatePickerOpen && emptyStateAddRef.current && (
                    <BlockTypePicker
                      anchorRect={emptyStateAddRef.current.getBoundingClientRect()}
                      onPick={(action) => { setEmptyStatePickerOpen(false); openAddRoot(action); }}
                      onClose={() => setEmptyStatePickerOpen(false)}
                    />
                  )}
                </div>
              </div>
            ) : (
              <TreeCanvas
                nodes={tree.nodes}
                selectedId={selectedNodeId}
                deletingId={deleting}
                onSelect={openEdit}
                onAddChild={openAddChild}
                onDelete={handleDeleteNode}
                onNodePositionChange={handleNodePositionChange}
                onHealPositions={handleHealPositions}
                onOpenTriggerConfig={() => setPipelineModalOpen(true)}
              />
            )}
          </Card>
        </div>

        {/* Right panel */}
        {isPanelOpen && (
          <div className="w-80 shrink-0">
            <Card className="p-4 space-y-4">
              <p className="text-sm font-semibold text-slate-800">{panelTitle}</p>

              {/* Context banner for child nodes */}
              {panelBanner}

              {/* "Add child" button when editing an existing node */}
              {panel.kind === 'edit' && (
                <button
                  onClick={() => openAddChild(panel.node.id, panel.node.label)}
                  className="w-full flex items-center gap-2 rounded-lg border-2 border-dashed border-violet-200 bg-violet-50/50 px-3 py-2 text-xs font-medium text-violet-600 hover:border-violet-400 hover:bg-violet-50 transition-colors"
                >
                  <CornerDownRight className="h-3.5 w-3.5" />
                  Add child node under this node
                </button>
              )}

              <label className="block space-y-1">
                <span className="text-xs font-medium text-gray-500 uppercase tracking-wide">Label</span>
                <input
                  className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/30"
                  placeholder="e.g. Asked about pricing"
                  value={form.label}
                  onChange={(e) => setForm((f) => ({ ...f, label: e.target.value }))}
                />
              </label>

              <label className="block space-y-1">
                <span className="text-xs font-medium text-gray-500 uppercase tracking-wide">Condition type</span>
                <select
                  className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/30 bg-white"
                  value={form.conditionType}
                  onChange={(e) => setForm((f) => ({ ...f, conditionType: e.target.value as ConditionType }))}
                >
                  {(Object.keys(CONDITION_LABELS) as ConditionType[]).map((ct) => (
                    <option key={ct} value={ct}>{CONDITION_LABELS[ct]}</option>
                  ))}
                </select>
              </label>

              {form.conditionType === 'keyword' && (
                <label className="block space-y-1">
                  <div className="flex items-center justify-between">
                    <span className="text-xs font-medium text-gray-500 uppercase tracking-wide">Keywords (comma separated)</span>
                    {suggesting && (
                      <span className="flex items-center gap-1 text-[11px] font-medium text-brand-600">
                        <Loader2 className="h-3 w-3 animate-spin" />
                        <Sparkles className="h-3 w-3" />
                        Suggesting…
                      </span>
                    )}
                  </div>
                  <input
                    className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/30"
                    placeholder="price, cost, fee, membership"
                    value={form.keywords}
                    onChange={(e) => {
                      setKeywordsIsAI(false);
                      lastSuggestKeyRef.current = null;
                      setForm((f) => ({ ...f, keywords: e.target.value }));
                    }}
                  />
                </label>
              )}
              {form.conditionType === 'intent' && (
                <label className="block space-y-1">
                  <span className="text-xs font-medium text-gray-500 uppercase tracking-wide">Intent</span>
                  <select
                    className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/30 bg-white"
                    value={form.keywords}
                    onChange={(e) => setForm((f) => ({ ...f, keywords: e.target.value }))}
                  >
                    <option value="">Select intent…</option>
                    <option value="greeting">Greeting — hi, hello, hey</option>
                    <option value="pricing_question">Pricing question — price, cost, how much</option>
                    <option value="booking_inquiry">Booking inquiry — book, trial, schedule, join</option>
                    <option value="complaint">Complaint — unhappy, terrible, refund, angry</option>
                    <option value="location">Location — where, address, directions</option>
                    <option value="hours">Hours — open, timing, what time</option>
                    <option value="off_topic">Off topic — weather, news, sports</option>
                  </select>
                </label>
              )}
              {form.conditionType === 'sentiment' && (
                <label className="block space-y-1">
                  <span className="text-xs font-medium text-gray-500 uppercase tracking-wide">Sentiment</span>
                  <select
                    className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/30 bg-white"
                    value={form.keywords}
                    onChange={(e) => setForm((f) => ({ ...f, keywords: e.target.value }))}
                  >
                    <option value="">Select…</option>
                    <option value="positive">Positive</option>
                    <option value="neutral">Neutral</option>
                    <option value="negative">Negative</option>
                  </select>
                </label>
              )}
              {form.conditionType === 'lead_status' && (
                <div className="block space-y-1">
                  <span className="text-xs font-medium text-gray-500 uppercase tracking-wide">Match if lead is…</span>
                  <div className="flex flex-col gap-1.5 pt-1">
                    {LEAD_STATUSES.map((s) => (
                      <label key={s.value} className="flex items-center gap-2 cursor-pointer">
                        <input
                          type="checkbox"
                          className="accent-brand-500"
                          checked={form.leadStatuses.includes(s.value)}
                          onChange={(e) => {
                            setForm((f) => ({
                              ...f,
                              leadStatuses: e.target.checked
                                ? [...f.leadStatuses, s.value]
                                : f.leadStatuses.filter((x) => x !== s.value),
                            }));
                          }}
                        />
                        <span className="text-sm text-gray-700">{s.label}</span>
                      </label>
                    ))}
                  </div>
                  <p className="text-xs text-gray-400 pt-1">Node only fires if the lead's current status matches one of these.</p>
                </div>
              )}

              <label className="block space-y-1">
                <span className="text-xs font-medium text-gray-500 uppercase tracking-wide">Action</span>
                <select
                  className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/30 bg-white"
                  value={form.action}
                  onChange={(e) => setForm((f) => ({ ...f, action: e.target.value as NodeAction }))}
                >
                  {(Object.keys(ACTION_LABELS) as NodeAction[]).map((a) => (
                    <option key={a} value={a}>{ACTION_LABELS[a]}</option>
                  ))}
                </select>
              </label>

              {(form.action === 'reply' || form.action === 'change_status') && (
                <label className="block space-y-1">
                  <span className="text-xs font-medium text-gray-500 uppercase tracking-wide">Reply template</span>
                  <textarea
                    className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/30 resize-none"
                    rows={4}
                    placeholder={
                      form.action === 'change_status'
                        ? 'Optional: e.g. "Hi {{lead_name}}, your status has been updated!"'
                        : 'Hi {{lead_name}}, great question! Our plans start at…'
                    }
                    value={form.replyTemplate}
                    onChange={(e) => setForm((f) => ({ ...f, replyTemplate: e.target.value }))}
                  />
                  <p className="text-xs text-gray-400">
                    Use &#123;&#123;lead_name&#125;&#125;, &#123;&#123;lead_first_name&#125;&#125;, &#123;&#123;studio_name&#125;&#125;
                    {form.action === 'change_status' && ' — leave blank for a silent status update'}
                  </p>
                </label>
              )}
              {form.action === 'change_status' && (
                <label className="block space-y-1">
                  <span className="text-xs font-medium text-gray-500 uppercase tracking-wide">Move lead to</span>
                  <select
                    className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/30 bg-white"
                    value={form.targetStatus}
                    onChange={(e) => setForm((f) => ({ ...f, targetStatus: e.target.value }))}
                  >
                    <option value="">Select target status…</option>
                    {LEAD_STATUSES.map((s) => (
                      <option key={s.value} value={s.value}>{s.label}</option>
                    ))}
                  </select>
                  <p className="text-xs text-gray-400">The lead will be moved to this pipeline stage when this node fires.</p>
                </label>
              )}

              <div className="flex gap-2 pt-1">
                <Button
                  size="sm"
                  className="flex-1"
                  disabled={saving || !form.label.trim()}
                  leftIcon={<Save className="h-3.5 w-3.5" />}
                  onClick={handleSave}
                >
                  {saving ? 'Saving…' : panel.kind === 'edit' ? 'Update' : 'Add node'}
                </Button>
                <Button size="sm" variant="ghost" onClick={closePanel}>
                  Cancel
                </Button>
              </div>

              {/* Delete button when editing */}
              {panel.kind === 'edit' && (
                <button
                  disabled={deleting === panel.node.id}
                  onClick={() => handleDeleteNode(panel.node.id)}
                  className="w-full text-xs text-red-400 hover:text-red-600 transition-colors pt-1"
                >
                  Delete this node
                </button>
              )}
            </Card>
          </div>
        )}
      </div>

      {/* Toast notifications */}
      <div className="fixed bottom-5 right-5 z-50 flex flex-col gap-2 pointer-events-none">
        {toasts.map((t) => (
          <div
            key={t.id}
            className={`flex items-center gap-3 rounded-xl px-4 py-3 shadow-lg text-sm font-medium pointer-events-auto transition-all border
              ${t.type === 'success'
                ? 'bg-white text-slate-800 border-slate-200'
                : 'bg-white text-red-600 border-red-200'}`}
          >
            {t.type === 'success'
              ? <CheckCircle2 className="h-4 w-4 shrink-0 text-brand-500" />
              : <X className="h-4 w-4 shrink-0 text-red-500" />}
            {t.message}
            <button
              className="ml-2 opacity-70 hover:opacity-100"
              onClick={() => setToasts((prev) => prev.filter((x) => x.id !== t.id))}
            >
              <X className="h-3.5 w-3.5" />
            </button>
          </div>
        ))}
      </div>

      {/* Confirm dialog */}
      {confirm && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center"
          onClick={(e) => { if (e.target === e.currentTarget) setConfirm(null); }}
        >
          <div className="w-full max-w-sm rounded-2xl bg-white dark:bg-slate-900 shadow-2xl border border-slate-200 dark:border-slate-700 p-6 space-y-4">
            <div className="flex items-start gap-3">
              <div className="grid h-9 w-9 shrink-0 place-items-center rounded-xl bg-red-100 dark:bg-red-900/30">
                <Trash2 className="h-4 w-4 text-red-600 dark:text-red-400" />
              </div>
              <div>
                <p className="text-sm font-semibold text-slate-900 dark:text-slate-100">{confirm.title}</p>
                <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">{confirm.message}</p>
              </div>
            </div>
            <div className="flex justify-end gap-2">
              <Button variant="ghost" size="sm" onClick={() => setConfirm(null)}>Cancel</Button>
              <Button
                size="sm"
                className="bg-red-600 hover:bg-red-700 text-white border-red-600"
                onClick={() => { confirm.onConfirm(); setConfirm(null); }}
              >
                {confirm.confirmLabel}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function PipelineGroupBanner({
  statuses,
  onSave,
  open,
  onOpenChange,
}: {
  statuses: string[];
  onSave: (statuses: string[]) => Promise<void>;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const [draft, setDraft] = useState<string[]>(statuses);
  const [saving, setSaving] = useState(false);
  const setOpen = onOpenChange;

  useEffect(() => {
    if (open) setDraft(statuses);
  }, [open, statuses]);

  async function save() {
    setSaving(true);
    try {
      await onSave(draft);
      setOpen(false);
    } finally {
      setSaving(false);
    }
  }

  const labels = statuses.map(
    (s) => LEAD_STATUSES.find((x) => x.value === s)?.label ?? s,
  );

  return (
    <>
      {/* Banner — always visible */}
      <div className="flex items-center gap-3 rounded-xl border border-slate-200 bg-slate-50 px-4 py-2.5 text-sm">
        <span className="text-slate-500 text-xs font-medium uppercase tracking-wide shrink-0">Responds to</span>
        {statuses.length === 0 ? (
          <span className="text-slate-500">All leads</span>
        ) : (
          <div className="flex flex-wrap gap-1.5">
            {labels.map((l) => (
              <span key={l} className="rounded-full bg-violet-100 text-violet-700 px-2.5 py-0.5 text-xs font-medium">{l}</span>
            ))}
          </div>
        )}
        <button
          onClick={() => { setDraft(statuses); setOpen(true); }}
          className="ml-auto text-xs text-slate-400 hover:text-slate-700 underline underline-offset-2 shrink-0"
        >
          Edit
        </button>
      </div>

      {/* Popup modal */}
      {open && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center"
          onClick={(e) => { if (e.target === e.currentTarget) setOpen(false); }}
        >
          <div className="w-full max-w-sm rounded-2xl bg-white shadow-2xl border border-slate-200 p-6 space-y-5">
            {/* Header */}
            <div className="flex items-start justify-between gap-3">
              <div className="flex items-center gap-3">
                <div className="grid h-9 w-9 place-items-center rounded-xl bg-violet-100">
                  <GitBranch className="h-4.5 w-4.5 text-violet-600" />
                </div>
                <div>
                  <p className="text-sm font-semibold text-slate-800">Pipeline group</p>
                  <p className="text-xs text-slate-500 mt-0.5">Who should this tree respond to?</p>
                </div>
              </div>
              <button onClick={() => setOpen(false)} className="text-slate-400 hover:text-slate-600 p-1">
                <X className="h-4 w-4" />
              </button>
            </div>

            {/* Dropdown */}
            <div className="space-y-1.5">
              <MultiSelect
                options={LEAD_STATUSES}
                value={draft}
                onChange={setDraft}
                placeholder="All leads (no filter)"
              />
              <p className="text-xs text-slate-400">
                {draft.length === 0
                  ? 'No filter — responds to all leads.'
                  : 'Only leads in the selected stages will trigger this tree.'}
              </p>
            </div>

            {/* Actions */}
            <div className="flex justify-end gap-2 pt-1">
              <Button variant="ghost" size="sm" onClick={() => setOpen(false)} disabled={saving}>
                Cancel
              </Button>
              <Button size="sm" onClick={save} disabled={saving}>
                {saving ? 'Saving…' : 'Save'}
              </Button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
