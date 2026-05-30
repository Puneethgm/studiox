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
      </div>
    </div>
  );
}
