import Link from 'next/link';
import {
  ArrowRight, Building2, Plus, Megaphone,
  Users, Activity, TrendingUp, CheckCircle2, Layers
} from 'lucide-react';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { EmptyState } from '@/components/ui/EmptyState';
import { brandInitials } from '@/lib/color';
import { serverFetch } from '@/lib/auth';
import type { Studio } from '@/lib/types';

interface ListResp {
  studios: Studio[];
}

export default async function StudiosListPage() {
  const { studios } = await serverFetch<ListResp>('/api/v1/admin/studios');

  const totalCampaigns = studios.reduce((sum, s) => sum + (s.campaignCount ?? 0), 0);
  const totalLeads     = studios.reduce((sum, s) => sum + (s.leadCount ?? 0), 0);
  const activeCount    = studios.filter((s) => s.active).length;
  const inactiveCount  = studios.length - activeCount;

  return (
    <div className="space-y-8 pb-12">

      {/* ── Page header ───────────────────────── */}
      <div
        className="relative overflow-hidden rounded-[26px] border border-white/30 p-6 backdrop-blur-2xl dark:border-white/5"
        style={{
          background: 'linear-gradient(135deg, rgba(255,255,255,0.30) 0%, rgba(237,233,254,0.22) 60%, rgba(219,234,254,0.20) 100%)',
          boxShadow: 'inset 0 0 0 1px rgba(255,255,255,0.2), 0 8px 32px rgba(139,92,246,0.07)',
        }}
      >
        {/* Glow blobs */}
        <div className="pointer-events-none absolute -right-16 -top-16 h-48 w-48 rounded-full bg-brand-500/10 blur-[70px]" />
        <div className="pointer-events-none absolute -bottom-12 -left-12 h-40 w-40 rounded-full bg-sky-400/10 blur-[60px]" />

        <div className="relative flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex items-center gap-4">
            <div className="grid h-12 w-12 place-items-center rounded-2xl bg-gradient-to-br from-brand-500 to-violet-600 text-white shadow-lg shadow-brand-500/25">
              <Layers className="h-6 w-6" />
            </div>
            <div>
              <h1 className="text-2xl font-black tracking-tight text-zinc-900 dark:text-white">Studios</h1>
              <p className="mt-0.5 text-xs font-semibold text-zinc-500 dark:text-zinc-400">
                Each studio is a tenant — its own admins, campaigns, leads, branding, and lead-capture URLs.
              </p>
            </div>
          </div>
          <Link href="/admin/studios/new">
            <Button
              leftIcon={<Plus className="h-4 w-4" />}
              suppressHydrationWarning
            >
              New Studio
            </Button>
          </Link>
        </div>
      </div>

      {studios.length === 0 ? (
        <div
          className="overflow-hidden rounded-[24px] border border-white/30 backdrop-blur-2xl dark:border-white/5"
          style={{ background: 'rgba(255,255,255,0.25)', boxShadow: 'inset 0 0 0 1px rgba(255,255,255,0.2)' }}
        >
          <EmptyState
            icon={<Building2 className="h-8 w-8" />}
            title="No studios yet"
            description="Create the first studio to onboard a vendor onto the platform."
            action={
              <Link href="/admin/studios/new">
                <Button leftIcon={<Plus className="h-4 w-4" />}>Create studio</Button>
              </Link>
            }
          />
        </div>
      ) : (
        <>
          {/* ── Summary stat cards ─────────────── */}
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <SummaryCard
              label="Total Studios"
              value={studios.length}
              icon={<Building2 className="h-5 w-5" />}
              color="violet"
              hint={`${activeCount} active`}
            />
            <SummaryCard
              label="Active Studios"
              value={activeCount}
              icon={<CheckCircle2 className="h-5 w-5" />}
              color="emerald"
              hint={inactiveCount > 0 ? `${inactiveCount} inactive` : 'All online'}
            />
            <SummaryCard
              label="Campaigns"
              value={totalCampaigns}
              icon={<Megaphone className="h-5 w-5" />}
              color="sky"
              hint="Across all studios"
            />
            <SummaryCard
              label="Total Leads"
              value={totalLeads}
              icon={<Users className="h-5 w-5" />}
              color="amber"
              hint="All-time submissions"
            />
          </div>

          {/* ── Studio cards grid ─────────────── */}
          <div className="grid gap-5 md:grid-cols-2 xl:grid-cols-3">
            {studios.map((s, idx) => (
              <StudioCard key={s.id} studio={s} idx={idx} />
            ))}
          </div>
        </>
      )}
    </div>
  );
}

