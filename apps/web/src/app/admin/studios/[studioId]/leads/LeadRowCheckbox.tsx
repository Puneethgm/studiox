'use client';

import { useLeadSelection } from './LeadSelectionContext';

// Unpositioned on purpose — the Leads list (row) and Pipeline board (card)
// need the checkbox in different spots, so each caller wraps this in its
// own positioned element instead of this component picking a fixed one.
export function LeadRowCheckbox({ leadId }: { leadId: string }) {
  const { selected, toggle } = useLeadSelection();
  const checked = selected.has(leadId);

  return (
    <div
      className={`transition-opacity ${checked ? 'opacity-100' : 'opacity-0 group-hover/row:opacity-100 group-hover:opacity-100'}`}
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
