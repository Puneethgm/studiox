import type { Metadata } from 'next';
import { headers } from 'next/headers';
import './globals.css';

export const metadata: Metadata = {
  title: '1herosocial.ai — Multi-Studio Gym Marketing & AI Operations',
  description: 'Automate lead streams, cross-channel messaging, member billing, and instant AI replies for fitness studios and gyms. Manage all your locations from a single dashboard.',
  keywords: [
    '1herosocial',
    '1herosocial.ai',
    'studio operations',
    'fitness studio software',
    'gym marketing automation',
    'Instagram auto-replies gym',
    'multi-studio management',
    'lead pipeline fitness',
    'gym owner CRM'
  ],
  icons: {
    icon: '/favicon.ico',
    shortcut: '/favicon.ico',
    apple: '/logo.png',
  },
  openGraph: {
    title: '1herosocial.ai — Multi-Studio Gym Marketing & AI Operations',
    description: 'Automate lead streams, cross-channel messaging, member billing, and instant AI replies for fitness studios.',
    url: 'https://1herosocial.ai',
    siteName: '1herosocial.ai',
    locale: 'en_US',
    type: 'website',
  },
  twitter: {
    card: 'summary_large_image',
    title: '1herosocial.ai — Gym Marketing & AI Operations',
    description: 'Automate lead streams, cross-channel messaging, member billing, and instant AI replies.',
  },
};

export default async function RootLayout({ children }: { children: React.ReactNode }) {
  // The nonce src/middleware.ts put on every request (platform-
  // security-hardening task 8.1) — this is the one inline script in
  // the root layout, so it needs the nonce attribute now that
  // script-src no longer carries 'unsafe-inline' in production.
  const nonce = (await headers()).get('x-nonce') ?? undefined;
  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        <script
          nonce={nonce}
          // Browsers hide a script's `nonce` attribute from DOM reads once
          // it's inserted (CSP spec — stops XSS from reading it back out),
          // so React's hydration check sees nonce="" and falsely reports a
          // mismatch against the real value that was actually rendered.
          suppressHydrationWarning
          dangerouslySetInnerHTML={{
            __html: `
              try {
                if (window.location.pathname.startsWith('/admin')) {
                  if (localStorage.getItem('theme') === 'dark' || (!('theme' in localStorage) && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
                    document.documentElement.classList.add('dark');
                  } else {
                    document.documentElement.classList.remove('dark');
                  }
                } else {
                  document.documentElement.classList.remove('dark');
                }
              } catch (_) {}
            `,
          }}
        />
      </head>
      <body className="min-h-screen bg-slate-50 text-slate-900 antialiased dark:bg-slate-950 dark:text-slate-100" suppressHydrationWarning>
        {children}
      </body>
    </html>
  );
}
