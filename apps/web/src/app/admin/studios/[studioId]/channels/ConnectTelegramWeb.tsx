'use client';

import { useRouter } from 'next/navigation';
import { useCallback, useEffect, useRef, useState } from 'react';
import { Send, CheckCircle2, XCircle, RefreshCw, History, Lock } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { Input } from '@/components/ui/Input';
import { Label } from '@/components/ui/Label';
import { api } from '@/lib/api';

type QRState =
  | { status: 'idle' }
  | { status: 'loading' }
  | { status: 'qr'; qr: string }
  | { status: 'password_required'; passwordHint: string | null }
  | { status: 'connected'; phone: string; username: string | null }
  | { status: 'error'; message: string };

type BackfillState = 'none' | 'running' | 'done' | 'failed';

export function ConnectTelegramWeb({
  studioId,
  connected,
  showToast,
}: {
  studioId: string;
  connected: boolean;
  showToast: (msg: string, type?: 'success' | 'error') => void;
}) {
  const router = useRouter();
  const [state, setState] = useState<QRState>({ status: 'idle' });
  const [password, setPassword] = useState('');
  const [submittingPassword, setSubmittingPassword] = useState(false);
  const [backfill, setBackfill] = useState<BackfillState>('none');
  const [backfillCount, setBackfillCount] = useState(0);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const backfillPollRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const prewarmQR = useRef<string | null>(null);

  const stopPolling = useCallback(() => {
    if (pollRef.current) {
      clearInterval(pollRef.current);
      pollRef.current = null;
    }
  }, []);

  const fetchQR = useCallback(async () => {
    try {
      const data = await api<{ status: string; qr?: string; phone?: string; username?: string; passwordHint?: string | null; error?: string }>(
        `/api/v1/studios/${studioId}/messaging/channels/telegram-web/qr`
      );
      if (data.status === 'connected' && data.phone) {
        setState({ status: 'connected', phone: data.phone, username: data.username || null });
        stopPolling();
        showToast('Telegram connected successfully!');
        router.refresh();
      } else if (data.status === 'qr' && data.qr) {
        setState({ status: 'qr', qr: data.qr });
      } else if (data.status === 'password_required') {
        setState({ status: 'password_required', passwordHint: data.passwordHint ?? null });
        stopPolling(); // wait for the admin to submit a password, not more QR polling
      } else if (data.status === 'error') {
        setState({ status: 'error', message: data.error || 'Could not start a Telegram session.' });
        stopPolling();
      } else {
        setState({ status: 'loading' });
      }
    } catch {
      setState({ status: 'error', message: 'Could not reach the Telegram service. Is it running?' });
      stopPolling();
    }
  }, [studioId, showToast, stopPolling, router]);

  const startSession = useCallback(() => {
    if (prewarmQR.current) {
      setState({ status: 'qr', qr: prewarmQR.current });
      prewarmQR.current = null;
      pollRef.current = setInterval(fetchQR, 3000);
      return;
    }
    setState({ status: 'loading' });
    fetchQR();
    pollRef.current = setInterval(fetchQR, 3000);
  }, [fetchQR]);

  const submitPassword = useCallback(async () => {
    if (!password) return;
    setSubmittingPassword(true);
    try {
      await api(`/api/v1/studios/${studioId}/messaging/channels/telegram-web/password`, {
        method: 'POST',
        json: { password },
      });
      setPassword('');
      setState({ status: 'loading' });
      pollRef.current = setInterval(fetchQR, 3000);
    } catch {
      showToast('Incorrect password. Try again.', 'error');
    } finally {
      setSubmittingPassword(false);
    }
  }, [studioId, password, fetchQR, showToast]);

  const disconnect = useCallback(async () => {
    stopPolling();
    setState({ status: 'loading' });
    try {
      await api(`/api/v1/studios/${studioId}/messaging/channels/telegram-web/disconnect`, {
        method: 'POST',
      });
      setState({ status: 'idle' });
      showToast('Telegram disconnected.');
      router.refresh();
    } catch {
      setState({ status: 'error', message: 'Disconnect failed.' });
    }
  }, [studioId, showToast, stopPolling, router]);

  // Check existing status on mount
  useEffect(() => {
    api<{ status: string; phone?: string; username?: string }>(
      `/api/v1/studios/${studioId}/messaging/channels/telegram-web/status`
    )
      .then((data) => {
        if (data.status === 'connected' && data.phone) {
          setState({ status: 'connected', phone: data.phone, username: data.username || null });
        }
      })
      .catch(() => {});
  }, [studioId]);

  const fetchBackfillStatus = useCallback(async () => {
    try {
      const data = await api<{ status: BackfillState; messageCount: number }>(
        `/api/v1/studios/${studioId}/messaging/channels/telegram-web/backfill`
      );
      setBackfill(data.status);
      setBackfillCount(data.messageCount || 0);
      if (data.status !== 'running' && backfillPollRef.current) {
        clearInterval(backfillPollRef.current);
        backfillPollRef.current = null;
        if (data.status === 'done') {
          showToast(
            data.messageCount > 0
              ? `Imported ${data.messageCount} historical messages.`
              : 'No chat history was available to import.'
          );
        }
        if (data.status === 'failed') showToast('History import failed.', 'error');
      }
    } catch {
      // ignore — status just stays as-is until next poll
    }
  }, [studioId, showToast]);

  useEffect(() => {
    fetchBackfillStatus().then(() => {});
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [studioId]);

  useEffect(() => {
    if (backfill === 'running' && !backfillPollRef.current) {
      backfillPollRef.current = setInterval(fetchBackfillStatus, 4000);
    }
    return () => {
      if (backfillPollRef.current) {
        clearInterval(backfillPollRef.current);
        backfillPollRef.current = null;
      }
    };
  }, [backfill, fetchBackfillStatus]);

  const startBackfill = useCallback(async () => {
    setBackfill('running');
    setBackfillCount(0);
    try {
      await api(`/api/v1/studios/${studioId}/messaging/channels/telegram-web/backfill`, {
        method: 'POST',
      });
      backfillPollRef.current = setInterval(fetchBackfillStatus, 4000);
    } catch {
      setBackfill('failed');
      showToast('Could not start history import.', 'error');
    }
  }, [studioId, fetchBackfillStatus, showToast]);

  // Silently pre-warm (resumes a persisted session only — see tg-web's
  // SessionManager.prewarm, which never mints a fresh QR unprompted).
  useEffect(() => {
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout>;

    const preFetch = async () => {
      try {
        const data = await api<{ status: string; qr?: string; phone?: string; username?: string }>(
          `/api/v1/studios/${studioId}/messaging/channels/telegram-web/status`
        );
        if (cancelled) return;
        if (data.status === 'connected' && data.phone) {
          setState({ status: 'connected', phone: data.phone, username: data.username || null });
        } else {
          timer = setTimeout(preFetch, 5000);
        }
      } catch {
        // Silently ignore — user can still click and trigger manually
      }
    };

    preFetch();
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [studioId]);

  // The server-provided `connected` prop is the source of truth for whether a
  // channel row still exists (e.g. disconnected from the "Connected channels"
  // list). Drop stale local "connected" state if that row is gone.
  useEffect(() => {
    if (!connected && state.status === 'connected') {
      setState({ status: 'idle' });
    }
  }, [connected, state.status]);

  useEffect(() => () => stopPolling(), [stopPolling]);
  useEffect(() => () => {
    if (backfillPollRef.current) clearInterval(backfillPollRef.current);
  }, []);

  return (
    <Card
      id="connect-telegram-web"
      title="Connect Telegram (QR Code)"
      subtitle="Link your existing personal or business Telegram account by scanning a QR code — sees your chat history, no bot needed."
    >
      <div className="space-y-5">
        {state.status !== 'connected' && (
          <ol className="text-sm text-slate-600 dark:text-slate-400 space-y-1 list-decimal list-inside">
            <li>Click <strong>Show QR Code</strong> below</li>
            <li>Open Telegram → <strong>Settings → Devices → Link Desktop Device</strong></li>
            <li>Scan the QR code — you&rsquo;re connected instantly</li>
          </ol>
        )}

        {state.status === 'loading' && (
          <div className="flex flex-col items-center gap-3 py-6">
            <RefreshCw className="h-8 w-8 animate-spin text-[#26A5E4]" />
            <p className="text-sm text-slate-500">Starting Telegram session…</p>
          </div>
        )}

        {state.status === 'qr' && (
          <div className="flex flex-col items-center gap-4">
            <div className="rounded-xl border-2 border-[#26A5E4] p-3 bg-white shadow-md">
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img src={state.qr} alt="Telegram QR Code" className="h-56 w-56" />
            </div>
            <p className="text-xs text-slate-500 text-center max-w-xs">
              QR code refreshes automatically. Scan within a minute.
            </p>
            <Button variant="ghost" size="sm" leftIcon={<RefreshCw className="h-4 w-4" />} onClick={fetchQR}>
              Refresh QR
            </Button>
          </div>
        )}

        {state.status === 'password_required' && (
          <div className="flex flex-col items-center gap-3 py-2">
            <Lock className="h-8 w-8 text-[#26A5E4]" />
            <p className="text-sm text-slate-600 dark:text-slate-400 text-center">
              This account has two-factor authentication enabled.
              {state.passwordHint ? ` Hint: ${state.passwordHint}` : ''}
            </p>
            <div className="w-full max-w-xs">
              <Label htmlFor="tg-2fa-password">Cloud Password</Label>
              <Input
                id="tg-2fa-password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && submitPassword()}
              />
            </div>
            <Button className="w-full max-w-xs" loading={submittingPassword} onClick={submitPassword}>
              Submit
            </Button>
          </div>
        )}

        {state.status === 'connected' && (
          <div className="flex flex-col items-center gap-4 py-4">
            <div className="flex items-center gap-2 rounded-full bg-sky-50 border border-sky-200 px-4 py-2 dark:bg-sky-900/20 dark:border-sky-800">
              <CheckCircle2 className="h-5 w-5 text-[#26A5E4]" />
              <span className="font-medium text-sky-700 dark:text-sky-400">
                Connected — {state.phone}{state.username ? ` (@${state.username})` : ''}
              </span>
            </div>
            <p className="text-xs text-slate-500 text-center">
              Telegram is linked. Messages will appear in your inbox automatically.
            </p>

            {backfill === 'none' && (
              <div className="flex flex-col items-center gap-1.5">
                <Button variant="ghost" size="sm" leftIcon={<History className="h-4 w-4" />} onClick={startBackfill}>
                  Import chat history
                </Button>
                <p className="text-[11px] text-slate-400 text-center max-w-xs">
                  Imports your most recent direct-message chats (group chats are skipped).
                </p>
              </div>
            )}
            {backfill === 'running' && (
              <div className="flex items-center gap-2 text-sm text-slate-500">
                <RefreshCw className="h-4 w-4 animate-spin" />
                Importing chat history… this can take a few minutes.
              </div>
            )}
            {backfill === 'done' && (
              <p className="text-xs text-sky-600 dark:text-sky-400">
                Imported {backfillCount} historical messages.
              </p>
            )}
            {backfill === 'failed' && (
              <div className="flex items-center gap-2">
                <p className="text-xs text-red-500">History import failed.</p>
                <Button variant="ghost" size="sm" onClick={startBackfill}>
                  Retry
                </Button>
              </div>
            )}

            <Button
              variant="ghost"
              size="sm"
              leftIcon={<XCircle className="h-4 w-4" />}
              onClick={disconnect}
              className="text-red-500 hover:text-red-600"
            >
              Disconnect
            </Button>
          </div>
        )}

        {state.status === 'error' && (
          <div className="flex items-start gap-2 rounded-lg bg-red-50 border border-red-200 p-3 dark:bg-red-900/20 dark:border-red-800">
            <XCircle className="h-4 w-4 text-red-500 mt-0.5 shrink-0" />
            <p className="text-sm text-red-700 dark:text-red-400">{state.message}</p>
          </div>
        )}

        {(state.status === 'idle' || state.status === 'error') && (
          <Button className="w-full h-11" leftIcon={<Send className="h-4 w-4" />} onClick={startSession}>
            Show QR Code
          </Button>
        )}

        {state.status === 'qr' && (
          <Button variant="ghost" className="w-full" onClick={disconnect}>
            Cancel
          </Button>
        )}
      </div>
    </Card>
  );
}
