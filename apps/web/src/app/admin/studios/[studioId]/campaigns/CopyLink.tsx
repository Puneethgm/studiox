'use client';

import { useState } from 'react';
import { Check, Copy as CopyIcon, ExternalLink } from 'lucide-react';
import { Button } from '@/components/ui/Button';

export function CopyLink({ url }: { url: string }) {
  const [copied, setCopied] = useState(false);

  async function copy() {
    try {
      await navigator.clipboard.writeText(url);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      window.prompt('Copy link:', url);
    }
  }

  return (
    <div className="flex items-center gap-2">
      <code className="flex-1 truncate rounded-xl bg-slate-100/50 px-3 py-2 font-mono text-[10px] font-bold text-slate-500 dark:bg-slate-800/50">
        {url}
      </code>
      <Button
        variant="outline"
        size="sm"
        onClick={copy}
        className="rounded-xl border-slate-200 bg-white shadow-sm hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-900"
        leftIcon={copied ? <Check className="h-3.5 w-3.5" /> : <CopyIcon className="h-3.5 w-3.5" />}
        suppressHydrationWarning
      >
        {copied ? 'Copied' : 'Copy'}
      </Button>
    </div>
  );
}
