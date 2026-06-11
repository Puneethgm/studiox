"use client";

import React, { useState, useEffect } from 'react';
import Link from 'next/link';
import { ReviewForm } from '@/components/ReviewForm';
import { ReviewsCarousel } from '@/components/ReviewsCarousel';
import {
  ArrowRight,
  Sparkles,
  Building2,
  MessageSquare,
  Users,
  BarChart3,
  Zap,
  Globe,
  CheckCircle,
  Star,
  ChevronDown,
  Activity,
  DollarSign,
  Target,
  Award,
  ChevronRight,
  TrendingUp,
  Inbox,
  ShieldCheck,
  Calendar,
  Lock
} from 'lucide-react';

interface SimulatedLead {
  id: string;
  name: string;
  source: 'Facebook' | 'Instagram' | 'TikTok' | 'WhatsApp';
  time: string;
  status: 'New Lead' | 'AI Responding' | 'Trial Booked' | 'Member Sold';
  message: string;
}

const INITIAL_SIMULATED_LEADS: SimulatedLead[] = [
  {
    id: '1',
    name: 'Jia Min',
    source: 'Instagram',
    time: 'Just now',
    status: 'Member Sold',
    message: 'I want to sign up for the Gold membership plan!'
  },
  {
    id: '2',
    name: 'Marcus Tan',
    source: 'Facebook',
    time: '2m ago',
    status: 'Trial Booked',
    message: 'Booked a trial class for tomorrow at 6 PM.'
  },
  {
    id: '3',
    name: 'Nurul Huda',
    source: 'WhatsApp',
    time: '5m ago',
    status: 'AI Responding',
    message: 'Is there a parking space near your fitness studio?'
  }
];

const LEAD_NAMES = ['Marcus Tan', 'Siti Aminah', 'Shermin Lim', 'Arjun Prasad', 'Jia Min', 'Ryan Teo', 'Nurul Huda', 'Farhan Rahim', 'Wei Jie', 'Priya Devi'];
const LEAD_SOURCES = ['Facebook', 'Instagram', 'TikTok', 'WhatsApp'] as const;
const LEAD_MESSAGES = [
  'How much is the 10-class trial pass?',
  'Can I bring a friend to my first class?',
  'Do you have beginner classes for yoga?',
  'What are your operating hours on weekends?',
  'I would like to book a private tour of the gym.',
  'Do you provide lockboxes and towels?'
];
const LEAD_STATUSES = ['New Lead', 'AI Responding', 'Trial Booked', 'Member Sold'] as const;

