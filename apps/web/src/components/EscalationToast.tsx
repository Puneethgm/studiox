'use client';

import { useEffect, useRef, useState } from 'react';
import { useRouter } from 'next/navigation';
import { AlertTriangle, X } from 'lucide-react';

interface EscalationEvent {
  kind: 'conversation.escalated';
  studioId: string;
  conversationId: string;
  reason?: string;
}

const AUTO_DISMISS_MS = 3500;

// Live push notification for a conversation just landing in the Inbox's
// Escalation tab — mounted once in AppShell so it fires regardless of which
// admin page the user is currently on, not just while the Inbox is open.
// Reuses the same SSE stream InboxLive.tsx already opens per-studio
// (internal/messaging/http.go's `stream` handler), just filtered to the
// conversation.escalated event kind (see internal/messaging/events.go).
export function EscalationToast({ studioId }: { studioId: string | undefined }) {
  const router = useRouter();
  const [toast, setToast] = useState<EscalationEvent | null>(null);
  const dismissTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    if (!studioId) return;
    const es = new EventSource(`/api/v1/studios/${studioId}/messaging/stream`, { withCredentials: true });

    es.addEventListener('conversation.escalated', (e: MessageEvent) => {
      try {
        const evt: EscalationEvent = JSON.parse(e.data);
        if (evt.studioId !== studioId) return;
        setToast(evt);
        if (dismissTimer.current) clearTimeout(dismissTimer.current);
        dismissTimer.current = setTimeout(() => setToast(null), AUTO_DISMISS_MS);
      } catch {
        // Ignore malformed.
      }
    });

    return () => {
      es.close();
      if (dismissTimer.current) clearTimeout(dismissTimer.current);
    };
  }, [studioId]);

  if (!toast || !studioId) return null;

  return (
    <button
      type="button"
      onClick={() => {
        setToast(null);
        router.push(`/admin/studios/${studioId}/inbox?tab=escalation`);
      }}
      className="fixed right-6 top-20 z-[9999] flex min-w-[320px] max-w-sm items-start gap-3 rounded-2xl border border-amber-500/30 bg-white/95 p-4 text-left shadow-2xl backdrop-blur-xl animate-in slide-in-from-top-5 fade-in duration-300 dark:bg-zinc-900/95"
    >
      <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-xl bg-amber-500/10 text-amber-500">
        <AlertTriangle className="h-5 w-5" />
      </div>
      <div className="flex-1">
        <p className="text-xs font-black uppercase tracking-wider text-zinc-800 dark:text-zinc-100">Conversation escalated</p>
        <p className="mt-0.5 text-[10px] font-semibold text-zinc-500 dark:text-zinc-400">
          {toast.reason || 'Needs a human — tap to open the Escalation tab.'}
        </p>
      </div>
      <span
        role="button"
        aria-label="Dismiss"
        onClick={(e) => {
          e.stopPropagation();
          setToast(null);
        }}
        className="rounded-lg p-1 text-zinc-400 transition-colors hover:bg-zinc-100 hover:text-zinc-600 dark:hover:bg-zinc-800 dark:hover:text-white"
      >
        <X className="h-4 w-4" />
      </span>
    </button>
  );
}
