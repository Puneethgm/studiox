import type { Metadata } from 'next';

export const metadata: Metadata = {
  title: '1herosocial.ai — AI Marketing & Operations for Multi-Studio Gyms',
  description: 'Centralize lead pipelines, automate cross-channel messaging, manage member billing, and deploy AI-powered replies across all your fitness studio locations from one dashboard.',
  keywords: [
    'gym software',
    'fitness studio management',
    'multi-location gym software',
    'AI lead management',
    'gym CRM',
    'WhatsApp auto-reply gym',
    'Instagram DM management',
    'lead pipeline fitness',
    'gym marketing automation'
  ],
};

export default function HomeLayout({ children }: { children: React.ReactNode }) {
  return children;
}
