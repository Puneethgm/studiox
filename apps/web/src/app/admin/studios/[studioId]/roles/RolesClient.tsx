'use client';

import { useState } from 'react';
import { Plus, ShieldCheck, Pencil, Trash2, Loader2, Power } from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Card } from '@/components/ui/Card';
import { Dialog, DialogHeader } from '@/components/ui/Dialog';
import { EmptyState } from '@/components/ui/EmptyState';
import { Input } from '@/components/ui/Input';
import { MultiSelect } from '@/components/ui/MultiSelect';
import { Textarea } from '@/components/ui/Textarea';
import { Label, FieldError } from '@/components/ui/Label';
import { Badge } from '@/components/ui/Badge';
import { cn } from '@/lib/cn';
import type { Permission, StudioRole } from '@/lib/types';
import { createRole, updateRole, deleteRole, setRoleActive } from './actions';

// Same look/behavior as admin/studios/StudioStatusToggle — a reversible
// switch, distinct from Delete. Deactivating a role blocks login for every
// user assigned to it (checked at login time — see identity.IsRoleActive).
function RoleStatusToggle({
  studioId,
  roleId,
  roleName,
  initialActive,
}: {
  studioId: string;
  roleId: string;
  roleName: string;
  initialActive: boolean;
}) {
  const [active, setActive] = useState(initialActive);
  const [loading, setLoading] = useState(false);

  async function handleToggle() {
    if (loading) return;
    setLoading(true);
    const next = !active;
    const res = await setRoleActive(studioId, roleId, next);
    setLoading(false);
    if (!res.ok) {
      alert(res.error);
      return;
    }
    setActive(next);
  }

  return (
    <button
      type="button"
      onClick={handleToggle}
      disabled={loading}
      className={cn(
        'relative inline-flex h-6 w-11 shrink-0 cursor-pointer items-center rounded-full transition-colors duration-300 focus:outline-none focus:ring-2 focus:ring-brand-500 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50',
        active ? 'bg-emerald-500' : 'bg-zinc-300 dark:bg-zinc-700',
      )}
      role="switch"
      aria-checked={active}
      title={active ? `Deactivate ${roleName}` : `Activate ${roleName}`}
    >
      <span
        className={cn(
          'pointer-events-none flex h-5 w-5 items-center justify-center rounded-full bg-white shadow-md ring-0 transition-transform duration-300 ease-out',
          active ? 'translate-x-5' : 'translate-x-0.5',
        )}
      >
        {loading ? (
          <Loader2 className="h-3 w-3 animate-spin text-zinc-500" />
        ) : (
          <Power className={cn('h-2.5 w-2.5', active ? 'text-emerald-600' : 'text-zinc-400')} />
        )}
      </span>
    </button>
  );
}

interface RoleFormState {
  name: string;
  description: string;
  permissionKeys: string[];
}

const emptyForm: RoleFormState = { name: '', description: '', permissionKeys: [] };

