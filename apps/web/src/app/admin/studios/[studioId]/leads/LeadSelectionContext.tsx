'use client';

import { createContext, useContext, useState, useCallback, type ReactNode } from 'react';

// Lightweight checkbox-selection state shared across the leads table and
// its bulk "Enable AI for Selected" action bar — the table itself stays a
// server component (data/pagination/filters all come from the URL), so
// this only needs to track which row IDs are checked, not own any data.
interface LeadSelectionState {
  selected: Set<string>;
  toggle: (id: string) => void;
  clear: () => void;
}

const LeadSelectionContext = createContext<LeadSelectionState | null>(null);

export function LeadSelectionProvider({ children }: { children: ReactNode }) {
  const [selected, setSelected] = useState<Set<string>>(new Set());

  const toggle = useCallback((id: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);

  const clear = useCallback(() => setSelected(new Set()), []);

  return (
    <LeadSelectionContext.Provider value={{ selected, toggle, clear }}>
      {children}
    </LeadSelectionContext.Provider>
  );
}

export function useLeadSelection(): LeadSelectionState {
  const ctx = useContext(LeadSelectionContext);
  if (!ctx) throw new Error('useLeadSelection must be used within LeadSelectionProvider');
  return ctx;
}
