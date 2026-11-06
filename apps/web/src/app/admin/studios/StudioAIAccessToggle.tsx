'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { Loader2, Bot } from 'lucide-react';
import { api } from '@/lib/api';
import { cn } from '@/lib/cn';

interface StudioAIAccessToggleProps {
  studioId: string;
  initialEnabled: boolean;
  studioName: string;
}

// Superadmin-only master switch for whether this studio gets AI replies at
// all (ai_access_enabled) — separate from StudioStatusToggle (active/
// inactive) and from a conversation's own per-conversation ai_enabled.
// Hits its own dedicated endpoint (not the generic studio update), which is
// gated to superadmin only on the backend.
export function StudioAIAccessToggle({
  studioId,
  initialEnabled,
  studioName,
}: StudioAIAccessToggleProps) {
  const [enabled, setEnabled] = useState(initialEnabled);
  const [isLoading, setIsLoading] = useState(false);
  const router = useRouter();

  const handleToggle = async (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (isLoading) return;

    setIsLoading(true);
    const next = !enabled;

    try {
      await api(`/api/v1/admin/studios/${studioId}/ai-access`, {
        method: 'PATCH',
        json: { enabled: next },
      });
      setEnabled(next);

      sessionStorage.setItem(
        'studiox_toast',
        JSON.stringify({
          message: `AI replies ${next ? 'enabled' : 'disabled'} for ${studioName}.`,
          type: 'success',
        })
      );
      window.dispatchEvent(new Event('studiox_toast_update'));
      router.refresh();
    } catch (error: any) {
      console.error('Failed to toggle studio AI access:', error);
      sessionStorage.setItem(
        'studiox_toast',
        JSON.stringify({
          message: error?.message || 'Failed to update AI access.',
          type: 'error',
        })
      );
      window.dispatchEvent(new Event('studiox_toast_update'));
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <button
      onClick={handleToggle}
      disabled={isLoading}
      className={cn(
        'relative inline-flex h-6 w-11 shrink-0 cursor-pointer items-center rounded-full transition-colors duration-300 focus:outline-none focus:ring-2 focus:ring-brand-500 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50',
        enabled ? 'bg-violet-500' : 'bg-zinc-300 dark:bg-zinc-700'
      )}
      role="switch"
      aria-checked={enabled}
      title={enabled ? 'Disable AI replies for this studio' : 'Enable AI replies for this studio'}
    >
      <span
        className={cn(
          'pointer-events-none flex h-5 w-5 items-center justify-center rounded-full bg-white shadow-md ring-0 transition-transform duration-300 ease-out',
          enabled ? 'translate-x-5' : 'translate-x-0.5'
        )}
      >
        {isLoading ? (
          <Loader2 className="h-3 w-3 animate-spin text-zinc-500" />
        ) : (
          <Bot className={cn('h-2.5 w-2.5', enabled ? 'text-violet-600' : 'text-zinc-400')} />
        )}
      </span>
    </button>
  );
}
