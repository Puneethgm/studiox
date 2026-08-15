'use client';

import { useState } from 'react';
import { Plus, Save, CheckCircle2, X, Trash2 } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { api, ApiError } from '@/lib/api';
import type { FollowupStep } from '@/lib/types';
import { FollowupCanvas } from './FollowupCanvas';
import type { StepDraft } from './followupLayout';

type Unit = StepDraft['delayUnit'];

interface Toast {
  id: number;
  message: string;
  type: 'success' | 'error';
}

function minutesToDraft(minutes: number, messageTemplate: string, key: number): StepDraft {
  if (minutes % 1440 === 0 && minutes > 0) {
    return { key, delayValue: minutes / 1440, delayUnit: 'days', messageTemplate };
  }
  if (minutes % 60 === 0 && minutes > 0) {
    return { key, delayValue: minutes / 60, delayUnit: 'hours', messageTemplate };
  }
  return { key, delayValue: minutes, delayUnit: 'minutes', messageTemplate };
}

function draftToMinutes(d: StepDraft): number {
  const multiplier = d.delayUnit === 'days' ? 1440 : d.delayUnit === 'hours' ? 60 : 1;
  return Math.max(1, Math.round(d.delayValue * multiplier));
}

export function FollowupsEditor({
  studioId,
  initialSteps,
}: {
  studioId: string;
  initialSteps: FollowupStep[];
}) {
  const [steps, setSteps] = useState<StepDraft[]>(
    initialSteps.map((s, i) => minutesToDraft(s.delayMinutes, s.messageTemplate, i)),
  );
  const [nextKey, setNextKey] = useState(initialSteps.length);
  const [selectedKey, setSelectedKey] = useState<number | null>(null);
  const [saving, setSaving] = useState(false);
  const [toasts, setToasts] = useState<Toast[]>([]);

  function showToast(message: string, type: 'success' | 'error' = 'success') {
    const id = Date.now();
    setToasts((prev) => [...prev, { id, message, type }]);
    setTimeout(() => setToasts((prev) => prev.filter((t) => t.id !== id)), 3000);
  }

  function addStep() {
    const key = nextKey;
    setSteps((prev) => [...prev, { key, delayValue: 1, delayUnit: 'hours', messageTemplate: '' }]);
    setNextKey((k) => k + 1);
    setSelectedKey(key);
  }

  function removeStep(key: number) {
    setSteps((prev) => prev.filter((s) => s.key !== key));
    setSelectedKey((cur) => (cur === key ? null : cur));
  }

  function updateStep(key: number, patch: Partial<StepDraft>) {
    setSteps((prev) => prev.map((s) => (s.key === key ? { ...s, ...patch } : s)));
  }

  async function save() {
    setSaving(true);
    try {
      const body = {
        steps: steps.map((s) => ({
          delayMinutes: draftToMinutes(s),
          messageTemplate: s.messageTemplate,
        })),
      };
      const resp = await api<{ steps: FollowupStep[] }>(
        `/api/v1/studios/${studioId}/messaging/followup-steps`,
        { method: 'PUT', json: body },
      );
      const nextSteps = resp.steps.map((s, i) => minutesToDraft(s.delayMinutes, s.messageTemplate, i));
      setSteps(nextSteps);
      setNextKey(nextSteps.length);
      setSelectedKey(null);
      showToast(steps.length === 0 ? 'Follow-ups turned off' : 'Follow-up cadence saved');
    } catch (err) {
      showToast(err instanceof ApiError ? err.message : 'Failed to save', 'error');
    } finally {
      setSaving(false);
    }
  }

  const selectedStep = steps.find((s) => s.key === selectedKey) ?? null;
  const selectedIndex = selectedStep ? steps.indexOf(selectedStep) : -1;

  return (
    <div className="space-y-6">
      <p className="text-sm text-gray-500 max-w-2xl">
        When a new lead doesn&apos;t reply after the initial greeting, these messages go out automatically at the delays
        below — in order, each measured from when they were first contacted. As soon as the lead replies, every
        step still waiting is cancelled. Delete every step to turn follow-ups off entirely.
        Placeholders: <code>{'{{lead_first_name}}'}</code>, <code>{'{{lead_name}}'}</code>, <code>{'{{studio_name}}'}</code>.
      </p>

      <div className="flex gap-4 items-start">
        {/* Canvas panel */}
        <div className="flex-1 min-w-0">
          <Card className="p-4">
            <div className="flex items-center justify-between mb-4">
              <p className="text-sm font-medium text-gray-600">Follow-up steps</p>
              <Button size="sm" variant="ghost" leftIcon={<Plus className="h-3.5 w-3.5" />} onClick={addStep}>
                Add step
              </Button>
            </div>
            {steps.length === 0 ? (
              <div className="text-center py-10 space-y-3">
                <p className="text-gray-400 text-sm">No follow-up steps yet — follow-ups are off for this studio.</p>
                <Button size="sm" leftIcon={<Plus className="h-3.5 w-3.5" />} onClick={addStep}>
                  Add first step
                </Button>
              </div>
            ) : (
              <FollowupCanvas
                steps={steps}
                selectedKey={selectedKey}
                onSelect={setSelectedKey}
                onAddNext={addStep}
                onDelete={removeStep}
              />
            )}
          </Card>
        </div>

        {/* Right panel */}
        {selectedStep && (
          <div className="w-80 shrink-0">
            <Card className="p-4 space-y-4">
              <div className="flex items-center justify-between">
                <p className="text-sm font-semibold text-slate-800">Step {selectedIndex + 1}</p>
                <button
                  onClick={() => setSelectedKey(null)}
                  className="text-gray-400 hover:text-gray-600"
                  title="Close"
                >
                  <X className="h-4 w-4" />
                </button>
              </div>

              <div>
                <label className="text-xs font-medium text-gray-500 uppercase tracking-wide">Wait</label>
                <div className="mt-1 flex gap-2">
                  <input
                    type="number"
                    min={1}
                    value={selectedStep.delayValue}
                    onChange={(e) => updateStep(selectedStep.key, { delayValue: Number(e.target.value) })}
                    className="w-20 rounded border border-gray-200 px-2 py-1.5 text-sm"
                  />
                  <select
                    value={selectedStep.delayUnit}
                    onChange={(e) => updateStep(selectedStep.key, { delayUnit: e.target.value as Unit })}
                    className="flex-1 rounded border border-gray-200 px-2 py-1.5 text-sm"
                  >
                    <option value="minutes">minutes</option>
                    <option value="hours">hours</option>
                    <option value="days">days</option>
                  </select>
                </div>
                <p className="mt-1 text-[11px] text-gray-400">
                  Measured from when the lead was first contacted, not from the previous step.
                </p>
              </div>

              <div>
                <label className="text-xs font-medium text-gray-500 uppercase tracking-wide">Message</label>
                <textarea
                  value={selectedStep.messageTemplate}
                  onChange={(e) => updateStep(selectedStep.key, { messageTemplate: e.target.value })}
                  rows={4}
                  placeholder="e.g. Hi {{lead_first_name}}, still thinking about joining {{studio_name}}?"
                  className="mt-1 w-full rounded border border-gray-200 px-3 py-2 text-sm resize-y"
                />
              </div>

              <button
                onClick={() => removeStep(selectedStep.key)}
                className="flex items-center gap-1.5 text-xs font-medium text-red-500 hover:text-red-600"
              >
                <Trash2 className="h-3.5 w-3.5" />
                Delete this step
              </button>
            </Card>
          </div>
        )}
      </div>

      <div className="flex justify-end">
        <Button onClick={save} disabled={saving}>
          <Save className="h-4 w-4" />
          {saving ? 'Saving…' : 'Save follow-up cadence'}
        </Button>
      </div>

      <div className="fixed bottom-5 right-5 z-50 flex flex-col gap-2 pointer-events-none">
        {toasts.map((t) => (
          <div
            key={t.id}
            className={`flex items-center gap-3 rounded-xl px-4 py-3 shadow-lg text-sm font-medium pointer-events-auto transition-all border
              ${t.type === 'success'
                ? 'bg-white text-slate-800 border-slate-200'
                : 'bg-white text-red-600 border-red-200'}`}
          >
            {t.type === 'success'
              ? <CheckCircle2 className="h-4 w-4 shrink-0 text-brand-500" />
              : <X className="h-4 w-4 shrink-0 text-red-500" />}
            {t.message}
            <button
              className="ml-2 opacity-70 hover:opacity-100"
              onClick={() => setToasts((prev) => prev.filter((x) => x.id !== t.id))}
            >
              <X className="h-3.5 w-3.5" />
            </button>
          </div>
        ))}
      </div>
    </div>
  );
}
