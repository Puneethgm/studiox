'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { Eye, EyeOff, Loader2, Plug, Plus, Trash2, ArrowUp, ArrowDown } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { FieldError, FieldHint, Label } from '@/components/ui/Label';
import { Badge } from '@/components/ui/Badge';
import {
  getAIModels,
  testAIProviderKey,
  addAndTestAIModel,
  setAIModelEnabled,
  deleteAIModel,
  reorderAIModels,
  type AIProvider,
  type AIProviderConfig,
  type AIModel,
} from './ai-actions';

// Official brand marks (path data from simple-icons, MIT-licensed —
// exactly what that project exists for: representing a third-party
// service/brand inside another product). Groq has no distinct icon mark of
// its own in simple-icons or on groq.com beyond a wordmark, so it gets a
// plain lettermark in Groq's own brand orange instead of a borrowed shape.
function GeminiMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" className={className} aria-hidden="true">
      <title>Google Gemini</title>
      <defs>
        <linearGradient id="gemini-mark-grad" x1="0%" y1="100%" x2="100%" y2="0%">
          <stop offset="0%" stopColor="#4285F4" />
          <stop offset="50%" stopColor="#9B72CB" />
          <stop offset="100%" stopColor="#D96570" />
        </linearGradient>
      </defs>
      <path
        fill="url(#gemini-mark-grad)"
        d="M11.04 19.32Q12 21.51 12 24q0-2.49.93-4.68.96-2.19 2.58-3.81t3.81-2.55Q21.51 12 24 12q-2.49 0-4.68-.93a12.3 12.3 0 0 1-3.81-2.58 12.3 12.3 0 0 1-2.58-3.81Q12 2.49 12 0q0 2.49-.96 4.68-.93 2.19-2.55 3.81a12.3 12.3 0 0 1-3.81 2.58Q2.49 12 0 12q2.49 0 4.68.96 2.19.93 3.81 2.55t2.55 3.81"
      />
    </svg>
  );
}

function AnthropicMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" className={className} aria-hidden="true" fill="#D97757">
      <title>Anthropic</title>
      <path d="M17.3041 3.541h-3.6718l6.696 16.918H24Zm-10.6082 0L0 20.459h3.7442l1.3693-3.5527h7.0052l1.3693 3.5528h3.7442L10.5363 3.5409Zm-.3712 10.2232 2.2914-5.9456 2.2914 5.9456Z" />
    </svg>
  );
}

function GroqMark({ className }: { className?: string }) {
  return (
    <span className={className} aria-hidden="true">
      G
    </span>
  );
}

const PROVIDER_META: Record<
  AIProvider,
  { label: string; keyPlaceholder: string; hint: string; icon: ReactNode; iconBg: string }
> = {
  groq: {
    label: 'Groq',
    keyPlaceholder: 'gsk_...',
    hint: 'Fast, cheap models — tried first before falling back to Gemini/Claude. Several enabled models are tried in order.',
    icon: <GroqMark className="text-base font-black leading-none text-white" />,
    iconBg: 'bg-[#F55036]',
  },
  gemini: {
    label: 'Google (Gemini)',
    keyPlaceholder: 'AIzaSy...',
    hint: 'Also powers intent classification, query expansion, and knowledge-base reranking (uses the first enabled model for those).',
    icon: <GeminiMark className="h-4 w-4" />,
    iconBg: 'bg-white dark:bg-zinc-200 ring-1 ring-inset ring-zinc-200 dark:ring-zinc-700',
  },
  claude: {
    label: 'Anthropic (Claude)',
    keyPlaceholder: 'sk-ant-...',
    hint: 'Highest-quality fallback when Groq and Gemini are unavailable or fail.',
    icon: <AnthropicMark className="h-4 w-4" />,
    iconBg: 'bg-[#F0EEE6] dark:bg-zinc-200 ring-1 ring-inset ring-zinc-200 dark:ring-zinc-700',
  },
};

const PROVIDER_ORDER: AIProvider[] = ['gemini', 'claude', 'groq'];

