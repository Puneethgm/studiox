'use client';

import { Clock, Plus, Trash2 } from 'lucide-react';
import { Handle, Position, type NodeProps } from '@xyflow/react';
import { Badge } from '@/components/ui/Badge';
import { cn } from '@/lib/cn';
import type { FollowupStepCardData } from './followupLayout';

function delaySummary(step: FollowupStepCardData['step']): string {
  const unit = step.delayValue === 1 ? step.delayUnit.replace(/s$/, '') : step.delayUnit;
  return `wait ${step.delayValue} ${unit}`;
}

export function FollowupStepCard({ data }: NodeProps & { data: FollowupStepCardData }) {
  const { step, index, isSelected, isLast, onSelect, onAddNext, onDelete } = data;

  return (
    <div
      className={cn(
        'relative w-[220px] cursor-pointer rounded-lg border bg-white px-3 py-2.5 shadow-sm transition-all dark:bg-zinc-900',
        isSelected ? 'border-violet-300 ring-2 ring-violet-100 dark:ring-violet-500/20' : 'border-zinc-200 dark:border-zinc-800',
      )}
      onClick={() => onSelect(step.key)}
    >
      <Handle type="target" position={Position.Top} className="!bg-violet-400" style={{ pointerEvents: 'none' }} />

      <div className="flex items-start justify-between gap-2">
        <div className="flex min-w-0 items-center gap-2">
          <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded border bg-blue-50 text-blue-700 border-blue-200">
            <Clock className="h-3.5 w-3.5" />
          </span>
          <span className="truncate text-sm font-semibold text-zinc-800 dark:text-zinc-100">
            Step {index + 1}
          </span>
        </div>
        <button
          type="button"
          title="Delete step"
          onClick={(e) => {
            e.stopPropagation();
            onDelete(step.key);
          }}
          className="nodrag shrink-0 text-zinc-300 hover:text-red-500 dark:text-zinc-600"
        >
          <Trash2 className="h-3.5 w-3.5" />
        </button>
      </div>

      <div className="mt-2">
        <Badge tone="neutral" className="max-w-full truncate normal-case">{delaySummary(step)}</Badge>
      </div>
      <p className="mt-1.5 truncate text-[11px] text-zinc-400" title={step.messageTemplate}>
        {step.messageTemplate || 'No message yet — click to edit'}
      </p>

      {isLast && (
        <button
          type="button"
          title="Add next step"
          onClick={(e) => {
            e.stopPropagation();
            onAddNext();
          }}
          className="nodrag absolute -bottom-3 left-1/2 flex h-6 w-6 -translate-x-1/2 items-center justify-center rounded-full border border-violet-200 bg-white text-violet-600 shadow-sm hover:bg-violet-50 dark:border-violet-500/30 dark:bg-zinc-900 dark:text-violet-300"
        >
          <Plus className="h-3.5 w-3.5" />
        </button>
      )}

      <Handle type="source" position={Position.Bottom} className="!bg-violet-400" style={{ pointerEvents: 'none' }} />

      {isLast && (
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
