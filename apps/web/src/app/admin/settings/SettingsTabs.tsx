'use client';

import { useState, useEffect } from 'react';
import { Settings as SettingsIcon, Plug, CheckCircle2 } from 'lucide-react';
import { CredentialsManager } from './CredentialsManager';
import { Button } from '@/components/ui/Button';

export function SettingsTabs() {
  const [activeTab, setActiveTab] = useState<'credentials' | 'plans'>('credentials');
  const [saving, setSaving] = useState(false);
  
  const [plans, setPlans] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetch('/api/v1/public/platform/plans')
      .then(res => res.json())
      .then(data => {
        if (Array.isArray(data)) {
          setPlans(data);
        }
      })
      .catch(err => console.error('Failed to load plans:', err))
      .finally(() => setLoading(false));
  }, []);

  const handlePlanChange = (index: number, field: string, value: any) => {
    const updated = [...plans];
    updated[index] = { ...updated[index], [field]: value };
    setPlans(updated);
  };

  const handleSavePlans = async () => {
    setSaving(true);
    try {
      const res = await fetch('/api/v1/me/studios/global/plans', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(plans)
      });
      if (!res.ok) throw new Error('Failed to save plans');
      alert('Platform plans successfully updated.');
    } catch (e) {
      alert('Error saving plans');
    } finally {
      setSaving(false);
    }
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
            
            <div className="grid gap-4 md:grid-cols-2">
              {plans.map((plan, idx) => {
                const accentHex = ['#a1a1aa', '#7c3aed', '#8b5cf6', '#10b981'][idx % 4];
                return (
                  <div
                    key={plan.name || idx}
                    style={{ borderTopColor: accentHex }}
                    className="flex flex-col rounded border border-zinc-200 border-t-4 bg-white dark:bg-zinc-900 dark:border-zinc-700 hover:border-brand-400 hover:shadow-md hover:shadow-brand-500/10 transition-all duration-200 overflow-hidden"
                  >
                    {/* Card Header */}
                    <div className="flex items-center justify-between bg-zinc-50 dark:bg-zinc-800/60 px-5 py-3 border-b border-zinc-200 dark:border-zinc-700">
                      <h3 className="text-sm font-black uppercase tracking-widest text-zinc-800 dark:text-zinc-100 flex items-center gap-2">
                        {plan.name}
                        {plan.highlight && <CheckCircle2 className="h-4 w-4 text-brand-500" />}
                      </h3>
                      <span
                        className="h-2 w-2 rounded-full"
                        style={{ backgroundColor: accentHex }}
                      />
                    </div>

                    {/* Card Fields */}
                    <div className="space-y-4 p-5">
                      <div className="grid grid-cols-2 gap-4">
                        <div>
                          <label className="mb-1.5 block text-[10px] font-bold uppercase tracking-wider text-zinc-500">Plan Name</label>
                          <input
                            type="text"
                            className="w-full rounded border border-zinc-200 dark:border-zinc-700 bg-white dark:bg-zinc-900 px-3 py-2 text-sm font-semibold text-zinc-900 dark:text-white focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
                            value={plan.name}
                            onChange={(e) => handlePlanChange(idx, 'name', e.target.value)}
                          />
                        </div>
                        <div>
                          <label className="mb-1.5 block text-[10px] font-bold uppercase tracking-wider text-zinc-500">Price (USD)</label>
                          <input
                            type="number"
                            className="w-full rounded border border-zinc-200 dark:border-zinc-700 bg-white dark:bg-zinc-900 px-3 py-2 text-sm font-semibold text-zinc-900 dark:text-white focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
                            value={plan.price}
                            onChange={(e) => handlePlanChange(idx, 'price', parseInt(e.target.value))}
                          />
                        </div>
                      </div>
                      <div>
                        <label className="mb-1.5 block text-[10px] font-bold uppercase tracking-wider text-zinc-500">Description</label>
                        <input
                          type="text"
                          className="w-full rounded border border-zinc-200 dark:border-zinc-700 bg-white dark:bg-zinc-900 px-3 py-2 text-sm font-semibold text-zinc-900 dark:text-white focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
                          value={plan.description}
                          onChange={(e) => handlePlanChange(idx, 'description', e.target.value)}
                        />
                      </div>
                      <div>
                        <label className="mb-1.5 block text-[10px] font-bold uppercase tracking-wider text-zinc-500">Features (Comma Separated)</label>
                        <textarea
                          className="w-full rounded border border-zinc-200 dark:border-zinc-700 bg-white dark:bg-zinc-900 px-3 py-2 text-sm font-semibold text-zinc-900 dark:text-white focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500"
                          rows={3}
                          value={plan.features ? plan.features.join(', ') : ''}
                          onChange={(e) => handlePlanChange(idx, 'features', e.target.value.split(',').map((s: string) => s.trim()))}
                        />
                      </div>
                      <label className="flex cursor-pointer items-center gap-3 rounded border border-zinc-100 dark:border-zinc-700 bg-zinc-50 dark:bg-zinc-800/50 p-3 hover:bg-zinc-100 dark:hover:bg-zinc-800 transition-colors">
                        <input
                          type="checkbox"
                          className="h-4 w-4 rounded border-zinc-300 text-brand-600 focus:ring-brand-500 cursor-pointer"
                          checked={plan.highlight || false}
                          onChange={(e) => handlePlanChange(idx, 'highlight', e.target.checked)}
                        />
                        <span className="text-xs font-bold text-zinc-700 dark:text-zinc-200">Highlight Plan (e.g. Most Popular)</span>
                      </label>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
