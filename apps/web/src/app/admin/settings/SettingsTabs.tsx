'use client';

import { useState } from 'react';
import { Settings as SettingsIcon, Plug, CheckCircle2 } from 'lucide-react';
import { CredentialsManager } from './CredentialsManager';
import { Button } from '@/components/ui/Button';

export function SettingsTabs() {
  const [activeTab, setActiveTab] = useState<'credentials' | 'plans'>('credentials');
  const [saving, setSaving] = useState(false);
  
  // Local state to simulate editing plans
  const [plans, setPlans] = useState([
    { id: 'trial', name: 'Trial Plan', channels: 1, aiLimit: 200, socialPlanner: false },
    { id: 'growth', name: 'Growth Plan', channels: 3, aiLimit: 2000, socialPlanner: false },
    { id: 'pro', name: 'Pro Plan', channels: 8, aiLimit: 10000, socialPlanner: true },
    { id: 'enterprise', name: 'Enterprise Plan', channels: 999, aiLimit: 999999, socialPlanner: true },
  ]);

  const handlePlanChange = (index: number, field: string, value: any) => {
    const updated = [...plans];
    updated[index] = { ...updated[index], [field]: value };
    setPlans(updated);
  };

  const handleSavePlans = () => {
    setSaving(true);
    setTimeout(() => {
      setSaving(false);
      alert('Platform plans successfully updated.');
    }, 800);
  };

  return (
    <div className="space-y-6">
      {/* ── Tabs List ──────────────────────────── */}
      <div className="flex space-x-2 border-b border-slate-200 dark:border-slate-800 pb-px">
        <button
          onClick={() => setActiveTab('credentials')}
          className={`flex items-center gap-2 border-b-2 px-4 py-3 text-sm font-bold transition-colors ${
            activeTab === 'credentials'
              ? 'border-brand-500 text-brand-600 dark:text-brand-400'
              : 'border-transparent text-slate-500 hover:border-slate-300 hover:text-slate-700 dark:text-slate-400 dark:hover:border-slate-700 dark:hover:text-slate-300'
          }`}
        >
          <Plug className="h-4 w-4" />
          API Integrations
        </button>
        <button
          onClick={() => setActiveTab('plans')}
          className={`flex items-center gap-2 border-b-2 px-4 py-3 text-sm font-bold transition-colors ${
            activeTab === 'plans'
              ? 'border-brand-500 text-brand-600 dark:text-brand-400'
              : 'border-transparent text-slate-500 hover:border-slate-300 hover:text-slate-700 dark:text-slate-400 dark:hover:border-slate-700 dark:hover:text-slate-300'
          }`}
        >
          <SettingsIcon className="h-4 w-4" />
          Plans Configuration
        </button>
      </div>

      {/* ── Tabs Content ───────────────────────── */}
      <div className="animate-in fade-in slide-in-from-bottom-2 duration-300">
        {activeTab === 'credentials' && <CredentialsManager />}

        {activeTab === 'plans' && (
          <div className="rounded-3xl border border-white/20 bg-white/10 p-6 backdrop-blur-xl dark:border-white/5 dark:bg-neutral-900/30">
            <div className="flex items-center justify-between mb-6">
              <div>
                <h2 className="text-lg font-bold text-zinc-900 dark:text-white">Platform Plan Limits</h2>
                <p className="mt-1 text-xs text-zinc-500 dark:text-zinc-400">
                  Update feature limits dynamically for all studios on these subscription tiers.
                </p>
              </div>
              <Button onClick={handleSavePlans} loading={saving}>
                Save Changes
              </Button>
            </div>
            
            <div className="grid gap-6 md:grid-cols-2">
              {plans.map((plan, idx) => (
                <div key={plan.id} className="rounded-2xl border border-white/20 bg-white/40 p-5 shadow-sm dark:border-white/5 dark:bg-neutral-800/40">
                  <h3 className="mb-4 text-base font-black text-zinc-900 dark:text-white flex items-center justify-between">
                    {plan.name}
                    {plan.socialPlanner && <CheckCircle2 className="h-4 w-4 text-brand-500" />}
                  </h3>
                  
                  <div className="space-y-4">
                    <div>
                      <label className="mb-1.5 block text-[10px] font-bold uppercase tracking-wider text-zinc-500">
                        Max Allowed Channels
                      </label>
                      <input
                        type="number"
                        className="w-full rounded-xl border border-white/40 bg-white/50 px-3 py-2 text-sm font-semibold text-zinc-900 shadow-sm focus:border-brand-500 focus:outline-none dark:border-white/10 dark:bg-neutral-900/50 dark:text-white"
                        value={plan.channels}
                        onChange={(e) => handlePlanChange(idx, 'channels', parseInt(e.target.value))}
                      />
                    </div>
                    <div>
                      <label className="mb-1.5 block text-[10px] font-bold uppercase tracking-wider text-zinc-500">
                        Max AI Replies (Monthly)
                      </label>
                      <input
                        type="number"
                        className="w-full rounded-xl border border-white/40 bg-white/50 px-3 py-2 text-sm font-semibold text-zinc-900 shadow-sm focus:border-brand-500 focus:outline-none dark:border-white/10 dark:bg-neutral-900/50 dark:text-white"
                        value={plan.aiLimit}
                        onChange={(e) => handlePlanChange(idx, 'aiLimit', parseInt(e.target.value))}
                      />
                    </div>
                    <label className="flex cursor-pointer items-center gap-3 rounded-xl border border-white/40 bg-white/30 p-3 hover:bg-white/50 dark:border-white/10 dark:bg-neutral-800/30 dark:hover:bg-neutral-800/50">
                      <input
                        type="checkbox"
                        className="h-4 w-4 rounded border-slate-300 text-brand-600 focus:ring-brand-500 dark:border-slate-600"
                        checked={plan.socialPlanner}
                        onChange={(e) => handlePlanChange(idx, 'socialPlanner', e.target.checked)}
                      />
                      <span className="text-xs font-bold text-zinc-700 dark:text-zinc-200">
                        Enable Social Planner Module
                      </span>
                    </label>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
