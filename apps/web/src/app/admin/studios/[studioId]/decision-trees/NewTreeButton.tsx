'use client';

import { useState, useRef, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { Plus, X, GitBranch } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { MultiSelect } from '@/components/ui/MultiSelect';
import { api } from '@/lib/api';
import type { DecisionTree } from '@/lib/types';

const LEAD_STATUSES = [
  { value: 'new', label: 'New' },
  { value: 'contacted', label: 'Contacted' },
  { value: 'trial_booked', label: 'Trial booked' },
  { value: 'member', label: 'Member' },
  { value: 'dropped', label: 'Dropped' },
  { value: 'paused', label: 'Paused' },
];

export function NewTreeButton({ studioId }: { studioId: string }) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState('');
  const [targetStatuses, setTargetStatuses] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (open) {
      setName('');
      setTargetStatuses([]);
      setError('');
      setTimeout(() => inputRef.current?.focus(), 50);
    }
  }, [open]);

  async function handleCreate() {
    if (!name.trim()) {
      setError('Please enter a name.');
      return;
    }
    setLoading(true);
    setError('');
    try {
      const tree = await api<DecisionTree>(
        `/api/v1/studios/${studioId}/decision-trees`,
        { method: 'POST', json: { name: name.trim(), targetStatuses } },
      );
      setOpen(false);
      router.push(`/admin/studios/${studioId}/decision-trees/${tree.id}`);
    } catch {
      setError('Failed to create. Please try again.');
    } finally {
      setLoading(false);
    }
  }

  function handleKey(e: React.KeyboardEvent) {
    if (e.key === 'Enter') handleCreate();
    if (e.key === 'Escape') setOpen(false);
  }

  return (
    <>
      <Button
        leftIcon={<Plus className="h-4 w-4" />}
        onClick={() => setOpen(true)}
      >
        New Tree
      </Button>

      {open && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center"
          onClick={(e) => { if (e.target === e.currentTarget) setOpen(false); }}
        >
          <div className="w-full max-w-md rounded-2xl bg-white dark:bg-slate-900 shadow-2xl border border-slate-200 dark:border-slate-700 p-6 space-y-5">

            {/* Header */}
            <div className="flex items-start justify-between gap-3">
              <div className="flex items-center gap-3">
                <div className="grid h-10 w-10 place-items-center rounded-xl bg-violet-100 dark:bg-violet-900/30">
                  <GitBranch className="h-5 w-5 text-violet-600 dark:text-violet-400" />
                </div>
                <div>
                  <h2 className="text-base font-semibold text-slate-900 dark:text-slate-100">New Decision Tree</h2>
                  <p className="text-xs text-slate-500 dark:text-slate-400">Set which pipeline group this tree responds to.</p>
                </div>
              </div>
              <button
                onClick={() => setOpen(false)}
                className="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 transition-colors p-1"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            {/* Name */}
            <div className="space-y-1.5">
              <label className="text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wide">
                Tree name
              </label>
              <Input
                ref={inputRef}
                placeholder="e.g. Member Support, Re-engage Dropped Leads…"
                value={name}
                onChange={(e) => { setName(e.target.value); setError(''); }}
                onKeyDown={handleKey}
                invalid={!!error}
              />
              {error && <p className="text-xs text-red-500">{error}</p>}
            </div>

            {/* Pipeline group */}
            <div className="space-y-1.5">
              <label className="text-xs font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wide">
                Pipeline group
              </label>
              <MultiSelect
                options={LEAD_STATUSES}
                value={targetStatuses}
                onChange={setTargetStatuses}
                placeholder="All leads (no filter)"
              />
              <p className="text-xs text-slate-400">
                {targetStatuses.length === 0
                  ? 'This tree will respond to all leads.'
                  : 'Only leads in these stages will trigger this tree.'}
              </p>
            </div>

            {/* Actions */}
            <div className="flex justify-end gap-2 pt-1">
              <Button variant="ghost" onClick={() => setOpen(false)} disabled={loading}>
                Cancel
              </Button>
              <Button onClick={handleCreate} disabled={loading || !name.trim()}>
                {loading ? 'Creating…' : 'Create Tree'}
              </Button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
