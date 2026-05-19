import { AppShell } from '@/components/AppShell';
import { requireSession } from '@/lib/auth';

export default async function AdminLayout({ children }: { children: React.ReactNode }) {
  const me = await requireSession();
  return <AppShell me={me}>{children}</AppShell>;
}
