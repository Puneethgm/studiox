import type { Metadata } from 'next';
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

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        <script
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
