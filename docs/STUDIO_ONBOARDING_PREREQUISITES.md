# Studio Onboarding — Prerequisites Checklist

Before a studio can go live on **1herosocial.ai**, five things need to be in
place. This doc is what to hand a prospective studio (or collect from them)
before/during the kickoff call. Each section links to the deeper technical
setup guide for whoever actually clicks the buttons.

| # | Prerequisite | Who owns it | Blocks |
|---|---|---|---|
| 1 | Studio Knowledge Document | Studio | AI reply quality on day one |
| 2 | Plan selection | Studio + Sales | Channel count, AI reply volume, feature access |
| 3 | WhatsApp — Meta Business verification | Studio | Message volume (unverified = throttled) |
| 4 | Google Ads account | Studio | Pro Tier+ ad channel only |
| 5 | Google Sheets | Studio | Lead export/sync |

---

## 1. Studio Knowledge Document

This is what the AI reads to answer customer questions, so it needs to exist
*before* the AI can sound like it knows the business. Ask the studio to
prepare one document (or a short set of files) covering:

- **Services / classes offered** — names, descriptions, schedule if fixed
- **Pricing & membership tiers** — trial pricing, monthly/annual plans, drop-in rates
- **Hours & location(s)** — per-location if multi-site
- **Policies** — cancellation, refund, trial-to-membership conversion, late/no-show
- **FAQs** — the questions their front desk answers most often
- **Greeting message** — the first auto-reply a new lead sees (supports
  `{{lead_first_name}}`, `{{lead_name}}`, `{{studio_name}}` placeholders)
- **Tone/voice guidance** — formal vs. casual, emoji usage, brand phrases to use/avoid

**Accepted formats:** PDF, Word, PowerPoint, Excel, plain text, CSV, or Markdown —
uploaded directly in **Knowledge Base** (Studio Admin → Knowledge Base), or
pasted as text. Each uploaded document can be tagged to a domain (General,
Fitness/Gym, Yoga, CrossFit, Pilates, Dance, Martial Arts, Nutrition,
Recovery) so multi-discipline studios get domain-accurate answers instead of
one document overriding another.

Reference: [`docs/AI_CHATBOT.md`](AI_CHATBOT.md), [`docs/AI_RESPONSE_GUIDE.md`](AI_RESPONSE_GUIDE.md)

---

## 2. Plan Selection

| Plan | Price | Channels | AI Auto-Replies | Notable features |
|---|---|---|---|---|
| **Trial Pass** | $300 one-time | 1 | 200/mo | 1-day automated follow-up, Google Sheets sync |
| **Growth Tier** | $999/mo | 3 | 2,000/mo | Dedicated Knowledge Base, visual pipeline, Stripe integration |
| **Pro Tier** | $1,299/mo | 8 | 10,000/mo | Dual model routing (Gemini + Claude), Advanced Social Planner, **Google Ads Channel Integration**, Studio Plan/Scheduling |
| **Enterprise Tier** | $1,599/mo | Unlimited | Unlimited | Multi-Location Hub, whitelabel dashboard, priority support |

- Google Ads is only available on **Pro Tier and above** — confirm the plan
  before promising a studio ad-channel access.
- The AI Auto-Reply cap is a *platform plan* limit — separate from the
  WhatsApp verification cap in §3 below. A studio can be under both caps at
  once; the lower one wins.

Current numbers live in `platform_plans.go`; confirm against **Superadmin →
Platform Settings → Plans** before quoting, since these are editable at
runtime.

---

## 3. WhatsApp — Meta Business Verification

WhatsApp connects through Meta's Cloud API directly (no BSP/Twilio markup).
Two things are required, and skipping the second one is the most common
reason a studio's messages stall:

### a) WhatsApp Business Account (WABA) + phone number
A phone number dedicated to the WhatsApp Business API (it cannot already be
active on the regular WhatsApp or WhatsApp Business consumer apps).

### b) Meta Business verification
**This is the one to flag early — it gates message volume, not just features.**

Meta puts every WhatsApp Business Account into a messaging tier based on
business verification + quality rating:

