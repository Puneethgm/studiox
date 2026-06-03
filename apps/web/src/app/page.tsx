import Link from 'next/link';
import { ArrowRight, Sparkles } from 'lucide-react';
import { Button } from '@/components/ui/Button';

export const metadata = {
  title: 'StudioX | The Ultimate Platform for Studios',
};

export default function Home() {
  return (
    <div className="min-h-screen bg-slate-50 dark:bg-slate-950 font-sans text-slate-900 dark:text-slate-100">
      {/* ── Navigation Bar ──────────────────────── */}
      <header className="sticky top-0 z-50 w-full border-b border-slate-200/50 bg-white/70 backdrop-blur-xl dark:border-slate-800/50 dark:bg-slate-950/70">
        <div className="mx-auto flex h-20 max-w-7xl items-center justify-between px-6 lg:px-8">
          <div className="flex items-center gap-2">
            <div className="grid h-10 w-10 place-items-center rounded-xl bg-gradient-to-br from-brand-500 to-violet-600 text-white shadow-lg shadow-brand-500/20">
              <Sparkles className="h-5 w-5" />
            </div>
            <span className="text-xl font-black tracking-tight text-slate-900 dark:text-white">StudioX</span>
          </div>
          
          <nav className="hidden md:flex gap-8">
            <Link href="#features" className="text-sm font-semibold text-slate-600 hover:text-brand-600 dark:text-slate-300 dark:hover:text-brand-400 transition-colors">
              Features
            </Link>
            <Link href="/pricing" className="text-sm font-semibold text-slate-600 hover:text-brand-600 dark:text-slate-300 dark:hover:text-brand-400 transition-colors">
              Plans & Pricing
            </Link>
          </nav>

          <div className="flex items-center gap-4">
            <Link href="/login">
              <Button variant="ghost" className="hidden sm:inline-flex text-sm font-bold">
                Log in
              </Button>
            </Link>
            <Link href="/pricing">
              <Button className="shadow-lg shadow-brand-500/25 text-sm font-bold">
                Get Started
                <ArrowRight className="ml-2 h-4 w-4" />
              </Button>
            </Link>
          </div>
        </div>
      </header>

      {/* ── Hero Section ────────────────────────── */}
      <main>
        <div className="relative isolate overflow-hidden pt-14">
          <div className="absolute inset-x-0 -top-40 -z-10 transform-gpu overflow-hidden blur-3xl sm:-top-80" aria-hidden="true">
            <div className="relative left-[calc(50%-11rem)] aspect-[1155/678] w-[36.125rem] -translate-x-1/2 rotate-[30deg] bg-gradient-to-tr from-brand-200 to-brand-600 opacity-20 sm:left-[calc(50%-30rem)] sm:w-[72.1875rem]" style={{ clipPath: 'polygon(74.1% 44.1%, 100% 61.6%, 97.5% 26.9%, 85.5% 0.1%, 80.7% 2%, 72.5% 32.5%, 60.2% 62.4%, 52.4% 68.1%, 47.5% 58.3%, 45.2% 34.5%, 27.5% 76.7%, 0.1% 64.9%, 17.9% 100%, 27.6% 76.8%, 76.1% 97.7%, 74.1% 44.1%)' }}></div>
          </div>
          
          <div className="mx-auto max-w-7xl px-6 py-24 sm:py-32 lg:px-8">
            <div className="mx-auto max-w-3xl text-center">
              <h1 className="text-4xl font-black tracking-tight text-slate-900 dark:text-white sm:text-6xl">
                Scale your studio with AI-powered engagement
              </h1>
              <p className="mt-6 text-lg leading-8 text-slate-600 dark:text-slate-400">
                Automate your messaging, manage leads seamlessly across platforms, and grow your revenue with our all-in-one studio operating system.
              </p>
              <div className="mt-10 flex items-center justify-center gap-x-6">
                <Link href="/pricing">
                  <Button size="lg" className="shadow-xl shadow-brand-500/20 text-base font-bold px-8 h-14">
                    View Pricing Plans
                  </Button>
                </Link>
                <Link href="#features" className="text-sm font-semibold leading-6 text-slate-900 dark:text-slate-100 flex items-center gap-1 group">
                  Learn more <span aria-hidden="true" className="group-hover:translate-x-1 transition-transform">→</span>
                </Link>
              </div>
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}
