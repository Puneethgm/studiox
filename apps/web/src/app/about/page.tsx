import type { Metadata } from 'next';
import Link from 'next/link';
import { Building2, Globe, Award, ArrowRight, Phone, Mail, MapPin } from 'lucide-react';

export const metadata: Metadata = {
  title: 'About Us — 1HeroSocial.ai',
  description: '1 Hero Club Pte. Ltd. is a Singapore-incorporated technology company helping fitness and wellness studio owners grow faster with AI-powered lead management and automation.',
  keywords: ['about 1herosocial', 'fitness studio software company', 'Singapore gym tech', '1 hero club'],
  openGraph: {
    title: 'About Us — 1HeroSocial.ai',
    description: 'Built by studio owners, for studio owners.',
    url: 'https://1herosocial.ai/about',
  },
};

export default function AboutPage() {
  return (
    <main className="min-h-screen bg-[#FAF9F6] font-sans text-slate-800">

      {/* Nav */}
      <header className="sticky top-0 z-50 w-full border-b border-slate-200/50 bg-[#FAF9F6]/75 backdrop-blur-md">
        <div className="mx-auto flex w-full max-w-[1600px] items-center justify-between px-4 md:px-12 py-3 md:py-4">
          <Link href="/" className="flex items-center gap-2 md:gap-3">
            <img src="/logo.png" alt="1herosocial.ai Logo" className="h-9 w-9 md:h-10 md:w-10 object-contain rounded-xl shadow-lg shadow-violet-600/10" />
            <span className="text-lg md:text-xl font-black tracking-tight text-slate-900">1herosocial.ai</span>
          </Link>
          <div className="flex items-center gap-4 md:gap-6 text-sm font-semibold text-slate-600">
            <Link href="/#features" className="hidden sm:block hover:text-violet-600 transition-colors">Features</Link>
            <Link href="/pricing" className="hidden sm:block hover:text-violet-600 transition-colors">Pricing</Link>
            <Link href="/login" className="hover:text-violet-600 transition-colors">Log in</Link>
            <Link href="/pricing">
              <button className="flex items-center gap-2 bg-gradient-to-r from-violet-600 to-indigo-600 hover:from-violet-700 hover:to-indigo-700 text-white px-5 py-2.5 rounded-xl text-sm font-bold transition-all shadow-md">
                Get Started <ArrowRight className="h-4 w-4" />
              </button>
            </Link>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <div className="max-w-[1600px] mx-auto px-4 md:px-12 py-4 md:py-6 space-y-6">

        {/* Header */}
        <div className="space-y-3">
          <span className="text-violet-600 font-bold uppercase tracking-wider text-xs bg-violet-50 border border-violet-100 px-3.5 py-1.5 rounded-full inline-block">
            About Us
          </span>
          <h1 className="text-3xl sm:text-5xl font-extrabold tracking-tight text-slate-900 leading-tight">
            Built by Studio Owners,{' '}
            <span className="text-transparent bg-clip-text bg-gradient-to-r from-violet-600 to-indigo-600">
              For Studio Owners.
            </span>
          </h1>
          <p className="text-base sm:text-lg text-slate-600 max-w-2xl leading-relaxed">
            1 Hero Club Pte. Ltd. is a Singapore technology company helping fitness and wellness studio owners grow faster with AI-powered lead management, automated messaging, and member billing — all in one place.
          </p>
        </div>

        {/* Two column: Company + Contact */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">

          {/* Company Details */}
          <div className="bg-white border border-slate-200 rounded-3xl p-6 sm:p-8 space-y-5">
            <h2 className="text-xl font-black text-slate-900">Company Details</h2>
            <div className="space-y-4">
              <div className="flex items-start gap-3">
                <div className="h-9 w-9 rounded-lg bg-violet-50 border border-violet-100 flex items-center justify-center shrink-0">
                  <Building2 className="h-4 w-4 text-violet-600" />
                </div>
                <div>
                  <div className="text-xs font-bold text-slate-400 uppercase tracking-wider">Company</div>
                  <div className="font-bold text-slate-900 text-sm mt-0.5">1 Hero Club Pte. Ltd.</div>
                </div>
              </div>
              <div className="flex items-start gap-3">
                <div className="h-9 w-9 rounded-lg bg-violet-50 border border-violet-100 flex items-center justify-center shrink-0">
                  <Award className="h-4 w-4 text-violet-600" />
                </div>
                <div>
                  <div className="text-xs font-bold text-slate-400 uppercase tracking-wider">Incorporated</div>
                  <div className="font-bold text-slate-900 text-sm mt-0.5">20 June 2025 · Singapore</div>
                  <div className="text-xs text-slate-500">Exempt Private Company Limited by Shares</div>
                </div>
              </div>
              <div className="flex items-start gap-3">
                <div className="h-9 w-9 rounded-lg bg-violet-50 border border-violet-100 flex items-center justify-center shrink-0">
                  <Globe className="h-4 w-4 text-violet-600" />
                </div>
                <div>
                  <div className="text-xs font-bold text-slate-400 uppercase tracking-wider">Address</div>
                  <div className="font-bold text-slate-900 text-sm mt-0.5">461 Ang Mo Kio Avenue 2</div>
                  <div className="text-xs text-slate-500">Horizon Gardens, Singapore 567886</div>
                </div>
              </div>
            </div>
          </div>

          {/* Contact */}
          <div className="bg-white border border-slate-200 rounded-3xl p-6 sm:p-8 space-y-5">
            <h2 className="text-xl font-black text-slate-900">Get In Touch</h2>
            <div className="space-y-4">
              <a href="tel:+6582274100" className="flex items-start gap-3 group">
                <div className="h-9 w-9 rounded-lg bg-violet-50 border border-violet-100 flex items-center justify-center shrink-0 group-hover:bg-violet-100 transition-colors">
                  <Phone className="h-4 w-4 text-violet-600" />
                </div>
                <div>
                  <div className="text-xs font-bold text-slate-400 uppercase tracking-wider">Phone</div>
                  <div className="font-bold text-slate-900 text-sm mt-0.5 group-hover:text-violet-600 transition-colors">+65 8227 4100</div>
                  <div className="text-xs text-slate-500">Call or WhatsApp us</div>
                </div>
              </a>
              <a href="mailto:1herosocialai@gmail.com" className="flex items-start gap-3 group">
                <div className="h-9 w-9 rounded-lg bg-indigo-50 border border-indigo-100 flex items-center justify-center shrink-0 group-hover:bg-indigo-100 transition-colors">
                  <Mail className="h-4 w-4 text-indigo-600" />
                </div>
                <div>
                  <div className="text-xs font-bold text-slate-400 uppercase tracking-wider">Email</div>
                  <div className="font-bold text-slate-900 text-sm mt-0.5 group-hover:text-indigo-600 transition-colors">1herosocialai@gmail.com</div>
                  <div className="text-xs text-slate-500">We reply within 1 business day</div>
                </div>
              </a>
              <a href="https://maps.google.com/?q=461+Ang+Mo+Kio+Avenue+2,+Singapore+567886" target="_blank" rel="noopener noreferrer" className="flex items-start gap-3 group">
                <div className="h-9 w-9 rounded-lg bg-fuchsia-50 border border-fuchsia-100 flex items-center justify-center shrink-0 group-hover:bg-fuchsia-100 transition-colors">
                  <MapPin className="h-4 w-4 text-fuchsia-600" />
                </div>
                <div>
                  <div className="text-xs font-bold text-slate-400 uppercase tracking-wider">Location</div>
                  <div className="font-bold text-slate-900 text-sm mt-0.5 group-hover:text-fuchsia-600 transition-colors">461 Ang Mo Kio Ave 2, Singapore</div>
                  <div className="text-xs text-slate-500">Mon – Fri, 9am – 6pm SGT</div>
                </div>
              </a>
            </div>
          </div>
        </div>

        {/* CTA */}
        <div className="bg-white border border-slate-200 rounded-3xl px-8 py-10 text-center space-y-4">
          <h2 className="text-2xl sm:text-3xl font-black text-slate-900">Ready to grow your studio?</h2>
          <p className="text-slate-500 text-sm max-w-md mx-auto">Join studio owners across Singapore scaling with 1HeroSocial.</p>
          <Link href="/pricing">
            <button className="inline-flex items-center gap-2 bg-gradient-to-r from-violet-600 to-indigo-600 hover:from-violet-700 hover:to-indigo-700 text-white px-7 py-3 rounded-xl text-sm font-bold transition-all shadow-md mt-2">
              View Pricing Plans <ArrowRight className="h-4 w-4" />
            </button>
          </Link>
        </div>

      </div>

      {/* Footer */}
      <footer className="bg-slate-50 text-slate-500 border-t border-slate-200/80">
        <div className="mx-auto max-w-[1600px] px-6 md:px-12 py-12">
          <div className="flex flex-col lg:flex-row justify-between gap-10 mb-10">
            {/* Brand */}
            <div className="max-w-xs text-left">
              <div className="flex items-center gap-2.5 mb-4">
                <img src="/logo.png" alt="1herosocial.ai Logo" className="h-10 w-10 object-contain rounded-xl shadow-md" />
                <span className="text-xl font-black tracking-tight text-slate-900">1herosocial.ai</span>
              </div>
              <p className="text-sm text-slate-500 leading-relaxed">
                The modern, central workspace built for multi-location fitness and wellness studios.
              </p>
              <div className="mt-4 space-y-2 text-sm text-slate-500">
                <a href="tel:+6582274100" className="flex items-center gap-2 hover:text-violet-600 transition-colors whitespace-nowrap">
                  <span>📞</span> +65 8227 4100
                </a>
                <a href="mailto:1herosocialai@gmail.com" className="flex items-center gap-2 hover:text-violet-600 transition-colors whitespace-nowrap">
                  <span>✉</span> 1herosocialai@gmail.com
                </a>
                <a href="https://maps.google.com/?q=461+Ang+Mo+Kio+Avenue+2,+Singapore+567886" target="_blank" rel="noopener noreferrer" className="flex items-center gap-2 hover:text-violet-600 transition-colors whitespace-nowrap">
                  <span>📍</span> 461 Ang Mo Kio Ave 2, Singapore 567886
                </a>
              </div>
            </div>
            {/* Links */}
            <div className="grid grid-cols-3 gap-8 text-sm text-left">
              <div>
                <h4 className="font-bold text-slate-800 mb-4">Product</h4>
                <ul className="space-y-3 text-slate-500">
                  <li><Link href="/#features" className="hover:text-violet-600 transition-colors">Features</Link></li>
                  <li><Link href="/pricing" className="hover:text-violet-600 transition-colors">Pricing</Link></li>
                </ul>
              </div>
              <div>
                <h4 className="font-bold text-slate-800 mb-4">Company</h4>
                <ul className="space-y-3 text-slate-500">
                  <li><Link href="/about" className="hover:text-violet-600 transition-colors">About Us</Link></li>
                  <li><Link href="/login" className="hover:text-violet-600 transition-colors">Log In</Link></li>
                </ul>
              </div>
              <div>
                <h4 className="font-bold text-slate-800 mb-4">Legal</h4>
                <ul className="space-y-3 text-slate-500">
                  <li><Link href="/privacy" className="hover:text-violet-600 transition-colors">Privacy Policy</Link></li>
                  <li><Link href="/terms" className="hover:text-violet-600 transition-colors">Terms of Service</Link></li>
                </ul>
              </div>
            </div>
          </div>
          <div className="border-t border-slate-200 pt-6 flex flex-col sm:flex-row items-center justify-between gap-4 text-xs text-slate-400">
            <p>© 2026 1herosocial.ai. All rights reserved.</p>
            <p>Built for fitness & wellness studios worldwide</p>
          </div>
        </div>
      </footer>

    </main>
  );
}
