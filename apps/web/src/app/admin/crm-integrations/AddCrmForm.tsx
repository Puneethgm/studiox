'use client';

import { useRef, useState, useTransition, type DragEvent } from 'react';
import { UploadCloud, FileText, Wand2, X } from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Textarea } from '@/components/ui/Textarea';
import { Label, FieldHint } from '@/components/ui/Label';
import { parseProviderDoc } from './actions';
import type { CrmOperation, CrmProvider } from './types';

export function AddCrmForm({
  aiConfigured,
  onAnalyzed,
  onCancel,
}: {
  aiConfigured: boolean;
  onAnalyzed: (provider: CrmProvider, operations: CrmOperation[]) => void;
  onCancel: () => void;
}) {
  const [name, setName] = useState('');
  const [docText, setDocText] = useState('');
  const [fileName, setFileName] = useState<string | null>(null);
  const [dragging, setDragging] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [pending, startTransition] = useTransition();
  const fileInputRef = useRef<HTMLInputElement>(null);

  const loadFile = (file: File) => {
    setFileName(file.name);
    const reader = new FileReader();
    reader.onload = () => setDocText(String(reader.result ?? ''));
    reader.readAsText(file);
  };

  const handleDrop = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    setDragging(false);
    const file = e.dataTransfer.files?.[0];
    if (file) loadFile(file);
  };

  const handleAnalyze = () => {
    setError(null);
    if (!name.trim()) {
      setError('Give this CRM a name first.');
      return;
    }
    if (!docText.trim()) {
      setError('Paste or drop the API documentation first.');
      return;
    }
    startTransition(async () => {
      const result = await parseProviderDoc(name.trim(), docText);
      if (!result.ok) {
        setError(result.error);
        return;
      }
      onAnalyzed(result.data.provider, result.data.operations);
    });
  };

  return (
    <Card
      title="Add a new CRM"
      subtitle="Give it a name and its API documentation — the AI will suggest which endpoint does what. You review and confirm before it goes live."
      action={
        <Button size="sm" variant="ghost" leftIcon={<X className="h-3.5 w-3.5" />} onClick={onCancel}>
          Close
        </Button>
      }
    >
      <div className="space-y-4">
        <div>
          <Label>CRM name</Label>
          <Input placeholder="e.g. Mindbody" value={name} onChange={(e) => setName(e.target.value)} />
        </div>

        <div>
          <Label>API documentation</Label>
          <div
            onDragOver={(e) => { e.preventDefault(); setDragging(true); }}
            onDragLeave={() => setDragging(false)}
            onDrop={handleDrop}
            onClick={() => fileInputRef.current?.click()}
            className={`flex cursor-pointer flex-col items-center justify-center gap-2 rounded-xl border-2 border-dashed p-6 text-center transition-colors ${
              dragging
                ? 'border-brand-500 bg-brand-50/50 dark:bg-brand-500/5'
                : 'border-zinc-200 hover:border-zinc-300 dark:border-zinc-800 dark:hover:border-zinc-700'
            }`}
          >
            <input
              ref={fileInputRef}
              type="file"
              accept=".json,.yaml,.yml,.txt,.md"
              className="hidden"
              onChange={(e) => { const f = e.target.files?.[0]; if (f) loadFile(f); }}
            />
            {fileName ? (
              <>
                <FileText className="h-6 w-6 text-brand-500" />
                <p className="text-sm font-semibold text-zinc-700 dark:text-zinc-200">{fileName}</p>
                <p className="text-xs text-zinc-500">Click to choose a different file</p>
              </>
            ) : (
              <>
                <UploadCloud className="h-6 w-6 text-zinc-400" />
                <p className="text-sm font-semibold text-zinc-700 dark:text-zinc-200">Drop a Swagger / OpenAPI / doc file here</p>
                <p className="text-xs text-zinc-500">or click to browse — .json, .yaml, .txt, .md</p>
              </>
            )}
          </div>
          <FieldHint>Or paste the documentation text directly below instead of uploading a file.</FieldHint>
          <Textarea
            className="mt-2 font-mono text-xs"
            rows={8}
            placeholder="Paste the CRM's API documentation here (Swagger/OpenAPI JSON or YAML, or free-form docs)..."
            value={docText}
            onChange={(e) => { setDocText(e.target.value); setFileName(null); }}
          />
        </div>

        {!aiConfigured && (
          <p className="text-sm font-medium text-amber-600 dark:text-amber-400">
            Set up "AI that reads CRM docs" above first — no provider is configured yet.
          </p>
        )}
        {error && <p className="text-sm font-medium text-red-600 dark:text-red-400">{error}</p>}

        <Button leftIcon={<Wand2 className="h-4 w-4" />} loading={pending} disabled={!aiConfigured} onClick={handleAnalyze}>
          Analyze with AI
        </Button>
      </div>
    </Card>
  );
}
