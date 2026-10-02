import { ClipboardCheck } from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { EmptyState } from '@/components/ui/EmptyState';
import { Pagination } from '@/components/ui/Pagination';
import { AutoRefresh } from '@/components/AutoRefresh';
import { serverFetch, requireSession } from '@/lib/auth';
import { formatDate, formatDateTime } from '@/lib/datetime';
import type { GlofoxAttendanceRow } from '@/lib/types';
import { ThresholdSetting } from './ThresholdSetting';

const PAGE_SIZE = 50;

interface AttendanceResp {
  members: GlofoxAttendanceRow[];
  total: number;
  qualifyingThreshold: number;
  qualifyingCount: number;
  lastSyncedAt: string | null;
}

export default async function AttendancePage({
  params,
  searchParams,
}: {
  params: Promise<{ studioId: string }>;
  searchParams: Promise<{ page?: string }>;
}) {
  const { studioId } = await params;
  const sp = await searchParams;
  await requireSession();

  const page = Math.max(1, Number(sp.page) || 1);
  const offset = (page - 1) * PAGE_SIZE;

  const { members, total, qualifyingThreshold, qualifyingCount, lastSyncedAt } = await serverFetch<AttendanceResp>(
    `/api/v1/studios/${studioId}/glofox/attendance?limit=${PAGE_SIZE}&offset=${offset}`,
  );

  return (
    <div className="space-y-4 pb-10">
      {/* Backend polls Glofox every 15 min; this just re-fetches our own
          cached snapshot more often so the page doesn't look frozen. */}
      <AutoRefresh intervalMs={60000} />

      <div className="flex flex-wrap items-center justify-between gap-4 px-2">
        <div className="text-[11px] font-semibold text-zinc-400 dark:text-zinc-500">
          Class attendance within each member&apos;s current plan period, pulled from Glofox. Read-only — no messages are sent from this page.
        </div>
        <div className="text-right">
          <div className="text-[9px] font-black uppercase tracking-[0.18em] text-zinc-400">
            Last synced:{' '}
            <span className="text-zinc-700 dark:text-zinc-200" suppressHydrationWarning>
              {lastSyncedAt ? formatDateTime(lastSyncedAt) : 'never — worker hasn’t completed its first poll yet'}
            </span>
          </div>
        </div>
      </div>

      {total === 0 ? (
        <Card>
          <EmptyState
            icon={<ClipboardCheck className="h-6 w-6" />}
            title="No attendance data yet"
            description="The background worker polls Glofox every 15 minutes. Check back shortly, or connect Glofox for this studio under CRM Integrations."
          />
        </Card>
      ) : (
        <>
          <div className="grid gap-4 sm:grid-cols-2">
            <Card>
              <div className="text-[10px] font-black uppercase tracking-widest text-zinc-400">Tracked members</div>
              <div className="mt-1 text-2xl font-extrabold text-zinc-900 dark:text-zinc-50">{total}</div>
            </Card>
            <Card>
              <div className="flex items-center justify-between gap-2">
                <div className="text-[10px] font-black uppercase tracking-widest text-zinc-400">Qualifying members</div>
                <ThresholdSetting studioId={studioId} threshold={qualifyingThreshold} />
              </div>
              <div className="mt-1 text-2xl font-extrabold text-zinc-900 dark:text-zinc-50">{qualifyingCount}</div>
            </Card>
          </div>

          <Card>
            <div className="overflow-x-auto">
              <table className="w-full min-w-[820px] text-left text-sm">
                <thead>
                  <tr className="border-b border-zinc-200 text-[10px] font-black uppercase tracking-widest text-zinc-400 dark:border-zinc-800">
                    <th className="py-2 pr-4">Name</th>
                    <th className="py-2 pr-4">Phone</th>
                    <th className="py-2 pr-4">Email</th>
                    <th className="py-2 pr-4">Plan</th>
                    <th className="py-2 pr-4">Plan window</th>
                    <th className="py-2 pr-4">Classes attended</th>
                  </tr>
                </thead>
                <tbody>
                  {members.map((m) => (
                    <tr key={m.glofoxUserId} className="border-b border-zinc-100 last:border-0 dark:border-zinc-900">
                      <td className="py-2.5 pr-4 font-semibold text-zinc-800 dark:text-zinc-100">{m.name || m.glofoxUserId}</td>
                      <td className="py-2.5 pr-4 text-zinc-500 dark:text-zinc-400">{m.phone || '—'}</td>
                      <td className="py-2.5 pr-4 text-zinc-500 dark:text-zinc-400">{m.email || '—'}</td>
                      <td className="py-2.5 pr-4 text-zinc-500 dark:text-zinc-400">{m.planName || '—'}</td>
                      <td className="py-2.5 pr-4 text-zinc-500 dark:text-zinc-400 whitespace-nowrap">
                        {m.planStart ? (
                          <>
                            {formatDate(m.planStart)} &rarr; {m.planEnd ? formatDate(m.planEnd) : 'ongoing'}
                          </>
                        ) : (
                          'lifetime (no active plan)'
                        )}
                      </td>
                      <td className="py-2.5 pr-4 font-bold text-zinc-700 dark:text-zinc-200">
                        {m.planLimit > 0 ? `${m.classesAttended} of ${m.planLimit}` : m.classesAttended}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <Pagination total={total} pageSize={PAGE_SIZE} page={page} />
          </Card>
        </>
      )}
    </div>
  );
}
