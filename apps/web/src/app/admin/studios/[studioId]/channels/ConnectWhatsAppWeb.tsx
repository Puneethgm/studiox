'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { Smartphone, CheckCircle2, XCircle, RefreshCw, Wifi } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { api } from '@/lib/api';

type QRState =
  | { status: 'idle' }
  | { status: 'loading' }
  | { status: 'qr'; qr: string }
  | { status: 'connected'; phone: string }
  | { status: 'error'; message: string };

export function ConnectWhatsAppWeb({
  studioId,
  showToast,
}: {
  studioId: string;
  showToast: (msg: string) => void;
}) {
  const [state, setState] = useState<QRState>({ status: 'idle' });
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const prewarmQR = useRef<string | null>(null);

  const stopPolling = useCallback(() => {
    if (pollRef.current) {
      clearInterval(pollRef.current);
      pollRef.current = null;
    }
  }, []);

  const fetchQR = useCallback(async () => {
    try {
      const data = await api<{ status: string; qr?: string; phone?: string }>(
        `/api/v1/studios/${studioId}/messaging/channels/whatsapp-web/qr`
      );
      if (data.status === 'connected' && data.phone) {
        setState({ status: 'connected', phone: data.phone });
        stopPolling();
        showToast('WhatsApp connected successfully!');
      } else if (data.status === 'qr' && data.qr) {
        setState({ status: 'qr', qr: data.qr });
      } else {
        setState({ status: 'loading' });
      }
    } catch {
      setState({ status: 'error', message: 'Could not reach WhatsApp Web service. Is it running?' });
      stopPolling();
    }
  }, [studioId, showToast, stopPolling]);

  const startSession = useCallback(() => {
    if (prewarmQR.current) {
      setState({ status: 'qr', qr: prewarmQR.current });
      prewarmQR.current = null;
      pollRef.current = setInterval(fetchQR, 3000);
      return;
    }
    setState({ status: 'loading' });
    fetchQR();
    // Poll every 3 seconds until connected or errored
    pollRef.current = setInterval(fetchQR, 3000);
  }, [fetchQR]);

  const disconnect = useCallback(async () => {
    stopPolling();
    setState({ status: 'loading' });
    try {
      await api(`/api/v1/studios/${studioId}/messaging/channels/whatsapp-web/disconnect`, {
        method: 'POST',
      });
      setState({ status: 'idle' });
      showToast('WhatsApp disconnected.');
    } catch {
      setState({ status: 'error', message: 'Disconnect failed.' });
    }
  }, [studioId, showToast, stopPolling]);

  // Check existing status on mount
  useEffect(() => {
    api<{ status: string; phone?: string }>(
      `/api/v1/studios/${studioId}/messaging/channels/whatsapp-web/status`
    )
      .then((data) => {
        if (data.status === 'connected' && data.phone) {
          setState({ status: 'connected', phone: data.phone });
        }
      })
      .catch(() => {});
  }, [studioId]);

  // Silently pre-warm Chrome and cache the QR in the background so clicking
  // "Show QR Code" is instant (<1s) instead of waiting for Chrome to start.
  useEffect(() => {
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout>;

    const preFetch = async () => {
      try {
        const data = await api<{ status: string; qr?: string; phone?: string }>(
          `/api/v1/studios/${studioId}/messaging/channels/whatsapp-web/qr`
        );
        if (cancelled) return;
        if (data.status === 'connected' && data.phone) {
          setState({ status: 'connected', phone: data.phone });
        } else if (data.status === 'qr' && data.qr) {
          prewarmQR.current = data.qr;
          // Re-poll to keep the cached QR fresh (QR expires ~60s)
          timer = setTimeout(preFetch, 20000);
        } else {
          // Still warming up — retry shortly
          timer = setTimeout(preFetch, 3000);
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

  // Cleanup on unmount
  useEffect(() => () => stopPolling(), [stopPolling]);

  return (
    <Card
      id="connect-whatsapp-web"
      title="Connect WhatsApp (QR Code)"
      subtitle="Link any WhatsApp number by scanning a QR code — no Meta API setup needed."
    >
      <div className="space-y-5">
        {/* Instructions */}
        {state.status !== 'connected' && (
          <ol className="text-sm text-slate-600 dark:text-slate-400 space-y-1 list-decimal list-inside">
            <li>Click <strong>Show QR Code</strong> below</li>
            <li>Open WhatsApp on your phone → tap <strong>⋮ → Linked Devices → Link a Device</strong></li>
            <li>Scan the QR code — you're connected instantly</li>
          </ol>
        )}

        {/* Loading */}
        {state.status === 'loading' && (
          <div className="flex flex-col items-center gap-3 py-6">
            <RefreshCw className="h-8 w-8 animate-spin text-green-500" />
            <p className="text-sm text-slate-500">Starting WhatsApp Web…</p>
            <p className="text-xs text-slate-400 text-center max-w-xs">
              First launch takes 20–30 seconds while Chrome starts up. Please wait.
            </p>
          </div>
        )}

        {/* QR Code */}
        {state.status === 'qr' && (
          <div className="flex flex-col items-center gap-4">
            <div className="rounded-xl border-2 border-green-400 p-3 bg-white shadow-md">
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img
                src={state.qr}
                alt="WhatsApp QR Code"
                className="h-56 w-56"
              />
            </div>
            <p className="text-xs text-slate-500 text-center max-w-xs">
              QR code refreshes automatically. Scan within 60 seconds.
            </p>
            <Button
              variant="ghost"
              size="sm"
              leftIcon={<RefreshCw className="h-4 w-4" />}
              onClick={fetchQR}
            >
              Refresh QR
            </Button>
          </div>
        )}

        {/* Connected */}
        {state.status === 'connected' && (
          <div className="flex flex-col items-center gap-4 py-4">
            <div className="flex items-center gap-2 rounded-full bg-green-50 border border-green-200 px-4 py-2 dark:bg-green-900/20 dark:border-green-800">
              <CheckCircle2 className="h-5 w-5 text-green-600" />
              <span className="font-medium text-green-700 dark:text-green-400">
                Connected — +{state.phone}
              </span>
            </div>
            <p className="text-xs text-slate-500 text-center">
              WhatsApp is linked. Messages will appear in your inbox automatically.
            </p>
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

        {/* Error */}
        {state.status === 'error' && (
          <div className="flex items-start gap-2 rounded-lg bg-red-50 border border-red-200 p-3 dark:bg-red-900/20 dark:border-red-800">
            <XCircle className="h-4 w-4 text-red-500 mt-0.5 shrink-0" />
            <p className="text-sm text-red-700 dark:text-red-400">{state.message}</p>
          </div>
        )}

        {/* Action button */}
        {(state.status === 'idle' || state.status === 'error') && (
          <Button
            className="w-full h-11"
            leftIcon={<Smartphone className="h-4 w-4" />}
            onClick={startSession}
          >
            Show QR Code
          </Button>
        )}

        {state.status === 'qr' && (
          <Button
            variant="ghost"
            className="w-full"
            onClick={disconnect}
          >
            Cancel
          </Button>
        )}
      </div>
    </Card>
  );
}
