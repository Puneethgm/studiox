'use client';

import { useEffect, useState } from 'react';
import { Eye, EyeOff, Loader2, Plug } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { FieldError, FieldHint, Label } from '@/components/ui/Label';
import { Badge } from '@/components/ui/Badge';
import { MistralMark } from '@/components/ai-provider-icons';
import { getMistralKeyStatus, testMistralKey, type MistralKeyStatus } from './ai-actions';

// Groq/Gemini/Claude (the chat-reply providers) moved to a centralized,
// superadmin-only page — see admin/settings/AIProviderPlatformSettings.tsx.
// Mistral is the one AI provider still configured per-studio, since it's
// only used for Knowledge Base image OCR, not the chat-reply waterfall.
export function AIProviderSettings({ studioId }: { studioId: string }) {
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [status, setStatus] = useState<MistralKeyStatus | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const result = await getMistralKeyStatus(studioId);
      if (cancelled) return;
      if (!result.ok) {
        setLoadError(result.error);
        setLoading(false);
        return;
      }
      setStatus(result.status);
      setLoading(false);
    })();
    return () => {
      cancelled = true;
    };
  }, [studioId]);

  const [apiKey, setApiKey] = useState('');
  const [showApiKey, setShowApiKey] = useState(false);
  const [testingKey, setTestingKey] = useState(false);
  const [keyError, setKeyError] = useState<string | null>(null);
  const [keyOk, setKeyOk] = useState(false);

  async function onTestKey(e: React.FormEvent) {
    e.preventDefault();
    setKeyError(null);
    setKeyOk(false);
    setTestingKey(true);
    try {
      const result = await testMistralKey(studioId, apiKey);
      if (!result.ok) {
        setKeyError(result.error);
        return;
      }
      setKeyOk(true);
      if (apiKey) {
        setStatus({ hasApiKey: true, keySuffix: apiKey.slice(-4) });
        setApiKey('');
      }
    } finally {
      setTestingKey(false);
    }
  }

  if (loading) {
    return (
      <div className="flex items-center gap-2 rounded-xl border border-zinc-200 bg-white p-6 text-sm text-zinc-500 dark:border-zinc-800 dark:bg-zinc-950 dark:text-zinc-400">
        <Loader2 className="h-4 w-4 animate-spin" />
        Loading AI Assistant settings…
      </div>
    );
  }

  if (loadError || !status) {
    return (
      <div className="rounded-xl border border-red-200 bg-red-50 p-6 text-sm text-red-700 dark:border-red-500/20 dark:bg-red-500/10 dark:text-red-400">
        Failed to load AI Assistant settings{loadError ? `: ${loadError}` : ''}.
      </div>
    );
  }

  return (
    <div className="overflow-hidden rounded-xl border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-950">
      <div className="flex items-center gap-3 border-b border-zinc-200 px-6 py-4 dark:border-zinc-800">
        <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-[#FA520F]">
          <MistralMark className="text-sm font-black leading-none text-white" />
        </span>
        <h3 className="text-sm font-bold text-zinc-800 dark:text-zinc-100">Mistral (OCR)</h3>
      </div>

      <div className="space-y-5 p-6">
        <p className="text-xs text-zinc-500 dark:text-zinc-400">
          Used only for Knowledge Base image OCR (extracting text from uploaded photos/screenshots). The chat-reply
          AI providers (Groq/Gemini/Claude) are configured platform-wide by a superadmin now, not per studio.
        </p>

        <form onSubmit={onTestKey} className="space-y-2">
          <Label htmlFor="mistral-api-key">API key</Label>
          <div className="relative">
            <Input
              id="mistral-api-key"
              type={showApiKey ? 'text' : 'password'}
              placeholder={status.hasApiKey ? '••••••••••••••••' : 'Paste your API key… (a1B2c3...)'}
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
              className="pr-12"
            />
            <button
              type="button"
              className="absolute right-3 top-1/2 -translate-y-1/2 text-zinc-500 transition hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200"
              onClick={() => setShowApiKey(!showApiKey)}
            >
              {showApiKey ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
            </button>
          </div>
          <FieldHint>
            {status.hasApiKey
              ? `Stored encrypted. A key is saved${status.keySuffix ? ` (ends in ${status.keySuffix})` : ''}; the full key is never shown.`
              : 'Stored encrypted. The full key is never shown once saved.'}
          </FieldHint>
          <Button
            type="submit"
            loading={testingKey}
            disabled={!apiKey && !status.hasApiKey}
            variant="secondary"
            size="sm"
            leftIcon={<Plug className="h-3.5 w-3.5" />}
          >
            Test
          </Button>
          <FieldError message={keyError ?? undefined} />
          {keyOk && <p className="text-xs font-medium text-emerald-600 dark:text-emerald-400">Connection OK.</p>}
        </form>

        <div className="space-y-2 border-t border-zinc-200 pt-4 dark:border-zinc-800">
          <Label>Model</Label>
          <div className="flex items-center gap-2.5">
            <span className="font-mono text-xs text-zinc-800 dark:text-zinc-200">mistral-ocr-latest</span>
            <Badge tone="neutral">Fixed</Badge>
          </div>
          <FieldHint>OCR uses this one model — nothing to add or choose here.</FieldHint>
        </div>
      </div>
    </div>
  );
}
