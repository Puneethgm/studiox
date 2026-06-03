import { Settings as SettingsIcon } from 'lucide-react';
import { CredentialsManager } from './CredentialsManager';

export const metadata = {
  title: 'Platform Settings | StudioX',
};

export default function SettingsPage() {
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
              <SettingsIcon className="h-6 w-6" />
            </div>
            <div>
              <h1 className="text-2xl font-black tracking-tight text-zinc-900 dark:text-white">Platform Settings</h1>
              <p className="mt-1 text-xs text-zinc-500 dark:text-zinc-400">
                Configure integrations, API credentials, and other system-wide configurations.
              </p>
            </div>
          </div>
        </div>
      </div>

      {/* ── Settings content ─────────────────── */}
      <div className="space-y-6">
        <CredentialsManager />

        {/* ── Plans Configuration Placeholder ────── */}
        <div className="rounded-3xl border border-white/20 bg-white/10 p-6 backdrop-blur-xl dark:border-white/5 dark:bg-neutral-900/30">
          <div className="flex items-center gap-3 mb-4">
            <SettingsIcon className="h-5 w-5 text-brand-500" />
            <h2 className="text-lg font-bold text-zinc-900 dark:text-white">Plans Configuration</h2>
          </div>
          <p className="text-sm text-zinc-500 dark:text-zinc-400 mb-6">
            Configure dynamic feature limits (e.g., AI max replies, Channel caps) per subscription tier. Note: Values map to the platform_settings database.
          </p>
          <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
            {['Trial', 'Growth', 'Pro', 'Enterprise'].map((tier) => (
              <div key={tier} className="rounded-xl border border-white/10 bg-white/5 p-4 dark:bg-neutral-800/50">
                <h3 className="font-bold text-zinc-800 dark:text-zinc-200">{tier} Plan</h3>
                <div className="mt-3 space-y-2 text-xs">
                  <div className="flex justify-between">
                    <span className="text-zinc-500">Max Channels:</span>
                    <span className="font-semibold text-zinc-900 dark:text-zinc-100">
                      {tier === 'Enterprise' ? 'Unlimited' : tier === 'Pro' ? 8 : tier === 'Growth' ? 3 : 1}
                    </span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-zinc-500">AI Limit:</span>
                    <span className="font-semibold text-zinc-900 dark:text-zinc-100">
                      {tier === 'Enterprise' ? 'Unlimited' : tier === 'Pro' ? '10k' : tier === 'Growth' ? '2k' : '200'}
                    </span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-zinc-500">Social Planner:</span>
                    <span className="font-semibold text-zinc-900 dark:text-zinc-100">
                      {tier === 'Enterprise' || tier === 'Pro' ? 'Enabled' : 'Disabled'}
                    </span>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
