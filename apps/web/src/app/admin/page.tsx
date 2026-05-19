import { redirect } from 'next/navigation';
import { requireSession } from '@/lib/auth';

// Role-based landing: super-admin → studios list, studio-admin → their campaigns.
export default async function AdminHome() {
  const me = await requireSession();
  if (me.role === 'super_admin') {
    redirect('/admin/studios');
  }
  redirect(`/admin/studios/${me.studioId}/campaigns`);
}
