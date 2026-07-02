'use client';

import { useState } from 'react';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Label } from '@/components/ui/Label';
import type { Plan } from '@/lib/types';
import { api } from '@/lib/api';
import { Check, Plus, Trash2 } from 'lucide-react';

const BILLING_CYCLES = [
  { value: 'monthly', label: 'Monthly' },
  { value: 'yearly', label: 'Yearly' },
  { value: 'one_time', label: 'One-time' },
];

const emptyNew = () => ({
  planName: '',
  priceSgd: '',
  billingCycle: 'monthly',
  features: '',
  isActive: true,
});

export function PlansManagement({
  studioId,
  initialPlans,
  onSaveSuccess,
}: {
  studioId: string;
  initialPlans: Plan[];
  onSaveSuccess: (msg: string) => void;
}) {
  const [plans, setPlans] = useState<Plan[]>(initialPlans);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [loadingId, setLoadingId] = useState<string | null>(null);
  const [editState, setEditState] = useState<{
    planName: string;
    priceSgd: string;
    billingCycle: string;
    features: string;
    isActive: boolean;
  } | null>(null);

  const [showAdd, setShowAdd] = useState(false);
  const [newPlan, setNewPlan] = useState(emptyNew());
  const [addLoading, setAddLoading] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  const startEdit = (plan: Plan) => {
    setEditingId(plan.id);
    setEditState({
      planName: plan.planName,
      priceSgd: (plan.priceSgd / 100).toString(),
      billingCycle: plan.billingCycle,
      features: plan.features.join('\n'),
      isActive: plan.isActive,
    });
  };

  const cancelEdit = () => {
    setEditingId(null);
    setEditState(null);
  };

  const saveEdit = async (plan: Plan) => {
    if (!editState) return;
    setLoadingId(plan.id);
    try {
      const priceSgd = Math.round(parseFloat(editState.priceSgd) * 100);
      const features = editState.features.split('\n').map((f) => f.trim()).filter(Boolean);
      await api(`/api/v1/me/studios/${studioId}/plans/${plan.id}`, {
        method: 'PUT',
        json: {
          planName: editState.planName,
          priceSgd,
          billingCycle: editState.billingCycle,
          features,
          isActive: editState.isActive,
        },
      });
      setPlans((prev) =>
        prev.map((p) =>
          p.id === plan.id
            ? { ...p, planName: editState.planName, priceSgd, billingCycle: editState.billingCycle, features, isActive: editState.isActive }
            : p
        )
      );
      setEditingId(null);
      setEditState(null);
      onSaveSuccess(`${editState.planName} updated.`);
    } catch (e: any) {
      alert(e.message || 'Failed to save');
    } finally {
      setLoadingId(null);
    }
  };

  const toggleStatus = async (plan: Plan) => {
    setLoadingId(plan.id);
    try {
      const isActive = !plan.isActive;
      await api(`/api/v1/me/studios/${studioId}/plans/${plan.id}`, {
        method: 'PUT',
        json: { isActive },
      });
      setPlans((prev) => prev.map((p) => (p.id === plan.id ? { ...p, isActive } : p)));
      onSaveSuccess(`${plan.planName} ${isActive ? 'enabled' : 'disabled'}.`);
    } catch (e: any) {
      alert(e.message || 'Failed to update');
    } finally {
      setLoadingId(null);
    }
  };

  const addPlan = async () => {
    if (!newPlan.planName.trim()) {
      alert('Plan name is required');
      return;
    }
    setAddLoading(true);
    try {
      const priceSgd = Math.round(parseFloat(newPlan.priceSgd || '0') * 100);
      const features = newPlan.features.split('\n').map((f) => f.trim()).filter(Boolean);
      const res = await api(`/api/v1/me/studios/${studioId}/plans`, {
        method: 'POST',
        json: {
          planName: newPlan.planName.trim(),
          priceSgd,
          billingCycle: newPlan.billingCycle,
          features,
          isActive: newPlan.isActive,
        },
      });
      const created: Plan = (res as { plan: Plan }).plan;
      setPlans((prev) => [...prev, created]);
      setShowAdd(false);
      setNewPlan(emptyNew());
      onSaveSuccess(`${created.planName} plan created.`);
    } catch (e: any) {
      alert(e.message || 'Failed to create plan');
    } finally {
      setAddLoading(false);
    }
  };

  const deletePlan = async (plan: Plan) => {
    if (!confirm(`Delete "${plan.planName}" plan? This cannot be undone.`)) return;
    setDeletingId(plan.id);
    try {
      await api(`/api/v1/me/studios/${studioId}/plans/${plan.id}`, { method: 'DELETE' });
      setPlans((prev) => prev.filter((p) => p.id !== plan.id));
      onSaveSuccess(`${plan.planName} deleted.`);
    } catch (e: any) {
      alert(e.message || 'Failed to delete plan');
    } finally {
      setDeletingId(null);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-end justify-between">
        <div>
          <h3 className="text-xs font-black uppercase tracking-wider text-zinc-400 mb-1">
            Membership Plans
          </h3>
          <p className="text-[10px] text-zinc-500">
            Configure subscription plans available to members via the WhatsApp bot.
          </p>
        </div>
        <Button
          onClick={() => { setShowAdd(true); setEditingId(null); }}
          className="flex items-center gap-1.5 bg-gradient-to-r from-brand-500 to-violet-600 text-white rounded-xl text-xs font-bold px-3 py-2"
        >
          <Plus className="h-3.5 w-3.5" />
          Add Plan
        </Button>
      </div>

      {/* Add Plan Form */}
      {showAdd && (
        <div className="rounded-[24px] border border-brand-500/30 bg-white/20 dark:bg-brand-950/20 backdrop-blur-2xl p-6 space-y-4">
          <h4 className="text-sm font-black text-zinc-900 dark:text-white">New Plan</h4>
          <div className="grid gap-4 sm:grid-cols-2">
            <div>
              <Label className="text-[10px]">Plan Name</Label>
              <Input
                value={newPlan.planName}
                onChange={(e) => setNewPlan({ ...newPlan, planName: e.target.value })}
                placeholder="e.g. Premium"
                className="mt-1 bg-white/50 dark:bg-black/50"
              />
            </div>
            <div>
              <Label className="text-[10px]">Price (S$)</Label>
              <Input
                type="number"
                value={newPlan.priceSgd}
                onChange={(e) => setNewPlan({ ...newPlan, priceSgd: e.target.value })}
                placeholder="0.00"
                className="mt-1 bg-white/50 dark:bg-black/50"
              />
            </div>
            <div>
              <Label className="text-[10px]">Billing Cycle</Label>
              <select
                value={newPlan.billingCycle}
                onChange={(e) => setNewPlan({ ...newPlan, billingCycle: e.target.value })}
                className="mt-1 w-full rounded-xl border border-white/20 bg-white/50 dark:bg-black/50 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
              >
                {BILLING_CYCLES.map((c) => (
                  <option key={c.value} value={c.value}>{c.label}</option>
                ))}
              </select>
            </div>
            <div className="flex items-center gap-2 pt-5">
              <input
                type="checkbox"
                id="new-active"
                checked={newPlan.isActive}
                onChange={(e) => setNewPlan({ ...newPlan, isActive: e.target.checked })}
                className="rounded"
              />
              <Label htmlFor="new-active" className="text-[10px] cursor-pointer">Active</Label>
            </div>
          </div>
          <div>
            <Label className="text-[10px]">Features (one per line)</Label>
            <textarea
              value={newPlan.features}
              onChange={(e) => setNewPlan({ ...newPlan, features: e.target.value })}
              placeholder="Unlimited classes&#10;Personal trainer&#10;Locker access"
              className="mt-1 w-full h-28 rounded-xl border border-white/20 bg-white/50 dark:bg-black/50 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
            />
          </div>
          <div className="flex gap-2 pt-2">
            <Button
              onClick={addPlan}
              loading={addLoading}
              className="flex-1 bg-gradient-to-r from-brand-500 to-violet-600 text-white rounded-xl text-xs font-bold"
            >
              Create Plan
            </Button>
            <Button
              variant="outline"
              onClick={() => { setShowAdd(false); setNewPlan(emptyNew()); }}
              disabled={addLoading}
              className="flex-1 rounded-xl text-xs font-bold"
            >
              Cancel
            </Button>
          </div>
        </div>
      )}

      {/* Plan Cards */}
      <div className="grid gap-6 sm:grid-cols-2">
        {plans.map((plan) => {
          const isEditing = editingId === plan.id;
          return (
            <div
              key={plan.id}
              className={`relative overflow-hidden rounded-[24px] border ${
                plan.isActive
                  ? 'border-brand-500/30 shadow-lg shadow-brand-500/10 bg-white/20 dark:bg-brand-950/20'
                  : 'border-white/10 bg-white/5 dark:bg-white/5 opacity-80'
              } backdrop-blur-2xl p-6 transition-all duration-300`}
            >
              <div className="flex justify-between items-start mb-4">
                <div>
                  <h4 className="text-xl font-black text-zinc-900 dark:text-white">
                    {plan.planName}
                  </h4>
                  <p className="text-[10px] font-bold uppercase tracking-widest text-zinc-500 mt-1">
                    {plan.billingCycle}
                  </p>
                </div>
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => toggleStatus(plan)}
                    disabled={loadingId === plan.id}
                    className={`text-[10px] font-bold px-3 py-1 rounded-full uppercase tracking-wider transition-colors ${
                      plan.isActive
                        ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 hover:bg-emerald-500/20'
                        : 'bg-zinc-500/10 text-zinc-600 dark:text-zinc-400 hover:bg-zinc-500/20'
                    }`}
                  >
                    {plan.isActive ? 'Active' : 'Inactive'}
                  </button>
                  <button
                    onClick={() => deletePlan(plan)}
                    disabled={deletingId === plan.id}
                    className="p-1.5 rounded-lg text-zinc-400 hover:text-red-500 hover:bg-red-500/10 transition-colors"
                    title="Delete plan"
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </button>
                </div>
              </div>

              {isEditing && editState ? (
                <div className="space-y-4">
                  <div className="grid gap-3 sm:grid-cols-2">
                    <div>
                      <Label className="text-[10px]">Plan Name</Label>
                      <Input
                        value={editState.planName}
                        onChange={(e) => setEditState({ ...editState, planName: e.target.value })}
                        className="mt-1 bg-white/50 dark:bg-black/50"
                      />
                    </div>
                    <div>
                      <Label className="text-[10px]">Price (S$)</Label>
                      <Input
                        type="number"
                        value={editState.priceSgd}
                        onChange={(e) => setEditState({ ...editState, priceSgd: e.target.value })}
                        className="mt-1 bg-white/50 dark:bg-black/50"
                      />
                    </div>
                  </div>
                  <div>
                    <Label className="text-[10px]">Billing Cycle</Label>
                    <select
                      value={editState.billingCycle}
                      onChange={(e) => setEditState({ ...editState, billingCycle: e.target.value })}
                      className="mt-1 w-full rounded-xl border border-white/20 bg-white/50 dark:bg-black/50 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                    >
                      {BILLING_CYCLES.map((c) => (
                        <option key={c.value} value={c.value}>{c.label}</option>
                      ))}
                    </select>
                  </div>
                  <div>
                    <Label className="text-[10px]">Features (one per line)</Label>
                    <textarea
                      value={editState.features}
                      onChange={(e) => setEditState({ ...editState, features: e.target.value })}
                      className="mt-1 w-full h-32 rounded-xl border border-white/20 bg-white/50 dark:bg-black/50 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                    />
                  </div>
                  <div className="flex items-center gap-2 pt-2">
                    <Button
                      onClick={() => saveEdit(plan)}
                      loading={loadingId === plan.id}
                      className="flex-1 bg-gradient-to-r from-brand-500 to-violet-600 text-white rounded-xl text-xs font-bold"
                    >
                      Save
                    </Button>
                    <Button
                      variant="outline"
                      onClick={cancelEdit}
                      disabled={loadingId === plan.id}
                      className="flex-1 rounded-xl text-xs font-bold"
                    >
                      Cancel
                    </Button>
                  </div>
                </div>
              ) : (
                <div className="space-y-4">
                  <div className="flex items-baseline gap-1">
                    <span className="text-3xl font-black tracking-tight text-zinc-900 dark:text-white">
                      S$ {(plan.priceSgd / 100).toFixed(2)}
                    </span>
                    {plan.billingCycle !== 'one_time' && (
                      <span className="text-sm font-semibold text-zinc-500">/mo</span>
                    )}
                  </div>
                  <div className="space-y-2">
                    {plan.features.map((f, i) => (
                      <div key={i} className="flex items-start gap-2 text-sm text-zinc-700 dark:text-zinc-300">
                        <Check className="h-4 w-4 shrink-0 text-emerald-500 mt-0.5" />
                        <span>{f}</span>
                      </div>
                    ))}
                  </div>
                  <Button
                    variant="outline"
                    onClick={() => startEdit(plan)}
                    className="w-full mt-4 rounded-xl border-white/20 hover:bg-white/10 text-xs font-bold uppercase tracking-wider"
                  >
                    Edit Plan
                  </Button>
                </div>
              )}
            </div>
          );
        })}

        {plans.length === 0 && !showAdd && (
          <div className="col-span-2 text-center py-12 text-zinc-500 text-sm">
            No plans yet. Click "Add Plan" to create your first membership plan.
          </div>
        )}
      </div>
    </div>
  );
}
