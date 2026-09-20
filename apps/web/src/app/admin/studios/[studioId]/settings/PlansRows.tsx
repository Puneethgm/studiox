'use client';

import { useState } from 'react';
import { Plus, Trash2, Check, X, Pencil, Loader2, CheckCircle2 } from 'lucide-react';
import type { Plan } from '@/lib/types';
import { api, ApiError } from '@/lib/api';
import { Button } from '@/components/ui/Button';
import { Label } from '@/components/ui/Label';
import { Input } from '@/components/ui/Input';

function bestApiError(e: unknown): string {
  if (e instanceof ApiError && e.details) {
    const first = Object.entries(e.details)[0];
    if (first) return `${first[0]}: ${first[1]}`;
  }
  return e instanceof Error ? e.message : 'Failed to save';
}

const BILLING_CYCLES = [
  { value: 'monthly', label: 'Monthly' },
  { value: 'yearly', label: 'Yearly' },
  { value: 'one_time', label: 'One-time' },
];

// Same accent-per-card convention as the Platform Billing pricing grid.
const ACCENTS = ['#a1a1aa', '#7c3aed', '#8b5cf6', '#10b981'];

const emptyNew = () => ({ planName: '', priceSgd: '', billingCycle: 'monthly', features: '', isActive: true });

// Plan cards styled to match the Platform Billing pricing grid (colored top
// accent, big price, checkmark feature list) but editable in place — click
// the pencil on price or features, click the Active badge to toggle it.
export function PlansRows({ studioId, initialPlans }: { studioId: string; initialPlans: Plan[] }) {
  const [plans, setPlans] = useState<Plan[]>(initialPlans);
  const [showAdd, setShowAdd] = useState(false);
  const [newPlan, setNewPlan] = useState(emptyNew());
  const [addLoading, setAddLoading] = useState(false);
  const [addError, setAddError] = useState<string | null>(null);

  async function addPlan() {
    if (!newPlan.planName.trim()) {
      setAddError('Plan name is required');
      return;
    }
    setAddLoading(true);
    setAddError(null);
    try {
      const priceSgd = Math.round(parseFloat(newPlan.priceSgd || '0') * 100);
      const features = newPlan.features.split('\n').map((f) => f.trim()).filter(Boolean);
      const res = await api<{ plan: Plan }>(`/api/v1/me/studios/${studioId}/plans`, {
        method: 'POST',
        json: { planName: newPlan.planName.trim(), priceSgd, billingCycle: newPlan.billingCycle, features, isActive: newPlan.isActive },
      });
      setPlans((prev) => [...prev, res.plan]);
      setShowAdd(false);
      setNewPlan(emptyNew());
    } catch (e) {
      setAddError(bestApiError(e));
    } finally {
      setAddLoading(false);
    }
  }

  async function patchPlan(plan: Plan, patch: Record<string, unknown>) {
    try {
      await api(`/api/v1/me/studios/${studioId}/plans/${plan.id}`, { method: 'PUT', json: patch });
      setPlans((prev) => prev.map((p) => (p.id === plan.id ? ({ ...p, ...patch } as Plan) : p)));
      return { ok: true };
    } catch (e) {
      return { ok: false, error: bestApiError(e) };
    }
  }

  async function deletePlan(plan: Plan) {
    if (!confirm(`Delete "${plan.planName}" plan? This cannot be undone.`)) return;
    try {
      await api(`/api/v1/me/studios/${studioId}/plans/${plan.id}`, { method: 'DELETE' });
      setPlans((prev) => prev.filter((p) => p.id !== plan.id));
    } catch (e) {
      alert(bestApiError(e));
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h3 className="text-base font-bold text-zinc-900 dark:text-white">Plans</h3>
          <p className="text-sm text-zinc-400">Membership plans available via the WhatsApp bot.</p>
        </div>
        <Button
          variant={showAdd ? 'secondary' : 'primary'}
          className={showAdd ? 'shrink-0 rounded-xl' : 'shrink-0 rounded-xl bg-[var(--brand,#7c3aed)] shadow-sm hover:brightness-110'}
          leftIcon={showAdd ? <X className="h-3.5 w-3.5" /> : <Plus className="h-3.5 w-3.5" />}
          onClick={() => setShowAdd((v) => !v)}
        >
          {showAdd ? 'Close' : 'Add Plan'}
        </Button>
      </div>

      {showAdd && (
        <div className="space-y-4 rounded-xl border border-zinc-200 bg-white p-5 dark:border-zinc-800 dark:bg-zinc-950">
          <h4 className="text-sm font-bold text-zinc-900 dark:text-white">New Plan</h4>
          <div className="grid gap-4 sm:grid-cols-2">
            <div>
              <Label className="text-xs">Plan Name</Label>
              <Input value={newPlan.planName} onChange={(e) => setNewPlan({ ...newPlan, planName: e.target.value })} placeholder="e.g. Premium" />
            </div>
            <div>
              <Label className="text-xs">Price (S$)</Label>
              <Input type="number" value={newPlan.priceSgd} onChange={(e) => setNewPlan({ ...newPlan, priceSgd: e.target.value })} placeholder="0.00" />
            </div>
            <div>
              <Label className="text-xs">Billing Cycle</Label>
              <select
                value={newPlan.billingCycle}
                onChange={(e) => setNewPlan({ ...newPlan, billingCycle: e.target.value })}
                className="mt-1 h-10 w-full rounded-xl border border-zinc-200 bg-white px-3 text-sm text-zinc-900 focus:outline-none focus:ring-1 focus:ring-brand-500 dark:border-zinc-800 dark:bg-zinc-950 dark:text-zinc-100"
              >
                {BILLING_CYCLES.map((c) => (
                  <option key={c.value} value={c.value}>
                    {c.label}
                  </option>
                ))}
              </select>
            </div>
            <div className="flex items-center gap-2 pt-6">
              <input
                type="checkbox"
                id="new-plan-active"
                checked={newPlan.isActive}
                onChange={(e) => setNewPlan({ ...newPlan, isActive: e.target.checked })}
              />
              <Label htmlFor="new-plan-active" className="cursor-pointer text-xs">
                Active
              </Label>
            </div>
          </div>
          <div>
            <Label className="text-xs">Features (one per line)</Label>
            <textarea
              value={newPlan.features}
              onChange={(e) => setNewPlan({ ...newPlan, features: e.target.value })}
              placeholder={'Unlimited classes\nPersonal trainer\nLocker access'}
              rows={3}
              className="mt-1 w-full rounded-xl border border-zinc-200 bg-white px-3 py-2 text-sm text-zinc-900 focus:outline-none focus:ring-1 focus:ring-brand-500 dark:border-zinc-800 dark:bg-zinc-950 dark:text-zinc-100"
            />
          </div>
          {addError && <p className="text-xs font-medium text-red-500">{addError}</p>}
          <div className="flex gap-2">
            <Button className="rounded-xl" onClick={addPlan} loading={addLoading}>
              Create Plan
            </Button>
            <Button variant="outline" className="rounded-xl" onClick={() => setShowAdd(false)} disabled={addLoading}>
              Cancel
            </Button>
          </div>
        </div>
      )}

      {plans.length === 0 && !showAdd && <p className="text-sm text-zinc-400">No plans yet — add one above.</p>}

      <div className="grid gap-4 sm:grid-cols-2">
        {plans.map((plan, idx) => (
          <PlanCard
            key={plan.id}
            plan={plan}
            accentHex={ACCENTS[idx % ACCENTS.length]!}
            onPatch={(patch) => patchPlan(plan, patch)}
            onDelete={() => deletePlan(plan)}
          />
        ))}
      </div>
    </div>
  );
}

function PlanCard({
  plan,
  accentHex,
  onPatch,
  onDelete,
}: {
  plan: Plan;
  accentHex: string;
  onPatch: (patch: Record<string, unknown>) => Promise<{ ok: boolean; error?: string }>;
  onDelete: () => void;
}) {
  const [editingPrice, setEditingPrice] = useState(false);
  const [price, setPrice] = useState((plan.priceSgd / 100).toFixed(2));
  const [cycle, setCycle] = useState(plan.billingCycle);
  const [priceSaving, setPriceSaving] = useState(false);
  const [priceError, setPriceError] = useState<string | null>(null);

  const [editingFeatures, setEditingFeatures] = useState(false);
  const [featuresDraft, setFeaturesDraft] = useState(plan.features.join('\n'));
  const [featuresSaving, setFeaturesSaving] = useState(false);
  const [featuresError, setFeaturesError] = useState<string | null>(null);

  const cycleLabel = BILLING_CYCLES.find((c) => c.value === plan.billingCycle)?.label ?? plan.billingCycle;

  async function savePrice() {
    setPriceSaving(true);
    setPriceError(null);
    const res = await onPatch({ priceSgd: Math.round((parseFloat(price) || 0) * 100), billingCycle: cycle });
    setPriceSaving(false);
    if (!res.ok) {
      setPriceError(res.error ?? 'Failed to save');
      return;
    }
    setEditingPrice(false);
  }

  async function saveFeatures() {
    setFeaturesSaving(true);
    setFeaturesError(null);
    const res = await onPatch({ features: featuresDraft.split('\n').map((f) => f.trim()).filter(Boolean) });
    setFeaturesSaving(false);
    if (!res.ok) {
      setFeaturesError(res.error ?? 'Failed to save');
      return;
    }
    setEditingFeatures(false);
  }

  return (
    <div
      style={{ borderTopColor: accentHex }}
      className="group relative flex flex-col overflow-hidden rounded-xl border border-t-4 border-zinc-200 bg-white transition-all duration-200 hover:shadow-md dark:border-zinc-700 dark:bg-zinc-900"
    >
      <div className="px-5 py-4">
        <div className="flex items-center justify-between">
          <h4 className="text-sm font-bold text-zinc-900 dark:text-zinc-100">{plan.planName}</h4>
          <div className="flex items-center gap-1">
            <button
              type="button"
              onClick={() => onPatch({ isActive: !plan.isActive })}
              className={`rounded-full px-2.5 py-0.5 text-[10px] font-bold uppercase tracking-wider transition-colors ${
                plan.isActive
                  ? 'bg-emerald-500/10 text-emerald-600 hover:bg-emerald-500/20 dark:text-emerald-400'
                  : 'bg-zinc-100 text-zinc-500 hover:bg-zinc-200 dark:bg-zinc-800 dark:text-zinc-400'
              }`}
            >
              {plan.isActive ? 'Active' : 'Inactive'}
            </button>
            <button
              type="button"
              onClick={onDelete}
              className="shrink-0 rounded-lg p-1 text-zinc-400 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-500/10 dark:hover:text-red-400"
              aria-label={`Delete ${plan.planName}`}
            >
              <Trash2 className="h-3.5 w-3.5" />
            </button>
          </div>
        </div>

        {editingPrice ? (
          <div className="mt-2">
            <div className="flex items-center gap-1.5">
              <span className="text-sm font-semibold text-zinc-500">S$</span>
              <Input value={price} onChange={(e) => setPrice(e.target.value)} className="h-8 w-20 text-sm" autoFocus />
              <select
                value={cycle}
                onChange={(e) => setCycle(e.target.value)}
                className="h-8 rounded-lg border border-zinc-200 bg-white px-2 text-xs font-semibold text-zinc-800 dark:border-zinc-700 dark:bg-zinc-900 dark:text-zinc-100"
              >
                {BILLING_CYCLES.map((c) => (
                  <option key={c.value} value={c.value}>
                    {c.label}
                  </option>
                ))}
              </select>
              <button
                type="button"
                onClick={savePrice}
                disabled={priceSaving}
                className="shrink-0 rounded-lg p-1.5 text-emerald-600 transition-colors hover:bg-emerald-50 disabled:opacity-50 dark:text-emerald-400 dark:hover:bg-emerald-500/10"
              >
                {priceSaving ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Check className="h-3.5 w-3.5" />}
              </button>
              <button
                type="button"
                onClick={() => setEditingPrice(false)}
                disabled={priceSaving}
                className="shrink-0 rounded-lg p-1.5 text-zinc-400 hover:bg-zinc-100 dark:hover:bg-zinc-900"
              >
                <X className="h-3.5 w-3.5" />
              </button>
            </div>
            {priceError && <p className="mt-1 text-xs font-medium text-red-500">{priceError}</p>}
          </div>
        ) : (
          <div className="mt-2 flex items-baseline gap-1">
            <span className="text-3xl font-black tracking-tight text-zinc-900 dark:text-white">
              S${(plan.priceSgd / 100).toFixed(2)}
            </span>
            {plan.billingCycle !== 'one_time' && <span className="text-xs font-semibold text-zinc-400">/{cycleLabel}</span>}
            <button
              type="button"
              onClick={() => setEditingPrice(true)}
              className="ml-0.5 shrink-0 rounded-lg p-1 text-zinc-300 opacity-0 transition-opacity hover:bg-zinc-100 hover:text-zinc-600 group-hover:opacity-100 dark:hover:bg-zinc-800"
              aria-label="Edit price"
            >
              <Pencil className="h-3 w-3" />
            </button>
          </div>
        )}
      </div>

      <div className="flex-1 space-y-2 border-t border-zinc-100 px-5 py-4 dark:border-zinc-800">
        {editingFeatures ? (
          <div>
            <textarea
              autoFocus
              value={featuresDraft}
              onChange={(e) => setFeaturesDraft(e.target.value)}
              rows={4}
              placeholder="One feature per line"
              className="w-full rounded-lg border border-zinc-200 bg-white px-2.5 py-2 text-xs text-zinc-800 focus:outline-none focus:ring-1 focus:ring-brand-500 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-100"
            />
            <div className="mt-1.5 flex items-center gap-1.5">
              <button
                type="button"
                onClick={saveFeatures}
                disabled={featuresSaving}
                className="inline-flex items-center gap-1 rounded-lg px-2 py-1 text-xs font-semibold text-emerald-600 transition-colors hover:bg-emerald-50 disabled:opacity-50 dark:text-emerald-400 dark:hover:bg-emerald-500/10"
              >
                {featuresSaving ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Check className="h-3.5 w-3.5" />}
                Save
              </button>
              <button
                type="button"
                onClick={() => {
                  setFeaturesDraft(plan.features.join('\n'));
                  setEditingFeatures(false);
                }}
                disabled={featuresSaving}
                className="inline-flex items-center gap-1 rounded-lg px-2 py-1 text-xs font-semibold text-zinc-400 hover:bg-zinc-100 dark:hover:bg-zinc-900"
              >
                <X className="h-3.5 w-3.5" />
                Cancel
              </button>
            </div>
            {featuresError && <p className="mt-1 text-xs font-medium text-red-500">{featuresError}</p>}
          </div>
        ) : (
          <>
            {plan.features.length > 0 ? (
              plan.features.map((f, i) => (
                <div key={i} className="flex items-start gap-2">
                  <CheckCircle2 className="mt-0.5 h-3.5 w-3.5 shrink-0 text-emerald-500" />
                  <span className="text-xs text-zinc-600 dark:text-zinc-300">{f}</span>
                </div>
              ))
            ) : (
              <p className="text-xs italic text-zinc-400">No features listed</p>
            )}
            <button
              type="button"
              onClick={() => setEditingFeatures(true)}
              className="mt-1 inline-flex items-center gap-1 text-[10px] font-semibold text-zinc-400 opacity-0 transition-opacity hover:text-zinc-700 group-hover:opacity-100 dark:hover:text-zinc-200"
            >
              <Pencil className="h-3 w-3" />
              Edit features
            </button>
          </>
        )}
      </div>
    </div>
  );
}
