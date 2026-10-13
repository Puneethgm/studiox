'use client';

import { useLeadSelection } from './LeadSelectionContext';

// Selects every lead currently shown on the page — which already respects
// whatever status/search/campaign filters LeadFilters has applied, so
// "select all" naturally means "all leads at this status" when a status
// filter is active, without needing separate status-based selection logic.
export function SelectAllCheckbox({ leadIds }: { leadIds: string[] }) {
  const { selected, toggle } = useLeadSelection();
  const allChecked = leadIds.length > 0 && leadIds.every((id) => selected.has(id));
  const someChecked = leadIds.some((id) => selected.has(id));

  function toggleAll() {
    if (allChecked) {
      leadIds.forEach((id) => { if (selected.has(id)) toggle(id); });
    } else {
      leadIds.forEach((id) => { if (!selected.has(id)) toggle(id); });
    }
  }

  return (
    <input
      type="checkbox"
      checked={allChecked}
      ref={(el) => { if (el) el.indeterminate = !allChecked && someChecked; }}
      onChange={toggleAll}
      title="Select all on this page"
      className="h-4 w-4 rounded border-zinc-300 text-brand-500 focus:ring-brand-500 dark:border-zinc-700"
    />
  );
}
