'use client';

import { useLeadSelection } from './LeadSelectionContext';

export function LeadRowCheckbox({ leadId }: { leadId: string }) {
  const { selected, toggle } = useLeadSelection();
  const checked = selected.has(leadId);

  return (
    <div
      className={`absolute left-2 top-1/2 z-10 -translate-y-1/2 transition-opacity ${
        checked ? 'opacity-100' : 'opacity-0 group-hover/row:opacity-100'
      }`}
      onClick={(e) => { e.preventDefault(); e.stopPropagation(); }}
    >
      <input
        type="checkbox"
        checked={checked}
        onChange={(e) => { e.stopPropagation(); toggle(leadId); }}
        className="h-4 w-4 rounded border-zinc-300 text-brand-500 focus:ring-brand-500 dark:border-zinc-700"
      />
    </div>
  );
}
