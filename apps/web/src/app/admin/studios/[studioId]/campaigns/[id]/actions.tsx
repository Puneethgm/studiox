'use client';

import { useRouter } from 'next/navigation';
import { useState } from 'react';
import { Button } from '@/components/ui/Button';
import { api } from '@/lib/api';

export function CampaignActions({
  studioId,
  id,
  active,
}: {
  studioId: string;
  id: string;
  active: boolean;
}) {
  const router = useRouter();
  const [pending, setPending] = useState(false);

  async function toggle() {
    setPending(true);
    try {
      await api(`/api/v1/studios/${studioId}/campaigns/${id}`, {
        method: 'PATCH',
        json: { active: !active },
      });
      router.refresh();
    } finally {
      setPending(false);
    }
  }

  return (
    <Button variant={active ? 'outline' : 'primary'} onClick={toggle} loading={pending}>
      {active ? 'Deactivate' : 'Activate'}
    </Button>
  );
}
