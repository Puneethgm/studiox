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
import { Pagination } from '@/components/ui/Pagination';
import { StudioFilters } from './StudioFilters';

interface ListResp {
  studios: Studio[];
}

interface SearchParams {
  page?: string;
  search?: string;
  status?: string;
}

export default async function StudiosListPage({
  searchParams,
}: {
  searchParams: Promise<SearchParams>;
}) {
  const sp = await searchParams;
  const page = Math.max(1, Number(sp.page) || 1);
  const PAGE_SIZE = 9;
  const offset = (page - 1) * PAGE_SIZE;

  const { studios } = await serverFetch<ListResp>('/api/v1/admin/studios');

  // Filter studios
  let filtered = studios;
  if (sp.search) {
    const sLower = sp.search.toLowerCase();
    filtered = filtered.filter(s =>
      s.name.toLowerCase().includes(sLower) ||
      s.slug.toLowerCase().includes(sLower) ||
      (s.contactEmail && s.contactEmail.toLowerCase().includes(sLower))
    );
  }
  if (sp.status) {
    const active = sp.status === 'active';
    filtered = filtered.filter(s => s.active === active);
  }

  const paginatedStudios = filtered.slice(offset, offset + PAGE_SIZE);

  const totalCampaigns = studios.reduce((sum, s) => sum + (s.campaignCount ?? 0), 0);
  const totalLeads     = studios.reduce((sum, s) => sum + (s.leadCount ?? 0), 0);
  const activeCount    = studios.filter((s) => s.active).length;
  const inactiveCount  = studios.length - activeCount;

  return (
    <div className="space-y-8 pb-12">

      {/* ── Page header ───────────────────────── */}
      <div
        className="relative overflow-hidden rounded-[26px] border border-white/30 p-6 backdrop-blur-2xl dark:border-white/5 bg-white/30 dark:bg-neutral-900/30"
        style={{
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

          {/* ── Search & Filter Controls ──────── */}
          <StudioFilters search={sp.search} status={sp.status} />

          {/* ── Studio cards grid ─────────────── */}
          {filtered.length === 0 ? (
            <div
              className="overflow-hidden rounded-[24px] border border-white/30 bg-white/30 backdrop-blur-2xl dark:border-white/5 dark:bg-neutral-900/30 p-8"
              style={{ boxShadow: 'inset 0 0 0 1px rgba(255,255,255,0.15)' }}
            >
              <EmptyState
                icon={<Building2 className="h-8 w-8 text-zinc-400" />}
                title="No studios match your search"
                description="Try clearing your search query or status filter to see all studios."
              />
            </div>
          ) : (
            <>
              <div className="grid gap-5 md:grid-cols-2 xl:grid-cols-3">
                {paginatedStudios.map((s, idx) => (
                  <StudioCard key={s.id} studio={s} idx={idx} />
                ))}
              </div>

              {/* Pagination */}
              <Pagination total={filtered.length} pageSize={PAGE_SIZE} page={page} />
            </>
          )}
        </>
      )}
    </div>
  );
}

// ─────────────────────────────────────────────────────
// Summary stat card
// ─────────────────────────────────────────────────────

type CardColor = 'violet' | 'emerald' | 'sky' | 'amber';

