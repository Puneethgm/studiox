'use client';

import { useEffect, useState, useCallback } from 'react';
import { Fingerprint, Laptop, LogOut, MapPin, ShieldCheck, Smartphone } from 'lucide-react';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { Dialog, DialogHeader } from '@/components/ui/Dialog';
import { api } from '@/lib/api';

interface SessionRes {
  id: string;
  device: string;
  browser: string;
  ip: string;
  country: string;
  twoFaPassed: boolean;
  createdAt: string;
  lastSeenAt: string;
  isCurrent: boolean;
}

function formatWhen(iso: string): string {
  try {
    return new Date(iso).toLocaleString(undefined, {
      dateStyle: 'medium',
      timeStyle: 'short',
    });
  } catch {
    return iso;
  }
}

// Full precision (seconds + timezone name) for the detail view — the list
// row's formatWhen() above is deliberately coarser for scanability.
function formatWhenFull(iso: string): string {
  try {
    return new Date(iso).toLocaleString(undefined, {
      dateStyle: 'full',
      timeStyle: 'long',
    });
  } catch {
    return iso;
  }
}

function DetailRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-start justify-between gap-4 py-2.5">
      <span className="text-xs font-bold uppercase tracking-wider text-zinc-400">{label}</span>
      <span className="text-right text-sm font-medium text-zinc-900 dark:text-zinc-100">{value}</span>
    </div>
  );
}