// ─────────────────────────────────────────────────────
// Summary stat card
// ─────────────────────────────────────────────────────

type CardColor = 'violet' | 'emerald' | 'sky' | 'amber';

const colorMap: Record<CardColor, {
  bg: string; border: string; iconBg: string; iconText: string;
  valueTx: string; hint: string; shadow: string; glow: string;
}> = {
  violet: {
    bg: 'rgba(245,243,255,0.45)', border: 'rgba(196,181,253,0.40)',
    iconBg: 'rgba(139,92,246,0.12)', iconText: '#7c3aed',
    valueTx: '#4c1d95', hint: '#7c3aed',
    shadow: '0 4px 20px rgba(139,92,246,0.10), inset 0 0 0 1px rgba(221,214,254,0.40)',
    glow: 'rgba(139,92,246,0.15)',
  },
  emerald: {
    bg: 'rgba(236,253,245,0.45)', border: 'rgba(110,231,183,0.40)',
    iconBg: 'rgba(16,185,129,0.12)', iconText: '#059669',
    valueTx: '#065f46', hint: '#10b981',
    shadow: '0 4px 20px rgba(16,185,129,0.10), inset 0 0 0 1px rgba(167,243,208,0.40)',
    glow: 'rgba(16,185,129,0.15)',
  },
  sky: {
    bg: 'rgba(240,249,255,0.45)', border: 'rgba(125,211,252,0.40)',
    iconBg: 'rgba(14,165,233,0.12)', iconText: '#0284c7',
    valueTx: '#0c4a6e', hint: '#0ea5e9',
    shadow: '0 4px 20px rgba(14,165,233,0.10), inset 0 0 0 1px rgba(186,230,253,0.40)',
    glow: 'rgba(14,165,233,0.15)',
  },
  amber: {
    bg: 'rgba(255,251,235,0.45)', border: 'rgba(252,211,77,0.40)',
    iconBg: 'rgba(245,158,11,0.12)', iconText: '#d97706',
    valueTx: '#78350f', hint: '#f59e0b',
    shadow: '0 4px 20px rgba(245,158,11,0.10), inset 0 0 0 1px rgba(253,230,138,0.40)',
    glow: 'rgba(245,158,11,0.15)',
  },
};

function SummaryCard({
  label, value, icon, color, hint,
}: {
  label: string; value: number; icon: React.ReactNode; color: CardColor; hint?: string;
}) {
  const c = colorMap[color];
  return (
    <div
      className="relative overflow-hidden rounded-[20px] p-5 backdrop-blur-2xl"
      style={{ background: c.bg, border: `1px solid ${c.border}`, boxShadow: c.shadow }}
    >
      {/* Glow blob */}
      <div
        className="pointer-events-none absolute -right-6 -top-6 h-20 w-20 rounded-full blur-2xl"
        style={{ background: c.glow }}
      />
      <div className="relative flex items-start justify-between gap-3">
        <div>
          <div className="text-[10px] font-black uppercase tracking-[0.18em]" style={{ color: c.iconText }}>
            {label}
          </div>
          <div className="mt-2 text-3xl font-black tracking-tight" style={{ color: c.valueTx }}>
            {value}
          </div>
          {hint && (
            <div className="mt-1 text-[11px] font-bold" style={{ color: c.hint }}>
              {hint}
            </div>
          )}
        </div>
        <div
          className="grid h-10 w-10 shrink-0 place-items-center rounded-xl"
          style={{ background: c.iconBg, color: c.iconText }}
        >
          {icon}
        </div>
      </div>
    </div>
  );
}

// ─────────────────────────────────────────────────────
// Studio card
// ─────────────────────────────────────────────────────

