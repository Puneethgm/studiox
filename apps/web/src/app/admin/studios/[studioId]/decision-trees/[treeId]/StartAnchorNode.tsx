'use client';

import { GitBranch, Settings2 } from 'lucide-react';
import { Handle, Position } from '@xyflow/react';

export function StartAnchorNode() {
  return (
    <div className="group flex cursor-pointer items-center gap-1.5 rounded-full border border-dashed border-violet-300 bg-violet-50 px-3 py-1.5 text-xs font-semibold text-violet-700 transition-colors hover:border-violet-400 hover:bg-violet-100 dark:border-violet-500/40 dark:bg-violet-500/10 dark:text-violet-300 dark:hover:bg-violet-500/20">
      <GitBranch className="h-3.5 w-3.5" />
      Start
      <Settings2 className="h-3 w-3 text-violet-400 opacity-0 transition-opacity group-hover:opacity-100" />
      <Handle type="source" position={Position.Bottom} className="!bg-violet-400" style={{ pointerEvents: 'none' }} />
    </div>
  );
}
