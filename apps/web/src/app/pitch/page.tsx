"use client";

import React, { useState } from "react";

export default function PitchPage() {
  const [isHovered, setIsHovered] = useState(false);

  return (
    <div className="min-h-screen bg-[#050505] text-white selection:bg-indigo-500/30 overflow-x-hidden font-sans">
      {/* Dynamic Background */}
      <div className="fixed inset-0 z-0 opacity-40 pointer-events-none">
        <div className="absolute top-[-10%] left-[-10%] w-[40%] h-[40%] rounded-full bg-indigo-600 blur-[150px] mix-blend-screen animate-pulse" />
        <div className="absolute bottom-[-10%] right-[-10%] w-[40%] h-[40%] rounded-full bg-violet-600 blur-[150px] mix-blend-screen opacity-50" />
      </div>

      <div className="relative z-10 max-w-7xl mx-auto px-6 sm:px-12 lg:px-24 pt-24 pb-32">
        {/* Navigation / Header */}
        <nav className="flex justify-between items-center mb-24">
          <div className="text-2xl font-black tracking-tighter bg-clip-text text-transparent bg-gradient-to-r from-white to-white/60">
            1HeroSocial<span className="text-indigo-500">.</span>
          </div>
          <a
            href="mailto:a.shaikfawaz@gmail.com?subject=Partnering%20with%201HeroSocial"
            className="px-6 py-2.5 rounded-full text-sm font-medium bg-white/5 hover:bg-white/10 border border-white/10 backdrop-blur-md transition-all duration-300 shadow-[0_0_15px_rgba(255,255,255,0.05)] hover:shadow-[0_0_25px_rgba(99,102,241,0.2)]"
          >
            Partner with us
          </a>
        </nav>

        {/* Hero Section */}
        <section className="flex flex-col items-center text-center space-y-8 mb-40 mt-12">
          <div className="inline-flex items-center space-x-2 px-4 py-2 rounded-full bg-indigo-500/10 border border-indigo-500/20 text-indigo-300 text-sm font-medium mb-4">
            <span className="w-2 h-2 rounded-full bg-indigo-500 animate-pulse" />
            <span>The Ultimate Orchestration Layer</span>
          </div>
          
          <h1 className="text-6xl sm:text-7xl lg:text-8xl font-bold tracking-tight leading-[1.1]">
            Turn your Studio into an <br />
            <span className="bg-clip-text text-transparent bg-gradient-to-r from-indigo-400 via-purple-400 to-pink-400">
              Automated Growth Engine.
            </span>
          </h1>
          
          <p className="max-w-3xl text-lg sm:text-xl text-gray-400 leading-relaxed font-light mt-6">
            1HeroSocial connects your WhatsApp, Meta, and CRM into one intelligent orchestration layer. Never miss a lead, automate follow-ups with AI, and scale your studio effortlessly.
          </p>

          <div className="flex flex-col sm:flex-row gap-6 mt-10">
            <a
              href="mailto:a.shaikfawaz@gmail.com?subject=See%201HeroSocial%20in%20action"
              onMouseEnter={() => setIsHovered(true)}
              onMouseLeave={() => setIsHovered(false)}
              className="relative group px-8 py-4 rounded-full font-semibold text-white bg-indigo-600 hover:bg-indigo-500 transition-all overflow-hidden flex items-center gap-3 shadow-[0_0_40px_rgba(99,102,241,0.4)] hover:shadow-[0_0_60px_rgba(99,102,241,0.6)]"
            >
              <div className="absolute inset-0 w-full h-full bg-gradient-to-r from-transparent via-white/20 to-transparent translate-x-[-100%] group-hover:animate-[shimmer_1.5s_infinite]" />
              See the Platform in Action
              <svg
                className={`w-5 h-5 transition-transform duration-300 ${isHovered ? "translate-x-1" : ""}`}
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M14 5l7 7m0 0l-7 7m7-7H3" />
              </svg>
            </a>
            <a
              href="#dashboard-preview"
              className="px-8 py-4 rounded-full font-semibold text-white bg-white/5 border border-white/10 hover:bg-white/10 backdrop-blur-md transition-all"
            >
              View Case Studies
            </a>
          </div>
        </section>

        {/* The Problem / Solution Dashboard Preview */}
        <section id="dashboard-preview" className="relative mb-40 group">
          <div className="absolute -inset-1 bg-gradient-to-r from-indigo-500 to-purple-600 rounded-2xl blur opacity-25 group-hover:opacity-40 transition duration-1000 group-hover:duration-200"></div>
          <div className="relative rounded-2xl bg-[#0a0a0a] border border-white/10 overflow-hidden shadow-2xl p-2">
            <div className="absolute inset-0 bg-grid-white/[0.02] bg-[size:32px_32px]" />
            <div className="relative bg-[#111] rounded-xl border border-white/5 overflow-hidden">
              {/* Mockup Top Bar */}
              <div className="flex items-center px-4 py-3 border-b border-white/5 bg-[#0a0a0a]">
                <div className="flex gap-2">
                  <div className="w-3 h-3 rounded-full bg-red-500/80" />
                  <div className="w-3 h-3 rounded-full bg-yellow-500/80" />
                  <div className="w-3 h-3 rounded-full bg-green-500/80" />
                </div>
                <div className="mx-auto text-xs text-gray-500 font-medium bg-white/5 px-3 py-1 rounded-md">mythos.1herosocial.com</div>
              </div>
              {/* Mockup Content */}
              <div className="p-8 grid grid-cols-1 md:grid-cols-3 gap-6">
                <div className="col-span-2 space-y-6">
                  <div className="flex items-center justify-between">
                    <h3 className="text-xl font-medium text-white">Active Leads Pipeline</h3>
                    <span className="text-xs font-medium text-emerald-400 bg-emerald-400/10 px-2 py-1 rounded">Live AI Agent</span>
                  </div>
                  <div className="space-y-4">
                    {[
                      { name: "Sarah J.", status: "Replied via WhatsApp", time: "2 min ago", avatar: "SJ" },
                      { name: "Mike T.", status: "Auto-booked Intro Class", time: "15 min ago", avatar: "MT" },
                      { name: "Emily R.", status: "AI Follow-up Sent", time: "1 hour ago", avatar: "ER" },
                    ].map((lead, i) => (
                      <div key={i} className="flex items-center justify-between p-4 rounded-lg bg-white/5 border border-white/5 hover:bg-white/10 transition-colors cursor-pointer">
                        <div className="flex items-center gap-4">
                          <div className="w-10 h-10 rounded-full bg-indigo-500/20 flex items-center justify-center text-indigo-300 font-bold text-sm">
                            {lead.avatar}
                          </div>
                          <div>
                            <p className="text-white font-medium text-sm">{lead.name}</p>
                            <p className="text-gray-400 text-xs">{lead.status}</p>
                          </div>
                        </div>
                        <span className="text-gray-500 text-xs">{lead.time}</span>
                      </div>
                    ))}
                  </div>
                </div>
                <div className="space-y-6">
                  <h3 className="text-xl font-medium text-white">Studio Health</h3>
                  <div className="p-5 rounded-xl bg-gradient-to-b from-indigo-500/10 to-transparent border border-indigo-500/20">
                    <p className="text-gray-400 text-sm mb-1">Conversion Rate</p>
                    <p className="text-4xl font-bold text-white mb-2">42.8%</p>
                    <p className="text-emerald-400 text-xs flex items-center gap-1">
                      <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 10l7-7m0 0l7 7m-7-7v18" />
                      </svg>
                      +12.5% this week
                    </p>
                  </div>
                  <div className="p-5 rounded-xl bg-white/5 border border-white/5">
                    <p className="text-gray-400 text-sm mb-1">AI Handled Messages</p>
                    <p className="text-3xl font-bold text-white mb-2">1,204</p>
                    <p className="text-gray-500 text-xs">Hours saved: 45h</p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </section>

        {/* Features Grid */}
        <section className="mb-40">
          <div className="text-center mb-16">
            <h2 className="text-3xl md:text-5xl font-bold mb-6">Built for Studio Growth.</h2>
            <p className="text-gray-400 max-w-2xl mx-auto text-lg">Stop managing tools and start managing your business. 1HeroSocial brings everything into one unified, intelligent workspace.</p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-8">
            {[
              {
                title: "Omnichannel Inbox",
                desc: "WhatsApp, Instagram, Facebook, and Web Leads route directly to one centralized inbox. Never miss a message again.",
                icon: (
                  <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M8 10h.01M12 10h.01M16 10h.01M9 16H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-5l-5 5v-5z" />
                  </svg>
                )
              },
              {
                title: "Custom AI Knowledge Base",
                desc: "Train an AI agent on your studio's pricing, schedules, and FAQs. It automatically answers inquiries and books trials 24/7.",
                icon: (
                  <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
                  </svg>
                )
              },
              {
                title: "Automated Lead Routing",
                desc: "Instantly capture leads from your funnels and instantly send personalized WhatsApp follow-ups while the lead is still hot.",
                icon: (
                  <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M13 10V3L4 14h7v7l9-11h-7z" />
                  </svg>
                )
              },
            ].map((feature, i) => (
              <div key={i} className="group p-8 rounded-2xl bg-white/5 border border-white/10 hover:bg-white/[0.07] hover:border-indigo-500/50 transition-all duration-300 relative overflow-hidden">
                <div className="absolute top-0 right-0 p-8 opacity-0 group-hover:opacity-10 transition-opacity duration-500 translate-x-4 -translate-y-4 text-indigo-500">
                  {feature.icon}
                </div>
                <div className="w-12 h-12 rounded-xl bg-indigo-500/10 text-indigo-400 flex items-center justify-center mb-6">
                  {feature.icon}
                </div>
                <h3 className="text-xl font-semibold text-white mb-3">{feature.title}</h3>
                <p className="text-gray-400 leading-relaxed text-sm">{feature.desc}</p>
              </div>
            ))}
          </div>
        </section>

        {/* CTA Section */}
        <section className="relative rounded-3xl overflow-hidden border border-white/10 bg-gradient-to-br from-[#0a0a0a] to-[#111]">
          <div className="absolute inset-0 bg-[url('https://www.transparenttextures.com/patterns/cubes.png')] opacity-20 mix-blend-overlay"></div>
          <div className="absolute right-0 top-0 w-1/2 h-full bg-gradient-to-l from-indigo-500/20 to-transparent blur-3xl"></div>
          
          <div className="relative p-12 md:p-20 flex flex-col md:flex-row items-center justify-between gap-12 text-center md:text-left">
            <div className="max-w-2xl">
              <h2 className="text-4xl md:text-5xl font-bold mb-6 text-white">Ready to dominate your local market?</h2>
              <p className="text-xl text-gray-400 font-light">Join the top studios using 1HeroSocial to automate their communication and fill their classes on autopilot.</p>
            </div>
            <div className="flex-shrink-0">
              <a
                href="mailto:a.shaikfawaz@gmail.com?subject=Early%20Access%20to%201HeroSocial"
                className="inline-block px-10 py-5 rounded-full font-bold text-white bg-indigo-600 hover:bg-indigo-500 hover:scale-105 transition-all shadow-[0_0_40px_rgba(99,102,241,0.5)]"
              >
                Get Early Access
              </a>
            </div>
          </div>
        </section>

        {/* Footer */}
        <footer className="mt-32 pt-12 border-t border-white/10 flex flex-col md:flex-row items-center justify-between gap-6 text-sm text-gray-500">
          <div className="flex items-center gap-2">
            <div className="w-6 h-6 rounded bg-gradient-to-br from-indigo-500 to-purple-600 flex items-center justify-center text-white font-bold text-[10px]">
              1H
            </div>
            <span>© {new Date().getFullYear()} 1HeroSocial. All rights reserved.</span>
          </div>
          <div className="flex gap-6">
            <a href="#" className="hover:text-white transition-colors">Privacy</a>
            <a href="#" className="hover:text-white transition-colors">Terms</a>
            <a href="#" className="hover:text-white transition-colors">Contact</a>
          </div>
        </footer>
      </div>
      
      <style dangerouslySetInnerHTML={{__html: `
        @keyframes shimmer {
          100% { transform: translateX(100%); }
        }
      `}} />
    </div>
  );
}
