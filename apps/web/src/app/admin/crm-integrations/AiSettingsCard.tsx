'use client';

import { useState, useTransition } from 'react';
import { Sparkles, Pencil, Check } from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Select } from '@/components/ui/Select';
import { Input } from '@/components/ui/Input';
import { Label, FieldHint } from '@/components/ui/Label';
import { Badge } from '@/components/ui/Badge';
import { updateAiTaskConfig } from './actions';
import { CRM_DOC_PARSING_PURPOSE, LLM_MODELS, type AiTaskConfig, type LlmProviderName } from './types';

const PROVIDER_LABELS: Record<LlmProviderName, string> = {
  claude: 'Claude',
  gemini: 'Gemini',
  groq: 'Groq',
};

export function AiSettingsCard({ initialConfig }: { initialConfig: AiTaskConfig }) {
  const [config, setConfig] = useState(initialConfig);
  const [editing, setEditing] = useState(!initialConfig.configured);
  const [provider, setProvider] = useState<LlmProviderName>(initialConfig.provider ?? 'claude');
  const [model, setModel] = useState(initialConfig.model ?? LLM_MODELS.claude[0] ?? '');
  const [apiKey, setApiKey] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [pending, startTransition] = useTransition();

  const handleProviderChange = (p: LlmProviderName) => {
    setProvider(p);
    setModel(LLM_MODELS[p][0] ?? '');
  };

  const handleSave = () => {
    setError(null);
    startTransition(async () => {
      const result = await updateAiTaskConfig(CRM_DOC_PARSING_PURPOSE, provider, model, apiKey);
      if (!result.ok) {
        setError(result.error);
        return;
      }
      setConfig({ purpose: CRM_DOC_PARSING_PURPOSE, configured: true, provider, model, hasApiKey: config.hasApiKey || apiKey !== '' });
      setApiKey('');
      setEditing(false);
    });
  };

  return (
    <Card
      title={
        <span className="flex items-center gap-2">
          <Sparkles className="h-4 w-4 text-brand-500" />
          AI that reads CRM docs
        </span>
      }
      subtitle="Whichever model you pick here is the one that reads an uploaded CRM doc and suggests the API mapping below."
      action={
        !editing && (
          <Button size="sm" variant="outline" leftIcon={<Pencil className="h-3.5 w-3.5" />} onClick={() => setEditing(true)}>
            Change
          </Button>
        )
      }
    >
      {!editing ? (
        <div className="flex items-center gap-3">
          {config.configured ? (
            <>
              <Badge tone="brand">{PROVIDER_LABELS[config.provider!]}</Badge>
              <span className="text-sm font-medium text-zinc-600 dark:text-zinc-300">{config.model}</span>
              {config.hasApiKey && <Badge tone="success">Custom key</Badge>}
            </>
          ) : (
            <span className="text-sm font-medium text-zinc-500">Not configured yet — pick a provider to enable "Add CRM".</span>
          )}
        </div>
      ) : (
        <div className="space-y-4">
          <div className="grid gap-4 sm:grid-cols-2">
            <div>
              <Label>Provider</Label>
              <Select value={provider} onChange={(e) => handleProviderChange(e.target.value as LlmProviderName)}>
                <option value="claude">Claude</option>
                <option value="gemini">Gemini</option>
                <option value="groq">Groq</option>
              </Select>
            </div>
            <div>
              <Label>Model</Label>
              <Select value={model} onChange={(e) => setModel(e.target.value)}>
                {LLM_MODELS[provider].map((m) => (
                  <option key={m} value={m}>{m}</option>
                ))}
              </Select>
            </div>
          </div>
          <div>
            <Label>API key override (optional)</Label>
            <Input
              type="password"
              placeholder={config.hasApiKey ? 'Already set — leave blank to keep it' : "Leave blank to use the platform's key"}
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
            />
            <FieldHint>Only needed if this task should use a different API key than the rest of the platform.</FieldHint>
          </div>
          {error && <p className="text-sm font-medium text-red-600 dark:text-red-400">{error}</p>}
          <div className="flex gap-2">
            <Button size="sm" leftIcon={<Check className="h-3.5 w-3.5" />} loading={pending} onClick={handleSave}>
              Save
            </Button>
            {config.configured && (
              <Button size="sm" variant="ghost" onClick={() => setEditing(false)} disabled={pending}>
                Cancel
              </Button>
            )}
          </div>
        </div>
      )}
    </Card>
  );
}