const colorClasses: Record<CardColor, {
  cardBg: string;
  cardBorder: string;
  labelText: string;
  valueText: string;
  hintText: string;
  iconBg: string;
  iconText: string;
  glowBg: string;
}> = {
  violet: {
    cardBg: 'bg-violet-50/45 dark:bg-violet-950/10',
    cardBorder: 'border-violet-200/40 dark:border-violet-900/20',
    labelText: 'text-violet-700 dark:text-violet-400',
    valueText: 'text-violet-950 dark:text-violet-100',
    hintText: 'text-violet-600 dark:text-violet-400/80',
    iconBg: 'bg-violet-500/10 dark:bg-violet-400/10',
    iconText: 'text-violet-700 dark:text-violet-400',
    glowBg: 'bg-violet-500/10 dark:bg-violet-400/10',
  },
  emerald: {
    cardBg: 'bg-emerald-50/45 dark:bg-emerald-950/10',
    cardBorder: 'border-emerald-200/40 dark:border-emerald-900/20',
    labelText: 'text-emerald-700 dark:text-emerald-400',
    valueText: 'text-emerald-950 dark:text-emerald-100',
    hintText: 'text-emerald-600 dark:text-emerald-400/80',
    iconBg: 'bg-emerald-500/10 dark:bg-emerald-400/10',
    iconText: 'text-emerald-700 dark:text-emerald-400',
    glowBg: 'bg-emerald-500/10 dark:bg-emerald-400/10',
  },
  sky: {
    cardBg: 'bg-sky-50/45 dark:bg-sky-950/10',
    cardBorder: 'border-sky-200/40 dark:border-sky-900/20',
    labelText: 'text-sky-700 dark:text-sky-400',
    valueText: 'text-sky-950 dark:text-sky-100',
    hintText: 'text-sky-600 dark:text-sky-400/80',
    iconBg: 'bg-sky-500/10 dark:bg-sky-400/10',
    iconText: 'text-sky-700 dark:text-sky-400',
    glowBg: 'bg-sky-500/10 dark:bg-sky-400/10',
  },
  amber: {
    cardBg: 'bg-amber-50/45 dark:bg-amber-950/10',
    cardBorder: 'border-amber-200/40 dark:border-amber-900/20',
    labelText: 'text-amber-700 dark:text-amber-400',
    valueText: 'text-amber-950 dark:text-amber-100',
    hintText: 'text-amber-600 dark:text-amber-400/80',
    iconBg: 'bg-amber-500/10 dark:bg-amber-400/10',
    iconText: 'text-amber-700 dark:text-amber-400',
    glowBg: 'bg-amber-500/10 dark:bg-amber-400/10',
  },
};

