import type { Metadata } from 'next';
import './globals.css';

export const metadata: Metadata = {
  title: '1herosocial.ai',
  description: 'AI-run marketing & studio operations platform for fitness studios.',
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" suppressHydrationWarning>
      <body className="min-h-screen bg-slate-50 text-slate-900 antialiased dark:bg-slate-950 dark:text-slate-100" suppressHydrationWarning>
        {children}
      </body>
    </html>
  );
}
