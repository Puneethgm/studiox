'use client';

import { useRouter } from 'next/navigation';
import { useState } from 'react';
import { Check, Pencil } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { ApiError, api } from '@/lib/api';

export function ThresholdSetting({ studioId, threshold }: { studioId: string; threshold: number }) {
  const router = useRouter();
  const [editing, setEditing] = useState(false);
  const [value, setValue] = useState(String(threshold));
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function save() {
    const n = Number(value);
    if (!Number.isInteger(n) || n <= 0) {
      setError('Enter a whole number greater than 0.');
      return;
    }
    setError(null);
    setSaving(true);
    try {
      await api(`/api/v1/studios/${studioId}/glofox/attendance/threshold`, {
        method: 'PUT',
        json: { qualifyingThreshold: n },
      });
      setEditing(false);
      router.refresh();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Could not save.');
    } finally {
      setSaving(false);
    }
  }

  if (!editing) {
    return (
      <button
        type="button"
        onClick={() => {
          setValue(String(threshold));
          setEditing(true);
        }}
        className="inline-flex items-center gap-1.5 text-[10px] font-black uppercase tracking-widest text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-300"
      >
        Qualifying at {threshold}+ classes
        <Pencil className="h-3 w-3" />
      </button>
    );
  }

  return (
    <div className="flex items-center gap-2">
      <Input
        type="number"
        min={1}
        value={value}
        onChange={(e) => setValue(e.target.value)}
        className="h-8 w-20 text-xs"
        suppressHydrationWarning
        aria-label="Qualifying classes threshold"
      />
      <Button size="sm" className="h-8" loading={saving} onClick={save} leftIcon={<Check className="h-3.5 w-3.5" />}>
        Save
      </Button>
      {error && <span className="text-xs font-semibold text-red-500">{error}</span>}
    </div>
  );
}
