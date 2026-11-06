import { AlertTriangle, CheckCircle2, Terminal, XCircle } from 'lucide-react';
import { Badge } from '@/components/ui/Badge';
import { Card } from '@/components/ui/Card';
import type { EvalReport } from './page';

export function EvalReportView({ report, loadError }: { report: EvalReport | null; loadError: string | null }) {
  if (loadError) {
    return (
      <p className="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-500/10 dark:text-red-400">
        {loadError}
      </p>
    );
  }

  if (!report) {
    return (
      <Card>
        <div className="flex items-start gap-3">
          <Terminal className="mt-0.5 h-5 w-5 shrink-0 text-zinc-400" />
          <div>
            <p className="font-bold text-zinc-900 dark:text-zinc-100">No eval report found yet</p>
            <p className="mt-1 text-sm text-zinc-500 dark:text-zinc-400">
              Run it on the API host, then refresh this page:
            </p>
            <code className="mt-3 block w-fit rounded-lg bg-zinc-50 px-4 py-2 text-xs dark:bg-zinc-800">
              cd apps/api && go run ./cmd/eval-replies -split all
            </code>
          </div>
        </div>
      </Card>
    );
  }

  const total = report.passCount + report.failCount + report.errorCount;

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-3 gap-4">
        <Card noPadding>
          <div className="p-5 text-center">
            <div className="flex items-center justify-center gap-1.5 text-emerald-600 dark:text-emerald-400">
              <CheckCircle2 className="h-4 w-4" />
              <span className="text-2xl font-black">{report.passCount}</span>
            </div>
            <p className="mt-1 text-xs font-bold uppercase tracking-wider text-zinc-400">Pass</p>
          </div>
        </Card>
        <Card noPadding>
          <div className="p-5 text-center">
            <div className="flex items-center justify-center gap-1.5 text-amber-600 dark:text-amber-400">
              <XCircle className="h-4 w-4" />
              <span className="text-2xl font-black">{report.failCount}</span>
            </div>
            <p className="mt-1 text-xs font-bold uppercase tracking-wider text-zinc-400">Fail</p>
          </div>
        </Card>
        <Card noPadding>
          <div className="p-5 text-center">
            <div className="flex items-center justify-center gap-1.5 text-red-600 dark:text-red-400">
              <AlertTriangle className="h-4 w-4" />
              <span className="text-2xl font-black">{report.errorCount}</span>
            </div>
            <p className="mt-1 text-xs font-bold uppercase tracking-wider text-zinc-400">Error</p>
          </div>
        </Card>
      </div>

      <p className="text-sm text-zinc-500 dark:text-zinc-400">
        {total === 0 ? 'No cases in this run.' : `${report.passCount} of ${total} cases passed.`}
      </p>

      <div className="space-y-3">
        {report.grades.map((g) => {
          const tone = g.err ? 'danger' : g.codeCheck.passed ? 'success' : 'warning';
          const label = g.err ? 'Error' : g.codeCheck.passed ? 'Passed' : 'Failed';
          return (
            <Card key={g.caseId} noPadding>
              <div className="p-5">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="font-mono text-sm font-bold text-zinc-900 dark:text-zinc-100">{g.caseId}</span>
                  <Badge tone={tone}>{label}</Badge>
                  {g.judgeRan && <Badge tone="info">Judge score {g.judge.score.toFixed(2)}</Badge>}
                </div>
                {g.err && <p className="mt-2 text-sm text-red-600 dark:text-red-400">{g.err}</p>}
                {!g.err && g.codeCheck.notes && g.codeCheck.notes.length > 0 && (
                  <ul className="mt-2 list-inside list-disc space-y-0.5 text-sm text-zinc-500 dark:text-zinc-400">
                    {g.codeCheck.notes.map((n, i) => (
                      <li key={i}>{n}</li>
                    ))}
                  </ul>
                )}
                {g.judgeRan && g.judge.notes && (
                  <p className="mt-2 text-xs text-zinc-400">Judge: {g.judge.notes}</p>
                )}
              </div>
            </Card>
          );
        })}
      </div>
    </div>
  );
}
