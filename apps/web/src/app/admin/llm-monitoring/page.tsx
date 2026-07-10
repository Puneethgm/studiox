'use client';

export const dynamic = 'force-dynamic';

import { useEffect, useState } from 'react';
import {
  BarChart,
  Bar,
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from 'recharts';
import { Activity, CheckCircle2, ChevronLeft, ChevronRight, ChevronDown, ChevronUp, Clock, Cpu, DollarSign, Search } from 'lucide-react';

const PAGE_SIZE = 10;

// Cost per 1M tokens (USD) — update when pricing changes
const COST_PER_1M: Record<string, { in: number; out: number }> = {
  'llama-3.1-8b-instant':    { in: 0.05,  out: 0.08  },
  'llama-3.3-70b-versatile': { in: 0.59,  out: 0.79  },
  'gemini-2.5-flash':        { in: 0.075, out: 0.30  },
  'gemini-2.0-flash':        { in: 0.075, out: 0.30  },
  'gemini-2.0-flash-lite':   { in: 0.075, out: 0.04  },
  'claude-haiku-4-5':        { in: 0.80,  out: 4.00  },
};

function computeCostUSD(model: string, tokensIn: number, tokensOut: number): number {
  const p = COST_PER_1M[model];
  if (!p) return 0;
  return (tokensIn / 1_000_000) * p.in + (tokensOut / 1_000_000) * p.out;
}

function fmtCost(usd: number): string {
  if (usd === 0) return '—';
  if (usd < 0.0001) return '<$0.0001';
  return `$${usd.toFixed(4)}`;
}

interface LLMStat {
  provider: string;
  model: string;
  count: number;
  successCount: number;
  avgLatencyMs: number;
  tokensIn: number;
  tokensOut: number;
  date: string;
}

interface LLMStudioStat {
  studioId: string | null;
  studioName: string;
  provider: string;
  model: string;
  count: number;
  successCount: number;
  avgLatencyMs: number;
  tokensIn: number;
  tokensOut: number;
}

interface AggregatedStudio {
  studioId: string;
  studioName: string;
  totalRequests: number;
  totalSuccess: number;
  avgLatencyMs: number;
  totalTokensIn: number;
  totalTokensOut: number;
  totalCostUSD: number;
  breakdown: LLMStudioStat[];
}

function aggregateStudioStats(studioStats: LLMStudioStat[]): AggregatedStudio[] {
  const groups: Record<string, AggregatedStudio> = {};
  for (const row of studioStats) {
    const key = row.studioId || 'platform';
    if (!groups[key]) {
      groups[key] = {
        studioId: key,
        studioName: row.studioName,
        totalRequests: 0,
        totalSuccess: 0,
        avgLatencyMs: 0,
        totalTokensIn: 0,
        totalTokensOut: 0,
        totalCostUSD: 0,
        breakdown: [],
      };
    }
    const group = groups[key];
    const cost = computeCostUSD(row.model, row.tokensIn, row.tokensOut);

    group.totalRequests += row.count;
    group.totalSuccess += row.successCount;
    group.avgLatencyMs += row.avgLatencyMs * row.count;
    group.totalTokensIn += row.tokensIn;
    group.totalTokensOut += row.tokensOut;
    group.totalCostUSD += cost;
    group.breakdown.push(row);
  }

  return Object.values(groups).map((group) => {
    if (group.totalRequests > 0) {
      group.avgLatencyMs = Math.round(group.avgLatencyMs / group.totalRequests);
    }
    return group;
  }).sort((a, b) => b.totalRequests - a.totalRequests);
}

interface Summary {
  todayTotal: number;
  successRate: number;
  avgLatency: number;
  topModel: string;
  totalCostUSD: number;
}

function computeSummary(stats: LLMStat[]): Summary {
  const today = new Date().toISOString().slice(0, 10);
  const todayRows = stats.filter((s) => s.date === today);
  const todayTotal = todayRows.reduce((acc, s) => acc + s.count, 0);

  const totalAll = stats.reduce((acc, s) => acc + s.count, 0);
  const totalSuccess = stats.reduce((acc, s) => acc + s.successCount, 0);
  const successRate = totalAll > 0 ? Math.round((totalSuccess / totalAll) * 100) : 0;

  const avgLatency =
    totalAll > 0
      ? Math.round(stats.reduce((acc, s) => acc + s.avgLatencyMs * s.count, 0) / totalAll)
      : 0;

  const modelCount: Record<string, number> = {};
  for (const s of stats) {
    modelCount[s.model] = (modelCount[s.model] ?? 0) + s.count;
  }
  const topModel = Object.entries(modelCount).sort((a, b) => b[1] - a[1])[0]?.[0] ?? '—';

  const totalCostUSD = stats.reduce(
    (acc, s) => acc + computeCostUSD(s.model, s.tokensIn, s.tokensOut),
    0,
  );

  return { todayTotal, successRate, avgLatency, topModel, totalCostUSD };
}

function buildBarData(stats: LLMStat[]) {
  const sevenDaysAgo = new Date();
  sevenDaysAgo.setDate(sevenDaysAgo.getDate() - 7);
  const cutoff = sevenDaysAgo.toISOString().slice(0, 10);

  const byDateProvider: Record<string, Record<string, number>> = {};
  const providers = new Set<string>();

  for (const s of stats) {
    if (s.date < cutoff) continue;
    providers.add(s.provider);
    const dateRow = byDateProvider[s.date] ?? {};
    byDateProvider[s.date] = dateRow;
    dateRow[s.provider] = (dateRow[s.provider] ?? 0) + s.count;
  }

  const dates = Object.keys(byDateProvider).sort();
  return {
    data: dates.map((date) => ({ date, ...(byDateProvider[date] ?? {}) })),
    providers: Array.from(providers),
  };
}

function buildLineData(stats: LLMStat[]) {
  const sevenDaysAgo = new Date();
  sevenDaysAgo.setDate(sevenDaysAgo.getDate() - 7);
  const cutoff = sevenDaysAgo.toISOString().slice(0, 10);

  const byDateProvider: Record<string, Record<string, { sumLat: number; count: number }>> = {};
  const providers = new Set<string>();

  for (const s of stats) {
    if (s.date < cutoff) continue;
    providers.add(s.provider);
    const dateRow = byDateProvider[s.date] ?? {};
    byDateProvider[s.date] = dateRow;
    const prev = dateRow[s.provider] ?? { sumLat: 0, count: 0 };
    dateRow[s.provider] = {
      sumLat: prev.sumLat + s.avgLatencyMs * s.count,
      count: prev.count + s.count,
    };
  }

  const dates = Object.keys(byDateProvider).sort();
  return {
    data: dates.map((date) => {
      const row: Record<string, string | number> = { date };
      const dateRow = byDateProvider[date] ?? {};
      for (const [prov, v] of Object.entries(dateRow)) {
        row[prov] = v.count > 0 ? Math.round(v.sumLat / v.count) : 0;
      }
      return row;
    }),
    providers: Array.from(providers),
  };
}

const PROVIDER_COLORS: Record<string, string> = {
  gemini: '#4285f4',
  claude: '#7c3aed',
  groq: '#10b981',
  fallback: '#f59e0b',
};

function providerColor(p: string) {
  return PROVIDER_COLORS[p] ?? '#6b7280';
}

export default function LLMMonitoringPage() {
  const [stats, setStats] = useState<LLMStat[]>([]);
  const [studioStats, setStudioStats] = useState<LLMStudioStat[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [page, setPage] = useState(0);
  const [studioPage, setStudioPage] = useState(0);
  const [studioSearch, setStudioSearch] = useState('');
  const [expandedStudios, setExpandedStudios] = useState<Record<string, boolean>>({});

  useEffect(() => {
    fetch('/api/v1/admin/llm-stats', { credentials: 'include' })
      .then((r) => {
        if (!r.ok) throw new Error(`HTTP ${r.status}`);
        return r.json() as Promise<{ stats: LLMStat[]; studioStats: LLMStudioStat[] }>;
      })
      .then((d) => {
        setStats(d.stats ?? []);
        setStudioStats(d.studioStats ?? []);
        setLoading(false);
      })
      .catch((e: Error) => {
        setError(e.message);
        setLoading(false);
      });
  }, []);

  const toggleStudioExpand = (id: string) => {
    setExpandedStudios((prev) => ({ ...prev, [id]: !prev[id] }));
  };

  const summary = computeSummary(stats);
  const { data: barData, providers: barProviders } = buildBarData(stats);
  const { data: lineData, providers: lineProviders } = buildLineData(stats);

  const totalPages = Math.ceil(stats.length / PAGE_SIZE);
  const pageRows = stats.slice(page * PAGE_SIZE, page * PAGE_SIZE + PAGE_SIZE);

  if (loading) {
    return (
      <div className="space-y-8">
        <div>
          <h1 className="text-2xl font-black tracking-tight text-zinc-900 dark:text-white">LLM Monitor</h1>
          <p className="text-sm text-zinc-500 dark:text-zinc-400 mt-1">AI provider usage — last 30 days</p>
        </div>
        <div className="grid grid-cols-2 gap-4 lg:grid-cols-5">
          {[...Array(5)].map((_, i) => (
            <div key={i} className="rounded-2xl border border-zinc-200 dark:border-white/10 bg-zinc-100 dark:bg-white/5 p-5 animate-pulse">
              <div className="h-4 w-24 rounded bg-zinc-200 dark:bg-white/10 mb-4" />
              <div className="h-8 w-16 rounded bg-zinc-200 dark:bg-white/10" />
            </div>
          ))}
        </div>
        <div className="grid grid-cols-1 gap-6 xl:grid-cols-2">
          {[...Array(2)].map((_, i) => (
            <div key={i} className="rounded-2xl border border-zinc-200 dark:border-white/10 bg-zinc-100 dark:bg-white/5 p-6 animate-pulse">
              <div className="h-4 w-48 rounded bg-zinc-200 dark:bg-white/10 mb-4" />
              <div className="h-[240px] rounded bg-zinc-200/50 dark:bg-white/5" />
            </div>
          ))}
        </div>
        <div className="rounded-2xl border border-zinc-200 dark:border-white/10 bg-zinc-100 dark:bg-white/5 animate-pulse">
          <div className="px-6 py-4 border-b border-zinc-200 dark:border-white/10">
            <div className="h-4 w-32 rounded bg-zinc-200 dark:bg-white/10" />
          </div>
          <div className="p-6 space-y-3">
            {[...Array(5)].map((_, i) => (
              <div key={i} className="h-10 rounded bg-zinc-200/50 dark:bg-white/5" />
            ))}
          </div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="rounded-2xl border border-red-200 dark:border-red-500/20 bg-red-50 dark:bg-red-500/10 p-6 text-red-600 dark:text-red-400">
        Failed to load: {error}
      </div>
    );
  }

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl font-black tracking-tight text-zinc-900 dark:text-white">LLM Monitor</h1>
        <p className="text-sm text-zinc-500 dark:text-zinc-400 mt-1">AI provider usage — last 30 days</p>
      </div>

      {/* Summary cards */}
      <div className="grid grid-cols-2 gap-4 lg:grid-cols-5">
        <SummaryCard
          icon={<Activity className="h-5 w-5 text-violet-600 dark:text-violet-400" />}
          label="Requests today"
          value={summary.todayTotal.toLocaleString()}
          color="violet"
        />
        <SummaryCard
          icon={<CheckCircle2 className="h-5 w-5 text-emerald-600 dark:text-emerald-400" />}
          label="Success rate"
          value={`${summary.successRate}%`}
          color="emerald"
        />
        <SummaryCard
          icon={<Clock className="h-5 w-5 text-blue-600 dark:text-blue-400" />}
          label="Avg latency"
          value={`${summary.avgLatency} ms`}
          color="blue"
        />
        <SummaryCard
          icon={<Cpu className="h-5 w-5 text-amber-600 dark:text-amber-400" />}
          label="Top model"
          value={summary.topModel}
          color="amber"
        />
        <SummaryCard
          icon={<DollarSign className="h-5 w-5 text-rose-600 dark:text-rose-400" />}
          label="Total cost (30d)"
          value={summary.totalCostUSD < 0.01 ? `$${summary.totalCostUSD.toFixed(5)}` : `$${summary.totalCostUSD.toFixed(4)}`}
          color="rose"
        />
      </div>

      {/* Charts row */}
      <div className="grid grid-cols-1 gap-6 xl:grid-cols-2">
        <div className="rounded-2xl border border-zinc-200 dark:border-white/10 bg-white dark:bg-white/5 p-6">
          <h2 className="text-sm font-bold text-zinc-700 dark:text-zinc-300 mb-4">Requests by provider (last 7 days)</h2>
          {barData.length === 0 ? (
            <EmptyChart />
          ) : (
            <ResponsiveContainer width="100%" height={240}>
              <BarChart data={barData} margin={{ top: 4, right: 8, bottom: 4, left: 0 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="rgba(0,0,0,0.06)" className="dark:[&>line]:stroke-white/10" />
                <XAxis
                  dataKey="date"
                  tick={{ fontSize: 11, fill: '#71717a' }}
                  tickFormatter={(v: string) => v.slice(5)}
                />
                <YAxis tick={{ fontSize: 11, fill: '#71717a' }} />
                <Tooltip
                  contentStyle={{
                    background: 'var(--tooltip-bg, #fff)',
                    border: '1px solid rgba(0,0,0,0.1)',
                    borderRadius: '12px',
                    fontSize: 12,
                    color: '#111',
                  }}
                />
                <Legend wrapperStyle={{ fontSize: 12 }} />
                {barProviders.map((p) => (
                  <Bar key={p} dataKey={p} stackId="a" fill={providerColor(p)} radius={[4, 4, 0, 0]} />
                ))}
              </BarChart>
            </ResponsiveContainer>
          )}
        </div>

        <div className="rounded-2xl border border-zinc-200 dark:border-white/10 bg-white dark:bg-white/5 p-6">
          <h2 className="text-sm font-bold text-zinc-700 dark:text-zinc-300 mb-4">Avg latency by provider (last 7 days, ms)</h2>
          {lineData.length === 0 ? (
            <EmptyChart />
          ) : (
            <ResponsiveContainer width="100%" height={240}>
              <LineChart data={lineData} margin={{ top: 4, right: 8, bottom: 4, left: 0 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="rgba(0,0,0,0.06)" />
                <XAxis
                  dataKey="date"
                  tick={{ fontSize: 11, fill: '#71717a' }}
                  tickFormatter={(v: string) => v.slice(5)}
                />
                <YAxis tick={{ fontSize: 11, fill: '#71717a' }} />
                <Tooltip
                  contentStyle={{
                    background: 'var(--tooltip-bg, #fff)',
                    border: '1px solid rgba(0,0,0,0.1)',
                    borderRadius: '12px',
                    fontSize: 12,
                    color: '#111',
                  }}
                />
                <Legend wrapperStyle={{ fontSize: 12 }} />
                {lineProviders.map((p) => (
                  <Line
                    key={p}
                    type="monotone"
                    dataKey={p}
                    stroke={providerColor(p)}
                    strokeWidth={2}
                    dot={{ r: 3, fill: providerColor(p) }}
                    activeDot={{ r: 5 }}
                  />
                ))}
              </LineChart>
            </ResponsiveContainer>
          )}
        </div>
      </div>

      {/* Studio usage table */}
      <div className="rounded-2xl border border-zinc-200 dark:border-white/10 bg-white dark:bg-white/5 overflow-hidden">
        <div className="px-6 py-4 border-b border-zinc-200 dark:border-white/10 flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
          <div>
            <h2 className="text-sm font-bold text-zinc-700 dark:text-zinc-300">Studio Usage Tracking</h2>
            <p className="text-xs text-zinc-500 dark:text-zinc-400 mt-0.5">LLM consumption broken down by studio tenant</p>
          </div>
          <div className="relative w-full sm:w-64">
            <Search className="absolute left-3 top-2.5 h-4 w-4 text-zinc-400" />
            <input
              type="text"
              placeholder="Search studios..."
              value={studioSearch}
              onChange={(e) => {
                setStudioSearch(e.target.value);
                setStudioPage(0);
              }}
              className="w-full rounded-lg border border-zinc-200 dark:border-white/10 bg-transparent py-1.5 pl-9 pr-4 text-xs placeholder-zinc-400 focus:border-violet-500 focus:outline-none dark:text-white"
            />
          </div>
        </div>
        {(() => {
          const aggregatedStudios = aggregateStudioStats(studioStats);
          const filteredStudios = aggregatedStudios.filter((s) =>
            s.studioName.toLowerCase().includes(studioSearch.toLowerCase())
          );
          const studioTotalPages = Math.ceil(filteredStudios.length / PAGE_SIZE);
          const studioPageRows = filteredStudios.slice(studioPage * PAGE_SIZE, studioPage * PAGE_SIZE + PAGE_SIZE);

          if (filteredStudios.length === 0) {
            return <div className="p-8 text-center text-zinc-400 text-sm">No studio usage recorded in this period.</div>;
          }

          return (
            <>
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-zinc-100 dark:border-white/10 text-zinc-500 dark:text-zinc-500 text-xs uppercase tracking-wider bg-zinc-50 dark:bg-transparent">
                      <th className="w-10 px-6 py-3" />
                      <th className="px-6 py-3 text-left font-semibold">Studio Name</th>
                      <th className="px-6 py-3 text-right font-semibold">Requests</th>
                      <th className="px-6 py-3 text-right font-semibold">Success Rate</th>
                      <th className="px-6 py-3 text-right font-semibold">Avg Latency</th>
                      <th className="px-6 py-3 text-right font-semibold">Tokens In</th>
                      <th className="px-6 py-3 text-right font-semibold">Tokens Out</th>
                      <th className="px-6 py-3 text-right font-semibold">Est. Cost</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-zinc-100 dark:divide-white/5">
                    {studioPageRows.map((studio) => {
                      const isExpanded = !!expandedStudios[studio.studioId];
                      return (
                        <tr key={studio.studioId} className="hover:bg-zinc-50 dark:hover:bg-white/5 transition-colors cursor-pointer" onClick={() => toggleStudioExpand(studio.studioId)}>
                          <td className="px-6 py-3 text-center">
                            {isExpanded ? (
                              <ChevronUp className="h-4 w-4 text-zinc-400 inline" />
                            ) : (
                              <ChevronDown className="h-4 w-4 text-zinc-400 inline" />
                            )}
                          </td>
                          <td className="px-6 py-3 font-semibold text-zinc-800 dark:text-zinc-200">
                            {studio.studioName}
                          </td>
                          <td className="px-6 py-3 text-right text-zinc-800 dark:text-zinc-200 tabular-nums font-medium">
                            {studio.totalRequests.toLocaleString()}
                          </td>
                          <td className="px-6 py-3 text-right">
                            <span
                              className={`text-xs font-semibold ${
                                studio.totalRequests > 0 && studio.totalSuccess / studio.totalRequests >= 0.9
                                  ? 'text-emerald-600 dark:text-emerald-400'
                                  : 'text-amber-600 dark:text-amber-400'
                              }`}
                            >
                              {studio.totalRequests > 0 ? Math.round((studio.totalSuccess / studio.totalRequests) * 100) : 0}%
                            </span>
                          </td>
                          <td className="px-6 py-3 text-right text-zinc-500 dark:text-zinc-400 tabular-nums font-mono text-xs">
                            {studio.avgLatencyMs} ms
                          </td>
                          <td className="px-6 py-3 text-right text-zinc-500 dark:text-zinc-400 tabular-nums font-mono text-xs">
                            {studio.totalTokensIn.toLocaleString()}
                          </td>
                          <td className="px-6 py-3 text-right text-zinc-500 dark:text-zinc-400 tabular-nums font-mono text-xs">
                            {studio.totalTokensOut.toLocaleString()}
                          </td>
                          <td className="px-6 py-3 text-right text-zinc-700 dark:text-zinc-300 tabular-nums font-mono text-xs font-semibold">
                            {fmtCost(studio.totalCostUSD)}
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>

              {/* Render expanded details outside the main table rows */}
              {studioPageRows.map((studio) => {
                const isExpanded = !!expandedStudios[studio.studioId];
                if (!isExpanded) return null;
                return (
                  <div key={`${studio.studioId}-expanded`} className="px-6 py-4 border-t border-zinc-100 dark:border-white/5 bg-zinc-50/50 dark:bg-white/[0.02]">
                    <div className="rounded-xl border border-zinc-200/60 dark:border-white/5 overflow-hidden bg-white dark:bg-zinc-900/40 p-4">
                      <h4 className="text-xs font-bold text-zinc-500 dark:text-zinc-400 mb-3 uppercase tracking-wider">Model Breakdown for {studio.studioName}</h4>
                      <table className="w-full text-xs text-left">
                        <thead>
                          <tr className="border-b border-zinc-100 dark:border-white/5 text-zinc-400 text-[10px] uppercase bg-zinc-50/70 dark:bg-transparent">
                            <th className="px-4 py-2 font-medium">Provider</th>
                            <th className="px-4 py-2 font-medium">Model</th>
                            <th className="px-4 py-2 text-right font-medium">Requests</th>
                            <th className="px-4 py-2 text-right font-medium">Success</th>
                            <th className="px-4 py-2 text-right font-medium">Latency</th>
                            <th className="px-4 py-2 text-right font-medium">Tokens In</th>
                            <th className="px-4 py-2 text-right font-medium">Tokens Out</th>
                            <th className="px-4 py-2 text-right font-medium">Cost</th>
                          </tr>
                        </thead>
                        <tbody className="divide-y divide-zinc-100/60 dark:divide-white/5">
                          {studio.breakdown.map((item, idx) => {
                            const itemCost = computeCostUSD(item.model, item.tokensIn, item.tokensOut);
                            return (
                              <tr key={idx} className="hover:bg-zinc-50/50 dark:hover:bg-white/5 transition-colors">
                                <td className="px-4 py-2">
                                  <span
                                    className="inline-flex items-center rounded-full px-2 py-0.5 text-[10px] font-semibold"
                                    style={{
                                      background: `${providerColor(item.provider)}18`,
                                      color: providerColor(item.provider),
                                    }}
                                  >
                                    {item.provider}
                                  </span>
                                </td>
                                <td className="px-4 py-2 font-mono text-zinc-600 dark:text-zinc-300">{item.model}</td>
                                <td className="px-4 py-2 text-right text-zinc-700 dark:text-zinc-200 tabular-nums">{item.count}</td>
                                <td className="px-4 py-2 text-right">
                                  <span
                                    className={
                                      item.count > 0 && item.successCount / item.count >= 0.9
                                        ? 'text-emerald-600 dark:text-emerald-400'
                                        : 'text-amber-600 dark:text-amber-400'
                                    }
                                  >
                                    {item.count > 0 ? Math.round((item.successCount / item.count) * 100) : 0}%
                                  </span>
                                </td>
                                <td className="px-4 py-2 text-right font-mono text-zinc-500">{Math.round(item.avgLatencyMs)} ms</td>
                                <td className="px-4 py-2 text-right font-mono text-zinc-500">{item.tokensIn.toLocaleString()}</td>
                                <td className="px-4 py-2 text-right font-mono text-zinc-500">{item.tokensOut.toLocaleString()}</td>
                                <td className="px-4 py-2 text-right font-mono text-zinc-600 dark:text-zinc-300 font-medium">{fmtCost(itemCost)}</td>
                              </tr>
                            );
                          })}
                        </tbody>
                      </table>
                    </div>
                  </div>
                );
              })}

              {/* Pagination for studios */}
              {studioTotalPages > 1 && (
                <div className="flex items-center justify-between px-6 py-4 border-t border-zinc-100 dark:border-white/10 bg-white dark:bg-transparent">
                  <span className="text-xs text-zinc-400">
                    Page {studioPage + 1} of {studioTotalPages}
                  </span>
                  <div className="flex items-center gap-1">
                    <button
                      onClick={() => setStudioPage((p) => Math.max(0, p - 1))}
                      disabled={studioPage === 0}
                      className="flex h-8 w-8 items-center justify-center rounded-lg border border-zinc-200 dark:border-white/10 text-zinc-500 dark:text-zinc-400 hover:bg-zinc-100 dark:hover:bg-white/10 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
                    >
                      <ChevronLeft className="h-4 w-4" />
                    </button>
                    {Array.from({ length: studioTotalPages }, (_, i) => i)
                      .filter((i) => Math.abs(i - studioPage) <= 2)
                      .map((i) => (
                        <button
                          key={i}
                          onClick={() => setStudioPage(i)}
                          className={`flex h-8 w-8 items-center justify-center rounded-lg text-xs font-semibold transition-colors ${
                            i === studioPage
                              ? 'bg-violet-600 text-white'
                              : 'border border-zinc-200 dark:border-white/10 text-zinc-500 dark:text-zinc-400 hover:bg-zinc-100 dark:hover:bg-white/10'
                          }`}
                        >
                          {i + 1}
                        </button>
                      ))}
                    <button
                      onClick={() => setStudioPage((p) => Math.min(studioTotalPages - 1, p + 1))}
                      disabled={studioPage === studioTotalPages - 1}
                      className="flex h-8 w-8 items-center justify-center rounded-lg border border-zinc-200 dark:border-white/10 text-zinc-500 dark:text-zinc-400 hover:bg-zinc-100 dark:hover:bg-white/10 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
                    >
                      <ChevronRight className="h-4 w-4" />
                    </button>
                  </div>
                </div>
              )}
            </>
          );
        })()}
      </div>

      {/* Table with pagination */}
      <div className="rounded-2xl border border-zinc-200 dark:border-white/10 bg-white dark:bg-white/5 overflow-hidden">
        <div className="px-6 py-4 border-b border-zinc-200 dark:border-white/10 flex items-center justify-between">
          <h2 className="text-sm font-bold text-zinc-700 dark:text-zinc-300">Recent usage logs</h2>
          {stats.length > 0 && (
            <span className="text-xs text-zinc-400">
              {stats.length} total rows
            </span>
          )}
        </div>
        {stats.length === 0 ? (
          <div className="p-8 text-center text-zinc-400 text-sm">No usage logs yet.</div>
        ) : (
          <>
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-zinc-100 dark:border-white/10 text-zinc-500 dark:text-zinc-500 text-xs uppercase tracking-wider bg-zinc-50 dark:bg-transparent">
                    <th className="px-6 py-3 text-left font-semibold">Date</th>
                    <th className="px-6 py-3 text-left font-semibold">Provider</th>
                    <th className="px-6 py-3 text-left font-semibold">Model</th>
                    <th className="px-6 py-3 text-right font-semibold">Requests</th>
                    <th className="px-6 py-3 text-right font-semibold">Success</th>
                    <th className="px-6 py-3 text-right font-semibold">Avg latency</th>
                    <th className="px-6 py-3 text-right font-semibold">Tokens in</th>
                    <th className="px-6 py-3 text-right font-semibold">Tokens out</th>
                    <th className="px-6 py-3 text-right font-semibold">Cost</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-zinc-100 dark:divide-white/5">
                  {pageRows.map((row, i) => (
                    <tr key={i} className="hover:bg-zinc-50 dark:hover:bg-white/5 transition-colors">
                      <td className="px-6 py-3 text-zinc-500 dark:text-zinc-400 font-mono text-xs">{row.date}</td>
                      <td className="px-6 py-3">
                        <span
                          className="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-semibold"
                          style={{
                            background: `${providerColor(row.provider)}18`,
                            color: providerColor(row.provider),
                          }}
                        >
                          {row.provider}
                        </span>
                      </td>
                      <td className="px-6 py-3 text-zinc-600 dark:text-zinc-300 font-mono text-xs">{row.model}</td>
                      <td className="px-6 py-3 text-right text-zinc-800 dark:text-zinc-200 tabular-nums font-medium">{row.count}</td>
                      <td className="px-6 py-3 text-right">
                        <span
                          className={`text-xs font-semibold ${
                            row.count > 0 && row.successCount / row.count >= 0.9
                              ? 'text-emerald-600 dark:text-emerald-400'
                              : 'text-amber-600 dark:text-amber-400'
                          }`}
                        >
                          {row.count > 0 ? Math.round((row.successCount / row.count) * 100) : 0}%
                        </span>
                      </td>
                      <td className="px-6 py-3 text-right text-zinc-500 dark:text-zinc-400 tabular-nums font-mono text-xs">
                        {Math.round(row.avgLatencyMs)} ms
                      </td>
                      <td className="px-6 py-3 text-right text-zinc-500 dark:text-zinc-400 tabular-nums font-mono text-xs">
                        {row.tokensIn > 0 ? row.tokensIn.toLocaleString() : '—'}
                      </td>
                      <td className="px-6 py-3 text-right text-zinc-500 dark:text-zinc-400 tabular-nums font-mono text-xs">
                        {row.tokensOut > 0 ? row.tokensOut.toLocaleString() : '—'}
                      </td>
                      <td className="px-6 py-3 text-right text-zinc-700 dark:text-zinc-300 tabular-nums font-mono text-xs font-medium">
                        {fmtCost(computeCostUSD(row.model, row.tokensIn, row.tokensOut))}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            {/* Pagination bar */}
            {totalPages > 1 && (
              <div className="flex items-center justify-between px-6 py-4 border-t border-zinc-100 dark:border-white/10">
                <span className="text-xs text-zinc-400">
                  Page {page + 1} of {totalPages}
                </span>
                <div className="flex items-center gap-1">
                  <button
                    onClick={() => setPage((p) => Math.max(0, p - 1))}
                    disabled={page === 0}
                    className="flex h-8 w-8 items-center justify-center rounded-lg border border-zinc-200 dark:border-white/10 text-zinc-500 dark:text-zinc-400 hover:bg-zinc-100 dark:hover:bg-white/10 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
                  >
                    <ChevronLeft className="h-4 w-4" />
                  </button>
                  {Array.from({ length: totalPages }, (_, i) => i)
                    .filter((i) => Math.abs(i - page) <= 2)
                    .map((i) => (
                      <button
                        key={i}
                        onClick={() => setPage(i)}
                        className={`flex h-8 w-8 items-center justify-center rounded-lg text-xs font-semibold transition-colors ${
                          i === page
                            ? 'bg-violet-600 text-white'
                            : 'border border-zinc-200 dark:border-white/10 text-zinc-500 dark:text-zinc-400 hover:bg-zinc-100 dark:hover:bg-white/10'
                        }`}
                      >
                        {i + 1}
                      </button>
                    ))}
                  <button
                    onClick={() => setPage((p) => Math.min(totalPages - 1, p + 1))}
                    disabled={page === totalPages - 1}
                    className="flex h-8 w-8 items-center justify-center rounded-lg border border-zinc-200 dark:border-white/10 text-zinc-500 dark:text-zinc-400 hover:bg-zinc-100 dark:hover:bg-white/10 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
                  >
                    <ChevronRight className="h-4 w-4" />
                  </button>
                </div>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}

function SummaryCard({
  icon,
  label,
  value,
  color,
}: {
  icon: React.ReactNode;
  label: string;
  value: string;
  color: 'violet' | 'emerald' | 'blue' | 'amber' | 'rose';
}) {
  const styles: Record<string, string> = {
    violet: 'bg-violet-50 dark:bg-violet-500/10 border-violet-200 dark:border-violet-500/20',
    emerald: 'bg-emerald-50 dark:bg-emerald-500/10 border-emerald-200 dark:border-emerald-500/20',
    blue: 'bg-blue-50 dark:bg-blue-500/10 border-blue-200 dark:border-blue-500/20',
    amber: 'bg-amber-50 dark:bg-amber-500/10 border-amber-200 dark:border-amber-500/20',
    rose: 'bg-rose-50 dark:bg-rose-500/10 border-rose-200 dark:border-rose-500/20',
  };
  return (
    <div className={`rounded-2xl border p-5 ${styles[color]}`}>
      <div className="flex items-center gap-2 mb-3">
        {icon}
        <span className="text-xs font-semibold text-zinc-500 dark:text-zinc-400 uppercase tracking-wider">{label}</span>
      </div>
      <div className="text-2xl font-black text-zinc-900 dark:text-white truncate">{value}</div>
    </div>
  );
}

function EmptyChart() {
  return (
    <div className="flex items-center justify-center h-[240px] text-zinc-400 text-sm">
      No data for this period
    </div>
  );
}
