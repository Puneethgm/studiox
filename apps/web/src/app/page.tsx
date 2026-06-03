import Link from 'next/link';
import { ArrowRight, Sparkles, Building2, MessageSquare, Users, BarChart3, Zap, Globe, CheckCircle, Star } from 'lucide-react';

export const metadata = {
  title: '1herosocial.ai | Multi-Studio Management Platform',
  description: 'Run multiple fitness & wellness studios from one powerful AI platform. Automate leads, messaging, bookings, and billing across all your locations.',
};

export default function Home() {
  return (
    <div className="min-h-screen bg-[#FAF9F7] font-sans text-[#1A1A1A]">

      {/* ── Navigation ──────────────────────────────── */}
      <header className="sticky top-0 z-50 w-full border-b border-[#EAE8E2] bg-[#FAF9F7]/80 backdrop-blur-xl">
        <div className="mx-auto flex h-18 max-w-7xl items-center justify-between px-6 lg:px-8 py-4">
          <div className="flex items-center gap-2.5">
            <div className="grid h-9 w-9 place-items-center rounded-xl bg-gradient-to-br from-violet-600 to-indigo-700 text-white shadow-md">
              <Sparkles className="h-4 w-4" />
            </div>
            <span className="text-lg font-black tracking-tight text-[#1A1A1A]">1herosocial.ai</span>
          </div>

          <nav className="hidden md:flex items-center gap-8">
            <a href="#features" className="text-sm font-medium text-[#525252] hover:text-violet-600 transition-colors">Features</a>
            <a href="#how-it-works" className="text-sm font-medium text-[#525252] hover:text-violet-600 transition-colors">How it works</a>
            <Link href="/pricing" className="text-sm font-medium text-[#525252] hover:text-violet-600 transition-colors">Pricing</Link>
          </nav>

          <div className="flex items-center gap-4">
            <Link href="/login" className="hidden sm:inline-flex text-sm font-medium text-[#525252] hover:text-violet-600 transition-colors">
              Log in
            </Link>
            <Link href="/pricing">
              <button className="flex items-center gap-2 bg-violet-600 text-white hover:bg-violet-700 px-4 py-2 rounded-lg text-sm font-medium transition-all shadow-md shadow-violet-600/20 hover:shadow-violet-600/40">
                Get Started <ArrowRight className="h-3.5 w-3.5" />
              </button>
            </Link>
          </div>
        </div>
      </header>

      {/* ── Hero ────────────────────────────────────── */}
      <section className="relative overflow-hidden pt-24 pb-20">
        <div className="absolute inset-0 -z-20 opacity-[0.05]">
          <img src="https://images.unsplash.com/photo-1534438327276-14e5300c3a48?q=80&w=2000&auto=format&fit=crop" alt="Fitness Studio Background" className="w-full h-full object-cover" />
        </div>
        <div className="absolute inset-0 -z-10">
          <div className="absolute top-0 left-1/2 -translate-x-1/2 w-[900px] h-[500px] bg-gradient-to-b from-violet-200/50 via-indigo-100/40 to-transparent rounded-full blur-3xl" />
        </div>

        <div className="mx-auto max-w-7xl px-6 lg:px-8 text-center">
          <div className="inline-flex items-center gap-2 bg-violet-50 border border-violet-200 text-violet-700 text-xs font-semibold px-3 py-1.5 rounded-full mb-8 shadow-sm">
            <Star className="h-3 w-3 fill-violet-500 text-violet-500" />
            Built for fitness & wellness studios
          </div>

          <h1 className="text-5xl sm:text-7xl font-black tracking-tight text-[#1A1A1A] max-w-4xl mx-auto leading-[1.05]">
            One platform.
            <span className="block text-transparent bg-clip-text bg-gradient-to-r from-violet-600 to-indigo-600">
              All your studios.
            </span>
          </h1>

          <p className="mt-8 text-xl text-[#525252] max-w-2xl mx-auto leading-relaxed">
            Manage multiple studio locations from a single dashboard. Automate lead follow-ups, WhatsApp messaging, member billing, and AI conversations — all in one place.
          </p>

          <div className="mt-12 flex flex-col sm:flex-row items-center justify-center gap-4">
            <Link href="/pricing">
              <button className="flex items-center gap-2 bg-violet-600 text-white hover:bg-violet-700 px-8 py-4 rounded-xl text-base font-semibold transition-all shadow-xl shadow-violet-600/25 hover:shadow-2xl hover:shadow-violet-600/40 hover:-translate-y-0.5">
                Get Started <ArrowRight className="h-4 w-4" />
              </button>
            </Link>
            <a href="#how-it-works" className="flex items-center gap-2 text-sm font-semibold text-violet-600 hover:text-violet-800 transition-colors">
              See how it works <ArrowRight className="h-3.5 w-3.5" />
            </a>
          </div>

          {/* Social proof */}
          <div className="mt-14 flex flex-wrap items-center justify-center gap-8 text-sm text-[#525252] font-medium">
            <div className="flex items-center gap-2">
              <CheckCircle className="h-4 w-4 text-violet-600" />
              No credit card required
            </div>
            <div className="flex items-center gap-2">
              <CheckCircle className="h-4 w-4 text-violet-600" />
              Setup in minutes
            </div>
            <div className="flex items-center gap-2">
              <CheckCircle className="h-4 w-4 text-violet-600" />
              Cancel anytime
            </div>
          </div>
        </div>
      </section>

      {/* ── Multi-Studio Visual ─────────────────────── */}
      <section className="mx-auto max-w-7xl px-6 lg:px-8 pb-24">
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-6">
          {[
            { name: 'Downtown Gym', members: 142, leads: 23, color: 'from-violet-500 to-indigo-600' },
            { name: 'Westside Yoga', members: 89, leads: 11, color: 'from-fuchsia-500 to-purple-600' },
            { name: 'North Boxing Club', members: 210, leads: 38, color: 'from-blue-500 to-violet-600' },
          ].map((studio) => (
            <div key={studio.name} className="bg-white/80 backdrop-blur-sm border border-[#EAE8E2] rounded-2xl p-6 shadow-sm hover:shadow-xl hover:shadow-violet-900/5 transition-all hover:-translate-y-1">
              <div className={`h-12 w-12 rounded-xl bg-gradient-to-br ${studio.color} mb-4 flex items-center justify-center text-white font-black text-lg shadow-inner`}>
                {studio.name[0]}
              </div>
              <h3 className="font-bold text-[#1A1A1A] mb-4 text-lg">{studio.name}</h3>
              <div className="flex gap-6 text-sm">
                <div>
                  <div className="text-3xl font-black text-[#1A1A1A]">{studio.members}</div>
                  <div className="text-[#737373] text-xs mt-1 font-medium uppercase tracking-wider">Members</div>
                </div>
                <div>
                  <div className="text-3xl font-black text-violet-600">{studio.leads}</div>
                  <div className="text-[#737373] text-xs mt-1 font-medium uppercase tracking-wider">Active Leads</div>
                </div>
              </div>
              <div className="mt-5 h-2 bg-[#F0EDE8] rounded-full overflow-hidden">
                <div className={`h-full bg-gradient-to-r ${studio.color} rounded-full`} style={{ width: `${(studio.leads / studio.members) * 100 * 3}%` }} />
              </div>
            </div>
          ))}
        </div>
        <p className="text-center text-sm font-medium text-violet-600 mt-6">↑ Manage all your locations from one unified dashboard</p>
      </section>

      {/* ── Features ────────────────────────────────── */}
      <section id="features" className="relative bg-white border-y border-[#EAE8E2] overflow-hidden">
        <div className="absolute inset-0 -z-10 opacity-[0.02]">
          <img src="https://images.unsplash.com/photo-1571019614242-c5c5dee9f50b?q=80&w=2000&auto=format&fit=crop" alt="Features Background" className="w-full h-full object-cover" />
        </div>
        
        <div className="mx-auto max-w-7xl px-6 lg:px-8 py-24">
          <div className="text-center mb-16">
            <h2 className="text-4xl font-black text-[#1A1A1A] tracking-tight">Everything your studios need</h2>
            <p className="mt-4 text-lg text-[#525252] max-w-xl mx-auto">From first contact to loyal member — automate the entire journey.</p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {[
              {
                icon: Building2,
                color: 'bg-violet-50 text-violet-600',
                title: 'Multi-Studio Management',
                desc: 'Run unlimited studio locations from one account. Each studio has its own leads, members, billing, and AI settings.'
              },
              {
                icon: MessageSquare,
                color: 'bg-fuchsia-50 text-fuchsia-600',
                title: 'WhatsApp AI Automation',
                desc: 'AI-powered auto-replies handle lead enquiries 24/7. Intelligent follow-ups that feel personal, not robotic.'
              },
              {
                icon: Users,
                color: 'bg-indigo-50 text-indigo-600',
                title: 'Lead Pipeline & CRM',
                desc: 'Track every lead from first message to signed member. Never let a hot lead go cold again.'
              },
              {
                icon: Zap,
                color: 'bg-purple-50 text-purple-600',
                title: 'Automated Follow-Ups',
                desc: 'Set up drip sequences that automatically nurture leads over days and weeks without manual effort.'
              },
              {
                icon: BarChart3,
                color: 'bg-pink-50 text-pink-600',
                title: 'Social Media Planner',
                desc: 'Schedule and publish content across Facebook, Instagram, and more — all from one calendar view.'
              },
              {
                icon: Globe,
                color: 'bg-violet-50 text-violet-700',
                title: 'Stripe Billing & Plans',
                desc: 'Sell memberships and trial passes directly. Automated receipts, subscription management, and payment tracking.'
              },
            ].map((f) => (
              <div key={f.title} className="p-8 rounded-3xl bg-white/60 backdrop-blur-md border border-[#EAE8E2] hover:border-violet-200 hover:shadow-xl hover:shadow-violet-900/5 transition-all group">
                <div className={`h-14 w-14 rounded-2xl ${f.color} flex items-center justify-center mb-6 group-hover:scale-110 transition-transform duration-300`}>
                  <f.icon className="h-6 w-6" />
                </div>
                <h3 className="text-xl font-bold text-[#1A1A1A] mb-3">{f.title}</h3>
                <p className="text-[#525252] leading-relaxed">{f.desc}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── How it works ────────────────────────────── */}
      <section id="how-it-works" className="mx-auto max-w-7xl px-6 lg:px-8 py-32">
        <div className="text-center mb-20">
          <h2 className="text-4xl font-black text-[#1A1A1A] tracking-tight">Up and running in 3 steps</h2>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-12">
          {[
            { step: '01', title: 'Create your studios', desc: 'Add each location, set your brand colors, and connect your WhatsApp business number.' },
            { step: '02', title: 'Connect your channels', desc: 'Link Facebook, Instagram, Google Ads, and Stripe. Your leads flow in automatically.' },
            { step: '03', title: 'Let AI do the work', desc: 'AI handles first responses, follow-ups, and bookings while you focus on running great classes.' },
          ].map((s) => (
            <div key={s.step} className="flex flex-col gap-4">
              <div className="text-5xl font-black text-violet-100 mb-2">{s.step}</div>
              <h3 className="text-xl font-bold text-[#1A1A1A]">{s.title}</h3>
              <p className="text-[#525252] leading-relaxed">{s.desc}</p>
            </div>
          ))}
        </div>
      </section>

      {/* ── Premium CTA ─────────────────────────────────────── */}
      <section className="relative overflow-hidden bg-violet-950">
        {/* Unsplash Background Image */}
        <div className="absolute inset-0 z-0 opacity-40">
          <img 
            src="https://images.unsplash.com/photo-1540497077202-7c8a3999166f?q=80&w=2000&auto=format&fit=crop" 
            alt="Gym background" 
            className="w-full h-full object-cover grayscale-[30%]" 
          />
        </div>
        {/* Dark Premium Gradient Overlay */}
        <div className="absolute inset-0 z-0 bg-gradient-to-br from-violet-950/90 via-indigo-950/80 to-[#0F0C29]/95" />
        
        <div className="mx-auto max-w-4xl px-6 lg:px-8 py-32 text-center relative z-10">
          <h2 className="text-4xl sm:text-5xl md:text-6xl font-black tracking-tight text-white mb-8 leading-tight">
            Ready to scale your studios?
          </h2>
          <p className="text-lg md:text-xl text-violet-200 mb-12 max-w-2xl mx-auto leading-relaxed">
            Join studio owners using 1herosocial.ai to convert more leads, retain more members, and grow faster.
          </p>
          <Link href="/pricing">
            <button className="inline-flex items-center gap-3 bg-white text-violet-900 hover:bg-violet-50 px-10 py-5 rounded-2xl text-lg font-bold transition-all shadow-2xl hover:shadow-white/20 hover:-translate-y-1">
              View Plans & Pricing <ArrowRight className="h-5 w-5" />
            </button>
          </Link>
        </div>
      </section>

      {/* ── Footer ──────────────────────────────────── */}
      <footer className="bg-[#1A1A1A] text-white">
        <div className="mx-auto max-w-7xl px-6 lg:px-8 py-20">
          {/* Top row */}
          <div className="flex flex-col md:flex-row justify-between gap-12 mb-16">
            {/* Brand */}
            <div className="max-w-xs">
              <div className="flex items-center gap-2.5 mb-6">
                <div className="grid h-10 w-10 place-items-center rounded-xl bg-gradient-to-br from-violet-500 to-indigo-500 text-white shadow-md">
                  <Sparkles className="h-5 w-5" />
                </div>
                <span className="text-xl font-black tracking-tight">1herosocial.ai</span>
              </div>
              <p className="text-sm text-gray-400 leading-relaxed">
                The all-in-one platform for fitness & wellness studio owners to manage leads, automate messaging, and grow faster.
              </p>
              {/* Social icons */}
              <div className="flex items-center gap-4 mt-8">
                {/* Instagram */}
                <a href="#" aria-label="Instagram" className="text-gray-400 hover:text-violet-400 transition-colors">
                  <svg viewBox="0 0 24 24" fill="currentColor" className="h-6 w-6"><path d="M12 2.163c3.204 0 3.584.012 4.85.07 3.252.148 4.771 1.691 4.919 4.919.058 1.265.069 1.645.069 4.849 0 3.205-.012 3.584-.069 4.849-.149 3.225-1.664 4.771-4.919 4.919-1.266.058-1.644.07-4.85.07-3.204 0-3.584-.012-4.849-.07-3.26-.149-4.771-1.699-4.919-4.92-.058-1.265-.07-1.644-.07-4.849 0-3.204.013-3.583.07-4.849.149-3.227 1.664-4.771 4.919-4.919 1.266-.057 1.645-.069 4.849-.069zm0-2.163c-3.259 0-3.667.014-4.947.072-4.358.2-6.78 2.618-6.98 6.98-.059 1.281-.073 1.689-.073 4.948 0 3.259.014 3.668.072 4.948.2 4.358 2.618 6.78 6.98 6.98 1.281.058 1.689.072 4.948.072 3.259 0 3.668-.014 4.948-.072 4.354-.2 6.782-2.618 6.979-6.98.059-1.28.073-1.689.073-4.948 0-3.259-.014-3.667-.072-4.947-.196-4.354-2.617-6.78-6.979-6.98-1.281-.059-1.69-.073-4.949-.073zm0 5.838c-3.403 0-6.162 2.759-6.162 6.162s2.759 6.163 6.162 6.163 6.162-2.759 6.162-6.163c0-3.403-2.759-6.162-6.162-6.162zm0 10.162c-2.209 0-4-1.79-4-4 0-2.209 1.791-4 4-4s4 1.791 4 4c0 2.21-1.791 4-4 4zm6.406-11.845c-.796 0-1.441.645-1.441 1.44s.645 1.44 1.441 1.44c.795 0 1.439-.645 1.439-1.44s-.644-1.44-1.439-1.44z"/></svg>
                </a>
                {/* Facebook */}
                <a href="https://www.facebook.com/share/1DC9m6HpZ8/?mibextid=wwXIfr" target="_blank" rel="noopener noreferrer" aria-label="Facebook" className="text-gray-400 hover:text-violet-400 transition-colors">
                  <svg viewBox="0 0 24 24" fill="currentColor" className="h-6 w-6"><path d="M24 12.073c0-6.627-5.373-12-12-12s-12 5.373-12 12c0 5.99 4.388 10.954 10.125 11.854v-8.385H7.078v-3.47h3.047V9.43c0-3.007 1.792-4.669 4.533-4.669 1.312 0 2.686.235 2.686.235v2.953H15.83c-1.491 0-1.956.925-1.956 1.874v2.25h3.328l-.532 3.47h-2.796v8.385C19.612 23.027 24 18.062 24 12.073z"/></svg>
                </a>
                {/* TikTok */}
                <a href="https://www.tiktok.com/@bft_tamanjurong?_r=1&_t=ZS-96tiNHuE5UC" target="_blank" rel="noopener noreferrer" aria-label="TikTok" className="text-gray-400 hover:text-violet-400 transition-colors">
                  <svg viewBox="0 0 24 24" fill="currentColor" className="h-6 w-6">
                    <path d="M12.525.02c1.31-.02 2.61-.01 3.91-.02.08 1.53.63 3.02 1.59 4.18.96 1.15 2.27 1.95 3.73 2.28-.01 1.42-.02 2.83-.02 4.25-1.58-.02-3.13-.53-4.42-1.46-.86-.62-1.56-1.44-2.06-2.39v7.94c0 1.25-.26 2.47-.79 3.59-.53 1.12-1.31 2.08-2.28 2.81-.97.74-2.11 1.26-3.32 1.51-1.21.25-2.47.24-3.68-.04-1.2-.28-2.32-.86-3.26-1.68-.94-.82-1.65-1.87-2.09-3.04C-.03 16.48-.09 15.2.1 13.98c.19-1.22.68-2.37 1.43-3.35.75-.98 1.74-1.74 2.87-2.21 1.13-.47 2.37-.67 3.58-.57.01 1.48.01 2.97.02 4.45-.6-.08-1.22-.03-1.8.14-.58.17-1.11.49-1.52.93-.42.44-.7 1-.82 1.6-.12.6-.07 1.23.14 1.8.21.57.58 1.07 1.07 1.44.49.37 1.08.59 1.7.63.62.04 1.24-.09 1.79-.38.56-.29 1.02-.73 1.34-1.28.32-.55.48-1.18.47-1.82V.02z" />
                  </svg>
                </a>
                {/* X / Twitter */}
                <a href="#" aria-label="X (Twitter)" className="text-gray-400 hover:text-violet-400 transition-colors">
                  <svg viewBox="0 0 24 24" fill="currentColor" className="h-6 w-6"><path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-4.714-6.231-5.401 6.231H2.744l7.737-8.835L1.254 2.25H8.08l4.259 5.631 5.905-5.631zm-1.161 17.52h1.833L7.084 4.126H5.117z"/></svg>
                </a>
                {/* YouTube */}
                <a href="#" aria-label="YouTube" className="text-gray-400 hover:text-violet-400 transition-colors">
                  <svg viewBox="0 0 24 24" fill="currentColor" className="h-6 w-6"><path d="M23.498 6.186a3.016 3.016 0 0 0-2.122-2.136C19.505 3.545 12 3.545 12 3.545s-7.505 0-9.377.505A3.017 3.017 0 0 0 .502 6.186C0 8.07 0 12 0 12s0 3.93.502 5.814a3.016 3.016 0 0 0 2.122 2.136c1.871.505 9.376.505 9.376.505s7.505 0 9.377-.505a3.015 3.015 0 0 0 2.122-2.136C24 15.93 24 12 24 12s0-3.93-.502-5.814zM9.545 15.568V8.432L15.818 12l-6.273 3.568z"/></svg>
                </a>
              </div>
            </div>

            {/* Links */}
            <div className="grid grid-cols-2 sm:grid-cols-3 gap-8 text-sm">
              <div>
                <h4 className="font-bold text-white mb-6">Platform</h4>
                <ul className="space-y-4 text-gray-400">
                  <li><a href="#features" className="hover:text-violet-400 transition-colors">Features</a></li>
                  <li><Link href="/pricing" className="hover:text-violet-400 transition-colors">Pricing</Link></li>
                  <li><a href="#how-it-works" className="hover:text-violet-400 transition-colors">How it works</a></li>
                </ul>
              </div>
              <div>
                <h4 className="font-bold text-white mb-6">Account</h4>
                <ul className="space-y-4 text-gray-400">
                  <li><Link href="/login" className="hover:text-violet-400 transition-colors">Log in</Link></li>
                </ul>
              </div>
              <div>
                <h4 className="font-bold text-white mb-6">Legal</h4>
                <ul className="space-y-4 text-gray-400">
                  <li><a href="#" className="hover:text-violet-400 transition-colors">Privacy Policy</a></li>
                  <li><a href="#" className="hover:text-violet-400 transition-colors">Terms of Service</a></li>
                </ul>
              </div>
            </div>
          </div>

          {/* Bottom bar */}
          <div className="border-t border-gray-800 pt-8 flex flex-col sm:flex-row items-center justify-between gap-4">
            <p className="text-sm text-gray-500">© 2025 1herosocial.ai. All rights reserved.</p>
            <p className="text-sm text-gray-500">Built for fitness & wellness studios worldwide 🌏</p>
          </div>
        </div>
      </footer>
    </div>
  );
}