function StudioCard({ studio: s, idx }: { studio: Studio; idx: number }) {
  return (
    <Link
      href={`/admin/studios/${s.id}`}
      className="group block focus:outline-none"
    >
      <div
        className="relative h-full overflow-hidden rounded-[24px] backdrop-blur-2xl transition-all duration-500 hover:-translate-y-1.5 hover:scale-[1.012]"
        style={{
          background: 'linear-gradient(145deg, rgba(255,255,255,0.38) 0%, rgba(248,245,255,0.30) 100%)',
          border: '1px solid rgba(255,255,255,0.35)',
          boxShadow: 'inset 0 0 0 1px rgba(255,255,255,0.20), 0 4px 24px rgba(0,0,0,0.06)',
        }}
      >
        {/* Brand color top bar with gradient fade */}
        <div
          className="relative h-1.5 w-full"
          style={{ background: `linear-gradient(90deg, ${s.brandColor} 0%, ${s.brandColor}88 100%)` }}
        />

        {/* Subtle brand color ambient glow */}
        <div
          className="pointer-events-none absolute -right-10 -top-10 h-32 w-32 rounded-full blur-3xl opacity-20 transition-opacity duration-500 group-hover:opacity-40"
          style={{ background: s.brandColor }}
        />

        <div className="p-6">
          {/* Top row: logo + name + badge */}
          <div className="flex items-start gap-4">
            <div
              className="relative grid h-14 w-14 shrink-0 place-items-center overflow-hidden rounded-[18px] text-lg font-black text-white shadow-lg ring-4 ring-white/40 transition-transform duration-500 group-hover:scale-110 group-hover:rotate-3 dark:ring-white/10"
              style={{ background: s.brandColor }}
            >
              {s.logoUrl ? (
                // eslint-disable-next-line @next/next/no-img-element
                <img src={s.logoUrl} alt="" className="h-full w-full object-cover" />
              ) : (
                brandInitials(s.name)
              )}
              {/* Shine overlay */}
              <div className="absolute inset-0 bg-gradient-to-br from-white/20 to-transparent" />
            </div>

            <div className="min-w-0 flex-1 pt-0.5">
              <div className="flex items-center justify-between gap-2">
                <h3 className="truncate text-base font-black text-zinc-900 transition-colors group-hover:text-brand-600 dark:text-white dark:group-hover:text-brand-400">
                  {s.name}
                </h3>
                <Badge
                  tone={s.active ? 'success' : 'neutral'}
                  className="shrink-0 text-[10px] font-black uppercase tracking-wider"
                >
                  {s.active ? 'Active' : 'Inactive'}
                </Badge>
              </div>
              <div className="mt-1 flex items-center gap-1.5 font-mono text-[10px] font-bold uppercase tracking-wider text-zinc-400">
                <Building2 className="h-3 w-3" />
                /{s.slug}
              </div>
              {/* Description line */}
              <p className="mt-1.5 text-[11px] font-semibold leading-relaxed text-zinc-500 dark:text-zinc-400 line-clamp-1">
                {s.contactEmail ?? 'No contact email set'}
              </p>
            </div>
          </div>

          {/* Divider */}
          <div className="my-5 h-px bg-gradient-to-r from-transparent via-white/30 to-transparent dark:via-white/10" />

          {/* Stats row */}
          <div className="flex items-center justify-between">
            <div className="flex gap-6">
              {/* Campaigns */}
              <div className="flex items-center gap-2">
                <div className="grid h-8 w-8 place-items-center rounded-xl bg-sky-50/70 text-sky-600 dark:bg-sky-500/10 dark:text-sky-400">
                  <Megaphone className="h-3.5 w-3.5" />
                </div>
                <div>
                  <div className="text-base font-black text-zinc-900 dark:text-white">{s.campaignCount ?? 0}</div>
                  <div className="text-[9px] font-black uppercase tracking-widest text-zinc-400">Campaigns</div>
                </div>
              </div>
              {/* Leads */}
              <div className="flex items-center gap-2">
                <div className="grid h-8 w-8 place-items-center rounded-xl bg-violet-50/70 text-violet-600 dark:bg-violet-500/10 dark:text-violet-400">
                  <Users className="h-3.5 w-3.5" />
                </div>
                <div>
                  <div className="text-base font-black text-zinc-900 dark:text-white">{s.leadCount ?? 0}</div>
                  <div className="text-[9px] font-black uppercase tracking-widest text-zinc-400">Leads</div>
                </div>
              </div>
            </div>

            {/* Arrow CTA */}
            <div
              className="grid h-10 w-10 place-items-center rounded-2xl text-zinc-400 transition-all duration-300 group-hover:scale-110 group-hover:text-white"
              style={{ background: 'rgba(255,255,255,0.4)' }}
            >
              <ArrowRight
                className="h-5 w-5 transition-transform duration-300 group-hover:translate-x-0.5"
                style={{ color: 'inherit' }}
              />
            </div>
          </div>
        </div>
      </div>
    </Link>
  );
}
