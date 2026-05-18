'use client';

import { useRouter } from 'next/navigation';
import { useEffect, useState } from 'react';
import { Plug } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { Input } from '@/components/ui/Input';
import { FieldError, FieldHint, Label } from '@/components/ui/Label';
import { ApiError, api } from '@/lib/api';
import type { ChannelKind } from '@/lib/types';

interface Props {
  studioId: string;
  kind: ChannelKind;
}

export function ConnectMetaChannel({ studioId, kind }: Props) {
  const router = useRouter();
  const [mounted, setMounted] = useState(false);
  const [externalId, setExternalId] = useState('');
  const [displayHandle, setDisplayHandle] = useState('');
  const [accessToken, setAccessToken] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  const isIG = kind === 'instagram_meta';
  const label = isIG ? 'Instagram' : 'Messenger';
  const idLabel = isIG ? 'Instagram Business Account ID' : 'Facebook Page ID';
  const idPlaceholder = isIG ? '17841401234567890' : '102938475610293';
  const handleLabel = isIG ? 'Instagram Handle' : 'Page Name';
  const handlePlaceholder = isIG ? '@yourstudio' : 'Your Studio Name';
  const endpoint = isIG ? 'instagram' : 'messenger';

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await api(`/api/v1/studios/${studioId}/messaging/channels/${endpoint}`, {
        method: 'POST',
        json: { externalId, displayHandle, accessToken },
      });
      setExternalId('');
      setDisplayHandle('');
      setAccessToken('');
      router.refresh();
    } catch (err) {
      if (err instanceof ApiError) setError(err.message);
      else setError('Could not connect.');
    } finally {
      setSubmitting(false);
    }
  }

  if (!mounted) return null;

  return (
    <Card title={`Connect ${label}`} subtitle={`Direct via Meta Graph API. Tokens are encrypted at rest.`}>
      <form onSubmit={onSubmit} className="space-y-4">
        <div>
          <Label htmlFor="externalId">{idLabel}</Label>
          <Input
            id="externalId"
            placeholder={idPlaceholder}
            required
            value={externalId}
            onChange={(e) => setExternalId(e.target.value)}
            className="font-mono text-xs"
          />
          <FieldHint>Find this in Meta Business Suite or your Page Settings.</FieldHint>
        </div>
        <div>
          <Label htmlFor="displayHandle">{handleLabel}</Label>
          <Input
            id="displayHandle"
            placeholder={handlePlaceholder}
            required
            value={displayHandle}
            onChange={(e) => setDisplayHandle(e.target.value)}
          />
          <FieldHint>Just for reference in your inbox header.</FieldHint>
        </div>
        <div>
          <Label htmlFor="accessToken">Access token</Label>
          <Input
            id="accessToken"
            type="password"
            required
            placeholder="EAAGxxxxxxxxxxxx"
            value={accessToken}
            onChange={(e) => setAccessToken(e.target.value)}
            className="font-mono text-xs"
          />
          <FieldHint>Requires <code>instagram_manage_messages</code> or <code>pages_messaging</code> scope.</FieldHint>
        </div>
        <FieldError message={error ?? undefined} />
        <Button 
          type="submit" 
          className="w-full h-11" 
          loading={submitting} 
          leftIcon={<Plug className="h-4 w-4" />}
        >
          Connect {label}
        </Button>
      </form>
    </Card>
  );
}
