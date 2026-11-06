'use client';

import { useState } from 'react';
import { Check, Pencil, Plus, Trash2, X } from 'lucide-react';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { Input } from '@/components/ui/Input';
import { Label } from '@/components/ui/Label';
import { api } from '@/lib/api';
import type { Offer } from './page';

interface FormState {
  name: string;
  description: string;
  terms: string;
  priceSgd: string; // dollars, as typed — converted to cents on save
  isActive: boolean;
}

const emptyForm = (): FormState => ({ name: '', description: '', terms: '', priceSgd: '', isActive: true });

function toForm(o: Offer): FormState {
  return {
    name: o.name,
    description: o.description,
    terms: o.terms,
    priceSgd: (o.priceSgd / 100).toString(),
    isActive: o.isActive,
  };
}

export function OffersManagement({ studioId, initialOffers }: { studioId: string; initialOffers: Offer[] }) {
  const [offers, setOffers] = useState<Offer[]>(initialOffers);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editForm, setEditForm] = useState<FormState>(emptyForm());
  const [showAdd, setShowAdd] = useState(false);
  const [addForm, setAddForm] = useState<FormState>(emptyForm());
  const [savingId, setSavingId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  function startEdit(o: Offer) {
    setEditingId(o.id);
    setEditForm(toForm(o));
  }

  async function saveEdit(id: string) {
    setSavingId(id);
    setError(null);
    try {
      await api(`/api/v1/me/studios/${studioId}/offers/${id}`, {
        method: 'PUT',
        json: {
          name: editForm.name,
          description: editForm.description,
          terms: editForm.terms,
          priceSgd: Math.round(parseFloat(editForm.priceSgd || '0') * 100),
          isActive: editForm.isActive,
        },
      });
      setOffers((prev) =>
        prev.map((o) =>
          o.id === id
            ? { ...o, name: editForm.name, description: editForm.description, terms: editForm.terms,
                priceSgd: Math.round(parseFloat(editForm.priceSgd || '0') * 100), isActive: editForm.isActive }
            : o,
        ),
      );
      setEditingId(null);
    } catch {
      setError('Could not save that offer. Try again.');
    } finally {
      setSavingId(null);
    }
  }

  async function createOffer() {
    setSavingId('new');
    setError(null);
    try {
      const res = await api<{ offer: Offer }>(`/api/v1/me/studios/${studioId}/offers`, {
        method: 'POST',
        json: {
          name: addForm.name,
          description: addForm.description,
          terms: addForm.terms,
          priceSgd: Math.round(parseFloat(addForm.priceSgd || '0') * 100),
          isActive: addForm.isActive,
        },
      });
      setOffers((prev) => [res.offer, ...prev]);
      setAddForm(emptyForm());
      setShowAdd(false);
    } catch {
      setError('Could not create that offer. Try again.');
    } finally {
      setSavingId(null);
    }
  }

  async function deleteOffer(id: string) {
    if (!confirm('Delete this offer? The AI will no longer be able to mention it.')) return;
    setSavingId(id);
    setError(null);
    try {
      await api(`/api/v1/me/studios/${studioId}/offers/${id}`, { method: 'DELETE' });
      setOffers((prev) => prev.filter((o) => o.id !== id));
    } catch {
      setError('Could not delete that offer. Try again.');
    } finally {
      setSavingId(null);
    }
  }

  return (
    <div className="space-y-4">
      {error && (
        <p className="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-500/10 dark:text-red-400">{error}</p>
      )}

      {offers.length === 0 && !showAdd && (
        <p className="text-sm text-zinc-500 dark:text-zinc-400">
          No offers yet — the AI has nothing structured to quote for pricing/trial questions beyond your plans.
        </p>
      )}

      <div className="space-y-3">
        {offers.map((o) => (
          <Card key={o.id} noPadding>
            {editingId === o.id ? (
              <div className="space-y-3 p-5">
                <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                  <div>
                    <Label className="text-[10px] font-black uppercase tracking-wider text-zinc-400">Name</Label>
                    <Input value={editForm.name} onChange={(e) => setEditForm({ ...editForm, name: e.target.value })} />
                  </div>
                  <div>
                    <Label className="text-[10px] font-black uppercase tracking-wider text-zinc-400">Price (SGD)</Label>
                    <Input
                      type="number"
                      step="0.01"
                      value={editForm.priceSgd}
                      onChange={(e) => setEditForm({ ...editForm, priceSgd: e.target.value })}
                    />
                  </div>
                </div>
                <div>
                  <Label className="text-[10px] font-black uppercase tracking-wider text-zinc-400">Description</Label>
                  <Input value={editForm.description} onChange={(e) => setEditForm({ ...editForm, description: e.target.value })} />
                </div>
                <div>
                  <Label className="text-[10px] font-black uppercase tracking-wider text-zinc-400">Terms</Label>
                  <Input value={editForm.terms} onChange={(e) => setEditForm({ ...editForm, terms: e.target.value })} />
                </div>
                <label className="flex cursor-pointer items-center gap-2 text-sm font-medium text-zinc-600 dark:text-zinc-300">
                  <input
                    type="checkbox"
                    checked={editForm.isActive}
                    onChange={(e) => setEditForm({ ...editForm, isActive: e.target.checked })}
                    className="h-4 w-4 rounded border-zinc-300 text-brand-600 focus:ring-brand-500"
                  />
                  Active (the AI may quote this offer)
                </label>
                <div className="flex gap-2">
                  <Button size="sm" onClick={() => saveEdit(o.id)} loading={savingId === o.id} leftIcon={<Check className="h-4 w-4" />}>
                    Save
                  </Button>
                  <Button size="sm" variant="secondary" onClick={() => setEditingId(null)} leftIcon={<X className="h-4 w-4" />}>
                    Cancel
                  </Button>
                </div>
              </div>
            ) : (
              <div className="flex items-start justify-between gap-4 p-5">
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-bold text-zinc-900 dark:text-zinc-100">{o.name}</span>
                    <Badge tone={o.isActive ? 'success' : 'neutral'}>{o.isActive ? 'Active' : 'Inactive'}</Badge>
                    <span className="text-sm font-semibold text-zinc-500">S${(o.priceSgd / 100).toFixed(2)}</span>
                  </div>
                  {o.description && <p className="mt-1 text-sm text-zinc-500 dark:text-zinc-400">{o.description}</p>}
                  {o.terms && <p className="mt-1 text-xs text-zinc-400">Terms: {o.terms}</p>}
                </div>
                <div className="flex shrink-0 gap-1">
                  <Button variant="ghost" size="sm" onClick={() => startEdit(o)} leftIcon={<Pencil className="h-3.5 w-3.5" />}>
                    Edit
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => deleteOffer(o.id)}
                    loading={savingId === o.id}
                    leftIcon={<Trash2 className="h-3.5 w-3.5" />}
                    className="text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-500/10"
                  >
                    Delete
                  </Button>
                </div>
              </div>
            )}
          </Card>
        ))}
      </div>

      {showAdd ? (
        <Card title="New offer">
          <div className="space-y-3">
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <div>
                <Label className="text-[10px] font-black uppercase tracking-wider text-zinc-400">Name</Label>
                <Input
                  autoFocus
                  value={addForm.name}
                  onChange={(e) => setAddForm({ ...addForm, name: e.target.value })}
                  placeholder="8 Classes Trial"
                />
              </div>
              <div>
                <Label className="text-[10px] font-black uppercase tracking-wider text-zinc-400">Price (SGD)</Label>
                <Input
                  type="number"
                  step="0.01"
                  value={addForm.priceSgd}
                  onChange={(e) => setAddForm({ ...addForm, priceSgd: e.target.value })}
                  placeholder="68.00"
                />
              </div>
            </div>
            <div>
              <Label className="text-[10px] font-black uppercase tracking-wider text-zinc-400">Description</Label>
              <Input
                value={addForm.description}
                onChange={(e) => setAddForm({ ...addForm, description: e.target.value })}
                placeholder="For first-time visitors only"
              />
            </div>
            <div>
              <Label className="text-[10px] font-black uppercase tracking-wider text-zinc-400">Terms</Label>
              <Input
                value={addForm.terms}
                onChange={(e) => setAddForm({ ...addForm, terms: e.target.value })}
                placeholder="Valid 30 days from purchase"
              />
            </div>
            <label className="flex cursor-pointer items-center gap-2 text-sm font-medium text-zinc-600 dark:text-zinc-300">
              <input
                type="checkbox"
                checked={addForm.isActive}
                onChange={(e) => setAddForm({ ...addForm, isActive: e.target.checked })}
                className="h-4 w-4 rounded border-zinc-300 text-brand-600 focus:ring-brand-500"
              />
              Active (the AI may quote this offer)
            </label>
            <div className="flex gap-2">
              <Button
                size="sm"
                onClick={createOffer}
                loading={savingId === 'new'}
                disabled={!addForm.name.trim()}
                leftIcon={<Check className="h-4 w-4" />}
              >
                Create offer
              </Button>
              <Button size="sm" variant="secondary" onClick={() => { setShowAdd(false); setAddForm(emptyForm()); }}>
                Cancel
              </Button>
            </div>
          </div>
        </Card>
      ) : (
        <Button variant="secondary" onClick={() => setShowAdd(true)} leftIcon={<Plus className="h-4 w-4" />}>
          Add offer
        </Button>
      )}
    </div>
  );
}