export function AIProviderSettings({ studioId }: { studioId: string }) {
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [providers, setProviders] = useState<Record<AIProvider, AIProviderConfig> | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const result = await getAIModels(studioId);
      if (cancelled) return;
      if (!result.ok) {
        setLoadError(result.error);
        setLoading(false);
        return;
      }
      const byProvider = {} as Record<AIProvider, AIProviderConfig>;
      for (const p of result.providers) byProvider[p.provider] = p;
      setProviders(byProvider);
      setLoading(false);
    })();
    return () => {
      cancelled = true;
    };
  }, [studioId]);

  if (loading) {
    return (
      <div className="flex items-center gap-2 rounded-xl border border-zinc-200 bg-white p-6 text-sm text-zinc-500 dark:border-zinc-800 dark:bg-zinc-950 dark:text-zinc-400">
        <Loader2 className="h-4 w-4 animate-spin" />
        Loading AI Assistant settings…
      </div>
    );
  }

  if (loadError || !providers) {
    return (
      <div className="rounded-xl border border-red-200 bg-red-50 p-6 text-sm text-red-700 dark:border-red-500/20 dark:bg-red-500/10 dark:text-red-400">
        Failed to load AI Assistant settings{loadError ? `: ${loadError}` : ''}.
      </div>
    );
  }

  return (
    <div className="space-y-5">
      {PROVIDER_ORDER.map((provider) => (
        <ProviderCard
          key={provider}
          studioId={studioId}
          provider={provider}
          config={providers[provider]}
          onChange={(next) => setProviders((prev) => (prev ? { ...prev, [provider]: next } : prev))}
        />
      ))}
    </div>
  );
}

