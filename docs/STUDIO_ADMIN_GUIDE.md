# Studio Admin Guide

_As of 2026-10-10_

Plain-language "how to use it" guide for every page in the admin sidebar — written for studio staff, not developers.

> **Two things called "templates"** — don't confuse them: **Inbox → Snippets** (staff's reusable quick-reply messages, inserted into chat replies) vs. the **Templates** sidebar page (the automatic payment-confirmation message sent after a trial/membership purchase, plus Glofox linking fields). Both are covered below, in their own sections.

> `docs/PLATFORM_GUIDE.md` is a separate, denser technical reference (routes, DB columns, internal function names) aimed at developers — this file is the plain-language counterpart for end users. The two aren't kept in sync automatically.

## Contents

1. [Dashboard](#1-dashboard)
2. [Inbox](#2-inbox)
3. [Pipeline](#3-pipeline)
4. [Campaigns](#4-campaigns)
5. [Leads](#5-leads)
6. [Social Planner](#6-social-planner)
7. [Payments](#7-payments)
8. [Channels](#8-channels)
9. [Attendance](#9-attendance)
10. [Knowledge Base](#10-knowledge-base)
11. [Decision Trees](#11-decision-trees)
12. [Templates](#12-templates)
13. [Users (Team Members)](#13-users-team-members)
14. [Roles](#14-roles)
15. [Settings](#15-settings)

---

## 1. Dashboard

The studio's home screen — a quick-glance summary of how the business is doing, plus a deeper "Detailed Analytics" view for digging into numbers over a specific time period.

- Lands on the **Overview** tab by default; click **Detailed Analytics** to switch.
- **New Campaign** (top right) jumps straight into creating a campaign.

### Overview tab — KPI cards

| Card | What it tells you |
| --- | --- |
| Campaigns | How many exist and how many are active. Click → Campaigns page. |
| Total Leads | Total ever collected, with a "N new waiting" note if unhandled. Click → Leads list. |
| Conversion Health | % of all leads who've booked a trial or become a member. |
| Escalations | Currently-escalated vs. already-resolved count. Click → Inbox's escalation queue. |

### Overview tab — funnel widgets

- **Pipeline Overview** — a horizontal funnel of how many leads sit at each stage (New → Contacted → Trial Booked → Member, etc.), to spot where leads pile up or fall off.
- **Lead Distribution** — the same breakdown as a donut chart, useful for proportions at a glance.

### Overview tab — bottom section

- **Active Campaigns** list — recent campaigns with lead counts and active/draft status; click to open, or "View all."
- **Latest Activity** — the 5 newest lead submissions.

### Analytics tab

**Date range**: click a preset (Today, 1 Week, 15 Days, 1 Month, 1 Quarter, 1 Year, All Time) or **Custom Date** for your own From/To. Every card below updates to that period.

| Card | What it means |
| --- | --- |
| Conversion (Trial → Member) | % of trial-bookers who became paying members |
| Outbound Messages Sent | Total messages sent across all channels — activity volume |
| Trials Purchased | How many leads are at "Trial Booked" — click to see the list |
| Members | How many leads are "Member" — click to see the list |
| Connection Ratio | % of contacted leads who ever replied |
| Conversion Ratio | % of ALL leads that ended up booking a trial or becoming a member |
| Daily Activity chart | Trend line of messages sent / leads connected / leads converted per day |
| Open Follow-ups | New/Contacted leads messaged fewer than 3 times — still need outreach |
| Unresponded Conversations | Open chats where the customer's last message is unanswered — your "reply now" queue |
| Members Dropped % / Paused % | Share of leads whose membership is cancelled or on hold — churn indicators |
| Avg Response Time | Average time to reply to an incoming message |
| Lead → Trial Duration | Average time from lead form submission to first trial booked |
| Trial → Member Duration | Average time from trial booked to paying member |
| Campaign Conversions table | Per-campaign leads/conversions/rate — click a campaign to open it |
| Acquisition Channels table | Same breakdown by platform/source instead of campaign |

**Paid Marketing ROI Analysis (calculator)** — drag two sliders, "Monthly Paid Ad Spend" and "Average Member Lifetime Value." Calculates live, from your own inputs: leads/members produced by paid channels, cost per lead, cost per member, estimated revenue return, and ROI %. These are estimates based on what you enter, not pulled from any real ad account.

*Note: "Conversion Health" (Overview) and "Conversion Ratio" (Analytics) are the same formula (trial+member leads ÷ total leads), just shown in two places with different labels.*

---

## 2. Inbox

The main messaging screen — a WhatsApp-style layout where studio staff read and reply to every customer conversation across all connected channels (WhatsApp Cloud API, WhatsApp QR/Web, Instagram, Messenger, SMS, Telegram bot, Telegram QR).

A **channel selector** at the top-right switches which connected channel's conversations you're viewing; changing it reloads the list and clears whatever conversation you had open. The **Live** / **Stream** badges are permanent indicators this page updates in real time — not clickable, not a status you need to monitor.

Five tabs: **Conversations**, **Escalation**, **Manual Actions**, **Snippets**, **Trigger Links**.

### Conversations tab

**Filtering and finding conversations**
- Click **All / Unread / Recents / Starred** above the list. Unread shows a count badge; Starred only shows flagged conversations.
- Type in the search box to filter by contact name or number. No match + Enter starts a brand-new conversation with whatever you typed.
- Toggle **Awaiting Reply** to show only conversations where the customer's last message hasn't been answered yet.
- Tick checkboxes (or the header checkbox) to star or delete multiple conversations at once.

Each card shows: contact name (or raw number if unknown), a color-coded channel badge, last message preview, timestamp, unread-count bubble, lead-status chip (New / Connected / Trial / Member / Dropped / Paused) when linked to a lead, who it's assigned to, and a star icon.

**Assigning and starring**
- Click the star icon on a card or in the chat header to flag a conversation.
- For lead-linked contacts, an **Assign Owner** dropdown in the chat header assigns responsibility to a staff member. The Contact Details panel has a fuller Owner + Followers picker — Owner is the one responsible staff member; Followers is a personal reminder list saved only in your own browser, not shared with teammates.

**Two separate AI auto-reply toggles — don't confuse them**
- **Per-conversation toggle** (Contact Details → DND tab): turns the AI on/off for that one contact only.
- **"AI Auto-Reply (Sheet Leads)"** (studio-wide, above the filters): only affects brand-new leads imported through the studio's connected Google Sheet. ON keeps the AI following up after the first greeting; OFF means just that one greeting, then a human takes over. Requires a Sheet import configured in Settings; never touches an already-open conversation or the per-conversation toggle.

**Message composer**
- Type and press **Enter** to send; **Shift+Enter** for a line break.
- Paperclip icon opens: **Templates** (insert a saved Snippet, with attachment if any), **Tracked Link** (insert a Trigger Link), **Photo/File** (attach media), **Availability** (insert your booking availability).
- An attached file previews above the text box — click X to remove before sending.
- Message status ticks (sent/delivered/read) — see the Message Status Ticks note below.

**Contact Details panel** (click the contact's name/avatar)
- Top: name, Owner dropdown, lead-status dropdown, Tags (system tags plus your own — custom tags saved only in your own browser).
- **All Fields**: a read-only, searchable view of everything on file for the lead.
- **DND**: holds both the per-conversation AI Auto-Reply toggle and the actual Do Not Disturb toggle, which silences all automated follow-ups and AI replies to this contact.
- **Actions**: "Mark Closed" / "Reassign Owner" exist but aren't wired up yet — not working today.
- A conversation with no linked lead only shows the name/number and the two toggles — no tags or detailed fields.

*Note: the Archive, Tag, Phone, and Video-call buttons in the chat header are currently decorative — they don't do anything yet.*

### Escalation tab

A dedicated queue of conversations the system decided need a real staff member instead of (or alongside) the AI. A conversation lands here from: class booking/cancel/reschedule requests, complaints/billing issues, a question the AI's knowledge base can't answer, a manual membership change, a Stripe-not-configured situation, or an explicit "escalate to human" decision-tree step.

- The sidebar lists only escalated conversations, tagged with their reason (e.g. "complaint", "needs_answers"), with a count badge on the tab.
- Has its own search box, but no new-conversation composer.
- Opening one shows a banner with the reason and how long ago it escalated.
- Click **Resolve escalation** in the chat header once handled — returns it to the normal Conversations list.

### Manual Actions tab

Lets staff schedule one-off outbound messages — immediately or at a future date/time — and manage anything still waiting or stuck. Two sub-views: **Scheduled Messages** and **Broadcasts** (bulk campaigns).

**Creating a scheduled message**: tick recipients (searchable, Select All/Deselect All), type the message (optionally insert a Snippet or Trigger Link, optionally attach a file/media URL), pick **Scheduled For** (blank = as soon as possible), click **Schedule Message**.

**Managing the queue** — each row: recipient, channel, message preview, scheduled time, status, attempt count:
- **Edit** (pencil) — reload into the form, change text/attachment/time, "Update Message".
- **Send Now / Retry** (play icon) — force it out immediately; reads "Retry" after a prior failure.
- **Cancel** (trash icon) — permanently removes it, with confirmation.

| Status | Meaning |
| --- | --- |
| Scheduled (purple) | Still waiting. If a prior attempt failed, shows "Retrying: &lt;reason&gt;" — keeps trying automatically. |
| Blocked (red) | Gave up after 6 failed attempts. Fix the issue (contact not on WhatsApp, channel disconnected, invalid login, daily limit hit) and click Retry. |

No separate "sent" status — a successfully sent message just disappears from the list.

### Snippets tab

A library of reusable, pre-written message templates for staff to drop into any reply.

**Create/edit form**: Template Name; Template Body (supports `{{contact.first_name}}`); Template Media/File (optional); Available Channels (WhatsApp, Instagram, Messenger, SMS); a **Write with AI** button to generate the text instead of writing by hand.

**Using it**: paperclip → **Templates** → search/scroll → click a snippet's name to insert it (and its attachment) into your draft. Also reused by Follow-up Steps (Decision Trees) and the Pipeline's Re-engage action.

*This is a simple, studio-managed template library — no WhatsApp "approved template" flow (category, language, Meta approval) here.*

### Trigger Links tab

Creates a short, trackable link (e.g. a booking page or offer) so you can see how many times each one gets clicked.

- **Creating one**: Link Name + Destination URL, click **Create Link**.
- **Per link**: name, live click count, real destination URL, generated tracked short link with **Copy**. Edit/Delete available.
- **Tracking**: a click is logged (tied to that lead's record if known) before redirecting to your real URL.
- **Using it elsewhere**: paperclip → **Tracked Link** in the composer — same picker available in Snippets and Manual Actions.

### Message status ticks

| Icon | Meaning |
| --- | --- |
| ··· | Still sending |
| ✓ (single) | Sent |
| ✓✓ (gray) | Delivered to the recipient's device |
| ✓✓ (blue) | Read by the recipient |
| ! | Failed to send |

Updates live while the conversation is open — no refresh needed. **Channel support**: WhatsApp (Cloud API) and WhatsApp (QR/Web) report real delivered/read receipts; Instagram, Messenger, SMS, and Telegram only ever show single-tick "sent" — those platforms don't feed delivery/read confirmations back into this system.

---

## 3. Pipeline

A Kanban-style board for visually managing every lead's position in your sales funnel, and for spotting and re-engaging leads that have gone quiet.

### The board

- 7 columns: **New, Contacted, Trial Booked, Member, Dropped, Paused**, plus a separate **Cold** column. (Cold isn't a lead status — it's a live-computed view over conversations.)
- **Drag any lead's card** to another column to change its status — saves instantly (briefly "pending"; on failure the card snaps back with an error).
- Click a card to open its full lead detail page.
- Each column shows a running count; "+N more" links to the full filtered Leads list.
- The bar above the board shows total leads, how many are "active," and overall conversion %.

### The Cold column

A contact shows up here when: **"Never replied"** (you messaged them, they never responded, and your studio's threshold has passed) or **"Stalled"** (they replied once, but it's been long enough since the last message with no progress). Excludes Member/Dropped leads. Re-checked every couple of hours; drops out once they reply, move stage, or get re-engaged. "Imported" tags mark conversation history that predates connecting this platform.

**Re-engaging**: tick checkboxes (or Select All), click **Re-engage (N)** — opens a popup pre-filled with your last-used message (or a saved template, or a generic fallback). Edit the text, or pick a saved template (from Inbox → Snippets) from the dropdown. Placeholders like `{{lead_first_name}}`/`{{studio_name}}` fill in per recipient. Click **Send** — whatever you used becomes the new default pre-fill next time. You can also drag a Cold card directly into a regular status column to move it into your pipeline without Re-engage.

---

## 4. Campaigns

A campaign is a shareable, trackable lead-capture page — a mini landing page with a short form, for an Instagram bio, a story link, an ad, etc. Each has its own public URL and tracks its own leads.

### Campaigns list page

Each campaign is a card: active/draft badge, name, URL slug, description, lead count, plan count, linked social posts (if any). **Copy** copies the public URL; the external-link icon previews the live page. **New Campaign** to create one.

### Create/edit form

- **Campaign name** (required)
- **URL slug** (optional, auto-generated from name if blank; must be unique within your own studio)
- **Description** (optional, shown above the form on the public page)
- **Fitness plans** (one per line — becomes the dropdown choices on the public form)

### Campaign detail page

Share link, status, total leads, plan count, creation date, a "Fitness plans offered" editor (updates the live form immediately), **Activate / Deactivate** toggle, and a recent-leads table with a clickable status breakdown.

### The public lead-capture form (`/l/[studioSlug]/[campaignSlug]`)

A branded form: First name, Last name, Email, Phone, plan dropdown, optional free-text box. Redirects to a thank-you page after submit. A 20-second per-visitor cooldown prevents accidental duplicate submissions. Generated automatically the moment a campaign is created.

---

## 5. Leads

The full list of everyone who has ever shown interest — browse, search, filter, bulk-manage, import, export, and edit individual records.

### The list

Each row: avatar/name, contact info, campaign, source, contact attempts, last message time, status flags, pipeline status, submission date. Flags: 📞 Needs Follow-up, 🔥 Hot, 🎟️ Trial, 💬 Made. Click a row to open the lead's detail page. Auto-refreshes every few seconds.

### Filters

Search (name/email/phone), Campaign, Period (Today/1 Week/15 Days/1 Month/Custom), Source, Status. On mobile these collapse into a **Filters** sheet (**Apply**/**Reset**). Results paginate 25 at a time.

### Header actions

- **Enable AI** — bulk action: pick pipeline statuses, every matching uncontacted lead is connected to AI auto-contact and messaged immediately. One-time "can't be undone" warning before confirming.
- **Import Leads** — upload CSV/Excel, pick a Default Campaign, drag-and-drop or browse, **Start Import** (template provided). Also auto-recognizes a **raw Glofox member-export file** and translates Glofox's status wording into this platform's pipeline stages.
- **Export** — downloads the currently filtered/searched list (full filtered set, ignoring pagination).
- **Sheets Connection** — shortcut to Settings' Google Sheets sync.

### Lead detail page — editable fields

First/Last name, Assigned to, Offer, Plan Name, Status, Monthly Fee + currency (SGD/INR), internal notes, tracking checkboxes (Contact Made, Hot Lead, Trial Purchased, Trial Attended, Member Sold). **Save changes** activates only once something's changed. Read-only context: email, phone, fitness plan, source, Glofox Member ID (if imported), goals, payment/subscription history.

### How a lead gets created

1. **Campaign form submission** — the normal, intentional path.
2. **Auto-created from an inbound message** — if someone messages the studio directly without a form first (not for the Telegram bot channel; skipped if the studio has no campaign to assign to).

---

## 6. Social Planner

Where a studio plans, writes (with AI help), schedules, and auto-publishes social/ad posts across Facebook, Instagram, X, and Google Ads — all from one screen. Three tabs: **Scheduler**, **AI Ad Creator**, **Connections**. Requires Social Planner turned on by the platform operator and an eligible plan tier.

### Scheduler tab

A form-plus-list layout: "Quick Scheduler" on the left, "Publishing Queue" on the right.

- Pick **Campaign** and **Target Platform**.
- Type the post text, or **Write with AI** for a short-prompt AI helper.
- Attach a photo/video (upload or URL).
- Pick a date/time, then **Save as Draft** or **Schedule Post**.
- Queued (draft/scheduled) posts can be edited (pencil) or deleted (trash).
- Status badges: **draft**, **scheduled**, **published**, **failed** (no on-screen reason shown for failures).
- Instagram feed posts can't carry a clickable link — after going live, an orange "Action needed: post link as a Story" reminder appears with a Copy button and a confirm-once-done button.

### AI Ad Creator tab

Uses the same AI-copy "waterfall" (Groq → Gemini → Claude) as elsewhere. Pick a Campaign and Destination Platform, describe what the ad should promote, optionally attach media, click **Generate Ad Content** for copy/hashtags/CTA plus a live platform mock-up. **Apply to Queue** sends it straight into the Scheduler form.

### Connections tab

Shows which ad platforms are linked (Meta, X, Google Ads) and lets you connect/reconnect so posts can actually go out.

---

## 7. Payments

The studio's view of money coming in through Stripe — a billing/transaction history plus the Stripe connection settings. Not a general bookkeeping tool. Two tabs: **Overview**, **Memberships**.

A payment record appears automatically whenever a lead pays for a trial or a membership — Stripe notifies the app (a webhook) and it logs the payment here and updates the lead's status.

- **Connect Stripe**: enter Account ID, Publishable Key, Secret Key, Webhook Secret (from your Stripe dashboard). Shows "Connected to Stripe" with a Webhook Endpoint URL to paste back into Stripe. Edit Credentials / Disconnect Account available.
- **Overview tab**: Outstanding Balance and Lifetime Payments stat cards; a Trial Booking Link card (search a lead, get a manual checkout link — normally the chatbot sends this itself); a Billing History & Invoices table (reference, date, buyer, description, amount, status, receipt link), filterable by date range.
- **Memberships tab**: a table of members' ongoing recurring-payment status.
- **No refund or manual-charge buttons exist** — refunds have to be done directly in the Stripe dashboard.

---

## 8. Channels

Where a studio connects every way customers can message it — WhatsApp, Instagram, Messenger, SMS, Telegram, email — so conversations flow into the shared Inbox.

A tab per channel type (dropdown on mobile); each shows a brand-color dot and a count badge once connected. A **Connected channels** list sits beside a **Connect** form.

**Connected-channel status badges**: Active ✓ (ready), Paused ⏸ (no messages in/out), Disconnected ✕ (reconnect to resume), Error ⚠️ (needs attention, with a "Renew token" link if a token issue). **Edit** updates credentials without disconnecting; **Disconnect** asks for confirmation (history is kept, incoming/outgoing stops).

**WhatsApp — two flows**: **Meta (Cloud API)** — WABA ID, Phone Number ID, display number, Access Token from Meta's dashboard; official/ToS-compliant, billed per Meta's pricing. **QR/Web** — scan a QR via WhatsApp → Linked Devices; simpler setup, optional "Import chat history" (best-effort, recent window only); depends on a background service staying linked.

**Telegram — two flows**: **Bot** — paste a token from @BotFather; customers reach you via `t.me/<bot>`; no 24-hour reply-window restriction. **QR** — links a real personal/business account (Settings → Devices → Link Desktop Device), brings existing history/contacts (optional import, DMs only), supports 2FA password entry. Both can run on the same studio at once.

**Instagram DMs / Messenger**: one shared form (Meta Graph API) — ID (IG Business Account ID or FB Page ID), display name, access token with the right permission.

**SMS (Twilio)**: Account SID, Auth Token, Twilio phone number.

**Email**: outbound-only SMTP (not an inbox) — Host, Port, From, Username, Password. Gmail users should use an App Password.

**Google Sheets**: not a per-studio Channels connection — a one-time platform-level setup that exports every lead as a spreadsheet row.

Google Ads and X (Twitter) DMs also appear as connect options, same ID/handle/token form pattern.

---

## 9. Attendance

A read-only report of how many classes each member has attended during their current plan period, synced from Glofox. Not a place to take attendance or send messages — no manual check-in action.

- Table: name, phone, email, plan name, plan dates, classes attended (e.g. "4 of 8").
- Summary tiles: total tracked members, how many currently "qualify" (minimum class count). Edit the threshold via the pencil icon.
- "Last synced" timestamp — a background worker polls Glofox roughly every 15 minutes; the page itself refreshes every 60 seconds.
- Empty state with a link to CRM Integrations if Glofox isn't connected yet.
- Paginated, 50 members per page.

---

## 10. Knowledge Base

Where the studio feeds the AI everything it needs to answer customer questions — written instructions, uploaded documents, a greeting message, and the tone to reply in. Three sub-tabs: **Instructions & Documents**, **Needs Answers**, **Communication Style**.

### Instructions & Documents

- **Greeting Message** — the automatic first reply on a new conversation (supports `{{lead_first_name}}`, `{{lead_name}}`, `{{studio_name}}`).
- **Instructions / General Info** — free-text FAQs/prices/policies the AI reads directly.
- Both fields auto-save ~1 second after you stop typing.
- **Uploaded Documents** — PDFs, Word, PowerPoint, Excel, text, CSV, images (OCR'd, with a review popup to edit extracted text). Tag each with a content Domain (General, Fitness/Gym, Yoga, CrossFit, Pilates, Dance, Martial Arts, Nutrition, Recovery).
- Saving triggers a background "embedding" step (a few seconds to a couple minutes); a popup offers a built-in test-chat once done.
- A **Program Schedule Start Date** anchors a week-by-week program document to a real calendar date.
- Availability time-slots/timezone are configured in **Settings**, not here.

### Needs Answers

Questions the AI couldn't answer — logged here and handed to a human instead of guessed. Shows customer name (if known), times asked, how long ago. Click **Add answer** to append the Q&A into the knowledge base and re-embed immediately (resolves the gap). Click **X (Dismiss)** to drop an irrelevant/one-off question. Auto-refreshes every 15 seconds; "All caught up" when nothing's pending.

### Communication Style

A text box auto-filled from real staff replies once there's enough history — a learned tone/phrasing summary. Editable manually (sticks until enough new replies justify a rebuild); **Save Communication Style** commits edits. A "Relearn at least every" control sets the minimum interval between automatic re-learning passes.

---

## 11. Decision Trees

Lets a studio define automatic branching logic for how the AI responds to incoming messages, based on message content or lead stage. Two sub-tabs: **Trees**, **Follow-ups**.

### Follow-ups

A configurable cascade of automatic nudges sent when a lead goes quiet. Each step can point at a saved Snippet template or use free-typed text with placeholders like `{{lead_first_name}}`.

### Trees

**List page**: each tree is a card — name, Active/Draft badge, applicable pipeline stages (or "All leads"), creation date. **New Tree** asks for a name and an optional pipeline-stage filter.

**Editor** — a canvas of connected node cards. Each node has:
- **Label** — plain description.
- **Condition type**: Keyword match (with an AI "Suggest" button for trigger words), AI intent (dropdown of pre-set intents), Sentiment (positive/neutral/negative), Lead status (checked pipeline stages), or Default (catch-all).
- **Action**: Send reply (template with placeholders), Escalate to human, Book trial, Send link, or Change lead status.
- **Child nodes** — "Add child node" lets branches cascade.

**Test** button opens an inline simulator (type a sample message, optionally a lead status) to see which node matches and what fires, without sending anything. A tree only runs for real once marked **Active**. Trees can be downloaded as `.xlsx` and re-imported to copy structure; deleting removes the tree and all its nodes.

---

## 12. Templates

> **Not the same as Inbox → Snippets** (staff's reusable quick-reply library). This page controls the automatic WhatsApp message a lead receives right after they pay.

- **Trial Payment Confirmation** — custom message sent after a trial purchase (blank = sensible default).
- **Membership Payment Confirmation** — same, for a membership purchase.
- Both support `{{lead_first_name}}`, `{{studio_name}}`, `{{amount}}`, `{{receipt_url}}`.
- **Glofox Membership Mapping** — four optional fields (Trial/Membership × Glofox Membership ID/Plan Code). Filled in → a real payment also creates a matching purchase inside Glofox, not just updates the lead here.
- **Save Templates** to commit.

---

## 13. Users (Team Members)

Managing staff/teammate logins — separate from customer "Leads." Where a studio admin invites coworkers and controls access.

- **Add teammate**: first/last name, username, email, and a **role** (create roles first on the Roles tab). Picking a role previews exactly which app sections it grants, as badges.
- Each teammate gets a unique, randomly generated temporary password and is forced to set their own on first login.
- Row shows name, email, role (Studio Admin badge for the owner account), status, date added.
- **Status**: Active, "Awaiting password reset," or Inactive.
- A toggle turns an account on/off without deleting it (reversible). **Delete** permanently removes a teammate (confirmation) — cannot be undone. The studio admin account itself can't be deleted/deactivated here.

---

## 14. Roles

Named permission sets ("Front Desk," "Marketing," etc.) controlling which sidebar sections a teammate can see — assigned on the Users page.

A role is a checklist of app sections it grants (Dashboard, Inbox, Pipeline, Campaigns, Leads, Social Planner, Payments, Channels, Knowledge Base, Decision Trees, Templates, Settings — "can see this section or not," nothing finer-grained). Enforced server-side, not just a UI hint. Attendance, Users, and Roles themselves aren't grantable — those are restricted to studio admins/super-admins regardless of role.

- **Add role**: name, optional description, multi-select **Access** field.
- Roles listed as cards with badges for every granted section.
- **Edit** to change name/description/access. A toggle deactivates/reactivates a role (reversible — blocks login for everyone assigned until reactivated). **Delete** permanently removes a role (confirmation).
- A permission change only takes effect the next time an affected teammate logs in or resets their password — not instantly mid-session.

---

## 15. Settings

The studio's own control panel. Tabs: General Info, Plans, Availability, Booking Page, Google Sheets, Integrations, AI Assistant, Security, Platform Billing.

- **General Info**: name, brand color, logo, contact email/phone, trial price, "Studio is active" toggle, "Managed by 1Hero" toggle (controls super-admin access to this studio).
- **Plans**: the studio's own class/membership plans and pricing.
- **Booking Page**: hero image or background video URL for the public booking/checkout page.
- **Google Sheets**: connect a Sheet for automatic lead export, or import leads from external Sheets with column mapping.
- **Integrations**: Meta App ID/Secret, Google Ads OAuth credentials, plus:

  | Setting | What it does |
  | --- | --- |
  | WhatsApp Message Pacing | Gap (0–300s) between consecutive WhatsApp messages, to avoid looking like spam |
  | WhatsApp Daily Message Limit | Cap on automated WhatsApp messages/day (Singapore time); live typed replies don't count |
  | Glofox First-Session Message | Auto-WhatsApp a member after their first attended class (off by default, not retroactive) |
  | Cold Lead: Never Replied | Days of silence before a contacted-but-never-replied lead is flagged Cold |
  | Cold Lead: Went Quiet | Days of no activity after at least one reply before being flagged Cold |
  | Initial Message Delay | Wait before the very first message to a brand-new lead |
  | AI Reply Delay | Wait before the AI replies within an already-started conversation |

- **AI Assistant**: separate cards for Groq, Gemini, Claude, Mistral (OCR only) — paste/test an API key (encrypted, never shown again in full), enable/disable and reorder fallback models.
- **Security**: change your own password; a "Danger Zone" with type-to-confirm **Delete Account**.
- **Platform Billing**: current subscription tier, upgrade via Stripe checkout, or "Manage Billing Methods & Invoices" via the Stripe customer portal if already subscribed.

### Super-admin: Studios (platform-wide equivalent)

`/admin/studios` — a super-admin-only directory of every studio on the platform. Summary tiles (total studios, active studios, total campaigns, total leads), search/filter, a card per studio with branding/counts/status and an activate/deactivate toggle. A card is only clickable if "Managed by 1Hero" is on for that studio.

**New Studio** form (two cards): **Studio identity** (name, slug, brand color, logo URL, contact email/phone, "Enable Social Planner" checkbox) and **First admin login** (just an email — they get a welcome email to set their own password). Submitting creates the studio + first admin and redirects into the new studio's dashboard.
