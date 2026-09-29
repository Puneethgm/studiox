'use client';

import { useState, useRef, useEffect, useCallback } from 'react';
import { createPortal } from 'react-dom';
import { ChevronDown, X, Check } from 'lucide-react';

export interface MultiSelectOption {
  value: string;
  label: string;
}

interface Props {
  options: MultiSelectOption[];
  value: string[];
  onChange: (value: string[]) => void;
  placeholder?: string;
  className?: string;
}

// The options panel is rendered through a portal into document.body with
// `position: fixed`, tracking the trigger's on-screen position — not
// nested under the trigger in the DOM. A plain absolutely-positioned
// dropdown gets clipped by the nearest scrollable/overflow-hidden ancestor
// (e.g. a Dialog's scrollable body), which cut it off when this was used
// inside a modal. Capped at a fixed height with its own scroll so a long
// option list (e.g. the permission catalog) doesn't grow the panel
// unboundedly — it scrolls internally instead.
export function MultiSelect({ options, value, onChange, placeholder = 'Select…', className = '' }: Props) {
  const [open, setOpen] = useState(false);
  const [rect, setRect] = useState<{ top: number; left: number; width: number } | null>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);

  const updateRect = useCallback(() => {
    const el = triggerRef.current;
    if (!el) return;
    const r = el.getBoundingClientRect();
    setRect({ top: r.bottom + 4, left: r.left, width: r.width });
  }, []);

  useEffect(() => {
    if (!open) return;
    updateRect();

    function handleClick(e: MouseEvent) {
      const target = e.target as Node;
      if (triggerRef.current?.contains(target) || panelRef.current?.contains(target)) return;
      setOpen(false);
    }
    function handleReposition() {
      updateRect();
    }

    document.addEventListener('mousedown', handleClick);
    window.addEventListener('resize', handleReposition);
    window.addEventListener('scroll', handleReposition, true);
    return () => {
      document.removeEventListener('mousedown', handleClick);
      window.removeEventListener('resize', handleReposition);
      window.removeEventListener('scroll', handleReposition, true);
    };
  }, [open, updateRect]);

  function toggle(v: string) {
    onChange(value.includes(v) ? value.filter((x) => x !== v) : [...value, v]);
  }

  function remove(v: string, e: React.MouseEvent) {
    e.stopPropagation();
    onChange(value.filter((x) => x !== v));
  }

  const selected = options.filter((o) => value.includes(o.value));

  return (
    <div className={`relative ${className}`}>
      <button
        ref={triggerRef}
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="w-full min-h-[38px] flex items-center gap-1.5 flex-wrap border rounded-lg px-3 py-1.5 text-sm bg-white text-left focus:outline-none focus:ring-2 focus:ring-brand-500/30 transition-colors hover:border-slate-300 dark:bg-zinc-900 dark:border-zinc-800"
      >
        {selected.length === 0 ? (
          <span className="text-slate-400 py-0.5">{placeholder}</span>
        ) : (
          selected.map((o) => (
            <span
              key={o.value}
              className="flex items-center gap-1 rounded-full bg-violet-100 text-violet-700 px-2 py-0.5 text-xs font-medium dark:bg-violet-500/20 dark:text-violet-300"
            >
              {o.label}
              <span
                role="button"
                tabIndex={0}
                onClick={(e) => remove(o.value, e)}
                onKeyDown={(e) => e.key === 'Enter' && remove(o.value, e as unknown as React.MouseEvent)}
                className="hover:text-violet-900 cursor-pointer dark:hover:text-violet-100"
              >
                <X className="h-3 w-3" />
              </span>
            </span>
          ))
        )}
        <ChevronDown className={`h-4 w-4 text-slate-400 ml-auto shrink-0 transition-transform ${open ? 'rotate-180' : ''}`} />
      </button>

      {open && rect && typeof document !== 'undefined' &&
        createPortal(
          <div
            ref={panelRef}
            style={{ position: 'fixed', top: rect.top, left: rect.left, width: rect.width }}
            className="z-[110] max-h-52 overflow-y-auto rounded-lg border border-slate-200 bg-white py-1 shadow-lg dark:border-zinc-800 dark:bg-zinc-900"
          >
            {options.map((o) => {
              const checked = value.includes(o.value);
              return (
                <button
                  key={o.value}
                  type="button"
                  onClick={() => toggle(o.value)}
                  className="w-full flex items-center gap-2.5 px-3 py-2 text-sm text-left hover:bg-slate-50 dark:hover:bg-zinc-800 transition-colors"
                >
                  <span
                    className={`h-4 w-4 rounded border-2 flex-shrink-0 flex items-center justify-center transition-colors
                    ${checked ? 'border-violet-500 bg-violet-500' : 'border-slate-300 dark:border-zinc-700'}`}
                  >
                    {checked && <Check className="h-2.5 w-2.5 text-white" strokeWidth={3} />}
                  </span>
                  <span className={checked ? 'text-slate-800 font-medium dark:text-zinc-100' : 'text-slate-600 dark:text-zinc-400'}>
                    {o.label}
                  </span>
                </button>
              );
            })}
            {selected.length > 0 && (
              <div className="border-t border-slate-100 mt-1 pt-1 dark:border-zinc-800">
                <button
                  type="button"
                  onClick={() => onChange([])}
                  className="w-full px-3 py-1.5 text-xs text-slate-400 hover:text-slate-600 dark:hover:text-zinc-300 text-left transition-colors"
                >
                  Clear all
                </button>
              </div>
            )}
          </div>,
          document.body,
        )}
    </div>
  );
}
