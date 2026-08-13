'use client';

import { createPortal } from 'react-dom';
import type { NodeAction } from '@/lib/types';
import { ACTION_LABELS, ACTION_ICONS, ACTION_COLORS } from './treeNodeMeta';

const ACTIONS: NodeAction[] = ['reply', 'escalate_human', 'book_trial', 'send_link', 'change_status'];

interface Props {
  /** Screen-space anchor (e.g. from the trigger button's getBoundingClientRect())
   * to position the popover below — portaled to document.body so it always
   * renders above every canvas node regardless of React Flow's per-node
   * stacking contexts. */
  anchorRect: { left: number; bottom: number; width: number };
  onPick: (action: NodeAction) => void;
  onClose: () => void;
}

export function BlockTypePicker({ anchorRect, onPick, onClose }: Props) {
  const left = anchorRect.left + anchorRect.width / 2;

  return createPortal(
    <>
      <div className="fixed inset-0 z-[100]" onClick={(e) => { e.stopPropagation(); onClose(); }} />
      <div
        className="fixed z-[101] w-56 -translate-x-1/2 rounded-lg border border-zinc-200 bg-white p-1.5 shadow-lg dark:border-zinc-800 dark:bg-zinc-900"
        style={{ left, top: anchorRect.bottom + 8 }}
      >
        <p className="px-2 py-1 text-[10px] font-black uppercase tracking-wider text-zinc-400">Choose a block</p>
        {ACTIONS.map((action) => {
          const Icon = ACTION_ICONS[action];
          return (
            <button
              key={action}
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                onPick(action);
              }}
              className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-xs font-semibold text-zinc-700 hover:bg-zinc-50 dark:text-zinc-200 dark:hover:bg-zinc-800"
            >
              <span className={`flex h-6 w-6 shrink-0 items-center justify-center rounded border ${ACTION_COLORS[action]}`}>
                <Icon className="h-3.5 w-3.5" />
              </span>
              {ACTION_LABELS[action]}
            </button>
          );
        })}
      </div>
    </>,
    document.body,
  );
}