function ProviderCard({
  studioId,
  provider,
  config,
  onChange,
}: {
  studioId: string;
  provider: AIProvider;
  config: AIProviderConfig;
  onChange: (next: AIProviderConfig) => void;
}) {
  const meta = PROVIDER_META[provider];
  const enabledCount = config.models.filter((m) => m.enabled).length;

  const [apiKey, setApiKey] = useState('');
  const [showApiKey, setShowApiKey] = useState(false);
  const [testingKey, setTestingKey] = useState(false);
  const [keyError, setKeyError] = useState<string | null>(null);
  const [keyOk, setKeyOk] = useState(false);

  const [newModelName, setNewModelName] = useState('');
  const [addingModel, setAddingModel] = useState(false);
  const [addModelError, setAddModelError] = useState<string | null>(null);

  const [busyModel, setBusyModel] = useState<string | null>(null); // modelName currently mid-request
  const [modelErrors, setModelErrors] = useState<Record<string, string>>({});

  async function onTestKey(e: React.FormEvent) {
    e.preventDefault();
    setKeyError(null);
    setKeyOk(false);
    setTestingKey(true);
    try {
      const result = await testAIProviderKey(studioId, provider, apiKey);
      if (!result.ok) {
        setKeyError(result.error);
        return;
      }
      setKeyOk(true);
      if (apiKey) {
        setApiKey('');
        onChange({ ...config, hasApiKey: true, keySuffix: apiKey.slice(-4) });
      }
    } finally {
      setTestingKey(false);
    }
  }

  // Checking a virtual "current default" checkbox, or adding a brand-new
  // model, both go through the same test+persist action.
  async function testAndUpsert(modelName: string) {
    setBusyModel(modelName);
    setModelErrors((prev) => ({ ...prev, [modelName]: '' }));
    try {
      const result = await addAndTestAIModel(studioId, provider, modelName);
      if (!result.ok) {
        setModelErrors((prev) => ({ ...prev, [modelName]: result.error }));
        return;
      }
      const nextModels = config.models.filter((m) => m.modelName !== modelName);
      nextModels.push({ ...result.model, isDefault: false });
      onChange({ ...config, models: nextModels });
    } finally {
      setBusyModel(null);
    }
  }

  async function onToggleModel(model: AIModel, enabled: boolean) {
    if (model.isDefault || !model.id) {
      // Virtual default — enabling it means testing it for real for the first time.
      if (enabled) await testAndUpsert(model.modelName);
      return;
    }
    setBusyModel(model.modelName);
    try {
      const result = await setAIModelEnabled(studioId, provider, model.id, enabled);
      if (!result.ok) {
        setModelErrors((prev) => ({ ...prev, [model.modelName]: result.error }));
        return;
      }
      onChange({
        ...config,
        models: config.models.map((m) => (m.id === model.id ? { ...m, enabled } : m)),
      });
    } finally {
      setBusyModel(null);
    }
  }

  async function onDeleteModel(model: AIModel) {
    if (!model.id) return;
    setBusyModel(model.modelName);
    try {
      const result = await deleteAIModel(studioId, provider, model.id);
      if (!result.ok) {
        setModelErrors((prev) => ({ ...prev, [model.modelName]: result.error }));
        return;
      }
      onChange({ ...config, models: config.models.filter((m) => m.id !== model.id) });
    } finally {
      setBusyModel(null);
    }
  }

  // Reordering only applies to persisted models (virtual "current default"
  // rows aren't real yet, so there's nothing to reorder until one is tested).
  const persistedModels = config.models.filter((m) => m.id);

  async function onMoveModel(model: AIModel, direction: -1 | 1) {
    const idx = persistedModels.findIndex((m) => m.id === model.id);
    const swapIdx = idx + direction;
    if (idx < 0 || swapIdx < 0 || swapIdx >= persistedModels.length) return;

    const reordered = [...persistedModels];
    const moved = reordered[idx]!;
    reordered[idx] = reordered[swapIdx]!;
    reordered[swapIdx] = moved;

    setBusyModel(model.modelName);
    try {
      const result = await reorderAIModels(studioId, provider, reordered.map((m) => m.id!));
      if (!result.ok) {
        setModelErrors((prev) => ({ ...prev, [model.modelName]: result.error }));
        return;
      }
      const virtualModels = config.models.filter((m) => !m.id);
      onChange({ ...config, models: [...reordered, ...virtualModels] });
    } finally {
      setBusyModel(null);
    }
  }

  async function onAddModel(e: React.FormEvent) {
    e.preventDefault();
    const name = newModelName.trim();
    if (!name) return;
    setAddModelError(null);
    setAddingModel(true);
    try {
      const result = await addAndTestAIModel(studioId, provider, name);
      if (!result.ok) {
        setAddModelError(result.error);
        return;
      }
      const nextModels = config.models.filter((m) => m.modelName !== name);
      nextModels.push({ ...result.model, isDefault: false });
      onChange({ ...config, models: nextModels });
      setNewModelName('');
    } finally {
      setAddingModel(false);
    }
  }

  return (
    <div className="overflow-hidden rounded-xl border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-950">
      <div className="flex items-center justify-between gap-2 border-b border-zinc-200 px-6 py-4 dark:border-zinc-800">
        <div className="flex items-center gap-3">
          <span className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-full ${meta.iconBg}`}>
            {meta.icon}
          </span>
          <h3 className="text-sm font-bold text-zinc-800 dark:text-zinc-100">{meta.label}</h3>
        </div>
        {enabledCount > 0 && (
          <Badge tone="neutral">
            {enabledCount} model{enabledCount === 1 ? '' : 's'}
          </Badge>
        )}
      </div>

      <div className="space-y-5 p-6">
        <form onSubmit={onTestKey} className="space-y-2">
          <Label htmlFor={`${provider}-api-key`}>API key</Label>
          <div className="relative">
            <Input
              id={`${provider}-api-key`}
              type={showApiKey ? 'text' : 'password'}
              placeholder={config.hasApiKey ? '••••••••••••••••' : `Paste your API key… (${meta.keyPlaceholder})`}
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
            {config.hasApiKey
              ? `Stored encrypted. A key is saved${config.keySuffix ? ` (ends in ${config.keySuffix})` : ''}; the full key is never shown.`
              : 'Stored encrypted. The full key is never shown once saved.'}
          </FieldHint>
          <Button type="submit" loading={testingKey} disabled={!apiKey && !config.hasApiKey} variant="secondary" size="sm" leftIcon={<Plug className="h-3.5 w-3.5" />}>
            Test
          </Button>
          <FieldError message={keyError ?? undefined} />
          {keyOk && <p className="text-xs font-medium text-emerald-600 dark:text-emerald-400">Connection OK.</p>}
        </form>

        <div className="space-y-2 border-t border-zinc-200 pt-4 dark:border-zinc-800">
          <div className="flex items-center justify-between">
            <Label>Models</Label>
            {persistedModels.length > 1 && (
              <span className="text-[10px] font-medium uppercase tracking-wide text-zinc-400">
                Tried top to bottom
              </span>
            )}
          </div>
          {config.models.length === 0 && (
            <p className="text-xs text-zinc-500 dark:text-zinc-400">No models configured yet.</p>
          )}

          {persistedModels.length > 0 ? (
            <ul className="space-y-1">
              {persistedModels.map((model, idx) => {
                const busy = busyModel === model.modelName;
                return (
                  <li
                    key={model.id}
                    className="group rounded border border-transparent px-1 py-1 hover:border-zinc-200 dark:hover:border-zinc-800"
                  >
                    <div className="flex items-center gap-2.5">
                      <span className="w-4 shrink-0 text-center text-[10px] font-bold text-zinc-400">{idx + 1}</span>
                      <input
                        id={`${provider}-model-${model.modelName}`}
                        type="checkbox"
                        checked={model.enabled}
                        disabled={busy}
                        onChange={(e) => onToggleModel(model, e.target.checked)}
                        style={{ accentColor: 'var(--brand, #7c3aed)' }}
                        className="h-4 w-4 shrink-0 rounded border-zinc-300 dark:border-zinc-700"
                      />
                      <label
                        htmlFor={`${provider}-model-${model.modelName}`}
                        className="min-w-0 flex-1 truncate font-mono text-xs text-zinc-800 dark:text-zinc-200"
                        title={model.modelName}
                      >
                        {model.modelName}
                      </label>
                      {busy && <Loader2 className="h-3.5 w-3.5 shrink-0 animate-spin text-zinc-400" />}
                      <div className="flex shrink-0 items-center gap-0.5 opacity-0 transition group-hover:opacity-100">
                        <button
                          type="button"
                          disabled={busy || idx === 0}
                          onClick={() => onMoveModel(model, -1)}
                          className="text-zinc-400 hover:text-zinc-700 disabled:pointer-events-none disabled:opacity-30 dark:hover:text-zinc-200"
                          aria-label={`Move ${model.modelName} up`}
                        >
                          <ArrowUp className="h-3.5 w-3.5" />
                        </button>
                        <button
                          type="button"
                          disabled={busy || idx === persistedModels.length - 1}
                          onClick={() => onMoveModel(model, 1)}
                          className="text-zinc-400 hover:text-zinc-700 disabled:pointer-events-none disabled:opacity-30 dark:hover:text-zinc-200"
                          aria-label={`Move ${model.modelName} down`}
                        >
                          <ArrowDown className="h-3.5 w-3.5" />
                        </button>
                        <button
                          type="button"
                          disabled={busy}
                          onClick={() => onDeleteModel(model)}
                          className="ml-1 text-zinc-400 hover:text-red-500 disabled:pointer-events-none disabled:opacity-30"
                          aria-label={`Remove ${model.modelName}`}
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </button>
                      </div>
                    </div>
                    {modelErrors[model.modelName] && (
                      <p className="mt-1 pl-6 text-xs font-medium text-red-600 dark:text-red-400">
                        {modelErrors[model.modelName]}
                      </p>
                    )}
                  </li>
                );
              })}
            </ul>
          ) : (
            <div className="grid gap-x-8 gap-y-2 sm:grid-cols-2">
              {config.models.map((model) => {
                const busy = busyModel === model.modelName;
                return (
                  <div key={model.modelName}>
                    <div className="flex items-center gap-2.5">
                      <input
                        id={`${provider}-model-${model.modelName}`}
                        type="checkbox"
                        checked={model.enabled}
                        disabled={busy}
                        onChange={(e) => onToggleModel(model, e.target.checked)}
                        style={{ accentColor: 'var(--brand, #7c3aed)' }}
                        className="h-4 w-4 shrink-0 rounded border-zinc-300 dark:border-zinc-700"
                      />
                      <label
                        htmlFor={`${provider}-model-${model.modelName}`}
                        className="min-w-0 flex-1 truncate font-mono text-xs text-zinc-800 dark:text-zinc-200"
                        title={model.modelName}
                      >
                        {model.modelName}
                      </label>
                      {busy && <Loader2 className="h-3.5 w-3.5 shrink-0 animate-spin text-zinc-400" />}
                      <Badge tone="neutral" className="shrink-0">
                        Default
                      </Badge>
                    </div>
                    {modelErrors[model.modelName] && (
                      <p className="mt-1 pl-6 text-xs font-medium text-red-600 dark:text-red-400">
                        {modelErrors[model.modelName]}
                      </p>
                    )}
                  </div>
                );
              })}
            </div>
          )}
        </div>

        <form onSubmit={onAddModel} className="flex items-start gap-2 border-t border-zinc-200 pt-4 dark:border-zinc-800">
          <div className="flex-1">
            <Input
              placeholder="Add a custom model name"
              value={newModelName}
              onChange={(e) => setNewModelName(e.target.value)}
              className="font-mono text-xs"
            />
            <FieldError message={addModelError ?? undefined} />
            {!config.hasApiKey && <FieldHint>Save an API key above before adding a model to test.</FieldHint>}
          </div>
          <Button type="submit" loading={addingModel} disabled={!newModelName.trim() || !config.hasApiKey} size="md" leftIcon={<Plus className="h-4 w-4" />}>
            Add
          </Button>
        </form>
      </div>
    </div>
  );
}
