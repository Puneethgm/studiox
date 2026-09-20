'use client';

import { useEffect, useState } from 'react';
import {
  Settings as SettingsIcon,
  Building,
  DollarSign,
  Calendar,
  Eye,
  Database,
  Cpu,
  Bot,
  Lock,
  Loader2,
} from 'lucide-react';
import { api } from '@/lib/api';
import type { Studio, Campaign, Plan } from '@/lib/types';
import { Dialog, DialogHeader } from '@/components/ui/Dialog';
import { SettingsForm } from './SettingsForm';
import { PlansRows } from './PlansRows';
import { AvailabilityRows } from './AvailabilityRows';
import { AIProviderSettings } from './AIProviderSettings';
import { GeneralSection, BookingSection, SecuritySection, IntegrationsSection, SheetsSection } from './SettingsSections';

type Section = 'general' | 'plans' | 'availability' | 'booking' | 'sheets' | 'integrations' | 'ai-assistant' | 'security' | 'billing';

const NAV: { id: Section; label: string; description: string; icon: React.ComponentType<{ className?: string }> }[] = [
  { id: 'general', label: 'General', description: 'Name, slug, and brand identity', icon: Building },
  { id: 'plans', label: 'Plans', description: 'Membership plans and pricing', icon: DollarSign },
  { id: 'availability', label: 'Availability', description: 'Weekly hours and timezone', icon: Calendar },
  { id: 'booking', label: 'Trial Page', description: 'Customize and preview checkout', icon: Eye },
  { id: 'sheets', label: 'Google Sheets', description: 'Lead sync and imports', icon: Database },
  { id: 'integrations', label: 'Integrations', description: 'Meta, Google Ads, pacing', icon: Cpu },
  { id: 'ai-assistant', label: 'AI Assistant', description: 'LLM provider and models', icon: Bot },
  { id: 'security', label: 'Security', description: 'Password and account', icon: Lock },
  { id: 'billing', label: 'Platform Billing', description: 'Subscription and invoices', icon: DollarSign },
];

