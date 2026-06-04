"use client";

import React, { useState, useEffect } from 'react';
import Link from 'next/link';
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
  TrendingUp
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
    name: 'Sarah Jenkins',
    source: 'Instagram',
    time: 'Just now',
    status: 'Member Sold',
    message: 'I want to sign up for the Gold membership plan!'
  },
  {
    id: '2',
    name: 'Marcus Chen',
    source: 'Facebook',
    time: '2m ago',
    status: 'Trial Booked',
    message: 'Booked a trial class for tomorrow at 6 PM.'
  },
  {
    id: '3',
    name: 'Aisha Rahman',
    source: 'WhatsApp',
    time: '5m ago',
    status: 'AI Responding',
    message: 'Is there a parking space near your fitness studio?'
  }
];

const LEAD_NAMES = ['Chloe Smith', 'David Miller', 'Emily Davis', 'James Wilson', 'Jessica Taylor', 'Ryan Garcia', 'Sophia Martinez', 'Daniel Kim'];
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
    <div className="min-h-screen bg-[#FAF9F6] font-sans text-slate-800 selection:bg-violet-600/10 selection:text-violet-900">
      
      {/* Background Soft Glows */}
      <div className="absolute top-0 left-1/4 -z-10 h-[600px] w-[600px] rounded-full bg-violet-100/60 blur-[100px] pointer-events-none" />
      <div className="absolute top-[800px] right-1/4 -z-10 h-[600px] w-[600px] rounded-full bg-indigo-100/50 blur-[100px] pointer-events-none" />
      <div className="absolute bottom-[400px] left-1/3 -z-10 h-[700px] w-[700px] rounded-full bg-fuchsia-100/40 blur-[120px] pointer-events-none" />

      {/* ── Navigation ──────────────────────────────── */}
      <header className="sticky top-0 z-50 w-full border-b border-slate-200/60 bg-[#FAF9F6]/80 backdrop-blur-xl">
        <div className="mx-auto flex h-18 max-w-7xl items-center justify-between px-6 lg:px-8 py-4">
          <div className="flex items-center gap-2.5">
            <div className="grid h-9 w-9 place-items-center rounded-xl bg-gradient-to-br from-violet-600 to-fuchsia-600 text-white shadow-md shadow-violet-600/10">
              <Sparkles className="h-4 w-4" />
            </div>
            <span className="text-lg font-black tracking-tight text-slate-900">1herosocial.ai</span>
          </div>

          <nav className="hidden md:flex items-center gap-8">
            <a href="#features" className="text-sm font-medium text-slate-600 hover:text-violet-600 transition-colors">Features</a>
            <a href="#simulator" className="text-sm font-medium text-slate-600 hover:text-violet-600 transition-colors">Lead Simulator</a>
            <a href="#calculator" className="text-sm font-medium text-slate-600 hover:text-violet-600 transition-colors">ROI Calculator</a>
            <Link href="/pricing" className="text-sm font-medium text-slate-600 hover:text-violet-600 transition-colors">Pricing</Link>
          </nav>

          <div className="flex items-center gap-4">
            <Link href="/login" className="text-sm font-medium text-slate-600 hover:text-violet-600 transition-colors">
              Log in
            </Link>
            <Link href="/pricing">
              <button className="flex items-center gap-2 bg-gradient-to-r from-violet-600 to-indigo-600 hover:from-violet-700 hover:to-indigo-700 text-white px-4 py-2 rounded-xl text-sm font-semibold transition-all shadow-md shadow-violet-600/10 hover:shadow-violet-600/20 hover:scale-[1.02] active:scale-[0.98]">
                Get Started <ArrowRight className="h-3.5 w-3.5" />
              </button>
            </Link>
          </div>
        </div>
      </header>

      {/* ── Hero ────────────────────────────────────── */}
      <section className="relative pt-24 pb-20 overflow-hidden">
        <div className="mx-auto max-w-7xl px-6 lg:px-8 text-center">
          <div className="inline-flex items-center gap-2 bg-violet-600/5 border border-violet-600/10 text-violet-700 text-xs font-semibold px-4 py-1.5 rounded-full mb-8 backdrop-blur-sm shadow-sm">
            <Star className="h-3 w-3 fill-violet-600 text-violet-600" />
            Empowering Multi-Studio Gym Owners Worldwide
          </div>

          <h1 className="text-5xl sm:text-7xl font-extrabold tracking-tight text-slate-900 max-w-4xl mx-auto leading-[1.1]">
            One Command Center.
            <span className="block mt-2 text-transparent bg-clip-text bg-gradient-to-r from-violet-600 via-fuchsia-600 to-indigo-600">
              Every Studio Location.
            </span>
          </h1>

          <p className="mt-8 text-lg sm:text-xl text-slate-600 max-w-2xl mx-auto leading-relaxed">
            Stop switching between tabs. Centralize lead pipelines, automate cross-channel messaging, manage member billing, and trigger instant AI replies across all locations.
          </p>

          <div className="mt-12 flex flex-col sm:flex-row items-center justify-center gap-4">
            <Link href="/pricing">
              <button className="w-full sm:w-auto flex items-center justify-center gap-2 bg-gradient-to-r from-violet-600 to-indigo-600 hover:from-violet-700 hover:to-indigo-700 text-white px-8 py-4 rounded-xl text-base font-bold transition-all shadow-lg shadow-violet-600/20 hover:shadow-violet-600/30 hover:-translate-y-0.5 active:translate-y-0">
                Start Free Trial <ArrowRight className="h-4 w-4" />
              </button>
            </Link>
            <a href="#simulator" className="flex items-center gap-2 text-sm font-semibold text-violet-600 hover:text-violet-800 transition-colors">
              Watch Live Simulator <ArrowRight className="h-3.5 w-3.5" />
            </a>
          </div>

          {/* Social Proof Badges */}
          <div className="mt-14 flex flex-wrap items-center justify-center gap-8 text-sm text-slate-500 font-medium">
            <div className="flex items-center gap-2">
              <CheckCircle className="h-4 w-4 text-violet-600" />
              No credit card required
            </div>
            <div className="flex items-center gap-2">
              <CheckCircle className="h-4 w-4 text-violet-600" />
              Setup in under 5 minutes
            </div>
            <div className="flex items-center gap-2">
              <CheckCircle className="h-4 w-4 text-violet-600" />
              Cancel subscription anytime
            </div>
          </div>
        </div>
      </section>

      {/* ── Live Lead Stream Simulator ──────────────── */}
      <section id="simulator" className="mx-auto max-w-7xl px-6 lg:px-8 pb-28">
        <div className="text-center mb-10">
          <h2 className="text-3xl font-extrabold text-slate-900 tracking-tight">Real-Time Lead & AI Activity</h2>
          <p className="mt-2 text-slate-600 max-w-xl mx-auto">Watch how incoming queries across social platforms are automatically processed by our AI pipeline.</p>
        </div>

        <div className="bg-white border border-slate-200/80 rounded-2xl p-6 shadow-xl shadow-slate-100 relative">
          <div className="absolute top-4 right-4 flex items-center gap-2 bg-emerald-50 border border-emerald-200 text-emerald-700 text-xs px-2.5 py-1 rounded-full">
            <span className="h-2 w-2 rounded-full bg-emerald-500 animate-ping" />
            Live Feed Simulating
          </div>
          
          <div className="flex items-center gap-2 mb-6 border-b border-slate-100 pb-4">
            <Activity className="h-5 w-5 text-violet-600" />
            <h3 className="font-bold text-slate-900 text-lg">Central Lead Stream</h3>
          </div>

          <div className="space-y-4">
            {simulatedLeads.map((lead) => (
              <div 
                key={lead.id} 
                className="bg-slate-50/50 border border-slate-200/60 rounded-xl p-4 flex flex-col md:flex-row md:items-center justify-between gap-4 transition-all hover:bg-white hover:border-slate-300 hover:shadow-sm"
              >
                <div className="flex items-start gap-4">
                  <div className={`h-10 w-10 rounded-xl flex items-center justify-center font-bold text-white shadow-sm bg-gradient-to-br ${
                    lead.source === 'Instagram' ? 'from-purple-500 via-pink-500 to-red-500' :
                    lead.source === 'Facebook' ? 'from-blue-600 to-blue-800' :
                    lead.source === 'TikTok' ? 'from-black via-gray-900 to-cyan-500' :
                    'from-emerald-500 to-green-600'
                  }`}>
                    {lead.source[0]}
                  </div>
                  <div>
                    <div className="flex items-center gap-2.5">
                      <span className="font-bold text-slate-900">{lead.name}</span>
                      <span className="text-xs text-slate-500">{lead.time}</span>
                      <span className={`text-[10px] uppercase font-bold tracking-wider px-2 py-0.5 rounded-full ${
                        lead.source === 'Instagram' ? 'bg-purple-100 text-purple-700 border border-purple-200' :
                        lead.source === 'Facebook' ? 'bg-blue-100 text-blue-700 border border-blue-200' :
                        lead.source === 'TikTok' ? 'bg-slate-200 text-slate-800 border border-slate-300' :
                        'bg-emerald-100 text-emerald-700 border border-emerald-200'
                      }`}>{lead.source}</span>
                    </div>
                    <p className="text-sm text-slate-600 mt-1 italic">"{lead.message}"</p>
                  </div>
                </div>

                <div className="flex items-center gap-4 self-end md:self-center">
                  <span className={`text-xs font-semibold px-3 py-1.5 rounded-lg border ${
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
      </section>

      {/* ── Interactive ROI & Pipeline Calculator ────── */}
      <section id="calculator" className="mx-auto max-w-7xl px-6 lg:px-8 pb-28">
        <div className="text-center mb-12">
          <h2 className="text-3xl font-extrabold text-slate-900 tracking-tight">Interactive Pipeline ROI Calculator</h2>
          <p className="mt-2 text-slate-600 max-w-xl mx-auto">Adjust the sliders to estimate how much revenue your studios can recover by optimizing lead conversions.</p>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-3 gap-8 items-stretch">
          {/* Controls */}
          <div className="lg:col-span-2 bg-white border border-slate-200/80 rounded-2xl p-6 flex flex-col justify-between gap-6 shadow-sm">
            <div>
              <div className="flex justify-between items-center mb-2">
                <label className="text-sm font-semibold text-slate-700">Monthly Ad Spend (Paid Social)</label>
                <span className="text-violet-600 font-bold">${adSpend.toLocaleString()}</span>
              </div>
              <input 
                type="range" 
                min="500" 
                max="10000" 
                step="250"
                value={adSpend} 
                onChange={(e) => setAdSpend(Number(e.target.value))}
                className="w-full h-1.5 bg-slate-200 rounded-lg appearance-none cursor-pointer accent-violet-600"
              />
              <div className="flex justify-between text-[10px] text-slate-400 mt-1">
                <span>$500</span>
                <span>$10,000</span>
              </div>
            </div>

            <div>
              <div className="flex justify-between items-center mb-2">
                <label className="text-sm font-semibold text-slate-700">Lead-to-Member Conversion Rate</label>
                <span className="text-fuchsia-600 font-bold">{conversionRate}%</span>
              </div>
              <input 
                type="range" 
                min="2" 
                max="40" 
                value={conversionRate} 
                onChange={(e) => setConversionRate(Number(e.target.value))}
                className="w-full h-1.5 bg-slate-200 rounded-lg appearance-none cursor-pointer accent-fuchsia-600"
              />
              <div className="flex justify-between text-[10px] text-slate-400 mt-1">
                <span>2% (Industry Low)</span>
                <span>40% (AI-Optimized)</span>
              </div>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label className="text-xs font-semibold text-slate-500 block mb-1.5">Avg Membership Fee (Monthly)</label>
                <div className="relative">
                  <span className="absolute left-3 top-2.5 text-slate-400 text-sm">$</span>
                  <input 
                    type="number"
                    value={monthlyMembershipFee}
                    onChange={(e) => setMonthlyMembershipFee(Math.max(0, Number(e.target.value)))}
                    className="w-full bg-slate-50 border border-slate-200/80 rounded-xl py-2 pl-7 pr-3 text-sm text-slate-900 focus:outline-none focus:border-violet-600 focus:bg-white"
                  />
                </div>
              </div>

              <div>
                <label className="text-xs font-semibold text-slate-500 block mb-1.5">Avg Member Retention (Months)</label>
                <div className="relative">
                  <input 
                    type="number"
                    value={memberLifetimeMonths}
                    onChange={(e) => setMemberLifetimeMonths(Math.max(1, Number(e.target.value)))}
                    className="w-full bg-slate-50 border border-slate-200/80 rounded-xl py-2 px-3 text-sm text-slate-900 focus:outline-none focus:border-violet-600 focus:bg-white"
                  />
                </div>
              </div>
            </div>
          </div>

          {/* Results Card */}
          <div className="bg-gradient-to-br from-violet-50 via-indigo-50/50 to-fuchsia-50 border border-violet-200/60 rounded-2xl p-6 flex flex-col justify-between shadow-lg relative overflow-hidden">
            <div className="absolute top-0 right-0 -translate-y-4 translate-x-4 h-24 w-24 rounded-full bg-violet-600/5 blur-xl pointer-events-none" />
            
            <div>
              <span className="text-[10px] uppercase font-extrabold tracking-wider bg-violet-600/10 text-violet-700 px-2.5 py-1 rounded-full border border-violet-200">
                Pipeline Forecast
              </span>
              
              <div className="mt-6 space-y-4">
                <div className="flex justify-between border-b border-slate-200/60 pb-2">
                  <span className="text-sm text-slate-500">Predicted Leads</span>
                  <span className="font-bold text-slate-900">{leadsGenerated}</span>
                </div>
                <div className="flex justify-between border-b border-slate-200/60 pb-2">
                  <span className="text-sm text-slate-500">New Members Won</span>
                  <span className="font-bold text-slate-900">{memberConversions}</span>
                </div>
                <div className="flex justify-between border-b border-slate-200/60 pb-2">
                  <span className="text-sm text-slate-500">Monthly Added MRR</span>
                  <span className="font-bold text-emerald-600 font-extrabold">+${newMonthlyRevenue.toLocaleString()}</span>
                </div>
              </div>
            </div>

            <div className="mt-8 border-t border-slate-200 pt-4">
              <div className="text-xs text-slate-500 uppercase tracking-wider font-semibold">Total Lifetime Value Won</div>
              <div className="text-4xl font-black text-slate-900 mt-1">
                ${lifetimeValueWon.toLocaleString()}
              </div>
              <div className="flex items-center gap-1 text-[11px] text-emerald-600 mt-2 font-bold">
                <TrendingUp className="h-3.5 w-3.5" />
                <span>+{netROI.toFixed(0)}% ROI Pipeline Impact</span>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ── Features ────────────────────────────────── */}
      <section id="features" className="relative bg-white border-y border-slate-200/60 py-24 overflow-hidden">
        <div className="mx-auto max-w-7xl px-6 lg:px-8">
          <div className="text-center mb-16">
            <h2 className="text-4xl font-black tracking-tight text-slate-900">Centralize & Automate Everything</h2>
            <p className="mt-4 text-lg text-slate-600 max-w-xl mx-auto">From the first ad click to the final membership signup, we handle the workflow.</p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {[
              {
                icon: Building2,
                color: 'bg-violet-600/5 text-violet-600 border-violet-600/10',
                title: 'Multi-Studio Architecture',
                desc: 'Easily manage and compare locations under a single owner login. Isolated data configurations per studio keep pipelines organized.'
              },
              {
                icon: MessageSquare,
                color: 'bg-fuchsia-600/5 text-fuchsia-600 border-fuchsia-600/10',
                title: 'Intelligent RAG AI Replies',
                desc: 'Context-aware AI scans your customized FAQs and instantly replies to prospective leads, booking them into active trial slots.'
              },
              {
                icon: Users,
                color: 'bg-indigo-600/5 text-indigo-600 border-indigo-600/10',
                title: 'Visual Deal Pipelines',
                desc: 'Categorize organic web queries, custom landing page forms, and Meta campaign leads. Spot drop-offs and optimize conversions.'
              },
              {
                icon: Zap,
                color: 'bg-purple-600/5 text-purple-600 border-purple-600/10',
                title: 'Cross-Channel Automation',
                desc: 'Create multi-day drip campaigns across SMS and WhatsApp to nurturing colder leads without manual intervention.'
              },
              {
                icon: BarChart3,
                color: 'bg-pink-600/5 text-pink-600 border-pink-600/10',
                title: 'Integrated Social Planner',
                desc: 'Queue posts across major social networks from one dashboard. Plan launches and studio events with ease.'
              },
              {
                icon: Globe,
                color: 'bg-violet-600/5 text-violet-600 border-violet-600/10',
                title: 'Direct Stripe Integrations',
                desc: 'Collect billing info, process card charges, sell trials, and run subscription plans directly inside the user dashboard.'
              },
            ].map((f) => (
              <div 
                key={f.title} 
                className="p-8 rounded-3xl bg-slate-50/30 hover:bg-slate-50 border border-slate-200/60 hover:border-violet-300 transition-all duration-300 group"
              >
                <div className={`h-14 w-14 rounded-2xl ${f.color} border flex items-center justify-center mb-6 group-hover:scale-110 transition-transform duration-300 shadow-sm`}>
                  <f.icon className="h-6 w-6" />
                </div>
                <h3 className="text-xl font-bold text-slate-900 mb-3">{f.title}</h3>
                <p className="text-slate-600 leading-relaxed text-sm">{f.desc}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── Testimonials ────────────────────────────── */}
      <section className="mx-auto max-w-7xl px-6 lg:px-8 py-28">
        <div className="text-center mb-16">
          <h2 className="text-3xl font-extrabold text-slate-900">Loved by Studio Owners</h2>
          <p className="text-slate-600 mt-2">See how our unified platform is shifting studio growth into high gear.</p>
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
            <div key={idx} className="bg-white border border-slate-200/80 rounded-2xl p-6 flex flex-col justify-between shadow-sm">
              <div>
                <div className="flex gap-1 mb-4 text-amber-400">
                  {[...Array(5)].map((_, i) => (
                    <Star key={i} className="h-4 w-4 fill-current" />
                  ))}
                </div>
                <p className="text-slate-600 italic text-sm leading-relaxed">"{t.quote}"</p>
              </div>
              <div className="mt-6 pt-4 border-t border-slate-100">
                <div className="font-bold text-slate-900 text-sm">{t.author}</div>
                <div className="text-xs text-violet-600 mt-0.5">{t.role}</div>
              </div>
            </div>
          ))}
        </div>
      </section>

      {/* ── Interactive FAQ Section ─────────────────── */}
      <section className="mx-auto max-w-5xl px-6 lg:px-8 pb-28">
        <div className="text-center mb-12">
          <h2 className="text-3xl font-extrabold text-slate-900 tracking-tight">Frequently Asked Questions</h2>
          <p className="mt-2 text-slate-600">Everything you need to know about the platform and deployment.</p>
        </div>

        <div className="space-y-4">
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
              className="bg-white border border-slate-200/80 rounded-xl overflow-hidden transition-all hover:bg-slate-50"
            >
              <button 
                onClick={() => toggleFaq(idx)}
                className="w-full py-5 px-6 flex items-center justify-between text-left focus:outline-none"
              >
                <span className="font-semibold text-slate-900 text-sm sm:text-base">{faq.q}</span>
                <ChevronDown className={`h-4 w-4 text-slate-400 transition-transform duration-300 ${openFaqIndex === idx ? 'rotate-180 text-violet-600' : ''}`} />
              </button>
              
              <div 
                className={`transition-all duration-300 ease-in-out overflow-hidden ${
                  openFaqIndex === idx ? 'max-h-48 border-t border-slate-100 bg-slate-50/50' : 'max-h-0'
                }`}
              >
                <p className="p-6 text-sm text-slate-600 leading-relaxed">{faq.a}</p>
              </div>
            </div>
          ))}
        </div>
      </section>

      {/* ── Premium CTA ─────────────────────────────── */}
      <section className="relative overflow-hidden bg-gradient-to-br from-violet-950 to-indigo-900 py-32 text-center text-white">
        <div className="absolute inset-0 z-0 opacity-10">
          <img 
            src="https://images.unsplash.com/photo-1540497077202-7c8a3999166f?q=80&w=2000&auto=format&fit=crop" 
            alt="Gym background" 
            className="w-full h-full object-cover grayscale" 
          />
        </div>
        <div className="absolute inset-0 z-0 bg-gradient-to-b from-transparent via-[#09070F]/20 to-[#09070F]/50" />
        
        <div className="relative z-10 mx-auto max-w-4xl px-6 lg:px-8">
          <h2 className="text-4xl sm:text-6xl font-black mb-6 leading-tight">
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

      {/* ── Footer ──────────────────────────────────── */}
      <footer className="bg-slate-50 text-slate-500 border-t border-slate-200/80">
        <div className="mx-auto max-w-7xl px-6 lg:px-8 py-16">
          <div className="flex flex-col md:flex-row justify-between gap-12 mb-12">
            {/* Brand */}
            <div className="max-w-xs">
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
            <div className="grid grid-cols-2 sm:grid-cols-3 gap-8 text-sm">
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
                  <li><Link href="/pricing" className="hover:text-violet-600 transition-colors">Register</Link></li>
                </ul>
              </div>
              <div>
                <h4 className="font-bold text-slate-800 mb-4">Legal</h4>
                <ul className="space-y-3 text-slate-500">
                  <li><a href="#" className="hover:text-violet-600 transition-colors">Privacy Policy</a></li>
                  <li><a href="#" className="hover:text-violet-600 transition-colors">Terms of Service</a></li>
                </ul>
              </div>
            </div>
          </div>

          <div className="border-t border-slate-200 pt-8 flex flex-col sm:flex-row items-center justify-between gap-4 text-xs text-slate-400">
            <p>© 2026 1herosocial.ai. All rights reserved.</p>
            <p>Built for fitness & wellness studios worldwide 🌏</p>
          </div>
        </div>
      </footer>
    </div>
  );
}
