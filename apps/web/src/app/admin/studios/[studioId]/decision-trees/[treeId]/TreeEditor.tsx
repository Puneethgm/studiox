'use client';

import { useState, useCallback, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import {
  GitBranch, Plus, Trash2, ChevronDown, ChevronRight,
  CheckCircle2, Circle, Play, ArrowLeft, Save, CornerDownRight, X,
} from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { Badge } from '@/components/ui/Badge';
import { MultiSelect } from '@/components/ui/MultiSelect';
import { api } from '@/lib/api';
import type { DecisionTree, TreeNode, ConditionType, NodeAction, SimulateResult } from '@/lib/types';
import { ImportNodesButton } from './ImportNodesButton';

const CONDITION_LABELS: Record<ConditionType, string> = {
  keyword: 'Keyword match',
  intent: 'AI intent',
  sentiment: 'Sentiment',
  default: 'Default (catch-all)',
  lead_status: 'Lead status',
};

const ACTION_LABELS: Record<NodeAction, string> = {
  reply: 'Send reply',
  escalate_human: 'Escalate to human',
  book_trial: 'Book trial',
  send_link: 'Send link',
  change_status: 'Change lead status',
};

const ACTION_COLORS: Record<NodeAction, string> = {
  reply: 'bg-blue-50 text-blue-700 border-blue-200',
  escalate_human: 'bg-amber-50 text-amber-700 border-amber-200',
  book_trial: 'bg-green-50 text-green-700 border-green-200',
  send_link: 'bg-purple-50 text-purple-700 border-purple-200',
  change_status: 'bg-orange-50 text-orange-700 border-orange-200',
};

const LEAD_STATUSES = [
  { value: 'new', label: 'New' },
  { value: 'contacted', label: 'Contacted' },
  { value: 'trial_booked', label: 'Trial booked' },
  { value: 'member', label: 'Member' },
  { value: 'dropped', label: 'Dropped' },
  { value: 'paused', label: 'Paused' },
];

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

  const refreshTree = useCallback(async () => {
    const updated = await api<DecisionTree>(
      `/api/v1/studios/${studioId}/decision-trees/${tree.id}`,
    );
    setTree(updated);
  }, [studioId, tree.id]);

  function openAddRoot() {
    setPanel({ kind: 'addRoot' });
    setForm({ ...defaultForm(), sortOrder: tree.nodes?.length ?? 0 });
  }

  function openAddChild(parentId: string, parentLabel: string) {
    setPanel({ kind: 'addChild', parentId, parentLabel });
    const childCount = countChildren(tree.nodes ?? [], parentId);
    setForm({ ...defaultForm(), sortOrder: Math.max(0, childCount) });
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
        await api(`/api/v1/studios/${studioId}/decision-trees/${tree.id}/nodes`, {
          method: 'POST',
          json: { ...nodePayload, parentId },
        });
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
              <Button
                size="sm"
                variant="ghost"
                leftIcon={<Plus className="h-3.5 w-3.5" />}
                onClick={openAddRoot}
              >
                Add root node
              </Button>
            </div>
            {(!tree.nodes || tree.nodes.length === 0) ? (
              <div className="text-center py-10 space-y-3">
                <p className="text-gray-400 text-sm">No nodes yet.</p>
                <Button size="sm" leftIcon={<Plus className="h-3.5 w-3.5" />} onClick={openAddRoot}>
                  Add first node
                </Button>
              </div>
            ) : (
              <NodeList
                nodes={tree.nodes}
                depth={0}
                selectedId={selectedNodeId}
                deletingId={deleting}
                onSelect={openEdit}
                onAddChild={openAddChild}
                onDelete={handleDeleteNode}
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
                  <span className="text-xs font-medium text-gray-500 uppercase tracking-wide">Keywords (comma separated)</span>
                  <input
                    className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/30"
                    placeholder="price, cost, fee, membership"
                    value={form.keywords}
                    onChange={(e) => setForm((f) => ({ ...f, keywords: e.target.value }))}
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

function NodeList({
  nodes, depth, selectedId, deletingId, onSelect, onAddChild, onDelete,
}: {
  nodes: TreeNode[];
  depth: number;
  selectedId?: string;
  deletingId: string | null;
  onSelect: (node: TreeNode) => void;
  onAddChild: (parentId: string, parentLabel: string) => void;
  onDelete: (nodeId: string) => void;
}) {
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());

  function toggle(id: string) {
    setCollapsed((prev) => {
      const next = new Set(prev);
      next.has(id) ? next.delete(id) : next.add(id);
      return next;
    });
  }

  return (
    <div className={depth > 0 ? 'ml-5 border-l-2 border-violet-100 pl-3 space-y-1 mt-1' : 'space-y-1'}>
      {nodes.map((node) => {
        const hasChildren = node.children && node.children.length > 0;
        const isCollapsed = collapsed.has(node.id);
        const isSelected = selectedId === node.id;

        return (
          <div key={node.id}>
            <div
              className={`flex items-center gap-2 rounded-lg px-2 py-2 cursor-pointer transition-colors ${
                isSelected
                  ? 'bg-violet-50 border border-violet-300'
                  : 'hover:bg-gray-50 border border-transparent'
              }`}
              onClick={() => onSelect(node)}
            >
              {/* Collapse toggle */}
              <button
                className="text-gray-300 hover:text-gray-500 shrink-0 w-4"
                onClick={(e) => { e.stopPropagation(); toggle(node.id); }}
              >
                {hasChildren
                  ? (isCollapsed
                      ? <ChevronRight className="h-3.5 w-3.5" />
                      : <ChevronDown className="h-3.5 w-3.5" />)
                  : <span className="h-3.5 w-3.5 block" />
                }
              </button>

              {/* Condition pill */}
              <span className="text-xs px-1.5 py-0.5 rounded bg-gray-100 text-gray-500 shrink-0 capitalize">
                {node.conditionType}
              </span>

              {/* Label */}
              <span className="text-sm flex-1 min-w-0 truncate font-medium text-slate-700">
                {node.label}
              </span>

              {/* Action badge */}
              <span className={`text-xs px-1.5 py-0.5 rounded border shrink-0 ${ACTION_COLORS[node.action]}`}>
                {ACTION_LABELS[node.action]}
              </span>

              {/* Always-visible action buttons */}
              <div className="flex items-center gap-1 shrink-0">
                <button
                  title="Add child node"
                  className="p-1 rounded text-gray-400 hover:text-violet-600 hover:bg-violet-50 transition-colors"
                  onClick={(e) => { e.stopPropagation(); onAddChild(node.id, node.label); }}
                >
                  <Plus className="h-3.5 w-3.5" />
                </button>
                <button
                  title="Delete node"
                  className="p-1 rounded text-gray-400 hover:text-red-500 hover:bg-red-50 transition-colors"
                  disabled={deletingId === node.id}
                  onClick={(e) => { e.stopPropagation(); onDelete(node.id); }}
                >
                  <Trash2 className="h-3.5 w-3.5" />
                </button>
              </div>
            </div>

            {hasChildren && !isCollapsed && (
              <NodeList
                nodes={node.children!}
                depth={depth + 1}
                selectedId={selectedId}
                deletingId={deletingId}
                onSelect={onSelect}
                onAddChild={onAddChild}
                onDelete={onDelete}
              />
            )}
          </div>
        );
      })}
    </div>
  );
}

function PipelineGroupBanner({
  statuses,
  onSave,
}: {
  statuses: string[];
  onSave: (statuses: string[]) => Promise<void>;
}) {
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState<string[]>(statuses);
  const [saving, setSaving] = useState(false);

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
