'use client';

import { useState, useRef, useEffect } from 'react';
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

export function MultiSelect({ options, value, onChange, placeholder = 'Select…', className = '' }: Props) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, []);

  function toggle(v: string) {
    onChange(value.includes(v) ? value.filter((x) => x !== v) : [...value, v]);
  }

  function remove(v: string, e: React.MouseEvent) {
    e.stopPropagation();
    onChange(value.filter((x) => x !== v));
  }

  const selected = options.filter((o) => value.includes(o.value));

  return (
    <div ref={ref} className={`relative ${className}`}>
      {/* Trigger */}
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="w-full min-h-[38px] flex items-center gap-1.5 flex-wrap border rounded-lg px-3 py-1.5 text-sm bg-white text-left focus:outline-none focus:ring-2 focus:ring-brand-500/30 transition-colors hover:border-slate-300"
      >
        {selected.length === 0 ? (
          <span className="text-slate-400 py-0.5">{placeholder}</span>
        ) : (
          selected.map((o) => (
            <span
              key={o.value}
              className="flex items-center gap-1 rounded-full bg-violet-100 text-violet-700 px-2 py-0.5 text-xs font-medium"
            >
              {o.label}
              <span
                role="button"
                tabIndex={0}
                onClick={(e) => remove(o.value, e)}
                onKeyDown={(e) => e.key === 'Enter' && remove(o.value, e as unknown as React.MouseEvent)}
                className="hover:text-violet-900 cursor-pointer"
              >
                <X className="h-3 w-3" />
              </span>
            </span>
          ))
        )}
        <ChevronDown className={`h-4 w-4 text-slate-400 ml-auto shrink-0 transition-transform ${open ? 'rotate-180' : ''}`} />
      </button>

      {/* Dropdown */}
      {open && (
        <div className="absolute z-50 mt-1 w-full rounded-lg border border-slate-200 bg-white shadow-lg py-1">
          {options.map((o) => {
            const checked = value.includes(o.value);
            return (
              <button
                key={o.value}
                type="button"
                onClick={() => toggle(o.value)}
                className="w-full flex items-center gap-2.5 px-3 py-2 text-sm text-left hover:bg-slate-50 transition-colors"
              >
                <span className={`h-4 w-4 rounded border-2 flex-shrink-0 flex items-center justify-center transition-colors
                  ${checked ? 'border-violet-500 bg-violet-500' : 'border-slate-300'}`}>
                  {checked && <Check className="h-2.5 w-2.5 text-white" strokeWidth={3} />}
                </span>
                <span className={checked ? 'text-slate-800 font-medium' : 'text-slate-600'}>
                  {o.label}
                </span>
              </button>
            );
          })}
          {selected.length > 0 && (
            <>
              <div className="border-t border-slate-100 mt-1 pt-1">
                <button
                  type="button"
                  onClick={() => onChange([])}
                  className="w-full px-3 py-1.5 text-xs text-slate-400 hover:text-slate-600 text-left transition-colors"
                >
                  Clear all
                </button>
              </div>
            </>
          )}
        </div>
      )}
    </div>
  );
}
