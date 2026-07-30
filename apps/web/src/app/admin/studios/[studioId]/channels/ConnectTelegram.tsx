'use client';

import { useRouter } from 'next/navigation';
import { useEffect, useState } from 'react';
import { Plug } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { Input } from '@/components/ui/Input';
import { FieldError, FieldHint, Label } from '@/components/ui/Label';
import { ApiError, api } from '@/lib/api';

export function ConnectTelegram({ studioId, showToast }: { studioId: string; showToast: (msg: string) => void }) {
  const router = useRouter();
  const [mounted, setMounted] = useState(false);
  const [botToken, setBotToken] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  const getErrorMessage = (msg: string): string => {
    if (msg.includes('already connected')) return '🔴 This bot is already connected to another studio. Disconnect it there first.';
    if (msg.includes('invalid bot token')) return '🔴 Invalid bot token. Double-check what @BotFather gave you.';
    return `🔴 ${msg}`;
  };

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await api(`/api/v1/studios/${studioId}/messaging/channels/telegram`, {
        method: 'POST',
        json: { botToken },
      });
      setBotToken('');
      showToast('Telegram bot connected successfully.');
      router.refresh();
    } catch (err) {
      if (err instanceof ApiError) setError(getErrorMessage(err.message));
      else setError(getErrorMessage('Could not connect.'));
    } finally {
      setSubmitting(false);
    }
  }

  if (!mounted) {
    return (
      <Card id="connect-telegram" title="Connect Telegram" subtitle="Connect a Telegram bot to send and receive DMs.">
        <div className="space-y-3 py-2">
          <div className="h-10 rounded-lg border border-dashed border-slate-200 bg-slate-50 dark:border-slate-800 dark:bg-slate-900/40" />
          <div className="h-10 rounded-lg bg-slate-100 dark:bg-slate-800" />
        </div>
      </Card>
    );
  }

  return (
    <Card id="connect-telegram" title="Connect Telegram" subtitle="Connect a Telegram bot to send and receive DMs. Tokens are encrypted at rest.">
      <form onSubmit={onSubmit} className="space-y-4">
        <div>
          <Label htmlFor="botToken">Bot Token</Label>
          <Input
            id="botToken"
            type="password"
            placeholder="123456789:AAExampleTokenFromBotFather"
            required
            value={botToken}
            onChange={(e) => setBotToken(e.target.value)}
            className="font-mono text-xs"
            suppressHydrationWarning
            aria-describedby="botToken-hint"
          />
          <FieldHint id="botToken-hint">
            Message @BotFather on Telegram, run /newbot, and paste the token it gives you. Encrypted at rest — never written to logs.
          </FieldHint>
        </div>
        <FieldError message={error ?? undefined} />
        <Button
          type="submit"
          className="w-full h-11"
          loading={submitting}
          leftIcon={<Plug className="h-4 w-4" />}
          aria-label="Connect Telegram"
        >
          Connect Telegram
        </Button>
      </form>
    </Card>
  );
}