export default function SessionsPage() {
  const [sessions, setSessions] = useState<SessionRes[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [endingId, setEndingId] = useState<string | null>(null);
  const [endingOthers, setEndingOthers] = useState(false);
  const [detailId, setDetailId] = useState<string | null>(null);

  const load = useCallback(() => {
    setError(null);
    api<SessionRes[]>('/api/v1/auth/sessions')
      .then(setSessions)
      .catch(() => setError('Could not load your sessions.'));
  }, []);

  useEffect(() => {
    load();
    // Poll instead of requiring a manual refresh — sessions only change on
    // a login/logout elsewhere, so a few-second interval is plenty; this
    // intentionally doesn't reset `sessions` to null between polls (see
    // `load` above), so a background refresh never flashes "Loading…"
    // over the list that's already showing. Paused while the tab isn't
    // visible so it doesn't poll a backgrounded browser tab forever.
    const interval = setInterval(() => {
      if (document.visibilityState === 'visible') load();
    }, 8000);

    function onVisible() {
      if (document.visibilityState === 'visible') load();
    }
    document.addEventListener('visibilitychange', onVisible);

    return () => {
      clearInterval(interval);
      document.removeEventListener('visibilitychange', onVisible);
    };
  }, [load]);

  async function endSession(id: string) {
    setEndingId(id);
    try {
      await api(`/api/v1/auth/sessions/${id}`, { method: 'DELETE' });
      load();
    } catch {
      setError('Could not end that session. Try again.');
    } finally {
      setEndingId(null);
    }
  }

  async function endOthers() {
    setEndingOthers(true);
    try {
      await api('/api/v1/auth/sessions/end-others', { method: 'POST' });
      load();
    } catch {
      setError('Could not end other sessions. Try again.');
    } finally {
      setEndingOthers(false);
    }
  }

  const otherCount = sessions?.filter((s) => !s.isCurrent).length ?? 0;

  return (
    <div className="mx-auto max-w-2xl space-y-6 py-10">
      <div>
        <h1 className="text-2xl font-black tracking-tight text-zinc-900 dark:text-white">
          Active sessions
        </h1>
        <p className="mt-1 text-sm font-medium text-zinc-500 dark:text-zinc-400">
          Everywhere you're currently signed in. End a session to sign it out immediately.
        </p>
      </div>

      {error && (
        <p className="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-500/10 dark:text-red-400">
          {error}
        </p>
      )}

      {sessions === null ? (
        <p className="text-sm text-zinc-500 dark:text-zinc-400">Loading…</p>
      ) : (
        <>
          {otherCount > 0 && (
            <Button
              variant="secondary"
              onClick={endOthers}
              loading={endingOthers}
              leftIcon={<LogOut className="h-4 w-4" />}
            >
              End {otherCount} other session{otherCount === 1 ? '' : 's'}
            </Button>
          )}

          <div className="space-y-3">
            {sessions.map((s) => (
              <Card key={s.id} noPadding>
                <button
                  type="button"
                  onClick={() => setDetailId(s.id)}
                  className="flex w-full items-start justify-between gap-4 p-5 text-left transition-colors hover:bg-zinc-50 dark:hover:bg-zinc-900/40"
                >
                  <div className="flex items-start gap-3 min-w-0">
                    <div className="mt-0.5 grid h-10 w-10 shrink-0 place-items-center rounded-xl bg-zinc-100 text-zinc-500 dark:bg-zinc-800 dark:text-zinc-400">
                      {/mobile|android|iphone/i.test(s.device) ? (
                        <Smartphone className="h-5 w-5" />
                      ) : (
                        <Laptop className="h-5 w-5" />
                      )}
                    </div>
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-bold text-zinc-900 dark:text-zinc-100">
                          {s.device || 'Unknown device'} &middot; {s.browser || 'Unknown browser'}
                        </span>
                        {s.isCurrent && <Badge tone="brand">This device</Badge>}
                        {s.twoFaPassed && (
                          <Badge tone="success">
                            <ShieldCheck className="mr-1 h-3 w-3" /> 2FA
                          </Badge>
                        )}
                      </div>
                      <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-zinc-500 dark:text-zinc-400">
                        <span className="inline-flex items-center gap-1">
                          <MapPin className="h-3 w-3" /> {s.ip}
                          {s.country ? ` (${s.country})` : ''}
                        </span>
                        <span>Last active {formatWhen(s.lastSeenAt)}</span>
                        <span>Signed in {formatWhen(s.createdAt)}</span>
                      </div>
                    </div>
                  </div>
                  {!s.isCurrent && (
                    <span
                      role="button"
                      tabIndex={0}
                      onClick={(e) => {
                        e.stopPropagation();
                        endSession(s.id);
                      }}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter' || e.key === ' ') {
                          e.stopPropagation();
                          endSession(s.id);
                        }
                      }}
                      className="shrink-0 rounded-lg px-3 py-1.5 text-xs font-bold text-red-600 transition-colors hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-500/10"
                    >
                      {endingId === s.id ? 'Ending…' : 'End'}
                    </span>
                  )}
                </button>
              </Card>
            ))}
          </div>
        </>
      )}

      {detailId && (() => {
        const s = sessions?.find((x) => x.id === detailId);
        if (!s) return null;
        return (
          <Dialog open onClose={() => setDetailId(null)}>
            <DialogHeader
              icon={<Fingerprint className="h-4 w-4 text-zinc-400" />}
              title="Session details"
              onClose={() => setDetailId(null)}
            />
            <div className="divide-y divide-zinc-100 px-5 dark:divide-zinc-900">
              <DetailRow label="Session ID" value={<span className="font-mono text-xs">{s.id}</span>} />
              <DetailRow
                label="Status"
                value={
                  <span className="inline-flex items-center gap-1.5">
                    {s.isCurrent && <Badge tone="brand">This device</Badge>}
                    {s.twoFaPassed ? (
                      <Badge tone="success">
                        <ShieldCheck className="mr-1 h-3 w-3" /> 2FA verified
                      </Badge>
                    ) : (
                      <Badge tone="warning">No fresh 2FA</Badge>
                    )}
                  </span>
                }
              />
              <DetailRow label="Device" value={s.device || 'Unknown device'} />
              <DetailRow label="Browser" value={s.browser || 'Unknown browser'} />
              <DetailRow
                label="IP address"
                value={s.country ? `${s.ip} (${s.country})` : s.ip}
              />
              <DetailRow label="Signed in" value={formatWhenFull(s.createdAt)} />
              <DetailRow label="Last active" value={formatWhenFull(s.lastSeenAt)} />
            </div>
            <div className="flex shrink-0 justify-end gap-2 border-t border-zinc-200 p-4 dark:border-zinc-800">
              {!s.isCurrent && (
                <Button
                  variant="danger"
                  size="sm"
                  onClick={() => {
                    endSession(s.id);
                    setDetailId(null);
                  }}
                  leftIcon={<LogOut className="h-4 w-4" />}
                >
                  End this session
                </Button>
              )}
              <Button variant="secondary" size="sm" onClick={() => setDetailId(null)}>
                Close
              </Button>
            </div>
          </Dialog>
        );
      })()}
    </div>
  );
}
