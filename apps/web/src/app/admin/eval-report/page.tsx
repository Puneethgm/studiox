import { requireSession, serverFetch } from '@/lib/auth';
import { redirect } from 'next/navigation';
import { PageHeader } from '@/components/ui/PageHeader';
import { EvalReportView } from './EvalReportView';

export interface CodeCheckResult {
  passed: boolean;
  notes: string[] | null;
}

export interface JudgeResult {
  score: number;
  notes: string;
}

export interface Grade {
  caseId: string;
  codeCheck: CodeCheckResult;
  judge: JudgeResult;
  judgeRan: boolean;
  err?: string;
}

export interface EvalReport {
  grades: Grade[];
  passCount: number;
  failCount: number;
  errorCount: number;
}

export default async function EvalReportPage() {
  const me = await requireSession();
  // Backend-enforced too (identity.RequireRole(RoleSuperAdmin) on this
  // route group) — this redirect is just so a non-super-admin never sees
  // a generic error-boundary crash for what's really "you can't be here."
  if (me.role !== 'super_admin') {
    redirect('/admin/studios');
  }

  let report: EvalReport | null = null;
  let loadError: string | null = null;
  try {
    report = await serverFetch<EvalReport>('/api/v1/admin/eval-report');
  } catch (err) {
    // serverFetch throws a plain Error("API <path>: <status>") for a
    // non-401/403-totp failure — a 404 here just means no eval has been
    // run on this host yet, which is the normal first-time state, not a
    // real error.
    if (err instanceof Error && err.message.includes(': 404')) {
      report = null;
    } else {
      loadError = 'Could not load the eval report.';
    }
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Eval report"
        description="The last ai-reply-flow-llm-design pipeline run against the eval case set. This page only reads the result — it never runs an eval itself."
      />
      <EvalReportView report={report} loadError={loadError} />
    </div>
  );
}
