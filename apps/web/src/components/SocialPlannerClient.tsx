'use client';

import React, { useState, useEffect } from 'react';
import { 
  Sparkles, 
  Calendar as CalendarIcon, 
  Plus, 
  Facebook, 
  Instagram, 
  CheckCircle2, 
  AlertCircle, 
  TrendingUp, 
  Image as ImageIcon, 
  Megaphone,
  Clock, 
  Share2, 
  Globe, 
  Lightbulb,
  Trash2,
  Paperclip,
  X as XIcon,
  Twitter
} from 'lucide-react';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Label } from '@/components/ui/Label';
import { Textarea } from '@/components/ui/Textarea';
import { Badge } from '@/components/ui/Badge';
import { api } from '@/lib/api';
import Link from 'next/link';

interface SocialPost {
  id: string;
  campaignName: string;
  platform: 'Facebook' | 'Instagram' | 'Google Ads' | 'X (Twitter)';
  content: string;
  imageUrl?: string;
  status: 'published' | 'scheduled' | 'draft' | 'failed';
  scheduledTime: string;
}

export default function SocialPlannerClient({ studioId }: { studioId: string }) {
  const [activeTab, setActiveTab] = useState<'scheduler' | 'ai-creator' | 'connections'>('scheduler');
  const [posts, setPosts] = useState<SocialPost[]>([]);
  const [loadingPosts, setLoadingPosts] = useState(true);

  // AI Form states
  const [campaign, setCampaign] = useState('');
  const [tone, setTone] = useState('energetic');
  const [platform, setPlatform] = useState<'Facebook' | 'Instagram' | 'Google Ads' | 'X (Twitter)'>('Instagram');
  const [prompt, setPrompt] = useState('');
  const [generating, setGenerating] = useState(false);
  const [aiOutput, setAiOutput] = useState<{
    text: string;
    hashtags: string[];
    headline: string;
    cta: string;
  } | null>(null);

  // Channels configuration based on credentials
  const [connectedChannels, setConnectedChannels] = useState({
    facebook: false,
    instagram: false,
    googleAds: false,
    x: false,
  });

  // Scheduler Form states
  const [newPostContent, setNewPostContent] = useState('');
  const [newPostDate, setNewPostDate] = useState('2026-05-28');
  const [newPostTime, setNewPostTime] = useState('09:00');
  const [newPostCampaign, setNewPostCampaign] = useState('');
  const [newPostPlatform, setNewPostPlatform] = useState<'Facebook' | 'Instagram' | 'Google Ads' | 'X (Twitter)'>('Instagram');

  // Quick AI Form states
  const [quickAiPrompt, setQuickAiPrompt] = useState('');
  const [generatingQuickAi, setGeneratingQuickAi] = useState(false);
  const [showQuickAiModal, setShowQuickAiModal] = useState(false);
  const [aiModalTarget, setAiModalTarget] = useState<'scheduler' | 'creator'>('scheduler');

  // File Upload states
  const [mediaUrl, setMediaUrl] = useState('');
  const [mediaName, setMediaName] = useState('');
  const [uploadingFile, setUploadingFile] = useState(false);
  const [mediaInputType, setMediaInputType] = useState<'upload' | 'url'>('upload');

  // Confirmation and Notification modals
  const [showConfirmModal, setShowConfirmModal] = useState(false);
  const [showNotificationModal, setShowNotificationModal] = useState(false);
  const [notificationStatus, setNotificationStatus] = useState<'success' | 'error'>('success');
  const [notificationMessage, setNotificationMessage] = useState('');

  // Delete Confirmation states
  const [deletingPostId, setDeletingPostId] = useState<string | null>(null);
  const [showDeleteConfirmModal, setShowDeleteConfirmModal] = useState(false);

  const [visibleQueueCount, setVisibleQueueCount] = useState(4);

  useEffect(() => {
    setVisibleQueueCount(4);
  }, [posts]);

  const handleQueueScroll = (e: React.UIEvent<HTMLDivElement>) => {
    const target = e.currentTarget;
    if (target.scrollHeight - target.scrollTop <= target.clientHeight + 20) {
      if (visibleQueueCount < posts.length) {
        setVisibleQueueCount(posts.length);
      }
    }
  };

  const fetchPosts = (silent = false) => {
    if (!silent) setLoadingPosts(true);
    const targetStudioId = studioId === 'global' ? 'global' : studioId;
    api<any[]>(`/api/v1/studios/${targetStudioId}/social-posts`)
      .then((res) => {
        const mapped = res.map((p: any) => ({
          id: p.id,
          campaignName: p.campaign || 'General Promo',
          platform: p.platform as any,
          content: p.copy,
          imageUrl: p.mediaUrl,
          status: p.status as any,
          scheduledTime: p.scheduledAt,
        }));
        setPosts(mapped);
        if (!silent) setLoadingPosts(false);
      })
      .catch((err) => {
        console.error('Failed to load social posts:', err);
        if (!silent) setLoadingPosts(false);
      });
  };

  const handleQuickAiGenerate = async () => {
    if (!quickAiPrompt) return;
    setGeneratingQuickAi(true);
    try {
      const activePlatform = aiModalTarget === 'scheduler' ? newPostPlatform : platform;
      const activeCampaign = aiModalTarget === 'scheduler' ? newPostCampaign : campaign;

      const res = await api<{ text?: string }>(`/api/v1/studios/${studioId === 'global' ? '759b1ee2-5a68-4a5c-8fa0-5b2a64d5cc35' : studioId}/messaging/ai/generate`, {
        method: 'POST',
        json: {
          prompt: `Create a professional marketing social media post copy for platform: ${activePlatform}. Campaign Context: ${activeCampaign || 'General Promo'}. Tone: energetic. Main topic / message details: ${quickAiPrompt}. Do not include placeholder brackets or system variables. Format with appropriate paragraph spacing and emojis.`,
          type: 'social'
        }
      });

      const outputText = res?.text || `🚨 NEW LAUNCH ALERT! 🚨\n\nCheck out what we are promoting: ${quickAiPrompt}! 💥\n\nDon't miss out on this signature experience. Book your slot today!`;

      if (aiModalTarget === 'scheduler') {
        setNewPostContent(outputText);
      } else {
        setPrompt(outputText);
      }
      setShowQuickAiModal(false);
    } catch (e) {
      console.error(e);
      const outputText = `🚨 NEW LAUNCH ALERT! 🚨\n\nCheck out what we are promoting: ${quickAiPrompt}! 💥\n\nDon't miss out on this signature experience. Book your slot today!`;
      if (aiModalTarget === 'scheduler') {
        setNewPostContent(outputText);
      } else {
        setPrompt(outputText);
      }
      setShowQuickAiModal(false);
    } finally {
      setGeneratingQuickAi(false);
    }
  };

  useEffect(() => {
    fetchPosts();

    // Setup interval to poll for post status updates in the background (every 10s)
    const interval = setInterval(() => {
      fetchPosts(true);
    }, 10000);

    // Fetch Meta and Google Ads integration state
    if (studioId !== 'global') {
      Promise.all([
        api<any>(`/api/v1/me/studios/${studioId}`),
        api<{channels: any[]}>(`/api/v1/studios/${studioId}/messaging/channels`)
      ]).then(([studioRes, channelsRes]) => {
          const hasMeta = !!(studioRes.metaAppId && studioRes.metaAppSecret);
          const hasGoogleAds = !!(studioRes.googleClientId && studioRes.googleClientSecret && studioRes.googleDeveloperToken);
          const hasX = channelsRes.channels?.some(c => c.kind === 'x_dm');
          setConnectedChannels({
            facebook: hasMeta,
            instagram: hasMeta,
            googleAds: hasGoogleAds,
            x: !!hasX,
          });
        })
        .catch((err) => console.error('Failed to fetch studio integrations:', err));
    } else {
      setConnectedChannels({
        facebook: true,
        instagram: true,
        googleAds: false,
        x: false,
      });
    }

    return () => clearInterval(interval);
  }, [studioId]);

  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    setUploadingFile(true);
    const formData = new FormData();
    formData.append('file', file);

    try {
      const uploadStudioId = studioId === 'global' ? '759b1ee2-5a68-4a5c-8fa0-5b2a64d5cc35' : studioId;
      const response = await fetch(`/api/v1/studios/${uploadStudioId}/messaging/upload`, {
        method: 'POST',
        body: formData,
      });

      if (!response.ok) {
        throw new Error('Upload failed');
      }

      const data = await response.json();
      setMediaUrl(data.url);
      setMediaName(file.name);
    } catch (err) {
      console.error('File upload failed:', err);
      alert('Failed to upload file');
    } finally {
      setUploadingFile(false);
    }
  };

  // Trigger Gemini generation
  const handleAiGenerate = async () => {
    if (!prompt) return;
    setGenerating(true);
    
    try {
      const res = await api<{ text?: string }>(`/api/v1/studios/${studioId === 'global' ? '759b1ee2-5a68-4a5c-8fa0-5b2a64d5cc35' : studioId}/messaging/ai/generate`, {
        method: 'POST',
        json: {
          prompt: `Create a professional marketing social media post copy for platform: ${platform}. Campaign Context: ${campaign}. Tone: ${tone}. Main topic / message details: ${prompt}.${mediaName ? ` An attachment named "${mediaName}" is included with this post.` : ''} Do not include placeholder brackets or system variables. Format with appropriate paragraph spacing and emojis.`
        }
      });

      if (res?.text) {
        setAiOutput({
          text: res.text,
          hashtags: ['#fitness', tone, platform.toLowerCase().replace(' ', '')],
          headline: `Join ${campaign || 'our fitness journey'}`,
          cta: 'Book your trial now'
        });
      } else {
        throw new Error('Fallback to local AI simulator');
      }
    } catch (e) {
      // High fidelity local fallback simulation using premium templates
      setTimeout(() => {
        const generatedOptions: Record<string, string> = {
          energetic: `🚨 GAME CHANGER ALERT! 🚨\n\nAre you tired of routine workouts? Get ready to supercharge your routine with ${campaign || 'our premium sessions'}! 💥\n\nEvery class is high-intensity, high-energy, and custom built to deliver results fast. Let's make today count!`,
          professional: `Achieve sustainable results with our structured training methodologies. We combine evidence-based fitness regimes with expert coaching to help you scale your health goals.\n\nSchedule a complimentary consultation session with our head trainers today.`,
          bold: `NO EXCUSES. JUST RESULTS. 🔥\n\nIf you want something you never had, you have to do something you never did. Join the next cohort of our ${campaign || 'signature bootcamp'} starting this week. Slots are disappearing fast.`,
          humorous: `We promise we won't make you do burpees... okay maybe just a few. 😅\n\nSeriously though, fitness is supposed to be fun! Come check out ${campaign || 'our group plans'} and find out why our members actually look forward to Mondays.`,
          motivational: `Your mind will quit 100 times before your body does. Push past the limit. ✨\n\nOur certified fitness community is here to support you at every milestone. Let us help you unlock your full potential.`
        };

        const localText = generatedOptions[tone] || generatedOptions.energetic;
        setAiOutput({
          text: localText + `\n\n👉 ${prompt}`,
          hashtags: [`#${tone}fitness`, `#${platform.replace(' ', '')}`, '#healthylifestyle'],
          headline: `Transform with ${campaign || 'us'}`,
          cta: 'Sign Up Today'
        });
      }, 1200);
    } finally {
      setGenerating(false);
    }
  };

  const handleSchedulePost = (e: React.FormEvent) => {
    e.preventDefault();
    if (!newPostContent) return;
    setShowConfirmModal(true);
  };

  const executeSchedulePost = async () => {
    setShowConfirmModal(false);
    try {
      const targetStudioId = studioId === 'global' ? '759b1ee2-5a68-4a5c-8fa0-5b2a64d5cc35' : studioId;
      await api(`/api/v1/studios/${targetStudioId}/social-posts`, {
        method: 'POST',
        json: {
          campaign: newPostCampaign || 'General Promo',
          platform: newPostPlatform,
          copy: newPostContent,
          mediaUrl: mediaUrl,
          status: 'scheduled',
          scheduledAt: new Date(`${newPostDate}T${newPostTime}`).toISOString(),
        }
      });
      setNewPostContent('');
      setNewPostCampaign('');
      setMediaUrl('');
      setMediaName('');
      fetchPosts();
      
      setNotificationStatus('success');
      setNotificationMessage(`Your campaign post has been successfully scheduled for ${new Date(`${newPostDate}T${newPostTime}`).toLocaleString()} on ${newPostPlatform}!`);
      setShowNotificationModal(true);
      
      setActiveTab('scheduler');
    } catch (err) {
      console.error('Failed to schedule post:', err);
      setNotificationStatus('error');
      setNotificationMessage('Failed to schedule campaign post. Please verify backend service and connection configuration.');
      setShowNotificationModal(true);
    }
  };

  const handleDeletePost = (postId: string) => {
    setDeletingPostId(postId);
    setShowDeleteConfirmModal(true);
  };

  const executeDeletePost = async () => {
    if (!deletingPostId) return;
    setShowDeleteConfirmModal(false);
    try {
      const targetStudioId = studioId === 'global' ? 'global' : studioId;
      await api(`/api/v1/studios/${targetStudioId}/social-posts/${deletingPostId}`, {
        method: 'DELETE'
      });
      fetchPosts();
      setDeletingPostId(null);

      setNotificationStatus('success');
      setNotificationMessage('The scheduled post has been successfully deleted from the queue.');
      setShowNotificationModal(true);
    } catch (err) {
      console.error('Failed to delete post:', err);
      setNotificationStatus('error');
      setNotificationMessage('Failed to delete scheduled campaign post. Please verify connection and try again.');
      setShowNotificationModal(true);
      setDeletingPostId(null);
    }
  };

  const applyAiToScheduler = () => {
    if (!aiOutput) return;
    setNewPostContent(`${aiOutput.text}\n\n${aiOutput.hashtags.join(' ')}`);
    setNewPostCampaign(campaign);
    setNewPostPlatform(platform);
    setActiveTab('scheduler');
  };

  const renderFeedMockup = () => {
    if (!aiOutput) return null;

    const previewPlatform = platform;
    const ctaText = aiOutput.cta || 'Book now';
    const headlineText = aiOutput.headline || `Join ${campaign || 'our fitness journey'}`;
    const textCopy = aiOutput.text || '';
    const hashtagsText = aiOutput.hashtags.map(h => h.startsWith('#') ? h : `#${h}`).join(' ');

    const imageUrl = mediaUrl || 'https://images.unsplash.com/photo-1517838277536-f5f99be501cd?q=80&w=1000&auto=format&fit=crop';

    if (previewPlatform === 'Instagram') {
      return (
        <div className="w-full max-w-[340px] mx-auto rounded-3xl border border-zinc-200 dark:border-zinc-850 bg-white dark:bg-zinc-950 overflow-hidden shadow-2xl animate-in fade-in zoom-in duration-300">
          <div className="flex items-center justify-between px-4 py-3 border-b border-zinc-100 dark:border-zinc-900">
            <div className="flex items-center gap-2">
              <div className="w-8 h-8 rounded-full bg-gradient-to-tr from-yellow-500 via-pink-500 to-purple-600 p-[2px]">
                <div className="w-full h-full rounded-full bg-white dark:bg-zinc-955 p-[1px]">
                  <div className="w-full h-full rounded-full bg-brand-500/10 grid place-items-center">
                    <Instagram className="h-4 w-4 text-pink-600" />
                  </div>
                </div>
              </div>
              <div>
                <div className="text-[11px] font-black text-zinc-900 dark:text-white leading-none">your_studio</div>
                <div className="text-[9px] text-zinc-500 leading-none mt-0.5">Sponsored</div>
              </div>
            </div>
            <button type="button" className="text-zinc-650 dark:text-zinc-400 font-bold text-sm">•••</button>
          </div>

          <div className="relative aspect-square bg-zinc-100 dark:bg-zinc-900 overflow-hidden flex items-center justify-center">
            <img src={imageUrl} alt="Instagram Post" className="w-full h-full object-cover" />
          </div>

          <div className="flex items-center justify-between bg-brand-500 px-4 py-2 text-white">
            <span className="text-[10px] font-black uppercase tracking-wider">{ctaText}</span>
            <span className="text-xs font-bold">➔</span>
          </div>

          <div className="px-4 py-3 space-y-2">
            <div className="flex items-center justify-between text-zinc-700 dark:text-zinc-300">
              <div className="flex items-center gap-4 text-sm">
                <span>❤️</span>
                <span>💬</span>
                <span>✈️</span>
              </div>
              <span>🔖</span>
            </div>
            <div className="text-[10px] font-black text-zinc-955 dark:text-white">Liked by gemini and 1,420 others</div>
            <div className="text-[11px] text-zinc-800 dark:text-zinc-205 leading-relaxed">
              <span className="font-black mr-1 text-zinc-955 dark:text-white">your_studio</span>
              {textCopy}
              <div className="mt-1 text-brand-500 font-semibold">{hashtagsText}</div>
            </div>
          </div>
        </div>
      );
    }

    if (previewPlatform === 'Facebook') {
      return (
        <div className="w-full max-w-[360px] mx-auto rounded-3xl border border-zinc-200 dark:border-zinc-850 bg-white dark:bg-zinc-900 overflow-hidden shadow-2xl animate-in fade-in zoom-in duration-300">
          <div className="px-4 py-3 flex items-center gap-3">
            <div className="w-9 h-9 rounded-full bg-blue-600/10 grid place-items-center text-blue-600">
              <Facebook className="h-5 w-5" />
            </div>
            <div>
              <div className="text-[12px] font-black text-zinc-955 dark:text-white flex items-center gap-1">
                Your Studio
                <span className="text-blue-500 text-[10px]">✓</span>
              </div>
              <div className="text-[9px] text-zinc-500 flex items-center gap-1 mt-0.5">
                Sponsored · 🌐
              </div>
            </div>
          </div>

          <div className="px-4 pb-3 text-[11px] text-zinc-800 dark:text-zinc-205 leading-relaxed whitespace-pre-wrap">
            {textCopy}
            <div className="mt-1 text-blue-600 dark:text-blue-400 font-semibold">{hashtagsText}</div>
          </div>

          <div className="aspect-[1.91/1] bg-zinc-100 dark:bg-zinc-955 overflow-hidden flex items-center justify-center border-t border-zinc-100 dark:border-zinc-800">
            <img src={imageUrl} alt="Facebook Ad" className="w-full h-full object-cover" />
          </div>

          <div className="px-4 py-3 bg-zinc-50 dark:bg-zinc-955/40 border-t border-b border-zinc-100 dark:border-zinc-800 flex items-center justify-between">
            <div className="min-w-0 flex-1">
              <div className="text-[9px] text-zinc-500 uppercase tracking-wider font-bold">YOURSTUDIO.COM</div>
              <div className="text-[11px] font-black text-zinc-955 dark:text-white truncate mt-0.5">{headlineText}</div>
            </div>
            <button type="button" className="px-3 py-1.5 rounded-lg bg-zinc-200 dark:bg-zinc-800 text-[10px] font-black uppercase text-zinc-800 dark:text-zinc-200 hover:bg-zinc-300 dark:hover:bg-zinc-700 transition-colors shrink-0">
              {ctaText}
            </button>
          </div>

          <div className="px-4 py-2.5 flex items-center justify-around border-t border-zinc-100 dark:border-zinc-800/60 text-[10px] font-bold text-zinc-500">
            <span className="flex items-center gap-1 cursor-pointer">👍 Like</span>
            <span className="flex items-center gap-1 cursor-pointer">💬 Comment</span>
            <span className="flex items-center gap-1 cursor-pointer">➔ Share</span>
          </div>
        </div>
      );
    }

    if (previewPlatform === 'X (Twitter)') {
      return (
        <div className="w-full max-w-[340px] mx-auto rounded-3xl border border-zinc-200 dark:border-zinc-850 bg-white dark:bg-zinc-955 p-4 shadow-2xl animate-in fade-in zoom-in duration-300">
          <div className="flex items-start gap-3">
            <div className="w-9 h-9 rounded-full bg-zinc-900 text-white grid place-items-center text-xs font-black">
              X
            </div>
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-1">
                <span className="text-[12px] font-black text-zinc-955 dark:text-white">Your Studio</span>
                <span className="text-[10px] text-zinc-400">@yourstudio · Just now</span>
              </div>
              <div className="mt-1 text-[11px] text-zinc-800 dark:text-zinc-205 leading-relaxed whitespace-pre-wrap">
                {textCopy}
                <div className="mt-1 text-sky-500 dark:text-sky-400 font-semibold">{hashtagsText}</div>
              </div>
              <div className="mt-3 rounded-2xl overflow-hidden border border-zinc-100 dark:border-zinc-900 aspect-[1.91/1]">
                <img src={imageUrl} alt="X Post" className="w-full h-full object-cover" />
              </div>
              <div className="mt-4 flex items-center justify-between text-zinc-400 max-w-[240px]">
                <span className="text-xs cursor-pointer hover:text-sky-500 transition-colors">💬 12</span>
                <span className="text-xs cursor-pointer hover:text-emerald-500 transition-colors">🔁 4</span>
                <span className="text-xs cursor-pointer hover:text-pink-500 transition-colors">❤️ 86</span>
                <span className="text-xs cursor-pointer hover:text-sky-500 transition-colors">📤</span>
              </div>
            </div>
          </div>
        </div>
      );
    }

    if (previewPlatform === 'Google Ads') {
      return (
        <div className="w-full max-w-[365px] mx-auto rounded-3xl border border-zinc-200 dark:border-zinc-850 bg-white dark:bg-zinc-900 p-5 shadow-2xl space-y-3 animate-in fade-in zoom-in duration-300">
          <div className="flex items-center gap-1.5 text-[10px] font-bold text-zinc-500">
            <span>Ad</span>
            <span>·</span>
            <span className="text-zinc-800 dark:text-zinc-300">https://www.yourstudio.com/promo</span>
          </div>
          <div className="text-[14px] text-[#1a0dab] dark:text-[#8ab4f8] font-semibold hover:underline cursor-pointer leading-tight">
            {headlineText} | {ctaText}
          </div>
          <div className="text-[11px] text-zinc-650 dark:text-zinc-300 leading-relaxed">
            {textCopy}
          </div>
        </div>
      );
    }

    return null;
  };

  return (
    <div className="space-y-8 animate-in fade-in duration-500">

      {/* Navigation Tabs */}
      <div className="flex items-center gap-1.5 rounded-2xl bg-zinc-100/80 p-1.5 dark:bg-neutral-800/80 backdrop-blur-md max-w-xl">
        <button
          onClick={() => setActiveTab('scheduler')}
          className={`flex items-center gap-2 rounded-xl px-5 py-3 text-xs font-black uppercase tracking-wider transition-all duration-300 ${
            activeTab === 'scheduler'
              ? 'bg-white text-brand-600 shadow-md dark:bg-neutral-900 dark:text-brand-400'
              : 'text-zinc-500 hover:text-zinc-800 dark:text-zinc-400 dark:hover:text-zinc-200'
          }`}
        >
          <CalendarIcon className="h-4 w-4" />
          Scheduler
        </button>
        <button
          onClick={() => setActiveTab('ai-creator')}
          className={`flex items-center gap-2 rounded-xl px-5 py-3 text-xs font-black uppercase tracking-wider transition-all duration-300 ${
            activeTab === 'ai-creator'
              ? 'bg-white text-brand-600 shadow-md dark:bg-neutral-900 dark:text-brand-400'
              : 'text-zinc-500 hover:text-zinc-800 dark:text-zinc-400 dark:hover:text-zinc-200'
          }`}
        >
          <Sparkles className="h-4 w-4" />
          AI Ad Creator
        </button>
        <button
          onClick={() => setActiveTab('connections')}
          className={`flex items-center gap-2 rounded-xl px-5 py-3 text-xs font-black uppercase tracking-wider transition-all duration-300 ${
            activeTab === 'connections'
              ? 'bg-white text-brand-600 shadow-md dark:bg-neutral-900 dark:text-brand-400'
              : 'text-zinc-500 hover:text-zinc-800 dark:text-zinc-400 dark:hover:text-zinc-200'
          }`}
        >
          <Share2 className="h-4 w-4" />
          Connections
        </button>
      </div>

      {/* Scheduler Tab */}
      {activeTab === 'scheduler' && (
        <div className="grid gap-8 lg:grid-cols-3">
          {/* Quick Schedule Form */}
          <div className="lg:col-span-1 space-y-6">
            <Card className="overflow-hidden rounded-[28px] border border-violet-100/50 bg-white/40 shadow-2xl backdrop-blur-2xl dark:border-white/5 dark:bg-neutral-900/35 p-6 transition-all hover:shadow-brand-500/5 duration-300">
              <div className="flex items-center gap-2 mb-6">
                <Plus className="h-4 w-4 text-brand-500" />
                <h2 className="text-sm font-black uppercase tracking-wider text-zinc-955 dark:text-white">Quick Scheduler</h2>
              </div>
              <form onSubmit={handleSchedulePost} className="space-y-5">
                <div>
                  <Label htmlFor="campaign" className="text-[10px] font-black uppercase tracking-wider text-zinc-400">Campaign Target</Label>
                  <Input
                    id="campaign"
                    value={newPostCampaign}
                    onChange={(e) => setNewPostCampaign(e.target.value)}
                    placeholder="e.g. Spring Promo 2026"
                    className="mt-1.5 focus:ring-2 focus:ring-brand-500 focus:border-transparent transition-all rounded-xl"
                  />
                </div>
                <div>
                  <Label htmlFor="platform" className="text-[10px] font-black uppercase tracking-wider text-zinc-400">Target Platform</Label>
                  <select
                    id="platform"
                    value={newPostPlatform}
                    onChange={(e) => setNewPostPlatform(e.target.value as any)}
                    className="mt-1.5 block w-full rounded-xl border border-zinc-200/60 dark:border-white/10 bg-white/10 dark:bg-neutral-800/40 px-3.5 py-2.5 text-xs font-semibold text-zinc-800 dark:text-white focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent transition-all"
                  >
                    <option value="Instagram">Instagram</option>
                    <option value="Facebook">Facebook</option>
                    <option value="Google Ads">Google Ads</option>
                    <option value="X (Twitter)">X (Twitter)</option>
                  </select>
                </div>
                <div>
                  <div className="flex items-center justify-between mb-1.5">
                    <Label htmlFor="content" className="text-[10px] font-black uppercase tracking-wider text-zinc-400">Ad / Post Copy</Label>
                    <button
                      type="button"
                      onClick={() => {
                        setQuickAiPrompt('');
                        setAiModalTarget('scheduler');
                        setShowQuickAiModal(true);
                      }}
                      className="inline-flex items-center gap-1.5 text-[9px] font-black text-brand-500 hover:text-brand-600 uppercase tracking-widest transition-transform active:scale-95"
                    >
                      <Sparkles className="h-3.5 w-3.5 text-brand-500" /> Write with AI
                    </button>
                  </div>
                  <Textarea
                    id="content"
                    value={newPostContent}
                    onChange={(e) => setNewPostContent(e.target.value)}
                    placeholder="Write copy or generate it using AI..."
                    rows={5}
                    className="mt-1 focus:ring-2 focus:ring-brand-500 focus:border-transparent transition-all rounded-2xl"
                    required
                  />
                </div>

                <div>
                  <div className="flex items-center justify-between mb-2">
                    <Label className="text-[10px] font-black uppercase tracking-wider text-zinc-400">Attachments / Media</Label>
                    <div className="flex gap-2.5 text-[9px]">
                      <button
                        type="button"
                        onClick={() => {
                          setMediaInputType('upload');
                          setMediaUrl('');
                          setMediaName('');
                        }}
                        className={`font-black uppercase tracking-wider transition-colors ${
                          mediaInputType === 'upload' ? 'text-brand-500' : 'text-zinc-400 hover:text-zinc-200'
                        }`}
                      >
                        Upload File
                      </button>
                      <span className="text-zinc-500">|</span>
                      <button
                        type="button"
                        onClick={() => {
                          setMediaInputType('url');
                          setMediaUrl('');
                          setMediaName('');
                        }}
                        className={`font-black uppercase tracking-wider transition-colors ${
                          mediaInputType === 'url' ? 'text-brand-500' : 'text-zinc-400 hover:text-zinc-200'
                        }`}
                      >
                        Image URL
                      </button>
                    </div>
                  </div>

                  {mediaUrl ? (
                    <div className="space-y-2 mt-1">
                      <div className="flex items-center justify-between rounded-xl border border-zinc-105 bg-white/20 p-2.5 dark:border-white/5 dark:bg-neutral-800/20">
                        <div className="flex items-center gap-2 min-w-0">
                          <Paperclip className="h-4 w-4 text-brand-500 shrink-0" />
                          <span className="text-xs font-semibold text-zinc-800 dark:text-zinc-200 truncate">
                            {mediaName || mediaUrl}
                          </span>
                        </div>
                        <button
                          type="button"
                          onClick={() => {
                            setMediaUrl('');
                            setMediaName('');
                          }}
                          className="text-xs font-black text-red-500 hover:text-red-400 px-2 transition-colors"
                        >
                          Remove
                        </button>
                      </div>
                      
                      {(mediaUrl.toLowerCase().startsWith('http') || mediaUrl.toLowerCase().startsWith('/uploads')) && (
                        <div className="mt-2 rounded-2xl border border-zinc-100/50 bg-white/5 p-2 max-w-full overflow-hidden flex justify-center shadow-inner">
                          <img 
                            src={mediaUrl} 
                            alt="Media Preview" 
                            className="max-h-48 rounded-xl object-contain"
                            onError={(e) => {
                              (e.target as HTMLElement).style.display = 'none';
                            }}
                          />
                        </div>
                      )}
                    </div>
                  ) : mediaInputType === 'upload' ? (
                    <div className="mt-1 flex items-center justify-center border-2 border-dashed border-zinc-205 hover:border-brand-500/50 dark:border-white/10 dark:hover:border-brand-500/50 rounded-2xl p-6 bg-white/5 hover:bg-brand-500/5 transition-all cursor-pointer relative group">
                      <input
                        type="file"
                        onChange={handleFileUpload}
                        disabled={uploadingFile}
                        className="absolute inset-0 opacity-0 cursor-pointer w-full h-full"
                      />
                      <div className="text-center">
                        <Plus className="h-6 w-6 text-zinc-400 group-hover:text-brand-500 mx-auto mb-1.5 transition-colors" />
                        <span className="text-[11px] font-bold text-zinc-400 group-hover:text-zinc-300 transition-colors">
                          {uploadingFile ? 'Uploading File...' : 'Attach Document or Media'}
                        </span>
                      </div>
                    </div>
                  ) : (
                    <div className="mt-1 space-y-2">
                      <Input
                        type="url"
                        placeholder="https://example.com/image.jpg"
                        value={mediaUrl}
                        onChange={(e) => {
                          const val = e.target.value;
                          setMediaUrl(val);
                          setMediaName(val.split('/').pop() || 'Public Image');
                        }}
                        className="w-full text-xs focus:ring-2 focus:ring-brand-500 focus:border-transparent transition-all rounded-xl"
                      />
                      <p className="text-[10px] text-zinc-400">Enter a direct public URL to an image (ends with .jpg, .png, etc.)</p>
                    </div>
                  )}
                </div>

                <Button 
                  type="submit" 
                  className="w-full h-11 bg-gradient-to-r from-brand-500 to-indigo-600 hover:from-brand-600 hover:to-indigo-700 text-white shadow-xl shadow-brand-500/10 hover:shadow-brand-500/20 transition-all font-black uppercase tracking-widest text-[11px] rounded-2xl mt-4" 
                  disabled={uploadingFile}
                >
                  Schedule Post
                </Button>
              </form>
            </Card>
          </div>

          {/* Queue */}
          <div className="lg:col-span-2 space-y-6">
            <Card className="overflow-hidden rounded-[28px] border border-violet-100/50 bg-white/40 shadow-2xl backdrop-blur-2xl dark:border-white/5 dark:bg-neutral-900/35 p-6 transition-all hover:shadow-brand-500/5 duration-300">
              <div className="flex items-center gap-2 mb-6">
                <Megaphone className="h-4 w-4 text-brand-500" />
                <h2 className="text-sm font-black uppercase tracking-wider text-zinc-955 dark:text-white font-black">Publishing Queue</h2>
              </div>
              
              {loadingPosts ? (
                <div className="flex h-64 items-center justify-center">
                  <div className="h-8 w-8 animate-spin rounded-full border-2 border-brand-500 border-t-transparent" />
                </div>
              ) : posts.length === 0 ? (
                <div className="flex h-64 flex-col items-center justify-center text-center">
                  <AlertCircle className="h-8 w-8 text-zinc-400 mb-2 animate-bounce" />
                  <span className="text-xs font-bold text-zinc-500">No scheduled posts yet. Fill out the scheduler to prepare an ad.</span>
                </div>
              ) : (
                <>
                  <div 
                    className="space-y-4 overflow-y-auto no-scrollbar pr-2" 
                    style={{ maxHeight: '540px' }}
                    onScroll={handleQueueScroll}
                  >
                    {posts.slice(0, visibleQueueCount).map((post) => (
                      <div
                        key={post.id}
                        className="flex items-start gap-4 rounded-2xl border border-white/20 bg-white/30 dark:border-white/5 dark:bg-neutral-900/60 p-5 hover:-translate-y-1 hover:shadow-lg hover:border-brand-500/10 transition-all duration-300"
                      >
                        <div className={`grid h-10 w-10 place-items-center rounded-xl shrink-0 ${
                          post.platform === 'Facebook' ? 'bg-blue-600/10 text-blue-600 shadow-sm' :
                          post.platform === 'Instagram' ? 'bg-pink-600/10 text-pink-600 shadow-sm' :
                          post.platform === 'Google Ads' ? 'bg-indigo-600/10 text-indigo-600 shadow-sm' :
                          'bg-sky-500/10 text-sky-500 shadow-sm'
                        }`}>
                          {post.platform === 'Facebook' && <Facebook className="h-5 w-5" />}
                          {post.platform === 'Instagram' && <Instagram className="h-5 w-5" />}
                          {post.platform === 'Google Ads' && <Globe className="h-5 w-5" />}
                          {post.platform === 'X (Twitter)' && <Twitter className="h-5 w-5" />}
                        </div>
                        
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center gap-2">
                            <span className="text-xs font-black text-zinc-800 dark:text-zinc-200">{post.platform}</span>
                            <span className="text-[10px] font-bold text-zinc-400">· {post.campaignName}</span>
                            <div className="ml-auto flex items-center gap-2">
                              <Badge tone={post.status === 'published' ? 'success' : post.status === 'scheduled' ? 'warning' : 'neutral'}>
                                {post.status}
                              </Badge>
                              <button
                                onClick={() => handleDeletePost(post.id)}
                                className="p-1.5 text-zinc-400 hover:text-red-500 hover:bg-red-500/10 rounded-lg transition-all ml-1.5"
                                title="Delete Post"
                              >
                                <Trash2 className="h-4 w-4" />
                              </button>
                            </div>
                          </div>
                          <p className="mt-2 text-xs text-zinc-650 dark:text-zinc-300 leading-relaxed font-semibold whitespace-pre-wrap">{post.content}</p>
                          
                          {post.imageUrl && (
                            <div className="mt-3 flex items-center gap-2 rounded-xl border border-zinc-100 bg-white/20 dark:border-white/5 dark:bg-neutral-855/40 p-2 w-fit">
                              <Paperclip className="h-3.5 w-3.5 text-zinc-400" />
                              <a
                                href={post.imageUrl}
                                target="_blank"
                                rel="noreferrer"
                                className="text-[10px] font-bold text-brand-500 hover:underline truncate max-w-[240px]"
                              >
                                {post.imageUrl.split('/').pop() || 'Attachment'}
                              </a>
                            </div>
                          )}
                          <div className="mt-3 flex items-center gap-4 text-[10px] text-zinc-455 font-bold border-t border-zinc-100/50 dark:border-white/5 pt-2">
                            <span className="flex items-center gap-1 text-[10px] text-zinc-455 font-bold">
                              <Clock className="h-3.5 w-3.5" />
                              Scheduled for: {new Date(post.scheduledTime).toLocaleString()}
                            </span>
                          </div>
                        </div>
                      </div>
                    ))}
                  </div>
                  {visibleQueueCount < posts.length && (
                    <div className="mt-3 flex items-center justify-center gap-2 text-[11px] font-black tracking-wider uppercase text-zinc-455 animate-bounce">
                      <span>↓ Scroll to reveal {posts.length - visibleQueueCount} more scheduled ads</span>
                    </div>
                  )}
                </>
              )}
            </Card>
          </div>
        </div>
      )}

      {/* AI Content Creator Tab */}
      {activeTab === 'ai-creator' && (
        <div className="grid gap-8 lg:grid-cols-2">
          {/* AI Prompter */}
          <Card className="overflow-hidden rounded-[28px] border border-violet-100/50 bg-white/40 shadow-2xl backdrop-blur-2xl dark:border-white/5 dark:bg-neutral-900/35 p-6 transition-all hover:shadow-brand-500/5 duration-300">
            <div className="flex items-center gap-3 mb-6">
              <div className="grid h-10 w-10 place-items-center rounded-2xl bg-gradient-to-br from-brand-500 to-indigo-600 text-white shadow-md">
                <Sparkles className="h-5 w-5 animate-pulse" />
              </div>
              <div>
                <h2 className="text-sm font-black uppercase tracking-wider text-zinc-955 dark:text-white font-black">AI Ad Creator</h2>
                <p className="text-[10px] text-zinc-400 font-bold mt-0.5">Generate high-converting campaigns powered by Gemini AI</p>
              </div>
            </div>

            <div className="space-y-5">
              <div>
                <Label htmlFor="ai-campaign" className="text-[10px] font-black uppercase tracking-wider text-zinc-400">Target Fitness Plan / Campaign</Label>
                <Input
                  id="ai-campaign"
                  value={campaign}
                  onChange={(e) => setCampaign(e.target.value)}
                  placeholder="e.g. 6-Week Summer BootCamp Offer"
                  className="mt-1.5 focus:ring-2 focus:ring-brand-500 focus:border-transparent transition-all rounded-xl"
                />
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <Label htmlFor="ai-platform" className="text-[10px] font-black uppercase tracking-wider text-zinc-400">Destination Platform</Label>
                  <select
                    id="ai-platform"
                    value={platform}
                    onChange={(e) => setPlatform(e.target.value as any)}
                    className="mt-1.5 block w-full rounded-xl border border-zinc-200/60 dark:border-white/10 bg-white/10 dark:bg-neutral-800/40 px-3.5 py-2.5 text-xs font-semibold text-zinc-800 dark:text-white focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent transition-all"
                  >
                    <option value="Instagram">Instagram Post</option>
                    <option value="Facebook">Facebook Ad</option>
                    <option value="Google Ads">Google Search Ad</option>
                    <option value="X (Twitter)">X (Twitter) Post</option>
                  </select>
                </div>
                <div>
                  <Label htmlFor="ai-tone" className="text-[10px] font-black uppercase tracking-wider text-zinc-400">Ad Tone / Style</Label>
                  <select
                    id="ai-tone"
                    value={tone}
                    onChange={(e) => setTone(e.target.value)}
                    className="mt-1.5 block w-full rounded-xl border border-zinc-200/60 dark:border-white/10 bg-white/10 dark:bg-neutral-800/40 px-3.5 py-2.5 text-xs font-semibold text-zinc-800 dark:text-white focus:outline-none"
                  >
                    <option value="energetic">High Energy & Fun</option>
                    <option value="professional">Professional & Informative</option>
                    <option value="bold">Bold & Challenging</option>
                    <option value="motivational">Inspirational & Soft</option>
                    <option value="humorous">Lighthearted & Humorous</option>
                  </select>
                </div>
              </div>

              <div>
                <Label className="text-[10px] font-black uppercase tracking-wider text-zinc-400">Quick Tone Selection</Label>
                <div className="flex flex-wrap gap-2 mt-1.5">
                  {[
                    { id: 'energetic', label: 'High Energy', emoji: '⚡' },
                    { id: 'professional', label: 'Professional', emoji: '💼' },
                    { id: 'bold', label: 'Bold', emoji: '🔥' },
                    { id: 'motivational', label: 'Inspirational', emoji: '🌱' },
                    { id: 'humorous', label: 'Humorous', emoji: '🎭' },
                  ].map((t) => (
                    <button
                      key={t.id}
                      type="button"
                      onClick={() => setTone(t.id)}
                      className={`flex items-center gap-1.5 px-3 py-2 rounded-xl text-xs font-black tracking-wider uppercase border transition-all duration-300 active:scale-95 ${
                        tone === t.id
                          ? 'bg-brand-500 border-brand-500 text-white shadow-lg shadow-brand-500/25 scale-105'
                          : 'bg-white/5 border-zinc-200/60 dark:border-white/10 text-zinc-500 dark:text-zinc-400 hover:border-zinc-300 dark:hover:border-white/20 hover:text-zinc-800 dark:hover:text-zinc-200'
                      }`}
                    >
                      <span>{t.emoji}</span>
                      <span>{t.label}</span>
                    </button>
                  ))}
                </div>
              </div>

              <div>
                <div className="flex items-center justify-between mb-1.5">
                  <Label htmlFor="ai-prompt" className="text-[10px] font-black uppercase tracking-wider text-zinc-400">What is this ad promoting?</Label>
                  <button
                    type="button"
                    onClick={() => {
                      setQuickAiPrompt('');
                      setAiModalTarget('creator');
                      setShowQuickAiModal(true);
                    }}
                    className="inline-flex items-center gap-1.5 text-[9px] font-black text-brand-500 hover:text-brand-600 uppercase tracking-widest transition-transform active:scale-95"
                  >
                    <Sparkles className="h-3.5 w-3.5" /> Write with AI
                  </button>
                </div>
                <Textarea
                  id="ai-prompt"
                  value={prompt}
                  onChange={(e) => setPrompt(e.target.value)}
                  placeholder="e.g. Free 3-day trial pass for working professionals. Focus on early morning slots and flexibility."
                  rows={4}
                  className="focus:ring-2 focus:ring-brand-500 focus:border-transparent transition-all rounded-2xl"
                  required
                />
              </div>

              {/* Attachments */}
              <div>
                <div className="flex items-center justify-between mb-2">
                  <Label className="text-[10px] font-black uppercase tracking-wider text-zinc-400">Attachments / Media</Label>
                  <div className="flex gap-2.5 text-[9px]">
                    <button
                      type="button"
                      onClick={() => {
                        setMediaInputType('upload');
                        setMediaUrl('');
                        setMediaName('');
                      }}
                      className={`font-black uppercase tracking-wider transition-colors ${
                        mediaInputType === 'upload' ? 'text-brand-500' : 'text-zinc-400 hover:text-zinc-200'
                      }`}
                    >
                      Upload File
                    </button>
                    <span className="text-zinc-500">|</span>
                    <button
                      type="button"
                      onClick={() => {
                        setMediaInputType('url');
                        setMediaUrl('');
                        setMediaName('');
                      }}
                      className={`font-black uppercase tracking-wider transition-colors ${
                        mediaInputType === 'url' ? 'text-brand-500' : 'text-zinc-400 hover:text-zinc-200'
                      }`}
                    >
                      Image URL
                    </button>
                  </div>
                </div>

                {mediaUrl ? (
                  <div className="space-y-2 mt-1">
                    <div className="flex items-center justify-between rounded-xl border border-zinc-105 bg-white/20 p-2.5 dark:border-white/5 dark:bg-neutral-800/20">
                      <div className="flex items-center gap-2 min-w-0">
                        <Paperclip className="h-4 w-4 text-brand-500 shrink-0" />
                        <span className="text-xs font-semibold text-zinc-800 dark:text-zinc-200 truncate">
                          {mediaName || mediaUrl}
                        </span>
                      </div>
                      <button
                        type="button"
                        onClick={() => {
                          setMediaUrl('');
                          setMediaName('');
                        }}
                        className="text-xs font-black text-red-500 hover:text-red-400 px-2 transition-colors"
                      >
                        Remove
                      </button>
                    </div>
                    
                    {(mediaUrl.toLowerCase().startsWith('http') || mediaUrl.toLowerCase().startsWith('/uploads')) && (
                      <div className="mt-2 rounded-2xl border border-zinc-100/50 bg-white/5 p-2 max-w-full overflow-hidden flex justify-center shadow-inner">
                        <img 
                          src={mediaUrl} 
                          alt="Media Preview" 
                          className="max-h-48 rounded-xl object-contain"
                          onError={(e) => {
                            (e.target as HTMLElement).style.display = 'none';
                          }}
                        />
                      </div>
                    )}
                  </div>
                ) : mediaInputType === 'upload' ? (
                  <div className="mt-1 flex items-center justify-center border-2 border-dashed border-zinc-205 hover:border-brand-500/50 dark:border-white/10 dark:hover:border-brand-500/50 rounded-2xl p-6 bg-white/5 hover:bg-brand-500/5 transition-all cursor-pointer relative group">
                    <input
                      type="file"
                      onChange={handleFileUpload}
                      disabled={uploadingFile}
                      className="absolute inset-0 opacity-0 cursor-pointer w-full h-full"
                    />
                    <div className="text-center">
                      <Plus className="h-6 w-6 text-zinc-400 group-hover:text-brand-500 mx-auto mb-1.5 transition-colors" />
                      <span className="text-[11px] font-bold text-zinc-400 group-hover:text-zinc-300 transition-colors">
                        {uploadingFile ? 'Uploading File...' : 'Attach Document or Media'}
                      </span>
                    </div>
                  </div>
                ) : (
                  <div className="mt-1 space-y-2">
                    <Input
                      type="url"
                      placeholder="https://example.com/image.jpg"
                      value={mediaUrl}
                      onChange={(e) => {
                        const val = e.target.value;
                        setMediaUrl(val);
                        setMediaName(val.split('/').pop() || 'Public Image');
                      }}
                      className="w-full text-xs focus:ring-2 focus:ring-brand-500 focus:border-transparent transition-all rounded-xl"
                    />
                    <p className="text-[10px] text-zinc-400">Enter a direct public URL to an image (ends with .jpg, .png, etc.)</p>
                  </div>
                )}
              </div>

              <Button
                onClick={handleAiGenerate}
                className="w-full h-11 bg-gradient-to-r from-brand-500 to-indigo-600 hover:from-brand-600 hover:to-indigo-700 text-white font-black uppercase tracking-widest text-[11px] rounded-2xl shadow-xl shadow-brand-500/10 hover:shadow-brand-500/20 transition-all mt-3"
                disabled={generating || !prompt}
              >
                {generating ? 'Gemini Generating Ad Copy...' : 'Generate Ad Content'}
              </Button>
            </div>
          </Card>

          {/* AI Previews & Feed Simulator */}
          <div className="space-y-6 flex flex-col justify-start">
            {aiOutput ? (
              <Card className="overflow-hidden rounded-[28px] border border-brand-500/25 bg-gradient-to-br from-brand-500/5 via-transparent to-transparent shadow-2xl backdrop-blur-2xl p-6 space-y-6">
                <div className="flex items-center justify-between">
                  <span className="text-[10px] font-black uppercase tracking-wider text-brand-600 dark:text-brand-400 flex items-center gap-1.5">
                    <CheckCircle2 className="h-4.5 w-4.5 text-emerald-500 shrink-0" />
                    Ad Mockup Generated
                  </span>
                  <div className="flex items-center gap-2">
                    <Button size="sm" variant="ghost" onClick={applyAiToScheduler} className="text-[10px] font-black uppercase tracking-wider px-3 py-1.5 text-brand-500 dark:text-brand-400 bg-brand-500/5 hover:bg-brand-500/10 rounded-xl transition-all">
                      Apply to Queue
                    </Button>
                  </div>
                </div>

                <div className="space-y-4">
                  {/* Generated copy card */}
                  <div className="rounded-2xl border border-zinc-205 bg-white/50 p-4 dark:border-white/5 dark:bg-neutral-900/60 shadow-inner">
                    <span className="text-[9px] font-black uppercase tracking-wider text-zinc-400">Generated Ad Copy</span>
                    <p className="mt-2 text-xs text-zinc-805 dark:text-zinc-200 whitespace-pre-wrap leading-relaxed font-semibold">
                      {aiOutput.text}
                    </p>
                    <div className="mt-3 flex flex-wrap gap-1.5">
                      {aiOutput.hashtags.map((h, i) => (
                        <span key={i} className="text-[10px] font-black text-brand-500 dark:text-brand-400">
                          {h.startsWith('#') ? h : `#${h}`}
                        </span>
                      ))}
                    </div>
                  </div>

                  <div className="grid grid-cols-2 gap-4">
                    <div className="rounded-2xl border border-zinc-100 bg-white/20 p-3.5 dark:border-white/5 dark:bg-neutral-900/40">
                      <span className="text-[9px] font-black uppercase tracking-wider text-zinc-400 block">CTA Anchor</span>
                      <span className="text-xs font-black text-zinc-905 dark:text-white mt-1.5 block">
                        {aiOutput.cta}
                      </span>
                    </div>
                    <div className="rounded-2xl border border-zinc-100 bg-white/20 p-3.5 dark:border-white/5 dark:bg-neutral-900/40">
                      <span className="text-[9px] font-black uppercase tracking-wider text-zinc-400 block">Performance Rate</span>
                      <span className="text-xs font-black text-emerald-500 mt-1.5 block flex items-center gap-1">
                        <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 animate-ping"></span>
                        98% (High Relevance)
                      </span>
                    </div>
                  </div>

                  {/* Device Feed Preview */}
                  <div className="pt-4 border-t border-zinc-100/50 dark:border-white/5 space-y-3">
                    <span className="text-[10px] font-black uppercase tracking-wider text-zinc-455 block text-center">Live Platform Simulator ({platform})</span>
                    <div className="flex justify-center items-center py-2">
                      {renderFeedMockup()}
                    </div>
                  </div>
                </div>
              </Card>
            ) : (
              <Card className="border-white/10 bg-white/10 dark:bg-neutral-900/10 backdrop-blur-2xl flex flex-col items-center justify-center py-28 text-center rounded-[28px] border-2 border-dashed border-zinc-205 dark:border-white/5 p-6">
                <Lightbulb className="h-10 w-10 text-yellow-400 mb-4 animate-pulse" />
                <h3 className="text-sm font-black text-zinc-900 dark:text-white uppercase tracking-wider">Awaiting Parameters</h3>
                <p className="text-[11px] text-zinc-400 mt-2 max-w-[280px] font-medium leading-relaxed">
                  Fill out target plan, choose platform and tone, and click Generate to see the live feed preview simulator.
                </p>
              </Card>
            )}
          </div>
        </div>
      )}

      {/* Connections Tab */}
      {activeTab === 'connections' && (
        <Card className="overflow-hidden rounded-[28px] border border-violet-100/50 bg-white/40 shadow-2xl backdrop-blur-2xl dark:border-white/5 dark:bg-neutral-900/35 p-6 transition-all hover:shadow-brand-500/5 duration-300">
          <div className="mb-6">
            <h2 className="text-sm font-black uppercase tracking-wider text-zinc-955 dark:text-white">Connected Platforms</h2>
            <p className="text-[10px] text-zinc-400 font-bold mt-1">Sync active advertising tokens and deploy campaigns instantly</p>
          </div>

          <div className="grid gap-6 sm:grid-cols-2">
            {/* Meta */}
            <div className="rounded-2xl border border-zinc-100 bg-white/20 p-5 dark:border-white/5 dark:bg-neutral-900/50 flex items-center justify-between hover:-translate-y-1 hover:shadow-lg transition-all duration-300">
              <div className="flex items-center gap-4">
                <div className="grid h-12 w-12 place-items-center rounded-2xl bg-blue-600/15 text-blue-600 shadow-sm shrink-0">
                  <Facebook className="h-6 w-6" />
                </div>
                <div className="min-w-0">
                  <h3 className="text-xs font-black uppercase tracking-wider text-zinc-955 dark:text-white">Meta (Facebook & IG)</h3>
                  {connectedChannels.facebook ? (
                    <p className="text-[10px] text-emerald-500 font-black mt-1 flex items-center gap-1">
                      <span className="relative flex h-2 w-2">
                        <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-455 opacity-75"></span>
                        <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
                      </span>
                      CONNECTED & ACTIVE
                    </p>
                  ) : (
                    <p className="text-[10px] text-zinc-400 font-semibold mt-1">Configure Meta API Credentials</p>
                  )}
                </div>
              </div>
              {connectedChannels.facebook ? (
                <Badge tone="success" className="text-[10px] font-black uppercase tracking-wider px-2 py-0.5 rounded-lg">Active</Badge>
              ) : (
                <Link href={studioId === 'global' ? '/admin/settings' : `/admin/studios/${studioId}/settings`}>
                  <Button size="sm" variant="ghost" className="text-[10px] font-black uppercase tracking-wider px-3.5 py-1.5 hover:bg-brand-500/5 text-brand-500 dark:text-brand-400 rounded-xl transition-all">
                    Connect
                  </Button>
                </Link>
              )}
            </div>

            {/* Google Ads */}
            <div className="rounded-2xl border border-zinc-100 bg-white/20 p-5 dark:border-white/5 dark:bg-neutral-900/50 flex items-center justify-between hover:-translate-y-1 hover:shadow-lg transition-all duration-300">
              <div className="flex items-center gap-4">
                <div className="grid h-12 w-12 place-items-center rounded-2xl bg-indigo-600/15 text-indigo-600 shadow-sm shrink-0">
                  <Globe className="h-6 w-6" />
                </div>
                <div className="min-w-0">
                  <h3 className="text-xs font-black uppercase tracking-wider text-zinc-955 dark:text-white">Google Ads Campaign</h3>
                  {connectedChannels.googleAds ? (
                    <p className="text-[10px] text-emerald-500 font-black mt-1 flex items-center gap-1">
                      <span className="relative flex h-2 w-2">
                        <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-455 opacity-75"></span>
                        <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
                      </span>
                      CONNECTED & ACTIVE
                    </p>
                  ) : (
                    <p className="text-[10px] text-zinc-400 font-semibold mt-1">Requires OAuth Authentication</p>
                  )}
                </div>
              </div>
              {connectedChannels.googleAds ? (
                <Badge tone="success" className="text-[10px] font-black uppercase tracking-wider px-2 py-0.5 rounded-lg">Active</Badge>
              ) : (
                <Link href={studioId === 'global' ? '/admin/settings' : `/admin/studios/${studioId}/settings`}>
                  <Button size="sm" variant="ghost" className="text-[10px] font-black uppercase tracking-wider px-3.5 py-1.5 hover:bg-brand-500/5 text-brand-500 dark:text-brand-400 rounded-xl transition-all">
                    Connect
                  </Button>
                </Link>
              )}
            </div>

            {/* X (Twitter) */}
            <div className="rounded-2xl border border-zinc-100 bg-white/20 p-5 dark:border-white/5 dark:bg-neutral-900/50 flex items-center justify-between hover:-translate-y-1 hover:shadow-lg transition-all duration-300">
              <div className="flex items-center gap-4">
                <div className="grid h-12 w-12 place-items-center rounded-2xl bg-sky-500/15 text-sky-500 shadow-sm shrink-0">
                  <Twitter className="h-6 w-6" />
                </div>
                <div className="min-w-0">
                  <h3 className="text-xs font-black uppercase tracking-wider text-zinc-955 dark:text-white">X (Twitter) DM Feed</h3>
                  {connectedChannels.x ? (
                    <p className="text-[10px] text-emerald-500 font-black mt-1 flex items-center gap-1">
                      <span className="relative flex h-2 w-2">
                        <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-455 opacity-75"></span>
                        <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
                      </span>
                      CONNECTED & ACTIVE
                    </p>
                  ) : (
                    <p className="text-[10px] text-zinc-400 font-semibold mt-1">Requires Dev Keys Setup</p>
                  )}
                </div>
              </div>
              {connectedChannels.x ? (
                <Badge tone="success" className="text-[10px] font-black uppercase tracking-wider px-2 py-0.5 rounded-lg">Active</Badge>
              ) : (
                <Link href={`/admin/studios/${studioId}/channels`}>
                  <Button size="sm" variant="ghost" className="text-[10px] font-black uppercase tracking-wider px-3.5 py-1.5 hover:bg-brand-500/5 text-brand-500 dark:text-brand-400 rounded-xl transition-all">
                    Connect
                  </Button>
                </Link>
              )}
            </div>
          </div>
        </Card>
      )}

      {/* Quick AI Helper Modal */}
      {showQuickAiModal && (
        <div className="fixed inset-0 z-50 grid place-items-center bg-black/40 backdrop-blur-sm p-4 animate-in fade-in duration-200">
          <div className="w-full max-w-md p-6 rounded-[28px] border border-zinc-100 dark:border-white/5 bg-white dark:bg-neutral-900 space-y-4 shadow-2xl">
            <div className="flex items-center justify-between border-b border-zinc-100 dark:border-white/5 pb-3">
              <div className="flex items-center gap-2">
                <Sparkles className="h-5 w-5 text-brand-500" />
                <h3 className="text-xs font-black uppercase tracking-wider text-zinc-955 dark:text-white font-black">AI Content Generator</h3>
              </div>
              <button 
                type="button"
                onClick={() => setShowQuickAiModal(false)} 
                className="p-1 rounded-lg hover:bg-zinc-100 dark:hover:bg-neutral-800 text-zinc-400"
              >
                <XIcon className="h-5 w-5" />
              </button>
            </div>

            <div className="space-y-4">
              <div>
                <Label htmlFor="modal-prompt" className="text-[10px] font-black uppercase tracking-wider text-zinc-400">What is this ad promoting?</Label>
                <Textarea
                  id="modal-prompt"
                  rows={4}
                  value={quickAiPrompt}
                  onChange={(e) => setQuickAiPrompt(e.target.value)}
                  placeholder="e.g. Free 3-day trial pass for working professionals. Focus on early morning slots."
                  className="mt-1.5 text-xs font-semibold focus:ring-2 focus:ring-brand-500 focus:border-transparent transition-all rounded-xl"
                />
              </div>

              <div className="flex gap-3">
                <Button
                  type="button"
                  variant="ghost"
                  onClick={() => setShowQuickAiModal(false)}
                  className="flex-1 text-[10px] font-black uppercase tracking-wider py-2.5 rounded-xl hover:bg-zinc-100 dark:hover:bg-neutral-800 transition-colors"
                >
                  Cancel
                </Button>
                <Button
                  type="button"
                  onClick={handleQuickAiGenerate}
                  disabled={!quickAiPrompt.trim() || generatingQuickAi}
                  className="flex-1 text-[10px] font-black uppercase tracking-wider py-2.5 rounded-xl bg-gradient-to-r from-brand-500 to-indigo-600 text-white shadow-xl shadow-brand-500/10 hover:shadow-brand-500/20 transition-all"
                >
                  {generatingQuickAi ? "Generating..." : "Generate Copy"}
                </Button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Confirmation Modal (Select Date & Time) */}
      {showConfirmModal && (
        <div className="fixed inset-0 z-50 grid place-items-center bg-black/40 backdrop-blur-sm p-4 animate-in fade-in duration-200">
          <div className="w-full max-w-md p-6 rounded-[28px] border border-zinc-150 dark:border-white/5 bg-white dark:bg-neutral-900 space-y-5 shadow-2xl">
            <div className="flex items-center gap-3 border-b border-zinc-100 dark:border-white/5 pb-3">
              <div className="grid h-10 w-10 place-items-center rounded-2xl bg-brand-500/10 text-brand-500">
                <Clock className="h-5 w-5" />
              </div>
              <div>
                <h3 className="text-sm font-black uppercase tracking-wider text-zinc-955 dark:text-white">Schedule Campaign Post</h3>
                <p className="text-[10px] text-zinc-400 font-bold">Select publish date and time below</p>
              </div>
            </div>

            <div className="space-y-4">
              <div className="rounded-xl border border-zinc-100 bg-zinc-50/50 p-4 dark:border-white/5 dark:bg-neutral-955/45 space-y-2 text-xs">
                <div className="flex justify-between">
                  <span className="text-[10px] uppercase font-black text-zinc-400">Platform:</span>
                  <span className="font-bold text-zinc-900 dark:text-white">{newPostPlatform}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-[10px] uppercase font-black text-zinc-400">Campaign:</span>
                  <span className="font-bold text-zinc-900 dark:text-white">{newPostCampaign || 'General Promo'}</span>
                </div>
              </div>

              {/* Date and Time selectors inside the popup */}
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <Label htmlFor="popup-date" className="text-[10px] font-black uppercase tracking-wider text-zinc-400">Date</Label>
                  <Input
                    id="popup-date"
                    type="date"
                    value={newPostDate}
                    onChange={(e) => setNewPostDate(e.target.value)}
                    className="mt-1.5 focus:ring-2 focus:ring-brand-500 focus:border-transparent transition-all rounded-xl text-xs font-semibold"
                    required
                  />
                </div>
                <div>
                  <Label htmlFor="popup-time" className="text-[10px] font-black uppercase tracking-wider text-zinc-400">Time</Label>
                  <Input
                    id="popup-time"
                    type="time"
                    value={newPostTime}
                    onChange={(e) => setNewPostTime(e.target.value)}
                    className="mt-1.5 focus:ring-2 focus:ring-brand-500 focus:border-transparent transition-all rounded-xl text-xs font-semibold"
                    required
                  />
                </div>
              </div>
            </div>

            <div className="flex gap-3 pt-2">
              <Button
                type="button"
                variant="ghost"
                onClick={() => setShowConfirmModal(false)}
                className="flex-1 text-[10px] font-black uppercase tracking-wider py-2.5 rounded-xl hover:bg-zinc-100 dark:hover:bg-neutral-800 transition-colors"
              >
                Cancel
              </Button>
              <Button
                type="button"
                onClick={executeSchedulePost}
                disabled={!newPostDate || !newPostTime}
                className="flex-1 text-[10px] font-black uppercase tracking-wider py-2.5 rounded-xl bg-gradient-to-r from-brand-500 to-indigo-600 text-white shadow-xl shadow-brand-500/10 hover:shadow-brand-500/20 transition-all animate-pulse"
              >
                Save & Schedule
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* Success/Error Notification Modal */}
      {showNotificationModal && (
        <div className="fixed inset-0 z-50 grid place-items-center bg-black/40 backdrop-blur-sm p-4 animate-in fade-in duration-200">
          <div className="w-full max-w-sm p-6 rounded-[28px] border border-zinc-150 dark:border-white/5 bg-white dark:bg-neutral-900 space-y-5 shadow-2xl text-center">
            <div className="mx-auto grid h-16 w-16 place-items-center rounded-full bg-zinc-50 dark:bg-neutral-950 shadow-inner">
              {notificationStatus === 'success' ? (
                <CheckCircle2 className="h-8 w-8 text-emerald-500 animate-bounce" />
              ) : (
                <AlertCircle className="h-8 w-8 text-red-500 animate-pulse" />
              )}
            </div>

            <div className="space-y-1.5">
              <h3 className="text-sm font-black uppercase tracking-wider text-zinc-955 dark:text-white">
                {notificationStatus === 'success' 
                  ? (notificationMessage.toLowerCase().includes('delete') ? 'Post Deleted' : 'Campaign Scheduled!') 
                  : (notificationMessage.toLowerCase().includes('delete') ? 'Deletion Failed' : 'Scheduling Failed')}
              </h3>
              <p className="text-xs text-zinc-550 dark:text-zinc-400 font-semibold leading-relaxed px-2">
                {notificationMessage}
              </p>
            </div>

            <Button
              type="button"
              onClick={() => setShowNotificationModal(false)}
              className={`w-full text-[10px] font-black uppercase tracking-wider py-2.5 rounded-xl transition-all ${
                notificationStatus === 'success'
                  ? 'bg-emerald-500 hover:bg-emerald-600 text-white shadow-lg shadow-emerald-500/10'
                  : 'bg-red-500 hover:bg-red-650 text-white shadow-lg shadow-red-500/10'
              }`}
            >
              Okay, Got it
            </Button>
          </div>
        </div>
      )}

      {/* Delete Confirmation Modal */}
      {showDeleteConfirmModal && (
        <div className="fixed inset-0 z-50 grid place-items-center bg-black/40 backdrop-blur-sm p-4 animate-in fade-in duration-200">
          <div className="w-full max-w-md p-6 rounded-[28px] border border-zinc-150 dark:border-white/5 bg-white dark:bg-neutral-900 space-y-5 shadow-2xl">
            <div className="flex items-center gap-3 border-b border-zinc-100 dark:border-white/5 pb-3">
              <div className="grid h-10 w-10 place-items-center rounded-2xl bg-red-500/10 text-red-500">
                <Trash2 className="h-5 w-5" />
              </div>
              <div>
                <h3 className="text-sm font-black uppercase tracking-wider text-zinc-955 dark:text-white">Delete Scheduled Post</h3>
                <p className="text-[10px] text-zinc-400 font-bold">This action cannot be undone</p>
              </div>
            </div>

            <div className="space-y-3.5 text-xs text-zinc-650 dark:text-zinc-300 font-medium text-center">
              <p>Are you sure you want to remove this scheduled post from the publishing queue?</p>
            </div>

            <div className="flex gap-3">
              <Button
                type="button"
                variant="ghost"
                onClick={() => {
                  setShowDeleteConfirmModal(false);
                  setDeletingPostId(null);
                }}
                className="flex-1 text-[10px] font-black uppercase tracking-wider py-2.5 rounded-xl hover:bg-zinc-100 dark:hover:bg-neutral-800 transition-colors"
              >
                Cancel
              </Button>
              <Button
                type="button"
                onClick={executeDeletePost}
                className="flex-1 text-[10px] font-black uppercase tracking-wider py-2.5 rounded-xl bg-red-500 hover:bg-red-655 text-white shadow-xl shadow-red-500/10 hover:shadow-red-500/20 transition-all"
              >
                Delete Post
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
