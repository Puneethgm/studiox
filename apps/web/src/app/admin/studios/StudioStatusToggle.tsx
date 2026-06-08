'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { Loader2, Power } from 'lucide-react';
import { api } from '@/lib/api';
import { cn } from '@/lib/cn';

interface StudioStatusToggleProps {
  studioId: string;
  initialActive: boolean;
  studioName: string;
}

export function StudioStatusToggle({
  studioId,
  initialActive,
  studioName,
}: StudioStatusToggleProps) {
  const [active, setActive] = useState(initialActive);
  const [isLoading, setIsLoading] = useState(false);
  const router = useRouter();

  const handleToggle = async (e: React.MouseEvent) => {
    // Prevent navigating to the studio page
    e.preventDefault();
    e.stopPropagation();

    if (isLoading) return;

    setIsLoading(true);
    const nextActive = !active;

    try {
      await api(`/api/v1/admin/studios/${studioId}`, {
        method: 'PATCH',
        json: { active: nextActive },
      });
      setActive(nextActive);

      // Trigger a custom event for the global toast (AppShell listens to this)
      sessionStorage.setItem(
        'studiox_toast',
        JSON.stringify({
          message: `${studioName} has been ${nextActive ? 'activated' : 'deactivated'} successfully.`,
          type: 'success',
        })
      );
      window.dispatchEvent(new Event('studiox_toast_update'));

      // Refresh the server component data
      router.refresh();
    } catch (error: any) {
      console.error('Failed to toggle studio status:', error);
      sessionStorage.setItem(
        'studiox_toast',
        JSON.stringify({
          message: error?.message || 'Failed to update studio status.',
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
        "relative inline-flex h-6 w-11 shrink-0 cursor-pointer items-center rounded-full transition-colors duration-300 focus:outline-none focus:ring-2 focus:ring-brand-500 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50",
        active ? "bg-emerald-500" : "bg-zinc-300 dark:bg-zinc-700"
      )}
      role="switch"
      aria-checked={active}
      title={active ? "Deactivate Studio" : "Activate Studio"}
    >
      <span
        className={cn(
          "pointer-events-none flex h-5 w-5 items-center justify-center rounded-full bg-white shadow-md ring-0 transition-transform duration-300 ease-out",
          active ? "translate-x-5" : "translate-x-0.5"
        )}
      >
        {isLoading ? (
          <Loader2 className="h-3 w-3 animate-spin text-zinc-500" />
        ) : (
          <Power className={cn("h-2.5 w-2.5", active ? "text-emerald-600" : "text-zinc-400")} />
        )}
      </span>
    </button>
  );
}
