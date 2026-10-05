# Project-X (StudioX)

AI-run marketing and studio-operations platform for fitness studios. A super-admin
creates **studios** (tenants); each studio gets an admin team that runs campaigns,
works leads, talks to customers across messaging channels, and lets an AI assistant
handle first replies.

This file is the map. The full, detailed description of every feature is in
[`docs/PLATFORM_GUIDE.md`](docs/PLATFORM_GUIDE.md), including a list of known risks.
Other details live in the linked docs. Conventions and "do not do
this" rules for contributors are in [`docs/skills.md`](docs/skills.md); the product
vision is in [`init.md`](init.md).

---

## Contents

1. [Architecture](#architecture)
2. [Features](#features)
3. [Messaging channels](#messaging-channels)
4. [Background workers](#background-workers)
5. [Roles and permissions](#roles-and-permissions)
6. [What changed in Oct 2026](#what-changed-in-oct-2026)
7. [Run it locally](#run-it-locally)
8. [Deploy to production](#deploy-to-production)
9. [Operating notes and known gaps](#operating-notes-and-known-gaps)
10. [Documentation index](#documentation-index)

---

## Architecture

| Piece | Where | What it is |
|---|---|---|
| API | `apps/api` | Go modular monolith: chi router, pgx, goose migrations, JWT-cookie auth, Swagger at `/swagger/index.html` |
| Web | `apps/web` | Single Next.js 15 app (App Router, TypeScript, Tailwind v3): admin, public lead form, login |
| WhatsApp Web | `apps/wa-web` | Node service that holds a QR-linked WhatsApp session per studio (Baileys) |
| Telegram QR | `apps/tg-web` | Node service for QR-linked personal Telegram accounts (MTProto) |
| Embeddings | `apps/embeddings` | Local FastAPI + sentence-transformers service for knowledge-base search |
| Data | Postgres 16, Redis 7 | Redis backs the AI answer cache |
| Prod | `deploy/` | One Ubuntu EC2 box, Docker Compose, nginx in front. Runbook: [`deploy/README.md`](deploy/README.md) |

Go packages under `apps/api/internal/`: `identity` (users, JWT, roles, permissions),
`studios`, `leads`, `messaging` (inbox, channels, workers, broadcasts),
`decisiontree`, `reviews`, `integrations/*` (sheets, glofox, crm, google, LLM
providers), `platform/*` (config, db, logging, mail, secrets, cache).

Multi-tenancy: a **studio is the tenant**. Every studio-scoped table has
`studio_id`, and every studio route runs behind the inactive-studio lockout and
the permission check.

---

## Features

Studio admin app (`/admin/studios/<studioId>/...`):

| Section | What it does |
|---|---|
| **Dashboard** | Campaign, lead and conversion overview with pipeline widgets |
| **Inbox** | Unified conversations across channels with live updates (SSE), reply composer, snippets, trigger links, escalation, and **Manual Actions** (scheduled messages and Broadcasts) |
| **Pipeline** | Kanban of lead statuses, including a Cold column for stalled leads |
| **Campaigns** | Campaigns with a public, studio-branded lead-capture URL `/l/<studio-slug>/<campaign-slug>` |
| **Leads** | Paginated lead list with filters, lead detail, CSV/Excel import, Google Sheets sync |
| **Social Planner** | AI-assisted social posts, published by a background worker |
| **Payments** | Studio payments page (Stripe OAuth route exists on the API) |
| **Channels** | Connect and manage messaging channels (see below) |
| **Attendance** | Per-studio Glofox attended-class counts |
| **Knowledge Base** | Studio knowledge the AI answers from, plus learned communication style |
| **Decision Trees** | Scripted conversation flows and follow-up steps, importable from Excel |
| **Templates** | Reusable message templates |
| **Users / Roles** | Staff accounts and custom roles with per-section permissions |
| **Settings** | Branding, plans, availability, booking page, Google Sheets, integrations, AI assistant, security |

Platform (super-admin) pages: **Studios**, **Analytics**, **Payments**, **LLM Monitor**,
**CRM Integrations**, **Settings**.

AI: per-studio model configuration over several providers (`integrations/claude`,
`groq`, `mistral`, `gemini`), a Redis answer cache, local embeddings for retrieval,
and deterministic code for things the model shouldn't guess (greeting word,
program-schedule lookup).

Integrations: Google Sheets (outbox-driven lead sync and a read-only external
leads sheet import), Glofox (first-session message, attendance), CRM connector,
Google (OAuth for Ads), SMTP email.

---

## Messaging channels

Channel kinds the platform can send on (see `internal/messaging/channels`):

- WhatsApp: Meta Cloud API, or WhatsApp Web (QR-linked number)
- Instagram DMs and Facebook Messenger (Meta)
- Telegram: bot, or QR-linked personal account (MTProto)
- SMS (Twilio)
- X (Twitter) DMs
- Email (SMTP)

**All outbound goes through one path**: `outbound_jobs` -> outbound worker ->
channel adapter. Manual replies, automations, AI auto-send and broadcasts all use
it; do not add a second path. Setup guides: [`docs/SETUP_META_WHATSAPP.md`](docs/SETUP_META_WHATSAPP.md),
[`docs/SETUP_TELEGRAM.md`](docs/SETUP_TELEGRAM.md), [`docs/SETUP_TELEGRAM_QR.md`](docs/SETUP_TELEGRAM_QR.md),
[`WHATSAPP_AI_SETUP.md`](WHATSAPP_AI_SETUP.md).

Safety rails that apply to automated sends:

- **Daily limit** per studio (default 48 WhatsApp automation/AI/Manual-Actions messages per Singapore-time day; 0 = unlimited). Live typed replies are exempt.
- **Send spacing** between WhatsApp messages on one number (default 20 s, per studio).
- **Do Not Disturb** on a lead or conversation blocks automated sends.

---

## Background workers

Started in `apps/api/cmd/server/main.go`:

| Worker | Job |
|---|---|
| Outbound | Claims due `outbound_jobs` and sends them. Studios are isolated: each studio's jobs run on their own goroutine, in order |
| Broadcast | Every minute, queues pending recipients of due Broadcast campaigns within the daily limit |
| Auto-contact | First outreach to new leads |
| AI | Drafts and sends AI replies |
| Cold-lead scanner | Flags leads that went quiet |
| Sheets / external sheet | Syncs leads out to, and in from, Google Sheets |
| Glofox first-session | Messages members after their first attended session. **Opt-in per studio** (see below) |
| Glofox attendance | Read-only; counts attended classes for the Attendance page. Never sends |
| Style | Learns each studio's communication style from staff messages |
| Social publisher | Publishes scheduled social posts |

---

## Roles and permissions

- **super_admin**: platform-wide, bypasses permission checks.
- **studio_admin**: full access to their own studio.
- **studio_staff**: gets a **studio role**, a named set of permission keys:
  `dashboard`, `inbox`, `pipeline`, `campaigns`, `leads`, `social-planner`,
  `payments`, `channels`, `knowledge-base`, `decision-trees`, `templates`, `settings`.

The API maps every route to the key(s) that grant it
(`internal/identity/rbac_middleware.go`); unmapped routes are denied. Permissions
are read from the login token, so a role change takes effect at the user's next login.
There is no GET-vs-write distinction per key, so pages must tolerate a 403 on
secondary data (the Leads page does this for the campaigns list).

---

## What changed in Oct 2026

**Broadcasts (Inbox -> Manual Actions -> Broadcasts).** Upload a `.csv`/`.xlsx`
with Client Name, Phone, Email columns (fuzzy-matched), pick a country code if
numbers lack one, then send or schedule one message to the whole list over
WhatsApp or email. Sends are paced and resume the next day if the daily limit
is hit. Cancelling a campaign also cancels its already-queued jobs.

**Email (SMTP) channel.** Connect SMTP per studio under Channels; usable for
Broadcasts and automated sends. Prod reads `SMTP_HOST/PORT/USER/PASSWORD/FROM`
from `deploy/.env`.

**Per-studio send isolation.** The outbound worker used to be one serial loop, so
a studio with a disconnected channel (jobs failing and retrying) held up every
other studio's messages. Now each studio sends independently. Test:
`internal/messaging/worker_studio_isolation_test.go`.

**Glofox first-session message is opt-in.** It used to start from env vars alone
and message every member who had ever attended: on prod this queued about 488
messages at once (48 sent, 440 blocked by DND). Now:
- Off by default for every studio; the worker itself runs for **one** studio only (the oldest studio with an active WhatsApp channel, picked at server start), so the toggle matters only for that one; toggle under **Settings -> Integrations -> Message Timing -> Glofox First-Session Message** (`GET/PUT /api/v1/studios/{id}/messaging/settings/glofox-first-session`).
- Only sessions attended after the switch was turned on, at most 3 days back.
- At most 20 members per 30-minute poll.

**Glofox attendance.** Per-studio attendance counts and settings on the new
Attendance page.

**Studio RBAC, password reset, messaging cache.** Custom roles, studio users
page, forgot/reset-password flow, Redis answer cache.

**Leads page permission fix.** A staff role with `leads` but not `campaigns` no
longer crashes the Leads page.

---

## Run it locally

```bash
cp .env.example .env            # set JWT_SECRET (32+ chars), SUPER_ADMIN_*, POSTGRES_*
make db-up                      # Postgres + Redis in Docker
make migrate-up                 # apply migrations
make seed-admin                 # create/update the super admin from .env
make install                    # JS deps
cp apps/web/.env.local.example apps/web/.env.local
make dev                        # API + web + wa-web + tg-web + embeddings
```

Other useful targets: `make api`, `make web`, `make test`, `make lint`,
`make swagger` (regenerate API docs after changing a route),
`make migrate-new name=add_xxx`. Ports come from `.env` / `apps/web/.env.local`
(this setup: web 3010, API 8099); `docs/skills.md` lists the defaults.

Tests: `cd apps/api && go test ./...`. Several messaging tests run against the
dev database from `.env`, so don't point `.env` at production.

---

## Deploy to production

Single EC2, Docker Compose. After pushing to `main`:

```bash
ssh -i <key>.pem ubuntu@<host>
cd studiox
git pull
bash deploy/deploy.sh           # pull -> build -> migrate -> seed -> up
```

`deploy.sh` runs migrations before the new API takes traffic and re-runs the
idempotent super-admin seed. Don't hand-edit tracked files on the server; it
blocks `git pull` (put env-specific values in `deploy/.env`, which is gitignored).
Full runbook, backups and first-time bootstrap: [`deploy/README.md`](deploy/README.md).

---

## Operating notes and known gaps

- **Delivery receipts are not recorded.** WhatsApp Web sends are marked `sent` when WhatsApp accepts them; nothing updates them to delivered/read, so `sent` does not prove the recipient got it.
- **Automations that still send without a per-feature switch** include lead auto-contact, trial-link messages and AI replies; they follow the studio's own configuration. Check a studio's setup before connecting a real WhatsApp number.
- **Glofox env vars are shared**: `GLOFOX_API_KEY/TOKEN/BRANCH_ID` also power lead and studio lookups, so removing them disables more than the first-session message. Use the per-studio switch instead.
- **Glofox booking times carry no time zone**, so the first-session cutoff can be off by up to about 8 hours.
- **Unexplained**: on prod, 440 first-session jobs were blocked with "dnd enabled" although the studio has only a handful of DND leads. Harmless (nothing was sent) but not root-caused.
- **No CI/CD**: deploys are manual.
- Never commit secrets. `.env` files, `secrets/` and credentials stay out of git.

---

## Documentation index

| Doc | Topic |
|---|---|
| [`docs/PLATFORM_GUIDE.md`](docs/PLATFORM_GUIDE.md) | Detailed guide to every feature, worker, integration and known risk |
| [`docs/skills.md`](docs/skills.md) | Contributor playbook: stack, conventions, do-not-do list |
| [`init.md`](init.md) | Product vision and phases |
| [`deploy/README.md`](deploy/README.md) | Production deploy runbook |
| [`docs/USER_MANUAL.md`](docs/USER_MANUAL.md), [`docs/CLIENT_USER_MANUAL.md`](docs/CLIENT_USER_MANUAL.md) | Sheets sync / lead lifecycle and client-facing manuals |
| [`docs/STUDIO_ONBOARDING_PREREQUISITES.md`](docs/STUDIO_ONBOARDING_PREREQUISITES.md) | What a new studio needs before onboarding |
| [`docs/AI_CHATBOT.md`](docs/AI_CHATBOT.md), [`docs/AI_RESPONSE_GUIDE.md`](docs/AI_RESPONSE_GUIDE.md) | AI assistant behaviour |
| [`docs/META_WHATSAPP_INTEGRATION.md`](docs/META_WHATSAPP_INTEGRATION.md), [`docs/META_MESSENGER_INTEGRATION.md`](docs/META_MESSENGER_INTEGRATION.md), [`META_APP_CONFIGURATION_GUIDE.md`](META_APP_CONFIGURATION_GUIDE.md) | Meta setup |
| [`docs/SETUP_GOOGLE_SHEETS.md`](docs/SETUP_GOOGLE_SHEETS.md), [`GOOGLE_SHEETS_SETUP_GUIDE.md`](GOOGLE_SHEETS_SETUP_GUIDE.md) | Google Sheets setup |
| [`docs/MESSAGING_NEXT_CHANNELS.md`](docs/MESSAGING_NEXT_CHANNELS.md) | Plan for further channels |
| [`DATA_DELETION_PROCEDURES.md`](DATA_DELETION_PROCEDURES.md), [`PRIVACY_POLICY.md`](PRIVACY_POLICY.md), [`TERMS_AND_CONDITIONS.md`](TERMS_AND_CONDITIONS.md) | Data and legal |
| [`openspec/`](openspec/) | Spec-driven change proposals |