function SummaryCard({
  label, value, icon, color, hint,
}: {
  label: string; value: number; icon: React.ReactNode; color: CardColor; hint?: string;
}) {
  const c = colorClasses[color];
  return (
    <div
      className={`relative overflow-hidden rounded-[20px] p-5 backdrop-blur-2xl border ${c.cardBg} ${c.cardBorder}`}
    >
      {/* Glow blob */}
      <div
        className={`pointer-events-none absolute -right-6 -top-6 h-20 w-20 rounded-full blur-2xl ${c.glowBg}`}
      />
      <div className="relative flex items-start justify-between gap-3">
        <div>
          <div className={`text-[10px] font-black uppercase tracking-[0.18em] ${c.labelText}`}>
            {label}
          </div>
          <div className={`mt-2 text-3xl font-black tracking-tight ${c.valueText}`}>
            {value}
          </div>
          {hint && (
            <div className={`mt-1 text-[11px] font-bold ${c.hintText}`}>
              {hint}
            </div>
          )}
        </div>
        <div
          className={`grid h-10 w-10 shrink-0 place-items-center rounded-xl ${c.iconBg} ${c.iconText}`}
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
        className="relative h-full overflow-hidden rounded-[24px] backdrop-blur-2xl transition-all duration-500 hover:-translate-y-2 hover:scale-[1.013] border border-white/30 bg-white/30 dark:border-white/5 dark:bg-neutral-900/30"
        style={{
          boxShadow: 'inset 0 0 0 1px rgba(255,255,255,0.15), 0 8px 32px rgba(0,0,0,0.07)',
        }}
      >
        {/* Full-width colored header strip */}
        <div
          className="relative h-24 w-full overflow-hidden"
          style={{ background: `linear-gradient(135deg, ${s.brandColor} 0%, ${s.brandColor}cc 60%, ${s.brandColor}88 100%)` }}
        >
          {/* Abstract pattern overlay */}
          <div className="absolute inset-0 opacity-20"
            style={{ backgroundImage: `radial-gradient(circle at 80% 20%, white 0%, transparent 50%), radial-gradient(circle at 20% 80%, white 0%, transparent 40%)` }}
          />
          {/* Status badge */}
          <div className="absolute right-4 top-4">
            <Badge
              tone={s.active ? 'success' : 'neutral'}
              className="text-[10px] font-black uppercase tracking-wider shadow-sm"
            >
              {s.active ? '● Active' : '○ Inactive'}
            </Badge>
          </div>
        </div>

        {/* Avatar overlapping the header/body boundary */}
        <div className="relative px-5 pb-5">
          <div className="-mt-7 mb-3 flex items-end justify-between">
            <div
              className="relative grid h-14 w-14 shrink-0 place-items-center overflow-hidden rounded-[16px] text-lg font-black text-white shadow-lg ring-4 ring-white transition-transform duration-500 group-hover:scale-110 group-hover:rotate-3 dark:ring-neutral-900"
              style={{ background: s.brandColor }}
            >
              {s.logoUrl ? (
                // eslint-disable-next-line @next/next/no-img-element
                <img src={s.logoUrl} alt="" className="h-full w-full object-cover" />
              ) : (
                brandInitials(s.name)
              )}
              <div className="absolute inset-0 bg-gradient-to-br from-white/25 to-transparent" />
            </div>

            {/* Arrow CTA */}
            <div
              className="grid h-9 w-9 place-items-center rounded-2xl transition-all duration-300 group-hover:scale-110"
              style={{ background: `${s.brandColor}18`, color: s.brandColor }}
            >
              <ArrowRight className="h-4 w-4 transition-transform duration-300 group-hover:translate-x-0.5" />
            </div>
          </div>

          {/* Name + slug */}
          <div className="min-w-0">
            <h3 className="truncate text-[15px] font-black text-zinc-900 transition-colors group-hover:text-brand-600 dark:text-white dark:group-hover:text-brand-400">
              {s.name}
            </h3>
            <div className="mt-0.5 flex items-center gap-1 font-mono text-[10px] font-bold uppercase tracking-wider text-zinc-400">
              <Building2 className="h-3 w-3" />
              /{s.slug}
            </div>
            <p className="mt-1 text-[11px] font-medium leading-relaxed text-zinc-500 dark:text-zinc-400 truncate">
              {s.contactEmail ?? 'No contact email set'}
            </p>
          </div>

          {/* Divider */}
          <div className="my-4 h-px bg-gradient-to-r from-transparent via-white/40 to-transparent dark:via-white/10" />

          {/* Stats row */}
          <div className="flex items-center gap-4">
            <div className="flex items-center gap-2 flex-1">
              <div className="grid h-8 w-8 place-items-center rounded-xl bg-sky-50/80 text-sky-600 dark:bg-sky-500/10 dark:text-sky-400">
                <Megaphone className="h-3.5 w-3.5" />
              </div>
              <div>
                <div className="text-sm font-black text-zinc-900 dark:text-white leading-none">{s.campaignCount ?? 0}</div>
                <div className="text-[9px] font-black uppercase tracking-widest text-zinc-400 mt-0.5">Campaigns</div>
              </div>
            </div>
            <div className="h-8 w-px bg-white/30 dark:bg-white/10" />
            <div className="flex items-center gap-2 flex-1">
              <div className="grid h-8 w-8 place-items-center rounded-xl bg-violet-50/80 text-violet-600 dark:bg-violet-500/10 dark:text-violet-400">
                <Users className="h-3.5 w-3.5" />
              </div>
              <div>
                <div className="text-sm font-black text-zinc-900 dark:text-white leading-none">{s.leadCount ?? 0}</div>
                <div className="text-[9px] font-black uppercase tracking-widest text-zinc-400 mt-0.5">Leads</div>
              </div>
            </div>
            <div className="h-8 w-px bg-white/30 dark:bg-white/10" />
            <div className="flex items-center gap-2 flex-1">
              <div className="grid h-8 w-8 place-items-center rounded-xl bg-emerald-50/80 text-emerald-600 dark:bg-emerald-500/10 dark:text-emerald-400">
                <Activity className="h-3.5 w-3.5" />
              </div>
              <div>
                <div className="text-[9px] font-black uppercase tracking-widest text-zinc-400 mt-0.5">Status</div>
                <div className="text-[10px] font-black mt-0.5" style={{ color: s.active ? '#10b981' : '#94a3b8' }}>
                  {s.active ? 'Live' : 'Paused'}
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Ambient glow on hover */}
        <div
          className="pointer-events-none absolute -right-10 -top-10 h-40 w-40 rounded-full blur-3xl opacity-0 transition-opacity duration-700 group-hover:opacity-20"
          style={{ background: s.brandColor }}
        />
      </div>
    </Link>
  );
}
