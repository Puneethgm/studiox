import type { Metadata } from 'next';

export const metadata: Metadata = {
  title: 'Login — 1herosocial.ai',
  description: 'Log in to your 1herosocial.ai account to manage your fitness studio leads, messaging, and operations from a single dashboard.',
  robots: 'noindex, follow',
};

export default function LoginLayout({ children }: { children: React.ReactNode }) {
  return children;
}
