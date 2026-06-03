'use client';

import { useState } from 'react';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Label } from '@/components/ui/Label';
import type { Plan } from '@/lib/types';
import { api } from '@/lib/api';
import { Check, X } from 'lucide-react';

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
    priceSgd: string;
    features: string;
    isActive: boolean;
  } | null>(null);

  const startEdit = (plan: Plan) => {
    setEditingId(plan.id);
    setEditState({
      priceSgd: (plan.priceSgd / 100).toString(),
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
      const features = editState.features
        .split('\n')
        .map((f) => f.trim())
        .filter(Boolean);
      const isActive = editState.isActive;

      const res = await api(`/api/v1/me/studios/${studioId}/plans/${plan.id}`, {
        method: 'PUT',
        json: { priceSgd, features, isActive },
      });

      setPlans((prev) =>
        prev.map((p) =>
          p.id === plan.id
            ? { ...p, priceSgd, features, isActive }
            : p
        )
      );
      setEditingId(null);
      setEditState(null);
      onSaveSuccess(`${plan.planName} plan updated successfully.`);
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
      const res = await api(`/api/v1/me/studios/${studioId}/plans/${plan.id}`, {
        method: 'PUT',
        json: { isActive },
      });

      setPlans((prev) =>
        prev.map((p) =>
          p.id === plan.id
            ? { ...p, isActive }
            : p
        )
      );
      onSaveSuccess(`${plan.planName} plan ${isActive ? 'enabled' : 'disabled'}.`);
    } catch (e: any) {
      alert(e.message || 'Failed to save');
    } finally {
      setLoadingId(null);
    }
  };

  return (
    <div className="space-y-6">
      <div>
        <h3 className="text-xs font-black uppercase tracking-wider text-zinc-400 mb-1">
          Membership Plans
        </h3>
        <p className="text-[10px] text-zinc-500">
          Configure subscription plans available to members via the WhatsApp bot.
        </p>
      </div>

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
              </div>

              {isEditing && editState ? (
                <div className="space-y-4">
                  <div>
                    <Label className="text-[10px]">Price (S$)</Label>
                    <Input
                      type="number"
                      value={editState.priceSgd}
                      onChange={(e) =>
                        setEditState({ ...editState, priceSgd: e.target.value })
                      }
                      className="mt-1 bg-white/50 dark:bg-black/50"
                    />
                  </div>
                  <div>
                    <Label className="text-[10px]">Features (One per line)</Label>
                    <textarea
                      value={editState.features}
                      onChange={(e) =>
                        setEditState({ ...editState, features: e.target.value })
                      }
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
                      <span className="text-sm font-semibold text-zinc-500">
                        /mo
                      </span>
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
      </div>
    </div>
  );
}