export default function Home() {
  // Live Lead Simulator State
  const [simulatedLeads, setSimulatedLeads] = useState<SimulatedLead[]>(INITIAL_SIMULATED_LEADS);

  // Interactive Calculator State
  const [adSpend, setAdSpend] = useState<number>(1500);
  const [conversionRate, setConversionRate] = useState<number>(15); // in percentage
  const [memberLifetimeMonths, setMemberLifetimeMonths] = useState<number>(9);
  const [monthlyMembershipFee, setMonthlyMembershipFee] = useState<number>(120);

  // FAQ Accordion State
  const [openFaqIndex, setOpenFaqIndex] = useState<number | null>(null);

  // Review Form State
  const [isReviewFormOpen, setIsReviewFormOpen] = useState(false);
  const [reviewsRefreshKey, setReviewsRefreshKey] = useState(0);

  // Live Lead Stream Simulator Interval
  useEffect(() => {
    const interval = setInterval(() => {
      const randomName = LEAD_NAMES[Math.floor(Math.random() * LEAD_NAMES.length)] || 'Anonymous';
      const randomSource = LEAD_SOURCES[Math.floor(Math.random() * LEAD_SOURCES.length)] || 'WhatsApp';
      const randomStatus = LEAD_STATUSES[Math.floor(Math.random() * LEAD_STATUSES.length)] || 'New Lead';
      const randomMessage = LEAD_MESSAGES[Math.floor(Math.random() * LEAD_MESSAGES.length)] || 'Hello!';
      
      const newLead: SimulatedLead = {
        id: Math.random().toString(),
        name: randomName,
        source: randomSource,
        time: 'Just now',
        status: randomStatus,
        message: randomMessage
      };

      setSimulatedLeads(prev => {
        const updated = [newLead, ...prev.map(l => {
          if (l.time === 'Just now') return { ...l, time: '1m ago' };
          if (l.time === '1m ago') return { ...l, time: '3m ago' };
          return { ...l, time: '5m ago' };
        })];
        return updated.slice(0, 4); // Keep last 4 leads
      });
    }, 4500);

    return () => clearInterval(interval);
  }, []);

  // ROI Calculations
  const leadsGenerated = Math.floor(adSpend / 15); // Assuming $15 per lead avg
  const memberConversions = Math.floor(leadsGenerated * (conversionRate / 100));
  const newMonthlyRevenue = memberConversions * monthlyMembershipFee;
  const lifetimeValueWon = memberConversions * monthlyMembershipFee * memberLifetimeMonths;
  const netROI = adSpend > 0 ? ((lifetimeValueWon - adSpend) / adSpend) * 100 : 0;

  const toggleFaq = (index: number) => {
    setOpenFaqIndex(openFaqIndex === index ? null : index);
  };

  return (
    <div className="min-h-screen bg-[#FAF9F6] font-sans text-slate-800 selection:bg-violet-600/10 selection:text-violet-900 overflow-x-hidden">
      
      {/* Dynamic Background Glows */}
      <div className="absolute top-0 right-0 -z-10 h-[800px] w-[800px] rounded-full bg-violet-100/50 blur-[130px] pointer-events-none" />
      <div className="absolute top-[600px] -left-10 -z-10 h-[700px] w-[700px] rounded-full bg-indigo-100/40 blur-[120px] pointer-events-none" />
      <div className="absolute bottom-[800px] right-10 -z-10 h-[900px] w-[900px] rounded-full bg-fuchsia-100/30 blur-[150px] pointer-events-none" />

      {/* ── Navigation (Full Width) ─────────────────── */}
      <header className="sticky top-0 z-50 w-full border-b border-slate-200/50 bg-[#FAF9F6]/75 backdrop-blur-md">
        <div className="mx-auto flex w-full max-w-[1600px] items-center justify-between px-4 md:px-12 py-3 md:py-4">
          <div className="flex items-center gap-2 md:gap-3">
            <div className="grid h-9 w-9 md:h-10 md:w-10 place-items-center rounded-xl bg-gradient-to-br from-violet-600 to-indigo-600 text-white shadow-lg shadow-violet-600/10">
              <Sparkles className="h-4.5 w-4.5 md:h-5 md:w-5" />
            </div>
            <span className="text-lg md:text-xl font-black tracking-tight text-slate-900">1herosocial.ai</span>
          </div>

          <nav className="hidden lg:flex items-center gap-10">
            <a href="#simulator" className="text-sm font-semibold text-slate-600 hover:text-violet-600 transition-colors">Lead Stream</a>
            <a href="#calculator" className="text-sm font-semibold text-slate-600 hover:text-violet-600 transition-colors">ROI Calculator</a>
            <a href="#features" className="text-sm font-semibold text-slate-600 hover:text-violet-600 transition-colors">Platform Features</a>
            <Link href="/pricing" className="text-sm font-semibold text-slate-600 hover:text-violet-600 transition-colors">Pricing Plans</Link>
          </nav>

          <div className="flex items-center gap-4 md:gap-6">
            <Link href="/login" className="text-sm font-semibold text-slate-600 hover:text-violet-600 transition-colors">
              Log in
            </Link>
            <Link href="/pricing" className="hidden sm:inline-block">
              <button className="flex items-center gap-2 bg-gradient-to-r from-violet-600 to-indigo-600 hover:from-violet-700 hover:to-indigo-700 text-white px-5 py-2.5 rounded-xl text-sm font-bold transition-all shadow-md shadow-violet-600/10 hover:shadow-violet-600/20 hover:scale-[1.02] active:scale-[0.98]">
                Get Started <ArrowRight className="h-4 w-4" />
              </button>
            </Link>
          </div>
        </div>
      </header>

      {/* ── Hero Split Layout (Wide Viewport) ───────── */}
      <section className="relative pt-12 pb-16 md:pt-28 md:pb-32 px-4 md:px-12 max-w-[1600px] mx-auto">
        <div className="grid grid-cols-1 lg:grid-cols-12 gap-10 lg:gap-8 items-center">
          
          {/* Left Text Column */}
          <div className="lg:col-span-7 space-y-6 md:space-y-8 text-left">
            <div className="inline-flex items-center gap-2 bg-violet-600/5 border border-violet-600/10 text-violet-700 text-xs font-bold px-4 py-2 rounded-full backdrop-blur-sm">
              <Star className="h-3.5 w-3.5 fill-violet-600 text-violet-600" />
              Empowering Multi-Studio Gym Owners Worldwide
            </div>

            <h1 className="text-3xl sm:text-5xl lg:text-6xl xl:text-7xl font-extrabold tracking-tight text-slate-900 leading-[1.1] lg:leading-[1.05]">
              One Command Center.
              <span className="block mt-1 md:mt-2 text-transparent bg-clip-text bg-gradient-to-r from-violet-600 via-fuchsia-600 to-indigo-600">
                Every Studio Location.
              </span>
            </h1>

            <p className="text-base sm:text-lg md:text-xl text-slate-600 max-w-2xl leading-relaxed">
              Stop switching between tabs. Centralize lead pipelines, automate cross-channel messaging, manage member billing, and trigger instant AI replies across all locations.
            </p>

            <div className="flex flex-col sm:flex-row items-stretch sm:items-center gap-4">
              <Link href="/pricing">
                <button className="w-full sm:w-auto flex items-center justify-center gap-2 bg-gradient-to-r from-violet-600 to-indigo-600 hover:from-violet-700 hover:to-indigo-700 text-white px-8 py-4 rounded-xl text-base font-bold transition-all shadow-lg shadow-violet-600/20 hover:shadow-violet-600/30 hover:-translate-y-0.5 active:translate-y-0">
                  View Pricing Plans <ArrowRight className="h-4 w-4" />
                </button>
              </Link>
              <a href="#simulator" className="flex items-center justify-center gap-2 text-base font-bold text-violet-600 hover:text-violet-800 transition-colors py-3">
                Watch Live Simulator <ArrowRight className="h-4 w-4" />
              </a>
            </div>

            {/* Benefit Checkmarks */}
            <div className="flex flex-wrap gap-x-8 gap-y-3 pt-2 md:pt-4 text-sm text-slate-500 font-semibold">
              <div className="flex items-center gap-2">
                <CheckCircle className="h-4.5 w-4.5 text-violet-600" />
                Setup in under 5 minutes
              </div>
              <div className="flex items-center gap-2">
                <CheckCircle className="h-4.5 w-4.5 text-violet-600" />
                Enterprise security & support
              </div>
              <div className="flex items-center gap-2">
                <CheckCircle className="h-4.5 w-4.5 text-violet-600" />
                Multi-location ready
              </div>
            </div>
          </div>

          {/* Right Hero Image/Graphic Column */}
          <div className="lg:col-span-5 relative">
            <div className="absolute inset-0 bg-gradient-to-br from-violet-600/10 to-indigo-600/10 rounded-3xl blur-2xl -z-10" />
            <div className="bg-white border border-slate-200 rounded-3xl p-4 sm:p-6 shadow-2xl relative overflow-hidden">
              <div className="flex justify-between items-center border-b border-slate-100 pb-4 mb-4">
                <div className="flex items-center gap-2">
                  <span className="h-3 w-3 rounded-full bg-red-400" />
                  <span className="h-3 w-3 rounded-full bg-yellow-400" />
                  <span className="h-3 w-3 rounded-full bg-green-400" />
                  <span className="text-xs font-bold text-slate-400 ml-2">Studio Command Hub</span>
                </div>
                <span className="text-[10px] bg-violet-50 text-violet-600 font-bold px-2 py-0.5 rounded border border-violet-100">Location A</span>
              </div>

              {/* Graphical representation of channels */}
              <div className="space-y-3.5">
                <div className="flex justify-between items-center bg-slate-50 p-3 rounded-xl border border-slate-200/50">
                  <div className="flex items-center gap-3">
                    <div className="h-8 w-8 rounded-lg bg-emerald-500 text-white flex items-center justify-center font-bold">W</div>
                    <div>
                      <div className="text-xs font-bold text-slate-800">WhatsApp Broadcast</div>
                      <div className="text-[10px] text-slate-500">Auto-reply template active</div>
                    </div>
                  </div>
                  <span className="text-[10px] text-emerald-600 font-bold bg-emerald-50 border border-emerald-100 px-2 py-0.5 rounded-full">Connected</span>
                </div>

                <div className="flex justify-between items-center bg-slate-50 p-3 rounded-xl border border-slate-200/50">
                  <div className="flex items-center gap-3">
                    <div className="h-8 w-8 rounded-lg bg-blue-600 text-white flex items-center justify-center font-bold">F</div>
                    <div>
                      <div className="text-xs font-bold text-slate-800">Facebook Messenger</div>
                      <div className="text-[10px] text-slate-500">2 leads queued today</div>
                    </div>
                  </div>
                  <span className="text-[10px] text-blue-600 font-bold bg-blue-50 border border-blue-100 px-2 py-0.5 rounded-full">Connected</span>
                </div>

                <div className="flex justify-between items-center bg-slate-50 p-3 rounded-xl border border-slate-200/50">
                  <div className="flex items-center gap-3">
                    <div className="h-8 w-8 rounded-lg bg-gradient-to-tr from-purple-500 via-pink-500 to-red-500 text-white flex items-center justify-center font-bold">I</div>
                    <div>
                      <div className="text-xs font-bold text-slate-800">Instagram DM</div>
                      <div className="text-[10px] text-slate-500">AI worker polling active</div>
                    </div>
                  </div>
                  <span className="text-[10px] text-purple-600 font-bold bg-purple-50 border border-purple-100 px-2 py-0.5 rounded-full">Connected</span>
                </div>
              </div>

              {/* Graph Preview */}
              <div className="mt-4 pt-4 border-t border-slate-100 flex items-center justify-between">
                <div>
                  <div className="text-[10px] text-slate-400 uppercase font-bold">Monthly Conversions</div>
                  <div className="text-2xl font-black text-slate-900">+48 Members</div>
                </div>
                <div className="h-10 w-24 bg-gradient-to-tr from-violet-100 to-fuchsia-50 rounded-lg flex items-end p-1 justify-around border border-violet-100/50">
                  <div className="w-2.5 h-3 bg-violet-400 rounded-sm" />
                  <div className="w-2.5 h-5 bg-violet-400 rounded-sm" />
                  <div className="w-2.5 h-4 bg-violet-400 rounded-sm" />
                  <div className="w-2.5 h-7 bg-violet-500 rounded-sm" />
                  <div className="w-2.5 h-6 bg-violet-600 rounded-sm animate-pulse" />
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ── Live Lead Stream Simulator (Split Column) ── */}
      <section id="simulator" className="border-t border-slate-200/60 bg-white py-16 md:py-24 px-4 md:px-12">
        <div className="max-w-[1600px] mx-auto grid grid-cols-1 lg:grid-cols-12 gap-10 lg:gap-12 items-start">
          
          {/* Left Text Column */}
          <div className="lg:col-span-4 text-left space-y-6 lg:sticky lg:top-24">
            <div className="text-violet-600 font-bold uppercase tracking-wider text-xs bg-violet-50 border border-violet-100 px-3.5 py-1.5 rounded-full inline-block">
              Conversational Engine
            </div>
            <h2 className="text-3xl sm:text-5xl font-black text-slate-900 tracking-tight leading-tight">
              Real-Time Lead & AI Activity
            </h2>
            <p className="text-slate-600 text-base sm:text-lg leading-relaxed">
              Watch how incoming queries across Instagram, Facebook, and WhatsApp are instantly handled by our AI pipeline. FAQs are scanned dynamically to draft replies and trigger conversions instantly.
            </p>
            <div className="pt-4 border-t border-slate-100 space-y-4">
              <div className="flex items-center gap-3">
                <div className="h-9 w-9 bg-emerald-50 rounded-lg flex items-center justify-center text-emerald-600 font-bold text-sm">99%</div>
                <span className="text-sm text-slate-600 font-semibold">Response speed under 5 seconds</span>
              </div>
              <div className="flex items-center gap-3">
                <div className="h-9 w-9 bg-violet-50 rounded-lg flex items-center justify-center text-violet-600 font-bold text-sm">3.4x</div>
                <span className="text-sm text-slate-600 font-semibold">Increase in booking rate vs manual typing</span>
              </div>
            </div>
          </div>

          {/* Right Simulator Card Column */}
          <div className="lg:col-span-8 bg-slate-50 border border-slate-200/60 rounded-3xl p-4 sm:p-6 shadow-sm relative">
            <div className="absolute top-4 right-4 flex items-center gap-2 bg-emerald-50 border border-emerald-200 text-emerald-700 text-xs px-3.5 py-1.5 rounded-full font-bold">
              <span className="h-2 w-2 rounded-full bg-emerald-500 animate-ping" />
              Live Feed Simulating
            </div>
            
            <div className="flex items-center gap-2.5 mb-6 border-b border-slate-200/60 pb-4">
              <Activity className="h-5 w-5 text-violet-600" />
              <h3 className="font-bold text-slate-900 text-lg">Central Lead Stream</h3>
            </div>

            <div className="space-y-4">
              {simulatedLeads.map((lead) => (
                <div 
                  key={lead.id} 
                  className="bg-white border border-slate-200/60 rounded-2xl p-4 sm:p-5 flex flex-col md:flex-row md:items-center justify-between gap-4 transition-all hover:border-violet-300 hover:shadow-md hover:shadow-violet-600/[0.02]"
                >
                  <div className="flex items-start gap-4">
                    <div className={`h-11 w-11 rounded-xl flex items-center justify-center font-black text-white shadow-sm bg-gradient-to-br ${
                      lead.source === 'Instagram' ? 'from-purple-500 via-pink-500 to-red-500' :
                      lead.source === 'Facebook' ? 'from-blue-600 to-blue-800' :
                      lead.source === 'TikTok' ? 'from-black via-gray-900 to-cyan-500' :
                      'from-emerald-500 to-green-600'
                    }`}>
                      {lead.source[0]}
                    </div>
                    <div className="text-left">
                      <div className="flex items-center flex-wrap gap-2">
                        <span className="font-bold text-slate-900">{lead.name}</span>
                        <span className="text-xs text-slate-400">{lead.time}</span>
                        <span className={`text-[10px] uppercase font-bold tracking-wider px-2 py-0.5 rounded-full ${
                          lead.source === 'Instagram' ? 'bg-purple-50 text-purple-700 border border-purple-100' :
                          lead.source === 'Facebook' ? 'bg-blue-50 text-blue-700 border border-blue-100' :
                          lead.source === 'TikTok' ? 'bg-slate-100 text-slate-800 border border-slate-200' :
                          'bg-emerald-50 text-emerald-700 border border-emerald-100'
                        }`}>{lead.source}</span>
                      </div>
                      <p className="text-sm text-slate-600 mt-1.5 italic font-medium">"{lead.message}"</p>
                    </div>
                  </div>

                  <div className="flex items-center gap-4 self-end md:self-center">
                    <span className={`text-xs font-bold px-3 py-1.5 rounded-lg border ${
                      lead.status === 'New Lead' ? 'bg-blue-50 text-blue-700 border-blue-100' :
                      lead.status === 'AI Responding' ? 'bg-yellow-50 text-yellow-700 border-yellow-200 animate-pulse' :
                      lead.status === 'Trial Booked' ? 'bg-purple-50 text-purple-700 border-purple-100' :
                      'bg-emerald-50 text-emerald-700 border-emerald-100'
                    }`}>
                      {lead.status}
                    </span>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      </section>

      {/* ── Interactive ROI & Pipeline Calculator (Split Column) ── */}
      <section id="calculator" className="border-t border-slate-200/60 py-16 md:py-24 px-4 md:px-12">
        <div className="max-w-[1600px] mx-auto grid grid-cols-1 lg:grid-cols-12 gap-10 lg:gap-12 items-start">
          
          {/* Left Text Column */}
          <div className="lg:col-span-4 text-left space-y-6 lg:sticky lg:top-24">
            <div className="text-indigo-600 font-bold uppercase tracking-wider text-xs bg-indigo-50 border border-indigo-100 px-3.5 py-1.5 rounded-full inline-block">
              Revenue Forecast
            </div>
            <h2 className="text-3xl sm:text-5xl font-black text-slate-900 tracking-tight leading-tight">
              Quantify Your Studio Growth
            </h2>
            <p className="text-slate-600 text-base sm:text-lg leading-relaxed">
              Adjust the spend and conversion sliders to forecast the potential revenue boost from deploying automated conversational nurturing.
            </p>
            <div className="pt-4 border-t border-slate-100 space-y-4">
              <div className="flex items-center gap-3">
                <CheckCircle className="h-5 w-5 text-indigo-600" />
                <span className="text-sm text-slate-600 font-semibold">Reduce WhatsApp drop-off by 60%</span>
              </div>
              <div className="flex items-center gap-3">
                <CheckCircle className="h-5 w-5 text-indigo-600" />
                <span className="text-sm text-slate-600 font-semibold">Reclaim ad spend ROI instantly</span>
              </div>
            </div>
          </div>

          {/* Right Calculator Card Column */}
          <div className="lg:col-span-8 grid grid-cols-1 md:grid-cols-3 gap-6 items-stretch">
            
            {/* Controls */}
            <div className="md:col-span-2 bg-white border border-slate-200 rounded-3xl p-4 sm:p-6 flex flex-col justify-between gap-6 shadow-sm">
              <div>
                <div className="flex justify-between items-center mb-3">
                  <label className="text-sm font-bold text-slate-700">Monthly Ad Spend</label>
                  <span className="text-violet-600 font-black text-base">${adSpend.toLocaleString()}</span>
                </div>
                <input 
                  type="range" 
                  min="500" 
                  max="10000" 
                  step="250"
                  value={adSpend} 
                  onChange={(e) => setAdSpend(Number(e.target.value))}
                  className="w-full h-2 bg-slate-100 rounded-lg appearance-none cursor-pointer accent-violet-600"
                />
                <div className="flex justify-between text-[10px] text-slate-400 font-semibold mt-1.5">
                  <span>$500</span>
                  <span>$10,000</span>
                </div>
              </div>

              <div>
                <div className="flex justify-between items-center mb-3">
                  <label className="text-sm font-bold text-slate-700">Lead-to-Member Conversion Rate</label>
                  <span className="text-fuchsia-600 font-black text-base">{conversionRate}%</span>
                </div>
                <input 
                  type="range" 
                  min="2" 
                  max="40" 
                  value={conversionRate} 
                  onChange={(e) => setConversionRate(Number(e.target.value))}
                  className="w-full h-2 bg-slate-100 rounded-lg appearance-none cursor-pointer accent-fuchsia-600"
                />
                <div className="flex justify-between text-[10px] text-slate-400 font-semibold mt-1.5">
                  <span>2% (Industry Low)</span>
                  <span>40% (AI-Optimized)</span>
                </div>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <label className="text-xs font-bold text-slate-500 block mb-1.5">Monthly Membership Fee</label>
                  <div className="relative">
                    <span className="absolute left-3.5 top-2.5 text-slate-400 text-sm font-bold">$</span>
                    <input 
                      type="number"
                      value={monthlyMembershipFee}
                      onChange={(e) => setMonthlyMembershipFee(Math.max(0, Number(e.target.value)))}
                      className="w-full bg-slate-50 border border-slate-200 rounded-xl py-2.5 pl-8 pr-3.5 text-sm text-slate-900 font-bold focus:outline-none focus:border-violet-600 focus:bg-white"
                    />
                  </div>
                </div>

                <div>
                  <label className="text-xs font-bold text-slate-500 block mb-1.5">Member Retention (Months)</label>
                  <div className="relative">
                    <input 
                      type="number"
                      value={memberLifetimeMonths}
                      onChange={(e) => setMemberLifetimeMonths(Math.max(1, Number(e.target.value)))}
                      className="w-full bg-slate-50 border border-slate-200 rounded-xl py-2.5 px-3.5 text-sm text-slate-900 font-bold focus:outline-none focus:border-violet-600 focus:bg-white"
                    />
                  </div>
                </div>
              </div>
            </div>

            {/* Results Display */}
            <div className="bg-gradient-to-br from-violet-500 to-indigo-600 text-white rounded-3xl p-4 sm:p-6 flex flex-col justify-between shadow-xl relative overflow-hidden">
              <div className="absolute top-0 right-0 -translate-y-4 translate-x-4 h-24 w-24 rounded-full bg-white/10 blur-xl pointer-events-none" />
              
              <div>
                <span className="text-[10px] uppercase font-extrabold tracking-wider bg-white/20 text-white px-3 py-1.5 rounded-full border border-white/20">
                  Pipeline Forecast
                </span>
                
                <div className="mt-6 space-y-4">
                  <div className="flex justify-between border-b border-white/10 pb-2">
                    <span className="text-sm text-violet-100">Leads Generated</span>
                    <span className="font-extrabold text-white text-base">{leadsGenerated}</span>
                  </div>
                  <div className="flex justify-between border-b border-white/10 pb-2">
                    <span className="text-sm text-violet-100">Conversions</span>
                    <span className="font-extrabold text-white text-base">{memberConversions}</span>
                  </div>
                  <div className="flex justify-between border-b border-white/10 pb-2">
                    <span className="text-sm text-violet-100">Monthly Added MRR</span>
                    <span className="font-black text-emerald-300 text-base">+${newMonthlyRevenue.toLocaleString()}</span>
                  </div>
                </div>
              </div>

              <div className="mt-8 border-t border-white/10 pt-4">
                <div className="text-[10px] text-violet-200 uppercase tracking-wider font-extrabold">Total Pipeline LTV</div>
                <div className="text-3xl font-black text-white mt-1">
                  ${lifetimeValueWon.toLocaleString()}
                </div>
                <div className="flex items-center gap-1.5 text-xs text-emerald-300 mt-2 font-bold">
                  <TrendingUp className="h-4 w-4" />
                  <span>+{netROI.toFixed(0)}% ROI Impact</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ── Platform Features (Extreme Bento Grid Layout) ──── */}
      <section id="features" className="border-t border-slate-200/60 bg-white py-16 md:py-24 px-4 md:px-12">
        <div className="max-w-[1600px] mx-auto space-y-16">
          <div className="text-left space-y-4">
            <span className="text-violet-600 font-bold uppercase tracking-wider text-xs bg-violet-50 border border-violet-100 px-3.5 py-1.5 rounded-full inline-block">
              Core Capabilities
            </span>
            <h2 className="text-3xl sm:text-5xl font-black text-slate-900 tracking-tight">
              One Workspace. Unlimited Growth.
            </h2>
            <p className="text-slate-600 text-base sm:text-lg max-w-2xl">
              Eliminate software bloat. Control your multi-location fitness studio workflow through unified communications, analytics, and billing modules.
            </p>
          </div>

          {/* Bento Grid */}
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            
            {/* Box 1 (Span 2 columns) */}
            <div className="md:col-span-2 p-5 sm:p-8 rounded-3xl bg-slate-50 border border-slate-200 hover:border-violet-400 transition-all group flex flex-col justify-between">
              <div>
                <div className="h-12 w-12 rounded-xl bg-violet-600/5 text-violet-600 border border-violet-600/10 flex items-center justify-center mb-6 group-hover:scale-105 transition-transform duration-200">
                  <Building2 className="h-6 w-6" />
                </div>
                <h3 className="text-xl font-bold text-slate-900 mb-2">Multi-Studio Isolated Command Architecture</h3>
                <p className="text-slate-600 leading-relaxed text-sm max-w-xl">
                  Manage independent databases, customized pricing charts, messaging channels, and client pipelines for multiple physical locations under a single overarching owner account. Keep operations separated cleanly.
                </p>
              </div>
              <div className="mt-8 flex gap-2">
                <span className="text-[10px] bg-slate-200/50 text-slate-600 font-bold px-2.5 py-1 rounded">Location Mapping</span>
                <span className="text-[10px] bg-slate-200/50 text-slate-600 font-bold px-2.5 py-1 rounded">Isolated CRM</span>
              </div>
            </div>

            {/* Box 2 (Span 1 column) */}
            <div className="p-5 sm:p-8 rounded-3xl bg-slate-50 border border-slate-200 hover:border-violet-400 transition-all group flex flex-col justify-between">
              <div>
                <div className="h-12 w-12 rounded-xl bg-indigo-600/5 text-indigo-600 border border-indigo-600/10 flex items-center justify-center mb-6 group-hover:scale-105 transition-transform duration-200">
                  <MessageSquare className="h-6 w-6" />
                </div>
                <h3 className="text-xl font-bold text-slate-900 mb-2">Intelligent RAG Answers</h3>
                <p className="text-slate-600 leading-relaxed text-sm">
                  Upload PDF or text FAQs. The AI searches matches with vector search capabilities to draft accurate studio answers.
                </p>
              </div>
              <div className="mt-8 flex gap-2">
                <span className="text-[10px] bg-indigo-50 text-indigo-600 font-bold px-2.5 py-1 rounded border border-indigo-100">Vector Embeddings</span>
              </div>
            </div>

            {/* Box 3 (Span 1 column) */}
            <div className="p-5 sm:p-8 rounded-3xl bg-slate-50 border border-slate-200 hover:border-violet-400 transition-all group flex flex-col justify-between">
              <div>
                <div className="h-12 w-12 rounded-xl bg-fuchsia-600/5 text-fuchsia-600 border border-fuchsia-600/10 flex items-center justify-center mb-6 group-hover:scale-105 transition-transform duration-200">
                  <Users className="h-6 w-6" />
                </div>
                <h3 className="text-xl font-bold text-slate-900 mb-2">Visual Deal Pipelines</h3>
                <p className="text-slate-600 leading-relaxed text-sm">
                  Drag and drop lead cards across custom stages to monitor class booking progressions.
                </p>
              </div>
              <div className="mt-8 flex gap-2">
                <span className="text-[10px] bg-fuchsia-50 text-fuchsia-700 font-bold px-2.5 py-1 rounded border border-fuchsia-100">CRM Pipelines</span>
              </div>
            </div>

            {/* Box 4 (Span 2 columns) */}
            <div className="md:col-span-2 p-5 sm:p-8 rounded-3xl bg-slate-50 border border-slate-200 hover:border-violet-400 transition-all group flex flex-col justify-between">
              <div>
                <div className="h-12 w-12 rounded-xl bg-emerald-600/5 text-emerald-600 border border-emerald-600/10 flex items-center justify-center mb-6 group-hover:scale-105 transition-transform duration-200">
                  <Globe className="h-6 w-6" />
                </div>
                <h3 className="text-xl font-bold text-slate-900 mb-2">Seamless Stripe Billing Integration</h3>
                <p className="text-slate-600 leading-relaxed text-sm max-w-xl">
                  Connect Stripe accounts to handle member payments, sell trial packages, and orchestrate monthly recurring subscriptions directly from the centralized lead feed. No additional software required.
                </p>
              </div>
              <div className="mt-8 flex gap-2">
                <span className="text-[10px] bg-emerald-50 text-emerald-700 font-bold px-2.5 py-1 rounded border border-emerald-100">Trial Invoicing</span>
                <span className="text-[10px] bg-emerald-50 text-emerald-700 font-bold px-2.5 py-1 rounded border border-emerald-100">Autopay Schedules</span>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ── Testimonials (3-Column Layout) ──────────── */}
      <section className="border-t border-slate-200/60 py-16 md:py-24 px-4 md:px-12">
        <div className="max-w-[1600px] mx-auto space-y-16">
          <div className="text-left space-y-4">
            <span className="text-indigo-600 font-bold uppercase tracking-wider text-xs bg-indigo-50 border border-indigo-100 px-3.5 py-1.5 rounded-full inline-block">
              Success Stories
            </span>
            <h2 className="text-3xl sm:text-5xl font-black text-slate-900 tracking-tight">
              Loved by Fitness Entrepreneurs
            </h2>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-8">
            {[
              {
                quote: "Our lead-to-trial booking conversion rate shot up from 8% to 26% within the first month. The RAG AI WhatsApp answers are incredibly realistic.",
                author: "Elena Rostova",
                role: "Founder, Zenith Pilates (3 locations)"
              },
              {
                quote: "The ability to manage Stripe subscriptions and outbox leads across both our studios in a single window has saved us 15+ admin hours every week.",
                author: "Devon Carter",
                role: "Owner, Iron Oak Gyms"
              },
              {
                quote: "Finally, an AI assistant that actually reads our studio FAQ documents and books class passes directly without making mistakes. Absolute game-changer.",
                author: "Nisha Patel",
                role: "Director, Prana Yoga Collective"
              }
            ].map((t, idx) => (
              <div key={idx} className="bg-white border border-slate-200 rounded-3xl p-5 sm:p-6 flex flex-col justify-between shadow-sm hover:shadow-md transition-shadow">
                <div>
                  <div className="flex gap-1 mb-5 text-amber-400">
                    {[...Array(5)].map((_, i) => (
                      <Star key={i} className="h-4.5 w-4.5 fill-current" />
                    ))}
                  </div>
                  <p className="text-slate-600 italic text-sm sm:text-base leading-relaxed">"{t.quote}"</p>
                </div>
                <div className="mt-8 pt-4 border-t border-slate-100 text-left">
                  <div className="font-extrabold text-slate-900 text-sm sm:text-base">{t.author}</div>
                  <div className="text-xs text-violet-600 font-bold mt-1">{t.role}</div>
                </div>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── FAQ Section (Split Column Layout) ───────── */}
      <section className="border-t border-slate-200/60 bg-white py-16 md:py-24 px-4 md:px-12">
        <div className="max-w-[1600px] mx-auto grid grid-cols-1 lg:grid-cols-12 gap-10 lg:gap-12 items-start">
          
          {/* Left Text Column */}
          <div className="lg:col-span-4 text-left space-y-6 lg:sticky lg:top-24">
            <span className="text-violet-600 font-bold uppercase tracking-wider text-xs bg-violet-50 border border-violet-100 px-3.5 py-1.5 rounded-full inline-block">
              Support Hub
            </span>
            <h2 className="text-3xl sm:text-5xl font-black text-slate-900 tracking-tight leading-tight">
              Frequently Asked Questions
            </h2>
            <p className="text-slate-600 text-base sm:text-lg">
              Have questions about platform capabilities or pricing? Find rapid answers here.
            </p>
          </div>

          {/* Right Accordion Column */}
          <div className="lg:col-span-8 space-y-4">
            {[
              {
                q: "Can I manage multiple isolated studios with a single subscription?",
                a: "Yes! The platform is designed from the ground up for multi-studio owners. You can add, configure, and review multiple studio locations using one central account. Each studio holds its own independent pipeline databases, messaging channels, and billing details."
              },
              {
                q: "How does the RAG-based AI auto-responder work?",
                a: "You simply upload your studio's FAQ text files (like membership rates, class schedules, parking details, etc.) to the Studio Knowledge Base. Our AI scans these files in real-time using pgvector embedding searches to answer lead queries with precise accuracy."
              },
              {
                q: "Does it integrate with existing WhatsApp Business numbers?",
                a: "Absolutely. You can link your Meta WhatsApp Business API credentials inside your studio settings to allow the platform's AI worker to handle all incoming queries on your official WhatsApp line."
              },
              {
                q: "Can I cancel or switch subscription plans later?",
                a: "Yes, you can easily downgrade, upgrade, or cancel your subscription at any time directly through your billing portal page."
              }
            ].map((faq, idx) => (
              <div 
                key={idx} 
                className="bg-[#FAF9F6] border border-slate-200 rounded-2xl overflow-hidden transition-all hover:bg-slate-50/50"
              >
                <button 
                  onClick={() => toggleFaq(idx)}
                  className="w-full py-4 px-5 sm:py-5 sm:px-6 flex items-center justify-between text-left focus:outline-none"
                >
                  <span className="font-bold text-slate-900 text-sm sm:text-base">{faq.q}</span>
                  <ChevronDown className={`h-4.5 w-4.5 text-slate-400 transition-transform duration-300 ${openFaqIndex === idx ? 'rotate-180 text-violet-600' : ''}`} />
                </button>
                
                <div 
                  className={`transition-all duration-300 ease-in-out overflow-hidden ${
                    openFaqIndex === idx ? 'max-h-48 border-t border-slate-200/60 bg-white' : 'max-h-0'
                  }`}
                >
                  <p className="p-5 sm:p-6 text-sm sm:text-base text-slate-600 leading-relaxed text-left">{faq.a}</p>
                </div>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── Premium High-Impact CTA (Full Width) ────── */}
      <section className="relative overflow-hidden bg-gradient-to-br from-violet-950 to-indigo-900 py-20 sm:py-32 px-4 sm:px-6 text-center text-white">
        <div className="absolute inset-0 z-0 opacity-10">
          <img 
            src="https://images.unsplash.com/photo-1540497077202-7c8a3999166f?q=80&w=2000&auto=format&fit=crop" 
            alt="Gym background" 
            className="w-full h-full object-cover grayscale" 
          />
        </div>
        <div className="absolute inset-0 z-0 bg-gradient-to-b from-transparent via-[#09070F]/20 to-[#09070F]/50" />
        
        <div className="relative z-10 mx-auto max-w-4xl px-6">
          <h2 className="text-3xl sm:text-5xl lg:text-6xl font-black mb-6 leading-tight">
            Supercharge Your Lead Pipeline
          </h2>
          <p className="text-lg md:text-xl text-violet-200 mb-10 max-w-2xl mx-auto leading-relaxed">
            Join other growth-minded studio owners scaling their operations with automated multi-channel messaging and smart RAG conversions.
          </p>
          <Link href="/pricing">
            <button className="inline-flex items-center gap-3 bg-white text-violet-950 hover:bg-violet-50 px-10 py-5 rounded-2xl text-lg font-bold transition-all shadow-2xl hover:shadow-white/10 hover:-translate-y-1 hover:scale-[1.02]">
              Choose Your Plan & Scale <ArrowRight className="h-5 w-5" />
            </button>
          </Link>
        </div>
      </section>

      {/* ── Reviews Section ──────────────────────────────────── */}
      <section className="bg-white dark:bg-slate-900 border-t border-slate-200/80 py-16">
        <div className="mx-auto max-w-[1600px] px-6 md:px-12">
          <div className="text-center mb-12">
            <h2 className="text-4xl font-black text-slate-900 dark:text-white mb-4">Client Reviews</h2>
            <p className="text-slate-600 dark:text-slate-400 text-lg">Hear what our clients say about 1HeroSocial</p>
          </div>
          <ReviewsCarousel key={reviewsRefreshKey} onAddReview={() => setIsReviewFormOpen(true)} />
        </div>
      </section>

      <ReviewForm
        isOpen={isReviewFormOpen}
        onClose={() => setIsReviewFormOpen(false)}
        onSuccess={() => {
          setReviewsRefreshKey(prev => prev + 1);
        }}
      />

      {/* ── Footer ──────────────────────────────────── */}
      <footer className="bg-slate-50 text-slate-500 border-t border-slate-200/80">
        <div className="mx-auto max-w-[1600px] px-6 md:px-12 py-16">
          <div className="flex flex-col lg:flex-row justify-between gap-12 mb-12">
            {/* Brand */}
            <div className="max-w-xs text-left">
              <div className="flex items-center gap-2.5 mb-6">
                <div className="grid h-10 w-10 place-items-center rounded-xl bg-gradient-to-br from-violet-600 to-indigo-600 text-white shadow-md">
                  <Sparkles className="h-5 w-5" />
                </div>
                <span className="text-xl font-black tracking-tight text-slate-900">1herosocial.ai</span>
              </div>
              <p className="text-sm text-slate-500 leading-relaxed">
                The modern, central workspace built for multi-location fitness and wellness studios to optimize lead conversions.
              </p>
              
              {/* Social Icons */}
              <div className="flex items-center gap-4 mt-8">
                <a href="#" aria-label="Instagram" className="text-slate-400 hover:text-violet-600 transition-colors">
                  <svg viewBox="0 0 24 24" fill="currentColor" className="h-5 w-5"><path d="M12 2.163c3.204 0 3.584.012 4.85.07 3.252.148 4.771 1.691 4.919 4.919.058 1.265.069 1.645.069 4.849 0 3.205-.012 3.584-.069 4.849-.149 3.225-1.664 4.771-4.919 4.919-1.266.058-1.644.07-4.85.07-3.204 0-3.584-.012-4.849-.07-3.26-.149-4.771-1.699-4.919-4.92-.058-1.265-.07-1.644-.07-4.849 0-3.204.013-3.583.07-4.849.149-3.227 1.664-4.771 4.919-4.919 1.266-.057 1.645-.069 4.849-.069zm0-2.163c-3.259 0-3.667.014-4.947.072-4.358.2-6.78 2.618-6.98 6.98-.059 1.281-.073 1.689-.073 4.948 0 3.259.014 3.668.072 4.948.2 4.358 2.618 6.78 6.98 6.98 1.281.058 1.689.072 4.948.072 3.259 0 3.668-.014 4.948-.072 4.354-.2 6.782-2.618 6.979-6.98.059-1.28.073-1.689.073-4.948 0-3.259-.014-3.667-.072-4.947-.196-4.354-2.617-6.78-6.979-6.98-1.281-.059-1.69-.073-4.949-.073zm0 5.838c-3.403 0-6.162 2.759-6.162 6.162s2.759 6.163 6.162 6.163 6.162-2.759 6.162-6.163c0-3.403-2.759-6.162-6.162-6.162zm0 10.162c-2.209 0-4-1.79-4-4 0-2.209 1.791-4 4-4s4 1.791 4 4c0 2.21-1.791 4-4 4zm6.406-11.845c-.796 0-1.441.645-1.441 1.44s.645 1.44 1.441 1.44c.795 0 1.439-.645 1.439-1.44s-.644-1.44-1.439-1.44z"/></svg>
                </a>
                <a href="https://www.facebook.com/share/1DC9m6HpZ8/?mibextid=wwXIfr" target="_blank" rel="noopener noreferrer" aria-label="Facebook" className="text-slate-400 hover:text-violet-600 transition-colors">
                  <svg viewBox="0 0 24 24" fill="currentColor" className="h-5 w-5"><path d="M24 12.073c0-6.627-5.373-12-12-12s-12 5.373-12 12c0 5.99 4.388 10.954 10.125 11.854v-8.385H7.078v-3.47h3.047V9.43c0-3.007 1.792-4.669 4.533-4.669 1.312 0 2.686.235 2.686.235v2.953H15.83c-1.491 0-1.956.925-1.956 1.874v2.25h3.328l-.532 3.47h-2.796v8.385C19.612 23.027 24 18.062 24 12.073z"/></svg>
                </a>
                <a href="https://www.tiktok.com/@bft_tamanjurong?_r=1&_t=ZS-96tiNHuE5UC" target="_blank" rel="noopener noreferrer" aria-label="TikTok" className="text-slate-400 hover:text-violet-600 transition-colors">
                  <svg viewBox="0 0 24 24" fill="currentColor" className="h-5 w-5">
                    <path d="M12.525.02c1.31-.02 2.61-.01 3.91-.02.08 1.53.63 3.02 1.59 4.18.96 1.15 2.27 1.95 3.73 2.28-.01 1.42-.02 2.83-.02 4.25-1.58-.02-3.13-.53-4.42-1.46-.86-.62-1.56-1.44-2.06-2.39v7.94c0 1.25-.26 2.47-.79 3.59-.53 1.12-1.31 2.08-2.28 2.81-.97.74-2.11 1.26-3.32 1.51-1.21.25-2.47.24-3.68-.04-1.2-.28-2.32-.86-3.26-1.68-.94-.82-1.65-1.87-2.09-3.04C-.03 16.48-.09 15.2.1 13.98c.19-1.22.68-2.37 1.43-3.35.75-.98 1.74-1.74 2.87-2.21 1.13-.47 2.37-.67 3.58-.57.01 1.48.01 2.97.02 4.45-.6-.08-1.22-.03-1.8.14-.58.17-1.11.49-1.52.93-.42.44-.7 1-.82 1.6-.12.6-.07 1.23.14 1.8.21.57.58 1.07 1.07 1.44.49.37 1.08.59 1.7.63.62.04 1.24-.09 1.79-.38.56-.29 1.02-.73 1.34-1.28.32-.55.48-1.18.47-1.82V.02z" />
                  </svg>
                </a>
                <a href="#" aria-label="X (Twitter)" className="text-slate-400 hover:text-violet-600 transition-colors">
                  <svg viewBox="0 0 24 24" fill="currentColor" className="h-5 w-5"><path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-4.714-6.231-5.401 6.231H2.744l7.737-8.835L1.254 2.25H8.08l4.259 5.631 5.905-5.631zm-1.161 17.52h1.833L7.084 4.126H5.117z"/></svg>
                </a>
              </div>
            </div>

            {/* Sitemap */}
            <div className="grid grid-cols-2 sm:grid-cols-3 gap-8 text-sm text-left">
              <div>
                <h4 className="font-bold text-slate-800 mb-4">Product</h4>
                <ul className="space-y-3 text-slate-500">
                  <li><a href="#features" className="hover:text-violet-600 transition-colors">Features</a></li>
                  <li><Link href="/pricing" className="hover:text-violet-600 transition-colors">Pricing</Link></li>
                  <li><a href="#simulator" className="hover:text-violet-600 transition-colors">Lead Stream</a></li>
                </ul>
              </div>
              <div>
                <h4 className="font-bold text-slate-800 mb-4">Account</h4>
                <ul className="space-y-3 text-slate-500">
                  <li><Link href="/login" className="hover:text-violet-600 transition-colors">Log In</Link></li>
                  <li><button
                    onClick={() => setIsReviewFormOpen(true)}
                    className="text-slate-500 hover:text-violet-600 transition-colors text-left"
                  >
                    Leave a Review
                  </button></li>
                </ul>
              </div>
              <div>
                <h4 className="font-bold text-slate-800 mb-4">Legal</h4>
                <ul className="space-y-3 text-slate-500">
                  <li><Link href="/privacy" className="hover:text-violet-600 transition-colors">Privacy Policy</Link></li>
                  <li><Link href="/terms" className="hover:text-violet-600 transition-colors">Terms of Service</Link></li>
                  <li><Link href="/delete-account" className="hover:text-violet-600 transition-colors">Delete Account</Link></li>
                </ul>
              </div>
            </div>
          </div>

          <div className="border-t border-slate-200 pt-8 flex flex-col sm:flex-row items-center justify-between gap-4 text-xs text-slate-400">
            <p>© 2026 1herosocial.ai. All rights reserved.</p>
            <p>Built for fitness & wellness studios worldwide</p>
          </div>
        </div>
      </footer>
    </div>
  );
}