// The full settings surface as a popup instead of a page — openable from
// anywhere via AppShell's "Settings" nav item. General/Booking/Security get
// a dedicated click-to-edit row UI (SettingsSections.tsx); Plans/Availability
// /AI Assistant reuse their own already-standalone components; the
// remaining, genuinely form-shaped tabs (Sheets, Integrations, Billing) fall
// back to the classic SettingsForm content for just that one tab (hideNav)
// rather than being force-fit into single-field rows.
export function SettingsModal({ studioId, open, onClose }: { studioId: string; open: boolean; onClose: () => void }) {
  const [section, setSection] = useState<Section>('general');
  // Each tab's content is a separate component that fetches its own data on
  // mount (models, sheets config, timing, etc.). Switching tabs used to
  // conditionally render only the active one, which unmounted the rest —
  // so every time you went back to a tab you'd already visited, it
  // remounted from scratch and re-fetched over the network, making tab
  // switches feel slow. Now a tab is mounted once on first visit and then
  // just hidden (not unmounted) when you switch away, so revisiting it is
  // instant and doesn't re-fetch.
  const [visited, setVisited] = useState<Set<Section>>(() => new Set(['general']));
  useEffect(() => {
    setVisited((prev) => (prev.has(section) ? prev : new Set(prev).add(section)));
  }, [section]);

  const [studio, setStudio] = useState<Studio | null>(null);
  const [previewHref, setPreviewHref] = useState<string | null>(null);
  const [plans, setPlans] = useState<Plan[]>([]);
  const [loadError, setLoadError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    setStudio(null);
    setLoadError(null);
    setSection('general');
    setVisited(new Set(['general']));

    (async () => {
      try {
        const [studioRes, campaignsRes, plansRes] = await Promise.all([
          api<Studio>(`/api/v1/me/studios/${studioId}`),
          api<{ campaigns: Campaign[] }>(`/api/v1/studios/${studioId}/campaigns`).catch(() => ({ campaigns: [] })),
          api<{ plans: Plan[] }>(`/api/v1/me/studios/${studioId}/plans`).catch(() => ({ plans: [] })),
        ]);
        if (cancelled) return;
        const previewCampaign = campaignsRes.campaigns.find((c) => c.active) ?? campaignsRes.campaigns[0] ?? null;
        setStudio(studioRes);
        setPreviewHref(previewCampaign ? `/l/${studioRes.slug}/${previewCampaign.slug}` : null);
        setPlans(plansRes.plans || []);
      } catch (err: any) {
        if (!cancelled) setLoadError(err.message || 'Failed to load studio');
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [open, studioId]);

  return (
    <Dialog open={open} onClose={onClose} widthClassName="max-w-6xl">
      <DialogHeader
        icon={<SettingsIcon className="h-4 w-4 text-zinc-400" />}
        title={studio ? `${studio.name} — Settings` : 'Settings'}
        onClose={onClose}
      />
      <div className="flex min-h-0 flex-1">
        <div className="w-64 shrink-0 overflow-y-auto border-r border-zinc-200 p-2 dark:border-zinc-800">
          <div className="px-2.5 py-2 text-[10px] font-bold uppercase tracking-wider text-zinc-400">Studio Settings</div>
          {NAV.map((item) => {
            const Icon = item.icon;
            const active = section === item.id;
            return (
              <button
                key={item.id}
                type="button"
                onClick={() => setSection(item.id)}
                className={`mb-0.5 flex w-full items-start gap-2.5 rounded-lg px-2.5 py-2 text-left transition-colors ${
                  active ? 'bg-[var(--brand,#7c3aed)]/10' : 'hover:bg-zinc-100 dark:hover:bg-zinc-900'
                }`}
              >
                <Icon className={`mt-0.5 h-4 w-4 shrink-0 ${active ? 'text-[var(--brand,#7c3aed)]' : 'text-zinc-400'}`} />
                <span className="min-w-0">
                  <span className={`block text-xs font-bold ${active ? 'text-[var(--brand,#7c3aed)]' : 'text-zinc-700 dark:text-zinc-200'}`}>
                    {item.label}
                  </span>
                  <span className="block truncate text-[10px] text-zinc-400">{item.description}</span>
                </span>
              </button>
            );
          })}
        </div>

        <div className="min-w-0 flex-1 overflow-y-auto p-6">
          {loadError && <p className="text-sm font-medium text-red-500">{loadError}</p>}
          {!loadError && !studio && (
            <div className="flex items-center gap-2 text-sm text-zinc-400">
              <Loader2 className="h-4 w-4 animate-spin" />
              Loading…
            </div>
          )}
          {studio && (
            <>
              {visited.has('general') && (
                <div hidden={section !== 'general'}>
                  <GeneralSection studio={studio} onChange={setStudio} />
                </div>
              )}
              {visited.has('booking') && (
                <div hidden={section !== 'booking'}>
                  <BookingSection studio={studio} />
                </div>
              )}
              {visited.has('security') && (
                <div hidden={section !== 'security'}>
                  <SecuritySection studio={studio} />
                </div>
              )}
              {visited.has('plans') && (
                <div hidden={section !== 'plans'}>
                  <PlansRows studioId={studio.id} initialPlans={plans} />
                </div>
              )}
              {visited.has('availability') && (
                <div hidden={section !== 'availability'}>
                  <AvailabilityRows studio={studio} onSaveSuccess={() => {}} />
                </div>
              )}
              {visited.has('ai-assistant') && (
                <div hidden={section !== 'ai-assistant'}>
                  <AIProviderSettings studioId={studio.id} />
                </div>
              )}
              {visited.has('integrations') && (
                <div hidden={section !== 'integrations'}>
                  <IntegrationsSection studio={studio} onChange={(patch) => setStudio({ ...studio, ...patch })} />
                </div>
              )}
              {visited.has('sheets') && (
                <div hidden={section !== 'sheets'}>
                  <SheetsSection studio={studio} />
                </div>
              )}
              {visited.has('billing') && (
                <div hidden={section !== 'billing'}>
                  <SettingsForm studio={studio} previewHref={previewHref} initialPlans={plans} forcedSection="billing" hideNav />
                </div>
              )}
            </>
          )}
        </div>
      </div>
    </Dialog>
  );
}
