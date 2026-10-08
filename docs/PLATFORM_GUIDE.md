# StudioX Platform Guide

The detailed, code-derived description of what the platform does and how. It was
compiled by reading the code (API, web app, node services, migrations, deploy
files) and checking the riskier claims directly. File paths are relative to the
repo root. Where behaviour is surprising it is called out as a **Gotcha**; the
important ones are collected in [Risks and known issues](#12-risks-and-known-issues).

For a shorter overview see the root [`README.md`](../README.md). For contributor
rules see [`skills.md`](skills.md). For production deploy steps see
[`../deploy/README.md`](../deploy/README.md).

> Not covered here: step-by-step setup of each external account (Meta, Telegram,
> Google). Those have their own guides, listed in [section 14](#14-related-docs).

---

## Contents

1. [System overview](#1-system-overview)
2. [Tenancy, users, roles and sessions](#2-tenancy-users-roles-and-sessions)
3. [Leads, campaigns and the pipeline](#3-leads-campaigns-and-the-pipeline)
4. [Messaging: inbox, channels, outbound pipeline](#4-messaging-inbox-channels-outbound-pipeline)
5. [Manual Actions and Broadcasts](#5-manual-actions-and-broadcasts)
6. [AI assistant, knowledge base, decision trees](#6-ai-assistant-knowledge-base-decision-trees)
7. [Integrations](#7-integrations)
8. [Social Planner, Payments, Reviews](#8-social-planner-payments-reviews)
9. [Platform (super-admin) pages](#9-platform-super-admin-pages)
10. [Background workers](#10-background-workers)
11. [Data model, configuration, deployment](#11-data-model-configuration-deployment)
12. [Risks and known issues](#12-risks-and-known-issues)
13. [Reference: placeholders, statuses, ports](#13-reference-placeholders-statuses-ports)
14. [Related docs](#14-related-docs)

---

## 1. System overview

### Components

| Component | Path | Role |
|---|---|---|
| API | `apps/api` | Go modular monolith. chi router, pgx, goose migrations, Swagger at `/swagger/index.html`. Entry point `cmd/server/main.go` wires every route and worker. |
| Web | `apps/web` | One Next.js 15 app: admin (`/admin/...`), public lead form (`/l/...`), login, password reset, privacy/terms, pricing, payment pages. Browser calls use relative `/api/...` paths; Next rewrites them to the API. |
| WhatsApp Web | `apps/wa-web` | Node/Express + Baileys. One QR-linked WhatsApp session per studio. Auth state on a Docker volume. Port 3100. |
| Telegram QR | `apps/tg-web` | Node + teleproto (MTProto). One QR-linked personal Telegram session per studio. Session string stored encrypted in Postgres. Port 3101. Needs `TELEGRAM_API_ID/HASH`. |
| Embeddings | `apps/embeddings` | FastAPI + sentence-transformers, default `intfloat/multilingual-e5-base` (768-dim). Used for knowledge-base search and style learning. |
| Postgres | pgvector/pg16 | System of record. Vector columns are fixed at 768 dimensions. |
| Redis | 7 | Login sessions (fail-closed) and the AI answer cache (fail-open). No volume in prod. |

Go packages under `apps/api/internal/`: `identity`, `studios`, `leads`,
`messaging` (+ `channels`), `decisiontree`, `reviews`,
`integrations/{claude,groq,gemini,mistral,llm,embeddings,sheets,glofox,crm,google}`,
`platform/{config,db,httpx,logger,mail,s3,secrets,rediscache,cache,billing}`.

### Request path

```
Browser -> nginx -> Next.js (web) --/api/*--> Go API --> Postgres / Redis
                                         ^
 wa-web / tg-web --/internal/* (x-internal-key)--+   (inbound messages, QR state)
 Meta / Stripe / Telegram / Twilio / X webhooks -> /api/v1/webhooks/...
```

### Conventions that hold everywhere

- **A studio is the tenant.** Studio-scoped routes live under
  `/api/v1/studios/{studioId}/...`; the "self" routes under `/api/v1/me/studios/{id}`.
- **Outbound goes through one path** (`outbound_jobs` -> worker -> channel adapter).
- **Errors** use `{error, code, details}`; validation errors are 422.
- **Migrations** are goose files in `apps/api/migrations` (about 110). Expand-then-contract
  is followed in practice (e.g. a column added, then dropped in a later migration), not enforced.

---

## 2. Tenancy, users, roles and sessions

### Roles

| Role | Studio | Access |
|---|---|---|
| `super_admin` | none | Everything; bypasses permission checks and the inactive-studio lockout. |
| `studio_admin` | one | Everything in their studio; bypasses the permission catalog. |
| `studio_staff` | one + a studio role | Only the sections their role grants. |

### Permission catalog (12 fixed keys)

`dashboard`, `inbox`, `pipeline`, `campaigns`, `leads`, `social-planner`,
`payments`, `channels`, `knowledge-base`, `decision-trees`, `templates`, `settings`.

- Studios create **roles** (named sets of keys) under **Roles** and assign them to
  staff under **Users**. Roles can be deactivated (blocks login for everyone on the
  role) and can't be deleted while users hold them.
- Enforcement is `RequirePermission` (`internal/identity/rbac_middleware.go`): a
  longest-prefix table maps each API path to the key(s) that grant it. **Any one**
  listed key is enough, and there is **no GET-vs-write distinction**. A path with no
  entry is denied (fail closed). `/users` and `/roles` are exempt (own gates).
- Examples: `/campaigns` needs `campaigns`; `/leads`, `/analytics`, `/messaging/leads`
  need `leads`; `/messaging/conversations`, `/stream`, `/jobs`, `/templates`,
  `/trigger-links` need `inbox`; `/messaging/channels` needs `channels`;
  `/messaging/settings` and other `/messaging/*` need `settings`.
- The web nav hides links the user lacks (`AppShell.tsx`), but pages that fetch
  data from a section the user lacks must tolerate a 403 (the Leads page now does).

**Gotcha:** `pipeline`, `payments` and `templates` keys have no matching API prefix
entry, and `/glofox/attendance` has no entry at all, so a studio_staff user is
denied there (fail closed). Attendance is effectively admin-only.

### Sessions and passwords

- Login `POST /api/v1/auth/login` (email + password; username is display-only).
  Errors: `invalid_credentials`, `account_inactive`, `role_inactive`.
- Session cookie `px_session` (HttpOnly, SameSite=Strict, Secure in prod). Its value is a
  HS256 JWT that is then AES-256-GCM encrypted, so it is opaque. Claims carry user id,
  studio id, role, the permission list (staff), a must-reset flag, idle expiry and a jti.
- Lifetimes: absolute `JWT_TTL` (30 min), sliding idle `SESSION_IDLE_TIMEOUT` (15 min,
  never extends the absolute cap). A new token is issued on every request.
- Live state is in Redis (`session:<jti>`), **fail-closed**: a Redis outage rejects
  requests with 500. Prod Redis has no volume, so a restart logs everyone out.
- Passwords are bcrypt cost 12. Login/password/forgot/reset are rate-limited to
  10 requests per minute per IP (in-memory, resets on restart).
- **Teammates** are created by an admin with a **random one-time password** (16 characters,
  unique per user, emailed to them) and a forced reset at first login (`password_reset_required`
  blocks everything except the `/auth/*` and `/permissions` routes until changed). The forced-reset
  screen asks for that temporary password plus the new one. A welcome email is sent best-effort.
- **Studio admins** created by a super-admin also get a random password, plus a welcome email with a
  1-hour reset link (if the link is missed they use **Forgot password**); self-signup admins
  (public Stripe provisioning) choose their own password.
- **Forgot password**: `POST /auth/forgot-password` returns 404 if the email has no
  active account (deliberately reveals existence). With SMTP on it emails a link
  `${FRONTEND_URL}/reset-password?token=...` valid 1 hour; only the SHA-256 hash is
  stored (`password_reset_tokens`). With SMTP off it silently returns 204. The reset
  is single-use and atomic but does **not** revoke existing sessions.
- **Permission changes take effect at next login** (permissions are baked into the
  token), including role edits and user deactivation.

### Studios

- Created by a super-admin (`POST /api/v1/admin/studios`): studio row, a studio_admin,
  and four seeded plans (Trial 0 one-time, Basic 2900, Pro 9900, Pro Plus 19900).
  Afterwards: best-effort Glofox user registration and a welcome email.
- **Inactive-studio lockout**: `RequireActiveStudio` returns 403 `studio_inactive` for
  non-super users of an inactive studio (and enforces studio-id ownership); the web
  shows a lockout screen. Super-admins bypass.
- **Tier gating** (`studios/gating.go`): trial = 1 channel, no social planner, 200 AI replies;
  growth = 3 / no / 2000; pro = 8 / yes / 10000; enterprise = 999 / yes / 999999.
- Studio fields cover branding, availability slots + timezone, plans, booking-page
  hero/video, trial prices (SGD/INR/USD), confirmation messages, Glofox plan mapping,
  AI keys, Meta/Google/Stripe credentials, knowledge base, greeting, program start date.

---

## 3. Leads, campaigns and the pipeline

### Campaigns and the public form

- A campaign has a slug (unique per studio), name, description, **fitness plans** (at least one),
  an `active` flag, and a `shareUrl` = `PUBLIC_FORM_BASE_URL/l/{studioSlug}/{campaignSlug}`.
- Only `fitnessPlans` and `active` can be edited after creation; there is no delete.
- Public page `apps/web/src/app/l/[studioSlug]/[campaignSlug]`: studio-branded (brand color
  + logo), fields first/last name, email, phone, plan, goals; 20 s client cooldown;
  footer says "Powered by 1herosocial.ai". Server validation and length caps apply; it
  records referrer, user agent and IP. A chosen plan containing "trial"/"trail" creates
  the lead as `trial_booked`, otherwise `new`. Source is `public_form`.
- Other public entry points: `POST /public/studios/{slug}/trial-signup` (uses the studio's
  oldest active campaign, source `trial_link`, status `trial_booked`), a booking flow
  (`/l/.../book`: plan -> Stripe checkout -> slot), and a customizable trial payment page
  `/trial-details/[leadId]?studio=slug`.

### Leads

- **Statuses (exactly six):** `new`, `contacted`, `trial_booked`, `member`, `dropped`, `paused`.
  "Cold" is **not** a status (see below).
- **Fields:** name parts, email, phone, plan, goals, source, notes, contact attempts, last contacted,
  flags (contactMade, hotLead, trialPurchased, trialAttended, memberSold, needsManualFollowup,
  dndEnabled), assignedTo, monthlyFee, currency, offer, furtherNotes, capture metadata,
  gender/DOB (trial page, sent to Glofox), `autoContactStage`.
- **Sources:** `public_form`, `trial_link`, `import`, `external_sheet`, or whatever a sheet's
  source column says.
- **List** `GET /leads` filters: campaign, status(es), maxAttempts, source, date range,
  duration, hotLead, contactMade, trialPurchased, search (name/email/phone); default limit 50,
  max 200. The web page shows 25 per page, refreshes every 5 s, and exposes campaign, time,
  source, status, max-attempts and search.
- **Detail page**: edit names, assignee, offer, plan, status, fee, notes; toggles for
  contact made, hot, trial purchased/attended, member sold; a payment card with the lead's
  member subscriptions and receipt links. (DND is toggled from the Inbox contact panel.)
- **Side effects of a status change:** `trial_booked` forces `trialPurchased`, `member` forces
  `memberSold`; both fire an async Glofox lead sync; every change queues a Google Sheets update.
- **Automatic status changes:** first outreach sets `contacted` **unconditionally**
  (it can downgrade `trial_booked`/`member`); Stripe trial/membership payments set
  `trial_booked`/`member`; a cancelled subscription sets `dropped`; the AI worker and
  decision trees move leads on intent/sentiment/menu choices. Nothing sets `paused` automatically.
- **Do Not Disturb:** per lead and per conversation. Turned on by the toggle, the API, or an
  exact inbound `stop` / `unsubscribe` / `opt out` / `optout`. Turning it on for a **lead**
  cancels queued automated messages; for a **conversation** it only sets the flag (queued
  jobs are blocked at send time and marked dead).

### Import (CSV / Excel)

`POST /leads/import` (max 10 MB, `.csv/.xlsx/.xls`, first sheet) with a default campaign.
Headers are matched case-insensitively by substring (first/last name, name, email,
phone/number/contact, plan, goal, note, status, barcode/member id). If no header maps,
positional columns apply. Rows with neither email nor phone are skipped. Status from the file
if it's a valid platform status, else translated from a Glofox-style value (`Active`→`member`,
`Trial`/`Pending`→`trial_booked`, `Inactive`/`Cancelled`/`Expired`/`Frozen`→`dropped`), else
`trial_booked` when the plan text contains "trial", else `new`; source `import`.
**A Glofox member export can be uploaded as-is**: `BarcodeID`/`Client Name`/`Membership Tier`
map to member ID/name/plan automatically, and the BarcodeID is kept on the lead
(`leads.glofox_member_id`, shown on the lead detail page) — nothing needs reformatting first.
**Gotchas:** no de-duplication; not transactional; a "campaign" column is ignored; every row
queues auto-contact (WhatsApp/SMS) if it has a phone and a channel exists.

### Pipeline and cold leads

- Kanban of the six statuses (drag and drop PATCHes status; any transition is allowed;
  50 cards per column; auto-refresh 4 s). Header shows total, active, conversion %.
- **Cold column** replaces an empty Paused column. A conversation is cold when the lead is not
  `member`/`dropped`, at least one outbound message exists, and either `never_replied`
  (no inbound and silence beyond `cold_never_replied_days`, default 1) or `stalled`
  (inbound exists and silence beyond `cold_stalled_days`, default 7). A scanner
  recomputes this every 15 minutes (so the column can lag). Cards can be dragged into a status
  (creating a lead if the contact had none) or **re-engaged** (resends the studio greeting and restarts the follow-up cascade).

### Analytics and dashboard

- Studio dashboard: campaign/lead/conversion stat cards, pipeline overview, lead distribution,
  active campaigns, latest leads, a **Detailed Analytics** tab, and a browser-only ROI calculator.
- `GET /analytics` returns counts and rates per status, `followupsRequired`
  (new/contacted with fewer than 3 attempts), `unrespondedMessages`, connection rate
  (contacted leads that ever replied), conversion rate (reached trial/member), average
  response time and stage time-lapses (using `updated_at` as a proxy), outbound volume,
  and breakdowns by campaign and by platform. **Gotcha:** the "Paid Ads" platform bucket
  matches any source/referrer containing "ad".

---

## 4. Messaging: inbox, channels, outbound pipeline

### Inbox (`/admin/studios/<id>/inbox`)

Five tabs: **Conversations** (sub-filters all/unread/recents/starred, per-channel tabs),
**Escalation**, **Manual Actions** (scheduled messages and Broadcasts), **Snippets**
(message templates) and **Trigger Links**. A right-hand contact panel holds the per-conversation
**DND** and **AI** toggles and the lead assignee.

- Conversation statuses are `open` and `closed` (closing archives; `snoozed` exists in the
  model but nothing sets it). `conversations.assigned_to` also exists but nothing writes it;
  the only assignee is the lead's `assignedTo`.
- Starred and unread are studio-wide, not per user. Messages dedupe on
  `(conversation_id, external_id)`.
- **Live updates:** `GET /messaging/stream` (SSE, ping every 20 s). The bus is in-process
  (single API replica) and drops events if a subscriber is slow; the browser listens for
  `message.received`, `message.sent`, `conversation.updated` only.
- **Escalation** (`EscalateAndNotify`) sets `escalated_at/reason`, turns the conversation's AI
  off, publishes an event and emails the studio's contact email (last 6 messages + deep
  link). Triggers: a customer asks for a human, a decision-tree `escalate_human` node, low
  knowledge-base confidence on pricing/general questions, billing-complaint phrases.
  "Resolve" clears it and turns AI back on.
- **Snippets** (message templates): name, body, channel kinds, attachments. Variables
  `{{contact.first_name}}`, `{{studio.name}}`, `{{campaign.name}}` resolve at send time.
  The stored WhatsApp template name/language are **not used** when sending.
- **Trigger links**: tracked redirect `GET /api/v1/links/{id}?leadId=` (http/https only), clicks stored.
- **Uploads**: `POST /messaging/upload`, up to 200 MB (nginx may cap lower), saved to local
  `uploads/`; extensions limited to common image/video/document types (no audio).

### Channels

| Kind | Connect | Inbound | Outbound |
|---|---|---|---|
| `whatsapp_meta` | form: WABA id, phone number id, display number, access token | `/webhooks/meta/whatsapp` (HMAC verified; studio app secret or app secret) | Graph API |
| `whatsapp_web` | QR via wa-web | wa-web -> `/internal/wa-web/inbound` | wa-web `/sessions/:id/send` |
| `instagram_meta`, `messenger_meta` | form | Meta webhooks | Graph API |
| `sms` | Twilio SID, token, number | `/webhooks/twilio` (verified with `X-Twilio-Signature`) | Twilio |
| `x_dm` | four OAuth1 keys | `/webhooks/x` (POST verified with `x-twitter-webhooks-signature`) | X API |
| `telegram` (bot) | bot token; platform calls `getMe` + `setWebhook` | `/webhooks/telegram/{botID}` + secret-token header | Bot API |
| `telegram_mtproto` | QR via tg-web (+2FA) | tg-web -> `/internal/tg-web/*` | tg-web |
| `email_smtp` | host/port/user/password/from (verified with a real AUTH handshake) | none (outbound only) | SMTP |
| `google_ads` | enum only | none | none; used only by the Social Planner for Google Ads posts |

- Tokens are AES-256-GCM encrypted in `channel_accounts.access_token_enc` and decrypted only
  where needed; list endpoints never return them. A channel's external id is unique platform-wide.
- Disconnecting hard-deletes the channel if no conversations reference it, otherwise marks it
  `disconnected`, and logs the QR session out.
- **Identity stitching:** `contact_identities` is unique per `(studio, kind, value)`
  (`phone`, `email`, `ig_psid`, `fb_psid`, `x_id`, `telegram_chat_id`). The same person on two
  channels is **two** conversations (no cross-channel merge). WhatsApp Web reuses an existing
  `@lid` or bare-digit identity before creating a new one.
- **Lead creation from inbound:** WhatsApp Web and Meta WhatsApp auto-create a lead from the
  studio's active campaign (skipped if the studio has no campaign); Telegram bot chats never do;
  Telegram QR creates one with the chat id as the phone.
- **History backfill** after pairing creates conversations and messages only (never leads),
  is idempotent, and triggers AI summaries and style catch-up.

### The outbound pipeline

Every send is a row in `outbound_jobs` handled by one worker (`internal/messaging/worker.go`).

1. **Claim:** every 2 s (or on an enqueue event), up to 10 due jobs **per studio**, oldest first,
   `FOR UPDATE SKIP LOCKED`, soft-requeued for 1 minute as crash safety.
2. **Studio isolation:** each studio's claimed jobs run in order on that studio's own goroutine,
   and a studio still sending is skipped by the next claim. One studio's slow pacing or dead
   channel cannot delay another.
3. **Per job:** load conversation; **DND safety net** (automation/AI sources only); load channel
   and decrypt token; resolve variables and flatten markdown tables; if the channel is not
   `active` try another active channel of the same kind, else fail (retryable); **daily cap**;
   choose the sender; **WhatsApp send spacing**; send; on success insert the outbound message
   and mark the job `sent`.
4. **Daily cap:** default 48 automation/AI/Manual-Actions WhatsApp messages per studio per
   Singapore-time day (fixed UTC+8); 0 = unlimited; live typed replies are exempt; over the cap
   the job is `dead` (`daily_limit_exceeded`). A lookup error fails open.
5. **Spacing:** default 20 s between WhatsApp messages on the same channel (0-300 s, per studio),
   tracked in memory per channel (lost on restart).
6. **Retry:** up to 6 attempts with backoff `2^n` seconds capped at 30 min. **Network-level failures** (DNS lookup, connection refused/reset, timeouts reaching wa-web/Meta/SMTP) are treated as transient: up to 12 attempts with 30 s, 1 m, 2 m, 4 m, 8 m, then 10 m waits (about 85 minutes) before a job is dead-lettered. Dead immediately on:
   invalid credentials (channel marked `error`), DND, daily limit, deleted channel, unknown kind.
   Dead jobs can be revived from Manual Actions. The `failed` status exists but is never written.
7. **Message sources:** `customer`, `studio_user`, `automation`, `ai`. Staff replies skip DND,
   the daily cap and the AI gate.

**Gotcha:** `sent` means the channel accepted the message. For WhatsApp Web there are no
delivery or read receipts, so it does not prove the recipient received it.

---

## 5. Manual Actions and Broadcasts

### Scheduled messages (one conversation)

`POST /messaging/jobs` with conversation, body/attachments and an optional time. A time with
no zone (`2006-01-02T15:04`) is read as **UTC**. Jobs use the `automation` source, so DND and
the daily cap apply. Edit, "Send now", delete, and revive dead jobs are supported. The list
includes every pending job for the studio, not only manually scheduled ones.

### Broadcasts (Inbox -> Manual Actions -> Broadcasts)

**Lists.** Upload `.csv/.xlsx` (max 10 MB; `.xls` is accepted by the check but may fail to open).
Headers are fuzzy matched, first match wins in this order: contains "name" -> name;
phone/mobile/number/contact -> phone; "email" -> email. A phone column is required.
**Gotcha:** a header like "Contact Email" matches phone first. The upload only **stages** the
file and reports how many numbers lack a country code; the confirm step takes a list name
and a default country code (UI default `65`) and creates the list in one transaction.
Numbers without `+` are prefixed with the code, so an already-prefixed number (`6591...`) gets
double-prefixed. No duplicate check. Staging rows are never cleaned up.

**Campaigns.** Pick a list, channel (`whatsapp` or `email`), subject (email only) and body, and
send now or schedule. Requires a non-empty list and a matching active channel (WhatsApp
prefers Meta, then WhatsApp Web; email needs SMTP). Statuses: `scheduled`, `sending`,
`completed`, `canceled`; recipients: `pending`, `enqueued`, `failed`.

**Worker.** Every minute it takes due `scheduled`/`sending` campaigns and enqueues pending
recipients: at most 200 per tick, and for WhatsApp at most the studio's remaining daily
allowance (limit minus messages already **sent today and messages queued but not yet sent**), resuming the next Singapore day automatically. Email ignores the daily cap. Each
recipient gets a conversation and an `outbound_jobs` row (`broadcast:<campaignId>`), so the normal
pipeline (DND, spacing, retries) applies. A recipient with no email on an email campaign is marked failed.

**Cancel** sets the campaign `canceled` and marks its pending jobs `dead` in one transaction;
already-sent messages are unaffected. **Automatic retry:** when a broadcast message dies for a temporary reason (network/DNS trouble, the daily limit, the channel offline), its recipient goes back to `pending` and the campaign re-opens as `sending`, so the worker queues it again as allowance frees up, up to 5 times per recipient (`retry_count`). Permanent failures (number not on WhatsApp, Do Not Disturb, bad credentials) stay failed. **Gotcha:** "completed" means all recipients were
**queued**, not delivered. The recipients view joins each recipient to its job to show the real
state (`pending/sent/failed/dead`). If the channel disappears after scheduling, the campaign
stays as it is indefinitely.

---

## 6. AI assistant, knowledge base, decision trees

### How an inbound message is handled (`internal/messaging/ai_worker.go`)

The AI worker reacts to each inbound `customer` message. First match wins:

1. Cancel the lead's pending no-reply follow-ups (always).
2. If the conversation's **AI is off**, stop. AI defaults to off per conversation (on for
   Telegram bot chats; auto-contact turns it on).
3. **DND / stop words:** stop, turn DND on, delete pending jobs.
4. Skip if a bot-owned `autoContactStage` (menu flow) owns the lead, or an automation/AI reply already exists.
5. **Greeting:** the first message of a new conversation gets the studio's greeting plus a fixed
   "you're chatting with <Studio>'s AI assistant" line, and nothing else.
6. **Human-request phrases** escalate with a canned handoff reply. **Complaints, negative feedback and bad-review threats** ("disappointed", "unacceptable", "refund", "leave a bad review"...) escalate the same way with an apologetic handoff, before any other handling; the classifier's `complaint_or_feedback` intent covers politely worded criticism when retrieval is available.
7. **Intent keyword overrides** ("trial" -> booking inquiry; "become a member", "sign me up" -> ready to buy).
8. **Retrieval** (needs a Gemini key): classify intent/sentiment, expand the query, embed, hybrid
   search (vector + full-text, fused by RRF), Gemini rerank to the top 4, plus similar past messages and staff style examples.
9. **Decision tree:** a matching node replies **without calling the LLM**.
10. **Shortcuts:** booking and membership menus ("1. Book a Trial 2. Become a Member"), payment
    links on buy wording, "yes" to a trial or plan-change offer.
11. **Class requests go to a person:** a request to book, cancel, reschedule, move or extend a class/session/credits (trial wording excluded, since trials have their own flow) escalates immediately, before the booking menu shortcut. The AI has no booking system and must never claim it booked anything.
12. **Low-confidence escalation** for pricing/general questions, and for any question or request about classes/sessions/the timetable, when fewer than 2 knowledge-base chunks matched (needs retrieval, i.e. a Gemini key). The exact question is logged to `knowledge_gaps` (best-effort — never blocks the handoff) and surfaces on the Knowledge Base page's **Needs Answers** tab (see below).
13. Otherwise **build the prompt and call the model waterfall**.

**Prompt** (one flat text): role and absolute rules, today's date/timezone, the verified program
session for today (if a program start date and schedule exist), a no-greeting rule, knowledge-base
chunks (or the whole KB text if retrieval is unavailable) with an "answer only from this, otherwise hand off" rule, a "you cannot book/cancel/reschedule classes and must never say you have" rule, active plans (SGD), lead line, menu
instructions by lead status, response strategy by intent, the learned style profile and examples,
tone, "2-4 sentences", past context and the last 15 messages.

**Models:** waterfall **Groq -> Gemini -> Claude**, each trying the studio's enabled models in
order (defaults apply if none are configured). Keys: the studio's own, then the platform's.
Mistral is used for **OCR only**. Every attempt is logged to `llm_usage_logs`. If all providers
fail, nothing is sent and nothing is retried.

**Post-processing and sending:** a leading "Good morning/afternoon/evening/night" is stripped from the
model's reply in code (the AI no longer uses time-of-day greetings; the studio's own configured first-message
greeting is unchanged), motivation questions are stripped for booking inquiries, and replies carry `source=ai`.

**Cache:** Redis `studio:<id>:ai:<kind>:<sha256(prompt)>`, TTL `AI_ANSWER_CACHE_TTL` (30 min).
Prompts embed history, so hits are mostly Test Chat and template generation.

**Gotchas:**
- The final LLM reply is scheduled at a **hard-coded +20 s**, ignoring the studio's AI reply delay
  (greetings, tree replies, shortcuts and handoffs honour it).
- Without a Gemini key there is no retrieval, no intent classification and **no low-confidence
  guard**: the model gets the entire knowledge base and answers unguarded.
- A root-level `default` decision-tree node matches every message in its status, so the AI never replies there.
- The Gemini helper calls (classify/expand/rerank) are not logged, so the LLM Monitor understates usage.

**Per-studio AI settings** (Settings -> AI Assistant, `ai-models` routes): per provider a key
(tested before saving), a model checklist (each added model is live-tested) and an order
(primary vs fallback). Provider keys are stored on the `studios` row **without the encryption used
for Stripe and channel tokens** (the UI shows only the last 4 characters).

### Knowledge Base (`/admin/studios/<id>/knowledge-base`)

- **Instructions tab:** greeting message, free-text instructions, uploaded documents, program
  schedule start date. Edits **autosave** about 900 ms after the last change and then trigger a sync.
- **Needs Answers tab:** every question the AI escalated instead of guessing (the low-confidence
  handoff above), newest first, with a badge showing how many are still open. Repeat asks of the
  same question while unanswered just bump a counter instead of piling up duplicates
  (`knowledge_gaps`, partial unique index on `(studio_id, lower(btrim(question)))` scoped to
  `status='open'`). Clicking **Add answer** appends a `Q: ... / A: ...` block to a single accumulating
  knowledge-base document ("Learned Answers (from customer questions)") and re-runs the normal
  embedding sync — the AI can use it starting with the next message, no separate save step.
  **Dismiss** closes a gap (off-topic, duplicate) without touching the knowledge base.
  `GET/POST /api/v1/studios/{id}/knowledge-base/gaps[/{gapId}/resolve|/dismiss]`.
- **Documents:** `.pdf .docx .pptx .xlsx .txt .csv .json .md` are parsed in Next; images go
  through Mistral OCR (needs the studio's Mistral key) with a review dialog. A per-document
  "domain" tag exists but **has no effect on retrieval** today.
- **Sync pipeline:** on a content change an async job (up to 30 min) parses program schedules,
  chunks text (800 characters, 150 overlap), embeds one chunk at a time (100 ms apart) and
  **replaces all** the studio's chunks (`studio_knowledge_chunks`). Status is `syncing`, `complete`
  or `error` (the UI polls `sync-status`). Without the embeddings service the sync ends in `error`.
  Failed chunks are silently skipped.
- **Program schedule:** a regex parser reads `Week N` / weekday / `Session:` / `Progression & changes:` /
  `Key focus:` lines; other layouts parse to nothing, silently. The week is computed in the studio timezone.
- **Style tab:** a learned communication-style profile (editable, up to 4000 characters) and a
  refresh interval (30-43200 min). The style worker learns only from **staff-typed** messages
  (including those typed on the linked phone) and rebuilds the profile after 20 new replies (or the
  interval). A rebuild **overwrites manual edits**.
- **Test Chat:** `POST /knowledge-base/test-chat` re-runs the retrieval and prompt pipeline without
  saving anything and shows the sources used. It skips decision trees, shortcuts, escalation and DND.

### Decision Trees (`/admin/studios/<id>/decision-trees`)

- A tree has a name, an active flag and **target statuses** (empty = all). One active tree per
  status; activating one deactivates others that overlap. Trees are cached in memory for 3 minutes.
- **Nodes:** condition (`keyword`, `intent`, `sentiment`, `lead_status`, `default`), reply template and
  action (`reply`, `escalate_human`, `book_trial`, `send_link`, `change_status`). Keyword matching is
  case-insensitive, whole-word for single words, substring for phrases, with fuzzy matching for longer words.
  Tree intent detection is built-in keyword scoring, not the Gemini classifier.
- Matching starts from the children of the last matched node and falls back to the root. Only a plain
  `reply` remembers the node.
- Editor: React Flow canvas with auto layout, a simulator, AI keyword suggestions, Excel
  import/export (columns: Label, Parent Label, Condition Type, Condition Value, Reply Template, Action,
  Action Value, Sort Order; `.xlsx` only, 10 MB; per-row errors; labels must be unique for parent lookup).
- **Follow-ups tab:** a no-reply cascade of delay + message steps (placeholders `{{lead_first_name}}`,
  `{{lead_name}}`, `{{studio_name}}`). All steps are queued when a lead is first contacted and are cancelled
  by the lead's first genuine reply. Not sent for sheet leads with "continue AI" off, and trial-booked leads get a single 1-day nudge instead.

### The two things called "templates"

| | Templates page | Snippets / message templates |
|---|---|---|
| What | Two fixed texts on the studio: trial and membership confirmation (sent after Stripe payment), plus Glofox plan mapping | A reusable library used by the Inbox composer |
| Variables | `{{lead_first_name}}`, `{{studio_name}}`, `{{amount}}`, `{{receipt_url}}` | `{{contact.first_name}}`, `{{studio.name}}`, `{{campaign.name}}` |

**Do not mix placeholder syntaxes across features** (see [section 13](#13-reference-placeholders-statuses-ports)).

---

## 7. Integrations

### Google Sheets

- **Outbound sync** (per studio, Settings -> Google Sheets): lead create/update events go through a
  transactional outbox (`outbox`) to one configured tab; polled every 5 s, up to 8 attempts with
  backoff, then dead. A super-admin uploads the service-account credentials once. Studios that enable sync
  later are **not back-filled**. One-way only. The 21-column layout (A-U) runs from Lead ID through Status,
  with "Predicted Revenue Won" = monthly fee x 9.
- **Inbound external sheets** (Settings -> Google Sheets -> External Leads Sheets): a studio can add **several**
  sheets (or several tabs of one), each with its own tab, column mapping and options (`GET/POST /leads/external-sheets`,
  `PUT/DELETE /leads/external-sheets/{sheetId}`); the same spreadsheet + tab can't be added twice to one studio. Read-only polling every 15 s of
  every active sheet, with configurable column letters, a per-sheet (and per-studio) **watermark** (only rows beyond the last imported row), and
  the oldest active campaign as target. If a date column is set, **only the current calendar month's** rows
  import (older rows are skipped permanently). Auto-contact is skipped when disabled, trial purchased, or
  the hot-lead cell is HOT/COLD. "Continue AI after greeting" only applies to leads whose source is
  exactly `external_sheet`. The `auto_contact_batch_limit` column is unused.

### Glofox

- **Client:** one platform-wide credential set from `GLOFOX_API_KEY/TOKEN/BRANCH_ID`. It also powers
  lead sync (status changes to trial/member, Stripe payment paths) and studio-user registration, so
  removing these variables disables more than the first-session message.
- **First-session message** (see [section 10](#10-background-workers)): runs for **one studio only**,
  chosen at server start as the oldest studio with an active WhatsApp channel. Opt-in per studio
  (`glofox_first_session_enabled`, default off; Settings -> Integrations -> Message Timing); only sessions
  attended after it was enabled and within 3 days count; at most 20 members per 30-minute poll; text is
  hard-coded. **The toggle has an effect only for the auto-detected studio.**
- **Attendance** (Attendance page): multi-studio, driven by each studio's own Glofox connection in
  **CRM Integrations**; polls every 15 minutes (and immediately on connect); read-only, never sends.
  For each booked member it calls Glofox once, counts attended classes in the member's current plan window
  (lifetime if there is no active plan or the lookup fails) and stores plan name and limit. Phone and email
  are stored only for members at or above the **qualifying threshold** (default 5, editable on the page).
  Data can be up to about 15 minutes old. Diagnostics: `cmd/glofox-attendance`, `cmd/glofox-attendance-tick`.

### CRM connector (super-admin: CRM Integrations)

A database-driven framework: a provider (base URL, auth type `bearer | api_key | basic | token_exchange`)
maps six fixed operations (`create_lead`, `register_user`, `purchase_membership`, `find_membership_plan`,
`get_member`, `list_bookings`) onto its endpoints; studios connect with their own credentials (AES-GCM
encrypted). Providers can be created by uploading an API spec that an LLM parses into a **draft** (up to 300 KB;
a super-admin reviews and activates). Seeded: Glofox and Mindbody.
**Limits:** only attendance uses the framework for Glofox; Glofox lead sync and first-session use the
env-configured client. Mindbody sync creates the lead and only **logs** a price match; it does not record sales.

### Meta, Google, Stripe, SMTP, S3

- **Meta:** WhatsApp Cloud, Instagram, Messenger webhooks with HMAC verification (per-studio app secret
  overrides the platform one). The data-deletion callback now verifies its `signed_request` but **only acknowledges**; it deletes nothing.
- **Google:** OAuth for Ads (per-studio client id/secret/developer token), Sheets service account.
- **Stripe:** see [section 8](#8-social-planner-payments-reviews).
- **SMTP:** plain `net/smtp`, HTML only, synchronous, no queue or retry. Used for password reset,
  welcome emails, escalation alerts and email channel sends.
- **S3:** image uploads for logos and social media; falls back to local `/uploads`.

---

## 8. Social Planner, Payments, Reviews

### Social Planner

Enabled per studio (`social_planner_enabled`, set at creation; also needs tier allowing it for gating).
Posts have a campaign name, share URL, platform, copy, media, schedule and status
(`draft`, `scheduled`, `published`, `failed`) plus a `deliveryMode` (`live`/`mock`). AI copy generation
reuses `POST /messaging/ai/generate`. A worker polls every 5 s and publishes due posts:

| Platform | Uses | Notes |
|---|---|---|
| Facebook | `messenger_meta` channel (page id + token) | photo or feed post; share link appended as text |
| Instagram | `instagram_meta` channel | media **required**; videos become Reels; links aren't clickable; "story link posted" is a manual flag |
| X (Twitter) | `x_dm` channel credentials | **text only**, no media, no link |
| Google Ads | `google_ads` channel + studio Google credentials | creates a **PAUSED** campaign with a hard-coded $10/day budget and $1 CPC |

In local mode with no channel, posts are "published" as `mock`. **Gotchas:** failed posts never retry
and the UI shows no reason (logs only); the global-mode AI call uses a hard-coded studio id; no timezone
handling beyond UTC.

### Payments (Stripe)

- **What exists:** Stripe configuration (manual keys: account id, publishable key, secret key, webhook
  secret; the secrets are encrypted and never returned), a plans CRUD (the plan named **Trial** gates the
  AI's trial payment link), member subscriptions view, public checkout/payment-intent flows, and platform billing
  (studios paying the platform: upgrade, billing portal, sync). A Connect OAuth route exists but has no
  frontend caller. There are no refunds, payouts or non-Stripe gateways.
- **Webhook** `POST /webhooks/stripe[/{studioId}]` (per-studio signing secret, else global; de-duplicated
  by event id). Handled: checkout/payment-link completion, payment intent succeeded, invoice paid/failed,
  subscription updated/deleted. Effects: trial paid -> lead `trial_booked` + Glofox/Mindbody sync + confirmation
  WhatsApp with receipt link; membership paid -> lead `member`, fee and plan recorded; cancelled -> lead
  `dropped`; failed invoice -> `past_due`.
- Public platform signup: `/public/platform/checkout` and `/provision`; plans from `/public/platform/plans`.

### Reviews

A public, global testimonial store for the marketing home page: unauthenticated `POST` and `GET`
`/api/v1/reviews`, no moderation, no rate limit, no edit or delete.

---

## 9. Platform (super-admin) pages

| Page | What it does |
|---|---|
| **Studios** | List/filter, create (studio + first admin), activate/deactivate, per-studio workspace |
| **Analytics** | Global analytics with a studio selector (uses `/admin/*` endpoints) |
| **Payments** | Global billing (`studioId="global"`), platform Stripe keys and history |
| **LLM Monitor** | 30-day usage from `llm_usage_logs` by provider/model/studio, success rate, latency; **cost is estimated in the browser from a hard-coded price table**, not billing data |
| **CRM Integrations** | Providers, operations review, per-studio connections, AI task config |
| **Settings** | Google Sheets service-account upload; platform plans |

---

## 10. Background workers

All started in `apps/api/cmd/server/main.go`.

| Worker | Interval | Does |
|---|---|---|
| Outbound | 2 s + events | Sends `outbound_jobs` (per-studio isolated) |
| Broadcast | 1 min | Enqueues recipients of due campaigns |
| Auto-contact | 5 s | On `lead.created`: greeting (after the initial delay), AI on, follow-up cascade |
| AI | event-driven | Replies to inbound messages; summarises backfilled chats |
| Cold-lead scanner | 15 min | Recomputes cold flags per studio |
| Style | 5 min + events | Learns staff style, rebuilds profiles |
| Sheets (out) | 5 s | Outbox -> Google Sheets |
| External sheet (in) | 15 s | Imports new rows |
| Glofox first-session | 30 min | Congratulates members after a first attended session (single studio, opt-in) |
| Glofox attendance | 15 min | Attendance snapshot per studio (read-only) |
| Social publisher | 5 s | Publishes scheduled posts |

Auto-contact specifics: phone numbers are sanitised, and a bare 10-digit number not starting with `65`
gets `91` prepended (India/Singapore assumption). Channel preference is WhatsApp Meta, WhatsApp Web, SMS.
The default greeting is "Hi {{lead_first_name}}, we saw your interest in {{studio_name}}... 1. Interested 2. Not Interested".

---

## 11. Data model, configuration, deployment

### Main tables by domain

- **Identity:** `users`, `permissions`, `studio_roles`, `studio_role_permissions`, `password_reset_tokens`
- **Studios and billing:** `studios`, `plans`, `user_subscriptions`, `studio_subscriptions`, `platform_settings`, `onboarding_tokens`, `processed_stripe_events`
- **Leads:** `campaigns`, `leads`, `outbox`, `studio_sheets_settings`, `studio_external_leads_sheet_settings`, `external_sheet_import_log`
- **Broadcasts:** `broadcast_lists`, `broadcast_contacts`, `broadcast_import_staging`, `broadcast_campaigns`, `broadcast_campaign_recipients`
- **Messaging:** `channel_accounts`, `contact_identities`, `conversations`, `messages`, `outbound_jobs`, `message_templates`, `trigger_links`, `trigger_link_clicks`, `automation_rules`, `automation_runs`, `ai_suggestions`, `message_analyses`
- **AI and knowledge:** `studio_knowledge_chunks` (vector 768), `studio_ai_models`, `llm_usage_logs`, `studio_program_sessions`, `studio_followup_steps`, `decision_trees`, `tree_nodes`
- **Integrations:** `crm_providers`, `crm_operations`, `crm_connections`, `ai_task_configs`, `glofox_attendance`, `glofox_attendance_settings`, `glofox_first_session_log`
- **Other:** `social_posts`, `reviews`

**Audit trail:** only columns. Many tables have `created_by`, `updated_by`, `deleted_by`, `deleted_at`;
there is **no audit log table**. `deleted_at` is enforced only for users.

### Configuration (names only; values live in `.env` files)

Server: `API_HTTP_ADDR`, `API_ENV` (`local` enables stub sends), `API_LOG_LEVEL`, `API_CORS_ORIGINS`,
`FRONTEND_URL`, `PUBLIC_FORM_BASE_URL`, `PUBLIC_API_BASE_URL`, `API_BASE_URL` (web, server-only).
Database: `DATABASE_URL` or `POSTGRES_HOST/PORT/USER/PASSWORD/DB/SSLMODE`.
Auth: `JWT_SECRET` (>= 32 chars), `JWT_TTL`, `SESSION_IDLE_TIMEOUT`, `COOKIE_NAME/DOMAIN/SECURE`,
`SUPER_ADMIN_EMAIL/PASSWORD`, `TOKEN_ENCRYPTION_KEY` (base64 of 32 bytes; **no rotation support**).
Redis: `REDIS_HOST/PORT/PASSWORD/DB`, `AI_ANSWER_CACHE_TTL`.
Mail: `SMTP_HOST/PORT/USER/PASSWORD/FROM`.
Meta: `META_APP_ID/SECRET`, `META_WEBHOOK_VERIFY_TOKEN`, `META_GRAPH_API_VERSION`.
AI: `CLAUDE_API_URL/KEY`, `GROQ_API_KEY`, `EMBEDDINGS_SERVICE_URL`.
Node services: `WA_WEB_SERVICE_URL`, `TG_WEB_SERVICE_URL`, `INTERNAL_API_KEY`, `TELEGRAM_API_ID/HASH`.
Storage: `AWS_REGION/ACCESS_KEY_ID/SECRET_ACCESS_KEY`, `S3_BUCKET`, `S3_PUBLIC_URL`.
Glofox: `GLOFOX_API_KEY/TOKEN/BRANCH_ID`. Sheets: `GOOGLE_CREDENTIALS_PATH`, `GOOGLE_SHEETS_ID/TAB`.
Stripe: `STRIPE_WEBHOOK_SECRET` (plus per-studio keys in the database).

### Local development

`make db-up`, `make migrate-up`, `make seed-admin`, `make install`, `make dev` (API with `air`, web,
wa-web, tg-web, embeddings). The dev database is reachable through PgBouncer (port from `.env`) and
directly on 5436; migrations use the direct port. `make swagger` regenerates API docs. Several Go tests
use the dev database from `.env`.

### Production

Single EC2, Docker Compose, nginx in front, no CI/CD, no image registry.
`bash deploy/deploy.sh`: `git pull --ff-only` -> `docker compose build --no-cache` -> migrate -> idempotent
super-admin seed -> `up -d`. Details and quirks:

- `deploy.sh` runs from `deploy/` and uses the default **`docker-compose.yml`**, not `docker-compose.prod.yml`.
  The two files differ (embeddings service, nginx config, SMTP/idle-timeout variables), so check which one a
  change belongs in.
- The API image is **alpine** (not distroless as the deploy README says), non-root, bundling `server`, `seed`,
  `backfill-embeddings` and `goose`. The Glofox and `test-*` commands are dev-only.
- Backups (`backup.sh`, nightly `pg_dump`, 14-day retention) cover the database only: **not** uploads,
  the WhatsApp auth volume or secrets, and nothing is copied off the box.
- Edit tracked files only in git. Environment-specific values belong in the gitignored `deploy/.env`;
  hand-editing tracked files on the server blocks `git pull`.
- Check that backup copies of env files (`.env.backup`, `.env.bak`, `.env.domain.backup`, `.env.new-ec2`)
  are gitignored.

---

## 12. Risks and known issues

Ordered roughly by impact. None of these was changed while writing this guide.

### Security

1. **Platform plans can be edited by any authenticated user.** `PUT /api/v1/me/studios/global/plans` has
   no role check in its handler (`internal/studios/platform_plans.go`). Restrict it to `super_admin`.
2. **Studio account deletion** (`DELETE /me/studios/{id}/delete-account`) is a hard cascading delete.
   It checks only that the caller belongs to the studio and supplies the studio's contact email, not that
   the caller is an admin. Restrict to `studio_admin`.
3. ~~Fixed default teammate password~~ **Fixed**: new accounts get a random one-time password.
   Accounts created **before** this change that never logged in still have `password123`.
4. **Provider and Meta/Google secrets are stored unencrypted on `studios`** (Gemini, Groq, Claude,
   Mistral keys, Meta app secret, Google client secret and developer token). Stripe secrets, channel tokens,
   CRM credentials and per-model LLM keys are encrypted.
5. ~~Unverified webhooks~~ **Fixed**: Twilio (`X-Twilio-Signature`), X POST (`x-twitter-webhooks-signature`)
   and the Meta data-deletion callback (`signed_request`) are now verified. The Meta callback still only
   **acknowledges**: it deletes nothing (Meta's user id doesn't map to our contact ids), so deletion remains manual.
6. **Public reviews endpoint** accepts unauthenticated writes with no rate limit or moderation.
7. **No encryption-key rotation** for `TOKEN_ENCRYPTION_KEY`; it also protects the session cookie.
8. A password reset does not revoke existing sessions; `forgot-password` reveals whether an email exists.

### Behaviour that surprises

1. **Glofox first-session runs for a single studio** picked at boot; the per-studio switch only matters for it.
2. **The final AI reply ignores the AI reply delay setting** (hard-coded 20 s).
3. **Without a Gemini key the AI has no retrieval and no low-confidence escalation.**
4. **`sent` is not "delivered"** (no receipts recorded for WhatsApp Web).
5. **Broadcast "completed" means queued**, and header matching can mis-map a "Contact Email" column.
6. Lead import has no de-duplication and queues auto-contact messages for every row with a phone.
7. First outreach forces status `contacted` even for `trial_booked`/`member` leads.
8. Cold detection is a 15-minute snapshot; Attendance data can be up to 15 minutes old.
9. The permission check has no GET-vs-write distinction; Attendance is admin-only because its path has no entry.
10. The knowledge-base "domain" tag does nothing; a rebuilt style profile overwrites manual edits.
11. `auto_contact_batch_limit`, `snoozed` conversations, `conversations.assigned_to`, the `failed` job
    status and the `google_ads` messaging channel are present but unused.
12. A hard-coded studio id is used for AI generation in the global Social Planner; the booking confirmation text
    in `BookTrialSlot` contains a typo.
13. 440 first-session jobs on prod were blocked as "dnd enabled" although few leads had DND on; the cause
    was not found.

### Operations

- Prod Redis has no volume (restart = everyone logged out). Rate limits and WhatsApp pacing are in memory.
- The SSE bus is single-replica only; the decision-tree cache is per instance (3 minutes).
- No CI/CD; no off-box backups; uploads, WhatsApp auth and secrets are not backed up.
- CORS allows GET, POST, PATCH, DELETE, OPTIONS only (no PUT); fine behind the same-origin proxy.
- `docs/AI_CHATBOT.md` and `docs/AI_RESPONSE_GUIDE.md` are out of date (see [section 14](#14-related-docs)).

---

## 13. Reference: placeholders, statuses, ports

### Placeholder syntaxes (do not mix)

| Used in | Syntax |
|---|---|
| Outbound worker (snippets, broadcasts, scheduled messages, Glofox first-session) | `{{contact.first_name}}`, `{{studio.name}}`, `{{campaign.name}}` |
| Greeting, auto-contact, decision-tree replies, follow-ups | `{{lead_first_name}}`, `{{lead_name}}`, `{{studio_name}}`, `{{lead_status}}` (the status/studio forms only in some tree actions) |
| Templates page confirmations | `{{lead_first_name}}`, `{{studio_name}}`, `{{amount}}`, `{{receipt_url}}` |

### Statuses

- **Lead:** `new`, `contacted`, `trial_booked`, `member`, `dropped`, `paused`
- **Conversation:** `open`, `closed` (`snoozed` unused); **message:** `pending`, `sent`, `delivered`, `read`, `failed`
- **Outbound job:** `pending`, `sent`, `dead` (`failed` unused)
- **Channel:** `active`, `paused`, `disconnected`, `error`
- **Broadcast campaign:** `scheduled`, `sending`, `completed`, `canceled`; recipient: `pending`, `enqueued`, `failed`
- **Social post:** `draft`, `scheduled`, `published`, `failed`; **CRM provider:** `draft`, `active`
- **Knowledge sync:** `idle`, `syncing`, `complete`, `error`

### Ports (this repo's local setup)

Web 3010, API 8099, wa-web 3100, tg-web 3101, embeddings 8001, Postgres 5436 (direct) and the PgBouncer port
from `.env`, Redis 6379. `docs/skills.md` lists older defaults (3000 / 8080).

---

## 14. Related docs

| Doc | Status |
|---|---|
| [`../README.md`](../README.md) | Overview and quick start |
| [`skills.md`](skills.md) | Contributor playbook; its "level" scope is behind the current product |
| [`../deploy/README.md`](../deploy/README.md) | Deploy runbook (some details differ from the code, see section 11) |
| [`USER_MANUAL.md`](USER_MANUAL.md), [`CLIENT_USER_MANUAL.md`](CLIENT_USER_MANUAL.md) | Outbound Sheets sync and import template only; its status list omits `paused`; it embeds a service-account email that should be removed before sharing |
| [`SETUP_META_WHATSAPP.md`](SETUP_META_WHATSAPP.md), [`META_WHATSAPP_INTEGRATION.md`](META_WHATSAPP_INTEGRATION.md), [`META_MESSENGER_INTEGRATION.md`](META_MESSENGER_INTEGRATION.md), [`../META_APP_CONFIGURATION_GUIDE.md`](../META_APP_CONFIGURATION_GUIDE.md) | Meta setup |
| [`SETUP_TELEGRAM.md`](SETUP_TELEGRAM.md), [`SETUP_TELEGRAM_QR.md`](SETUP_TELEGRAM_QR.md) | Telegram setup |
| [`SETUP_GOOGLE_SHEETS.md`](SETUP_GOOGLE_SHEETS.md), [`../GOOGLE_SHEETS_SETUP_GUIDE.md`](../GOOGLE_SHEETS_SETUP_GUIDE.md) | Google Sheets setup |
| [`STUDIO_ONBOARDING_PREREQUISITES.md`](STUDIO_ONBOARDING_PREREQUISITES.md) | What a studio needs before onboarding |
| [`MESSAGING_NEXT_CHANNELS.md`](MESSAGING_NEXT_CHANNELS.md) | Plan for further channels |
| [`AI_CHATBOT.md`](AI_CHATBOT.md), [`AI_RESPONSE_GUIDE.md`](AI_RESPONSE_GUIDE.md), [`groq-multi-llm-plan.md`](groq-multi-llm-plan.md) | **Out of date**: describe a Claude-only, status-new-only, WhatsApp/Messenger-only bot. Use section 6 of this guide instead |
| [`../DATA_DELETION_PROCEDURES.md`](../DATA_DELETION_PROCEDURES.md), [`../PRIVACY_POLICY.md`](../PRIVACY_POLICY.md), [`../TERMS_AND_CONDITIONS.md`](../TERMS_AND_CONDITIONS.md) | Legal. The deletion procedure describes verified, audited deletion that the code does not implement (see section 12); `docs/TERMS_AND_CONDITIONS.md` differs from the root copy |