- **Unverified** business → lowest tier, capped around **50 unique
  conversations per rolling 24 hours**. Past that, new outbound
  conversations (including AI auto-replies to new leads) are throttled by
  Meta until the window rolls over — this is a Meta-side limit, not
  something we can raise from our side.
- **Verified** business (Meta Business Manager → Business Settings →
  Security Center → Start Verification: legal business name, registered
  address, business registration number, and a document proving it) →
  tier increases to 1K / 10K / 100K+ conversations/day as quality rating
  holds up over time.

**What to tell the studio:** get Meta Business verification started as
early as possible in onboarding — it can take Meta anywhere from a day to
a couple of weeks depending on document review, and nothing else in this
checklist depends on Meta review time.

**What we need from the studio to connect the channel** (Studio Admin →
Channels → Connect WhatsApp):

| Value | Where to find it |
|---|---|
| WABA ID | Meta WhatsApp → API Setup → "WhatsApp Business Account ID" |
| Phone Number ID | Meta WhatsApp → API Setup → "From" dropdown |
| Display Phone Number | The public-facing number, e.g. `+1 555 645 5341` |
| Access Token | Permanent System User token (production) — temporary 24h token is fine for testing only |

Reference: [`docs/SETUP_META_WHATSAPP.md`](SETUP_META_WHATSAPP.md) (full
runbook, including how to generate a permanent System User token),
[`docs/CLIENT_USER_MANUAL.md`](CLIENT_USER_MANUAL.md) §3 Step 2.

---

## 4. Google Ads (Pro Tier and above only)

Needed only if the studio is on Pro/Enterprise and wants the Ads channel.
Five values, gathered from Google Ads + Google Cloud Console
(Studio Admin → Channels → Connect Google Ads):

| Value | Source |
|---|---|
| Google Client ID | Google Cloud Console → OAuth 2.0 credentials |
| Google Client Secret | Same OAuth client |
| Google Developer Token | Google Ads → API Center (needs Basic access approval from Google — this can take a few days, start early) |
| Google Ads Customer ID | The studio's Ads account, digits only |
| Login Customer ID (optional) | Only if the account sits under a manager/MCC account |

**Prerequisite:** the studio must have an **active Google Ads account with
billing configured** before this step — a brand-new, unfunded account will
authenticate but can't run campaigns.

---

## 5. Google Sheets (Lead Sync)

Every lead syncs to a Google Sheet within ~5 seconds of creation, with
automatic retry if Sheets is temporarily unreachable (leads are never lost
— they save to the database first regardless).

Studio needs to:
1. Create a Google Sheet ahead of time (any name), with a tab for leads
   (e.g. `Leads`).
2. Share that sheet as **Editor** with the platform's service account
   email (get this from Superadmin → Platform Settings → Credentials
   Manager — it's project-specific, e.g.
   `studiox-sheets-writer@<project>.iam.gserviceaccount.com`).
3. Copy the **Spreadsheet ID** from the sheet's URL
   (`.../spreadsheets/d/<THIS_PART>/edit`).
4. Enter Spreadsheet ID + tab name in Studio Settings → Google Sheets Sync.

Reference: [`docs/SETUP_GOOGLE_SHEETS.md`](SETUP_GOOGLE_SHEETS.md),
[`GOOGLE_SHEETS_SETUP_GUIDE.md`](../GOOGLE_SHEETS_SETUP_GUIDE.md) (root —
platform-owner side, service account creation).

---

## Pre-Kickoff Checklist (give this to the studio)

```
[ ] Studio info doc drafted (services, pricing, hours, policies, FAQs)
[ ] Plan chosen (Trial / Growth / Pro / Enterprise)
[ ] Meta Business Manager account created
[ ] Meta Business verification started (do this first — it's the long pole)
[ ] Dedicated phone number ready for WhatsApp Business API
[ ] Google Ads account active with billing set up      (Pro+ only)
[ ] Google Ads Developer Token requested                (Pro+ only)
[ ] Google Sheet created for lead sync
[ ] Google Sheet shared with platform service account (Editor)
```

Once all boxes are checked, connecting channels in the platform itself
(Channels page, Knowledge Base page, Settings page) takes minutes — the
lead time in this whole process is Meta's business verification and
Google's developer token approval, both of which are outside our control.