export function RolesClient({
  studioId,
  roles,
  permissions,
}: {
  studioId: string;
  roles: StudioRole[];
  permissions: Permission[];
}) {
  const [creating, setCreating] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  const editingRole = editingId ? roles.find((r) => r.id === editingId) ?? null : null;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-bold text-zinc-700 dark:text-zinc-300">
          {roles.length} role{roles.length === 1 ? '' : 's'}
        </h2>
        <Button size="sm" onClick={() => setCreating(true)} leftIcon={<Plus className="h-4 w-4" />}>
          Add role
        </Button>
      </div>

      <Dialog open={creating} onClose={() => setCreating(false)} widthClassName="max-w-xl">
        <DialogHeader icon={<ShieldCheck className="h-5 w-5 text-brand-500" />} title="New role" onClose={() => setCreating(false)} />
        <RoleForm
          studioId={studioId}
          permissions={permissions}
          initial={emptyForm}
          submitLabel="Create role"
          onDone={() => setCreating(false)}
        />
      </Dialog>

      <Dialog open={editingRole !== null} onClose={() => setEditingId(null)} widthClassName="max-w-xl">
        <DialogHeader icon={<ShieldCheck className="h-5 w-5 text-brand-500" />} title="Edit role" onClose={() => setEditingId(null)} />
        {editingRole && (
          <RoleForm
            studioId={studioId}
            permissions={permissions}
            initial={{ name: editingRole.name, description: editingRole.description, permissionKeys: editingRole.permissionKeys }}
            roleId={editingRole.id}
            submitLabel="Save changes"
            onDone={() => setEditingId(null)}
          />
        )}
      </Dialog>

      {roles.length === 0 && (
        <EmptyState
          icon={<ShieldCheck className="h-6 w-6" />}
          title="No roles yet"
          description="Create a role to control what a teammate can see and do in this studio."
          action={<Button size="sm" onClick={() => setCreating(true)}>Add role</Button>}
        />
      )}

      {roles.map((role) => (
          <Card key={role.id} className={role.active ? undefined : 'opacity-60'}>
            <div className="flex items-start justify-between gap-4">
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <h3 className="text-sm font-bold text-zinc-900 dark:text-zinc-100">{role.name}</h3>
                  {!role.active && <Badge tone="neutral">Inactive</Badge>}
                </div>
                {role.description && (
                  <p className="mt-0.5 text-xs text-zinc-500 dark:text-zinc-400">{role.description}</p>
                )}
                <div className="mt-2 flex flex-wrap gap-1.5">
                  {role.permissionKeys.length === 0 ? (
                    <Badge tone="neutral">No access granted</Badge>
                  ) : (
                    role.permissionKeys.map((key) => {
                      const p = permissions.find((perm) => perm.key === key);
                      return <Badge key={key} tone="brand">{p?.label ?? key}</Badge>;
                    })
                  )}
                </div>
              </div>
              <div className="flex shrink-0 items-center gap-3">
                <RoleStatusToggle
                  studioId={studioId}
                  roleId={role.id}
                  roleName={role.name}
                  initialActive={role.active}
                />
                <Button size="sm" variant="outline" onClick={() => setEditingId(role.id)} leftIcon={<Pencil className="h-3.5 w-3.5" />}>
                  Edit
                </Button>
                <Button
                  size="sm"
                  variant="danger"
                  onClick={async () => {
                    if (!confirm(`Delete the "${role.name}" role? This can't be undone.`)) return;
                    setDeletingId(role.id);
                    const res = await deleteRole(studioId, role.id);
                    setDeletingId(null);
                    if (!res.ok) alert(res.error);
                  }}
                  disabled={deletingId === role.id}
                  leftIcon={<Trash2 className="h-3.5 w-3.5" />}
                >
                  Delete
                </Button>
              </div>
            </div>
          </Card>
      ))}
    </div>
  );
}

function RoleForm({
  studioId,
  roleId,
  permissions,
  initial,
  submitLabel,
  onDone,
}: {
  studioId: string;
  roleId?: string;
  permissions: Permission[];
  initial: RoleFormState;
  submitLabel: string;
  onDone: () => void;
}) {
  const [form, setForm] = useState(initial);
  const [saving, setSaving] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});

  const permissionOptions = permissions.map((p) => ({ value: p.key, label: p.label }));

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setErrors({});
    if (!form.name.trim()) {
      setErrors({ name: 'required' });
      return;
    }
    setSaving(true);
    try {
      const res = roleId
        ? await updateRole(studioId, roleId, form)
        : await createRole(studioId, form);
      if (!res.ok) {
        setErrors(res.details ?? { _: res.error });
        return;
      }
      onDone();
    } finally {
      setSaving(false);
    }
  }

  return (
      <form onSubmit={onSubmit} className="max-h-[calc(92vh-64px)] space-y-4 overflow-y-auto px-5 py-4">
        <div>
          <Label>Name</Label>
          <Input
            value={form.name}
            onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
            placeholder="e.g. Front Desk"
            invalid={!!errors.name}
          />
          <FieldError message={errors.name} />
        </div>

        <div>
          <Label>Description</Label>
          <Textarea
            value={form.description}
            onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))}
            placeholder="What this role is for"
          />
        </div>

        <div>
          <Label>Access</Label>
          <MultiSelect
            options={permissionOptions}
            value={form.permissionKeys}
            onChange={(permissionKeys) => setForm((f) => ({ ...f, permissionKeys }))}
            placeholder="Select access…"
          />
        </div>

        {errors._ && <FieldError message={errors._} />}

        <div className="flex justify-end gap-2">
          <Button type="button" variant="outline" onClick={onDone}>Cancel</Button>
          <Button type="submit" disabled={saving} suppressHydrationWarning>
            {saving ? 'Saving…' : submitLabel}
          </Button>
        </div>
      </form>
  );
}
