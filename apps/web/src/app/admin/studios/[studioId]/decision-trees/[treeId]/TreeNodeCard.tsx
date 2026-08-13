'use client';

import { useRef, useState } from 'react';
import { Plus, Trash2 } from 'lucide-react';
import { Handle, Position, type NodeProps } from '@xyflow/react';
import { Badge } from '@/components/ui/Badge';
import { cn } from '@/lib/cn';
import { ACTION_COLORS, ACTION_ICONS } from './treeNodeMeta';
import { BlockTypePicker } from './BlockTypePicker';
import type { TreeNodeCardData } from './treeLayout';

function conditionSummary(data: TreeNodeCardData['node']): string {
  const v = data.conditionValue ?? {};
  switch (data.conditionType) {
    case 'keyword':
      return `keyword: ${((v.keywords as string[] | undefined) ?? []).join(', ') || '—'}`;
    case 'intent':
      return `intent: ${(v.intent as string | undefined) || '—'}`;
    case 'sentiment':
      return `sentiment: ${(v.sentiment as string | undefined) || '—'}`;
    case 'lead_status':
      return `status: ${((v.statuses as string[] | undefined) ?? []).join(', ') || '—'}`;
    default:
      return 'default';
  }
}

export function TreeNodeCard({ data }: NodeProps & { data: TreeNodeCardData }) {
  const { node, isSelected, isDeleting, hasChildren, onAddChild, onDelete } = data;
  const Icon = ACTION_ICONS[node.action];
  const [pickerOpen, setPickerOpen] = useState(false);
  const addButtonRef = useRef<HTMLButtonElement>(null);

  return (
    <div
      className={cn(
        'relative w-[220px] rounded-lg border bg-white px-3 py-2.5 shadow-sm transition-all dark:bg-zinc-900',
        isSelected ? 'border-violet-300 ring-2 ring-violet-100 dark:ring-violet-500/20' : 'border-zinc-200 dark:border-zinc-800',
      )}
    >
      <Handle type="target" position={Position.Top} className="!bg-violet-400" style={{ pointerEvents: 'none' }} />

      <div className="flex items-start justify-between gap-2">
        <div className="flex min-w-0 items-center gap-2">
          <span className={cn('flex h-6 w-6 shrink-0 items-center justify-center rounded border', ACTION_COLORS[node.action])}>
            <Icon className="h-3.5 w-3.5" />
          </span>
          <span className="truncate text-sm font-semibold text-zinc-800 dark:text-zinc-100">{node.label}</span>
        </div>
        <button
          type="button"
          title="Delete node"
          disabled={isDeleting}
          onClick={(e) => {
            e.stopPropagation();
            onDelete(node.id);
          }}
          className="nodrag shrink-0 text-zinc-300 hover:text-red-500 disabled:opacity-50 dark:text-zinc-600"
        >
          <Trash2 className="h-3.5 w-3.5" />
        </button>
      </div>

      <div className="mt-2">
        <Badge tone="neutral" className="max-w-full truncate normal-case">{conditionSummary(node)}</Badge>
      </div>

      <button
        ref={addButtonRef}
        type="button"
        title="Add child node"
        onClick={(e) => {
          e.stopPropagation();
          setPickerOpen(true);
        }}
        className="nodrag absolute -bottom-3 left-1/2 flex h-6 w-6 -translate-x-1/2 items-center justify-center rounded-full border border-violet-200 bg-white text-violet-600 shadow-sm hover:bg-violet-50 dark:border-violet-500/30 dark:bg-zinc-900 dark:text-violet-300"
      >
        <Plus className="h-3.5 w-3.5" />
      </button>
      {pickerOpen && addButtonRef.current && (
        <BlockTypePicker
          anchorRect={addButtonRef.current.getBoundingClientRect()}
          onPick={(action) => {
            setPickerOpen(false);
            onAddChild(node.id, node.label, action);
          }}
          onClose={() => setPickerOpen(false)}
        />
      )}

      <Handle type="source" position={Position.Bottom} className="!bg-violet-400" style={{ pointerEvents: 'none' }} />

      {!hasChildren && (
        <div className="pointer-events-none absolute -bottom-14 left-1/2 flex -translate-x-1/2 flex-col items-center">
          <div className="h-8 w-px bg-zinc-200 dark:bg-zinc-700" />
          <span className="rounded-full border border-dashed border-zinc-300 bg-zinc-50 px-2.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-zinc-400 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-500">
            End
          </span>
        </div>
      )}
    </div>
  );
}
