# Project-X — Agent Playbook (skills.md)

> Read this **before** taking any non-trivial action in this repo. It encodes the
> decisions, conventions, and guardrails that aren't obvious from the code yet
> (because the code is still small).

---

## 1. What this project is

An end-to-end **AI-run marketing + studio operations platform** for fitness
studios. Full vision lives in [`init.md`](../init.md). Long-term architecture
lives in this file's sibling decisions and in PR descriptions.

We are building it in **levels** that ladder up to the SRS phases:

| Level | Scope (1-line) | Maps to SRS phase |
|---|---|---|
| **L1 (now)** | Multi-tenant: super-admin creates **studios** (each with name, brand color, logo, first admin login). Each studio's admin runs campaigns + sees their leads. Public lead-capture URL is `/l/<studio-slug>/<campaign-slug>` and is **branded per studio**. Leads land in Postgres + Google Sheet via outbox. | Pre-Phase 1 foundation |
| L2 | Multi-location vendors (vendor with N studios), lead status workflow polish, logo file uploads, studio self-onboarding | Phase 1 partial |
| L3 | Channel integrations (Meta/IG/Google), AI reply drafting, sentiment scoring | Phase 1 MVP |
| L4+ | See `init.md` Phases 2–7 |

**Where we are right now: L1.** Don't build L2+ features unless explicitly asked.

---

## 2. Stack — locked decisions

| Layer | Choice | Notes |
|---|---|---|
| Backend language | **Go 1.23+** | One binary today, microservices later |
| HTTP router | **chi** | Std-lib compatible, no surprises |
| DB driver | **pgx/v5** + **sqlc** for typed queries | No ORM. Ever. |
| Migrations | **goose** | `make migrate-up` / `migrate-down` |
| DB | **Postgres 16** | Local via `docker-compose` |
| Cache / queue | **Redis 7** + **Asynq** | Add when first needed, not before |
| Auth (admin) | Email/password → JWT in HTTP-only cookie | Replace with OIDC at L3 |
| Frontend | **Next.js 15 (App Router)** + **TypeScript** + **Tailwind v3.4** | Single app `apps/web` (no monorepo of apps). v3 (not v4) — stable in this exact stack. |
| UI components | **Custom components** in `apps/web/src/components/ui/*.tsx` | Built directly on Tailwind utilities — NOT shadcn. Variant-class-map pattern (see Button). |
| Forms | **react-hook-form** + **zod** at L2+. L1 uses plain controlled state — fine for short forms. | Zod will be shared with backend types via codegen when added. |
| Server state | **TanStack Query** when client-side cache appears. L1 uses RSC `serverFetch` for reads + plain `fetch` for mutations. | Don't add it before it earns the dep. |
| Styling | **Tailwind v3** with `brand.*` color scale + `brand.primary`/`brand.dark` in `apps/web/tailwind.config.js`. | Default platform brand `#7c3aed`. Per-studio brand color is **applied via inline `style={{ background: studio.brandColor }}`** on the public form only — admin UI stays neutral. |
| Package mgr | **pnpm** workspaces (one app right now) | |
| Logs | `slog` (Go), JSON | Always include `request_id`, `tenant_id` (when present) |
| Tests | `testify` + **testcontainers-go** for DB | Real Postgres in CI, no mocks for DB |
| Lint | `golangci-lint` (Go), ESLint + Prettier + `prettier-plugin-tailwindcss` (TS) | |

---

## 3. Repository layout

```
Project-X/
  apps/
    api/                          # Go modular monolith (future microservices)
      cmd/server/main.go
      cmd/seed/main.go            # creates the platform super-admin
      internal/
        identity/                 # users, sessions, JWT — future identity service
        studios/                  # studios CRUD + create-with-admin (new at L1)
        leads/                    # campaigns, leads, public submission, outbox
        integrations/sheets/      # Google Sheets worker
        platform/                 # logger, config, db, http middleware, errors
      migrations/                 # goose SQL migrations
    web/                          # SINGLE Next.js app: admin + public + auth
      src/
        app/
          login/                  # /login — single login URL (both roles)
          admin/                  # /admin/* — auth-gated, role-aware
            layout.tsx            # wraps everything in <AppShell>
            studios/              # super-admin: studios CRUD
              [studioId]/         # both roles: campaigns + leads + settings
                campaigns/
                leads/
                settings/         # studio identity (name/brand/logo)
          l/[studioSlug]/[campaignSlug]/  # public lead-capture (per-studio brand)
        components/
          AppShell.tsx            # sidebar + topbar + role-based theming
                                  # (sets --brand / --brand-soft / etc. CSS vars)
          ui/                     # Button, Card, Input, Select, Textarea,
                                  # Label, Badge, PageHeader, EmptyState, StatCard
        lib/
          api.ts                  # client-side fetch helper
          auth.ts                 # server-side requireSession + serverFetch
          public.ts               # server-side public fetches (no cookie)
          cn.ts                   # clsx + tailwind-merge
          color.ts                # withAlpha, brandInitials helpers
          types.ts                # Studio, Campaign, Lead, LeadStatus, Me, ...
      tailwind.config.js          # platform brand scale, fonts, shadows
      postcss.config.js
  docs/
    skills.md
    SETUP_GOOGLE_SHEETS.md
  docker-compose.yml              # postgres
  Makefile                        # make dev, make migrate-up, ...
  .env.example
```

**Why a single Next.js app:** at L1 there's not enough divergence between admin
and public to justify two deploy lifecycles or a shared-component package. The
public form is one route. We collapsed back from a two-app monorepo because the
overhead wasn't earning its keep. Re-split if/when L2 brings ≥3 frontends with
different deploy needs.

**Why a modular monolith for the API:** package boundaries today *are* service
boundaries tomorrow. `studios`, `identity`, `leads`, `integrations/sheets` will
each become their own service when scale demands. The transport changes; the
domain code doesn't.

---

## 4. Conventions — non-negotiable

### 4.1 Backend (Go)

- **Hexagonal**: `domain/` has no I/O, `app/` orchestrates use cases,
  `adapters/` talks to the outside world. Handlers stay thin.
- **Errors**: never return raw `pgx`/`sql` errors past the repo layer. Wrap with
  `cockroachdb/errors` or a domain error type.
- **Context**: every function that does I/O takes `context.Context` first.
- **No global state.** Config, DB, logger are constructed in `main` and passed.
- **DB writes that emit events use the outbox pattern.** Write the row + an
  `outbox` row in the same transaction. A separate worker publishes to the
  external system (Google Sheets, later Kafka). This is how we guarantee a lead
  is never lost because Sheets was down.
- **Multi-tenancy: a studio is the tenant.** Every studio-scoped table has
  `studio_id`. The JWT carries `studio_id` for studio_admins (NULL for
  super_admins). Authorization is done via `Claims.EffectiveStudioID(...)` and
  `resolveStudioID(...)` middleware — super-admins act on the studio in the URL,
  studio-admins MUST match their own. Fail closed: 403 if mismatch.
- **Inactive-studio lockout.** Every studio-scoped route group is wrapped in
  `studiosHandler.RequireActiveStudio` (in `cmd/server/main.go`). When a
  super-admin marks a studio inactive, every API call from that studio's
  admins returns 403 with `code: "studio_inactive"`. Super-admins bypass the
  middleware so they can still manage / reactivate. The `/me` endpoint
  itself stays open (so the frontend can read `studio.active=false` and
  render the lockout screen). Don't add new studio-scoped routes outside
  this middleware group.
- **Slugs are unique per-studio**, not globally. Two studios can both use
  `spring-promo`. Constraint: `UNIQUE (studio_id, slug)` on `campaigns`.
- **Public endpoints are explicitly marked.** Default is auth-required. The
  middleware fails closed.

### 4.2 Frontend (TS / React / Tailwind)

#### Tailwind setup
- **Tailwind v3**. Single `apps/web/tailwind.config.js` defines the brand
  scale (`brand.50…brand.900` + `brand.primary`/`brand.dark`), Inter font,
  `card` / `card-hover` shadows, `brand-gradient` utility.
- **PostCSS**: `tailwindcss` + `autoprefixer` in `apps/web/postcss.config.js`.
- **Globals**: `apps/web/src/app/globals.css` — `@tailwind base/components/
  utilities` + `@layer base` for font + body bg.

#### Components
- **All UI components live in `apps/web/src/components/ui/<PascalCase>.tsx`**.
  Imports from `@/components/ui/Button`, etc.
- **Variant pattern (mandatory):** map variants and sizes to className strings
  using a `Record<Variant, string>` and join with the `cn()` helper from
  `@/lib/cn`. Mirror the `Button` component for any new one.
- **Class-based dark mode.** Every theme-aware utility ships its `dark:`
  counterpart inline. Toggle is `class="dark"` on `<html>` (no UI toggle yet).
- **Icons**: `lucide-react`. Sizes are `h-4 w-4` for inline / `h-5 w-5` for
  card heads / `h-[18px] w-[18px]` for nav. Don't hand-roll SVGs unless lucide
  doesn't have it.

#### Brand theming (the most important convention here)

Tailwind's `brand.*` scale is the **platform** brand only. It's used for:
- The platform login page (`/login`)
- The platform sidebar logo block when role = `super_admin`

Inside `<AppShell>`, the active brand is exposed as **CSS variables**:
- `--brand` — the active hex (studio's brandColor for studio_admin, platform
  violet for super_admin)
- `--brand-soft` — `--brand` at ~8% alpha (for tinted backgrounds)
- `--brand-softer` — `--brand` at ~16% alpha (for focus rings)
- `--brand-onbrand` — text color on top of `--brand` (white)

**Anywhere a brand-colored accent appears INSIDE the admin shell** (active nav
item, primary button, link hovers, focus rings, badges that should follow the
tenant), use the variable form:
- Tailwind: `bg-[var(--brand,#7c3aed)]`, `text-[color:var(--brand,#7c3aed)]`,
  `ring-[color:var(--brand-softer,...)]`
- Inline: `style={{ background: 'var(--brand)' }}`

The `<AppShell>` component (`apps/web/src/components/AppShell.tsx`) is the
single source that sets these vars based on the user's role + studio. The
public form and the studio login page set their own scope inline (because no
shell wraps them).

**Don't** use the static `brand-primary`, `brand-300`, etc. classes for any
admin chrome that should follow the tenant's color. Those are fine for the
platform login and a few super-admin-only pieces (the platform sidebar block,
hero sections on `/login`).

#### Pages / data
- **Mutations use Server Actions, not client `fetch`.** A bare client-side
  `fetch` + `router.back()` doesn't invalidate Next.js's RSC data cache,
  so the destination can render stale data (we hit this on the pipeline
  page). Use a Server Action co-located with the page (`actions.ts`) that:
  1. Forwards the auth cookie to the Go API
  2. On success, calls `revalidatePath(...)` for **every** page that shows
     the changed data (pipeline, leads list, dashboard, lead detail, etc.)
  3. Returns a discriminated-union result (`{ ok: true } | { ok: false,
     error, details? }`) — never throws, so the client component renders
     errors cleanly.
  Examples to mirror: `app/admin/studios/[studioId]/leads/[id]/actions.ts`
  and `.../settings/actions.ts`.
- **Save → back navigation.** After the action returns ok, navigate back
  to wherever the user came from with `router.back()` (fall back to a
  sensible parent if `window.history.length <= 1`). No `router.refresh()`
  needed — `revalidatePath` already invalidated the destination's cache.
  Don't leave users sitting on the detail page with a "Saved" toast.
- **Server Components by default**, Client Components only when you need state
  or effects. The lead form is a Client Component; the admin lead table is RSC
  with a small Client island for filters.
- **Data fetching**: `serverFetch()` (in `@/lib/auth.ts`) in RSC for protected
  reads (forwards the cookie). For public RSCs use the helpers in
  `@/lib/public.ts` (no cookie). Plain `fetch` in Client Components for
  mutations. Never call the API from `useEffect` directly.

#### API URL — never hardcoded in the browser
- **Browser-side fetches use relative paths only** — `fetch('/api/v1/...')`.
  The browser resolves them against `window.location.origin` automatically,
  so the same code works on `localhost`, staging, and prod with no env-var
  acrobatics in client code.
- **Next.js `rewrites`** in `next.config.mjs` proxy `/api/*` to the Go API
  (read from `API_BASE_URL`). This is what makes the relative paths work in
  dev. In prod the deploy's ingress points `/api/*` at the API service, same
  effect.
- **`API_BASE_URL` is server-only** — used by `next.config.mjs` (rewrites),
  `lib/auth.ts`, `lib/public.ts`. There is **no `NEXT_PUBLIC_API_BASE_URL`**
  — and there should never be one. If you need a server-side absolute URL,
  read `process.env.API_BASE_URL` (it's safe; it's not in the client bundle).
- If you find yourself adding a hardcoded backend host in any `.tsx` file
  under `apps/web/src/app/` or `components/`, that's a bug — use a relative
  path or, in RSC, call `serverFetch`/`fetchPublicStudio`/etc.
- **Forms**: at L1, plain controlled state is fine (small forms). When forms
  grow or we share schemas with the API, switch to react-hook-form + zod.
- **A11y is shipping criteria**, not a follow-up.

#### Lists, pagination, and charts
- **Pagination**: any list that can grow past ~50 rows uses the
  `<Pagination>` component (`@/components/ui/Pagination`) with `?page=N` URL
  state. Page size is **25** by default. The component preserves all other
  searchParams — so filters (`?status=new`) survive page navigation. RSC
  reads `page`, computes `offset`, and passes `limit`/`offset` to the API.
- **Aggregations**: don't fetch full lists to compute counts. Add a small
  GROUP BY endpoint (see `/leads/stats`) and consume it in widgets. One
  round-trip, server-side aggregation, scales without changing the UI.
- **Charts**: prefer hand-rolled SVG/CSS for simple visualizations
  (FunnelStrip, StatusDonut). No charts library (recharts/visx/etc.) —
  bundle cost outweighs benefit for what we need at L1. If we need
  interactive line/area charts later, that's the time to evaluate one.
- **Widget components live in `apps/web/src/components/widgets/`** —
  separate folder from `components/ui/` so it's clear which are domain
  visualizations vs. generic primitives.

#### Date / time formatting (no hydration footguns)
- **Never call `toLocaleDateString()` / `toLocaleTimeString()` /
  `toLocaleString()` without an explicit locale arg.** The server (Next.js
  Node process) and the user's browser have different defaults — same
  `new Date()` renders as `07/05/2026` on the server and `5/7/2026` on the
  client → React hydration mismatch. Use `formatDate` / `formatDateTime` /
  `formatTime` from `@/lib/datetime` — they pin `'en-GB'` so output is
  identical everywhere.
- **`Date.now()` in render is also a hydration trap** (it advances between
  SSR and hydration). For relative timestamps ("5m ago") use
  `relativeTime()` from `@/lib/datetime` AND wrap the rendering element
  with `suppressHydrationWarning` — the difference is bounded by SSR
  latency and React tolerates it silently.

#### Mobile responsive (every page)
- **Layout**: mobile-first. The `AppShell` sidebar is a fixed-position drawer
  on `<lg`, slides in via `translate-x` toggled from a hamburger button in the
  topbar. On `lg+` it becomes a sticky in-flow column. Backdrop locks body
  scroll while open. Drawer auto-closes on route change.
- **Padding**: use `px-4 sm:px-6 lg:px-10` and `py-6 sm:py-8` on main content
  shells. Don't hard-code `px-8` — that breaks small screens.
- **Tables**: every table sits inside a `<div class="overflow-x-auto">` with
  `min-w-[Npx]` on the table itself so it scrolls horizontally rather than
  squashes on mobile. Pick min-width based on column count (560px–820px).
- **Topbar**: hide non-essential text (email) below `sm:`. Keep hamburger
  (mobile only) + sign-out icon (always).
- **Login**: the dark hero is `hidden lg:flex`. Mobile gets a clean centered
  form with a small PX logo header on top.
- **Test at**: 360px (small phone), 768px (tablet), 1280px (desktop).

### 4.3 Both sides

- **Tests**: every bug fix lands with a regression test. New feature lands with
  at least one happy-path integration test.
- **Migrations are expand-then-contract.** Never a destructive migration in the
  same release as the code change that needs it.
- **No secrets in the repo.** `.env.example` has the keys; real values in `.env`
  (gitignored) or in a secrets manager later.

---

## 5. How to run things (kept current as we add commands)

### First-time setup

```bash
# 0. Tooling — one time per machine
corepack enable
corepack prepare pnpm@9.10.0 --activate     # or: npm i -g pnpm@9.10.0

# 1. Env
cp .env.example .env
# Edit .env BEFORE running migrations:
#   - JWT_SECRET must be 32+ chars
#   - POSTGRES_PORT defaults to 5434 (5432 / 5433 collide with common installs)
#   - SUPER_ADMIN_EMAIL / SUPER_ADMIN_PASSWORD become your admin login

# 2. Postgres
make db-up          # starts Postgres in Docker on POSTGRES_PORT, waits for health
make migrate-up     # applies all migrations

# 3. Super admin
make seed-admin     # creates / updates the super admin from .env (idempotent)

# 4. JS deps + web env
make install
cp apps/web/.env.local.example apps/web/.env.local

# 5. (optional) Google Sheets — see docs/SETUP_GOOGLE_SHEETS.md
```

### Day-to-day

| Command | What it does |
|---|---|
| `make dev` | Runs API + web concurrently (uses `npx concurrently`) |
| `make api` | Just the Go API on `:8080` |
| `make web` | Just the Next.js web app on `:3000` |
| `make migrate-up` / `migrate-down` / `migrate-status` | DB migrations |
| `make migrate-new name=add_xxx` | New SQL migration |
| `make test` | `go test ./...` |
| `make lint` | `go vet` + `pnpm -r lint` |
| `make db-down` | Stop Postgres |

### URLs

- API: http://localhost:8080 (health: `/health`)
- Web app: http://localhost:3000
  - **`/login`** — single login URL for *both* roles. The JWT determines
    where each user lands. (We had `/s/<slug>/login` briefly — deleted; one
    URL is the right call.)
  - Super-admin home: `/admin/studios`
  - Studio-admin home: `/admin/studios/<their-studio-id>` (overview)
  - Public form: `/l/<studio-slug>/<campaign-slug>`

### Common port conflicts (notes from setup)

This machine has multiple Postgres/Node processes around. We've already
encountered:

| Port | Conflict |
|---|---|
| 5432 | Native (Homebrew/Postgres.app) Postgres → moved our container to 5434 |
| 5433 | `ibms-postgres` container from another project |
| 8080 | Sometimes a stray `node` process — kill it with `lsof -nP -iTCP:8080 -sTCP:LISTEN` then `kill <pid>` |

If `make db-up` fails with "port already allocated", run
`lsof -nP -iTCP:<port> -sTCP:LISTEN` to identify, then either kill the process
or bump `POSTGRES_PORT` in `.env` and `docker-compose.yml`.

> **Update this section every time a command changes.** If a future agent runs
> `make dev` and it fails, that's a bug in this file.

---

## 6. Things to NOT do (lessons baked in upfront)

- **Don't introduce Kafka, K8s, OPA, or microservice splits in L1.** They're in
  the architecture for a reason — when we hit the pain. Premature now.
- **Don't dual-write.** If you write to Postgres *and* an external system in a
  handler without an outbox, you've created a data-loss bug. Use the outbox.
- **Don't add an ORM.** sqlc gives us types without the magic. We've decided.
- **Don't reach for a state library before TanStack Query proves insufficient.**
- **Don't write planning/decision/summary `.md` files unless asked.** Decisions
  go in `docs/decisions/NNNN-title.md` (ADR format) when they're load-bearing.
- **Don't bypass the design tokens** with `style={{ color: '#...' }}` or
  `bg-[#abc123]` in admin / login / shared UI. Add the token to
  `apps/web/tailwind.config.js`. **Exception**: per-studio `brandColor` on the
  PUBLIC form is dynamic per-tenant — that's the only inline-color allowance.
- **Don't import shadcn/ui or any other component lib.** We build directly on
  Tailwind utilities. Mirror the existing `Button.tsx` variant pattern.
- **Don't upgrade to Tailwind v4.** We tried; the `@theme` + `@source` model
  played poorly with Next.js + monorepo class scanning. v3 is the decision
  until v4 tooling is unambiguously stable in this stack.
- **Don't re-split the frontend into multiple apps** until L2 brings ≥3
  frontends with different deploy needs. We collapsed back from a two-app
  monorepo because the overhead wasn't earning its keep.
- **Don't ship a feature without telling the user how to try it.** End-of-task
  message includes the URL / command.

---

## 7. Working agreements with the user

- The user wants **CTO-level rigor** but also **shipped features** — bias
  toward "smallest version that's structurally correct," not "smallest version
  that works."
- Keep responses tight. Show what changed and how to try it. Don't re-explain
  the architecture every turn.
- When a decision has real tradeoffs (cost, lock-in, complexity), surface them
  in 2 lines and let the user pick. Don't unilaterally pick the "enterprise"
  option just because it's in `init.md`.

---

## 8. What's actually built right now (L1, multi-tenant)

### Backend (`apps/api`)

Modular monolith. Chi router, pgx pool, outbox-pattern Sheets sync, JWT-cookie
auth. Roles: `super_admin` (NULL studio_id) + `studio_admin` (NULL-not-allowed
studio_id, enforced by a CHECK constraint).

| Surface | Endpoints |
|---|---|
| Auth | `POST /api/v1/auth/login` · `POST /auth/logout` · `GET /auth/me` |
| Public | `GET /api/v1/public/studios/:studioSlug` (brand info)<br>`GET /public/studios/:studioSlug/campaigns/:campaignSlug`<br>`POST .../leads` |
| Super-admin: studios | `GET/POST /api/v1/admin/studios` · `GET/PATCH /admin/studios/:id`<br>`POST /admin/studios` creates studio + first studio_admin in one tx |
| Self (any auth) | `GET/PATCH /api/v1/me/studios/:id` (used by Settings; super sees any, studio sees own) |
| Studio-scoped (campaigns + leads) | `/api/v1/studios/:studioId/campaigns` (GET/POST), `/campaigns/:id` (GET/PATCH)<br>`/leads` (GET, filters: `campaignId`, `status`, `limit`, `offset`), `/leads/:id` (GET/PATCH)<br>`/leads/stats` (GET — `{total, byStatus}` for widgets) |

### Web app (`apps/web`, port 3000)

Single Next.js app with **sidebar layout** (`AppShell`) and **role-aware
theming via CSS variables** (see §4.2 Brand theming).

**Auth surface:**
- `/login` — single login URL for both super-admin and studio-admin. Dark
  hero on the left (violet gradient + features), form on the right. After
  submit, the JWT determines where to send each role next. Studio brand
  themes the post-login admin shell — the login page itself is platform-
  branded for everyone.

**Inside `<AppShell>` (sidebar + role-themed):**
- `/admin/studios` (super-admin): summary tiles + studio cards (brand-color
  accent strip per card, logo tile, lead/campaign counts)
- `/admin/studios/new` (super-admin): two-card form — Identity + First admin
  login. Color picker, logo URL, live preview.
- `/admin/studios/[studioId]` overview: 3 stat cards (Campaigns / Leads /
  Brand) + a row of pipeline widgets (FunnelStrip + StatusDonut, fed by
  `GET /leads/stats`), recent campaigns table, latest leads list
- `/admin/studios/[studioId]/pipeline` — **Kanban view**: 5 columns (new /
  contacted / trial_booked / member / dropped) with lead cards in each.
  Server-rendered, capped at 50 cards/column with a "+N more" link to the
  filtered `/leads?status=...` view. Horizontal-scroll on mobile.
- `/admin/studios/[studioId]/campaigns` + `/new` + `/[id]`
- `/admin/studios/[studioId]/leads` + `/[id]` — **paginated** at 25/page,
  `?page=N` URL state, status filter preserved across pages
- `/admin/studios/[studioId]/settings` — live brand preview reflecting form
  state in real-time

**Public:**
- `/l/[studioSlug]/[campaignSlug]` — per-studio branded form (already used
  brand color before; unchanged)

### DB
`studios`, `users`, `campaigns` (with `UNIQUE (studio_id, slug)`), `leads`, `outbox`.

### Sheets
Outbox worker drains every 5s, exponential backoff, dies after 8 attempts.

### Messaging (Inbox + Channels) — `internal/messaging/`

Per-studio multi-channel inbox with WhatsApp wired today and the data model
ready for IG / Messenger / X. **Direct Meta WhatsApp Cloud API**, no BSP.

| Surface | Endpoints |
|---|---|
| Public webhook | `GET/POST /api/v1/webhooks/meta/whatsapp` (verify + receive) |
| Studio-scoped REST | `/api/v1/studios/:id/messaging/channels` (GET/POST/DELETE)<br>`/conversations` (GET, list+filter)<br>`/conversations/:id` (GET)<br>`/conversations/:id/messages` (GET, POST, paginated)<br>`/conversations/:id/read` (POST)<br>`/stream` (SSE — live updates) |

**Architecture invariants for messaging (don't break):**
- **Outbound goes through one path**: `outbound_jobs` → worker → `Sender`
  interface → channel adapter. Manual reply, automation (phase D), AI
  auto-send (phase E) all hit the same dispatcher. Don't add a second path.
- **Channel adapters take primitives, not domain types** — keeps `channels`
  package decoupled from `messaging` (no import cycle). The `Sender` signature
  is `SendText(ctx, accessToken, channelExternalID, recipient, body)`.
- **Identity stitching is per studio**: `contact_identities (studio_id, kind,
  value)` is unique; same person across channels = one row per channel,
  optionally linked to one `lead_id`.
- **Tokens encrypted at rest** via `internal/platform/secrets` (AES-256-GCM
  from `TOKEN_ENCRYPTION_KEY`). Decrypt only in repo methods that need it
  (`GetChannelByID`, `GetChannelByExternalID`); list endpoints leave it
  empty.
- **Events bus** (`messaging.Bus`, in-process) is the seam for SSE today and
  automations + AI tomorrow. Add subscribers, never wiring. When we
  multi-replica, swap the impl for Postgres LISTEN/NOTIFY — interface stays.
- **Phase D + E tables already exist** (`message_templates`,
  `automation_rules`, `automation_runs`, `ai_suggestions`,
  `message_analyses`). Schema is locked; just add writers when those phases
  ship.
- **Inbound is idempotent**: `messages.UNIQUE (conversation_id, external_id)`
  collapses Meta's webhook retries.
- See [`docs/SETUP_META_WHATSAPP.md`](SETUP_META_WHATSAPP.md) for the
  one-time Meta App setup + per-studio onboarding.
- **Adding more channels (Instagram / Messenger / X)** — the plan is in
  [`docs/MESSAGING_NEXT_CHANNELS.md`](MESSAGING_NEXT_CHANNELS.md). Don't
  start until WhatsApp has been validated with a real studio for a couple
  of weeks; the doc explains why and lists per-channel work, scopes, and
  gotchas.

## 9. Production deploy (single EC2)

All deploy config lives in [`deploy/`](../deploy/). Single Ubuntu box,
Docker Compose, no CI/CD, no image registry. The runbook is
[`deploy/README.md`](../deploy/README.md). Day-to-day: SSH in, `bash
deploy/deploy.sh`. That's it.

| File | What |
|---|---|
| `deploy/api.Dockerfile` | Multi-stage Go → distroless. Bundles `server`, `seed`, `goose`. |
| `deploy/web.Dockerfile` | Multi-stage Node → Next.js standalone runtime. |
| `deploy/docker-compose.yml` | postgres + api + web + nginx + one-shot `migrate`/`seed` (in `tools` profile). |
| `deploy/nginx/default.conf` | `/api/*` → api:8080, `/*` → web:3000. Same-origin to the browser. |
| `deploy/setup-ec2.sh` | One-time bootstrap: docker, swap, dirs. |
| `deploy/deploy.sh` | git pull → build → migrate → seed → up. |
| `deploy/backup.sh` | Nightly `pg_dump` (cron). |
| `deploy/.env.example` | Production env template (real `deploy/.env` is gitignored). |

**Architecture invariants for deploys (enforce when changing):**
- Browser only ever talks to nginx on the box (port 80 today, 443 later).
  All `/api/*` calls are same-origin → same code as local dev.
- The api container has zero shell (distroless). For debugging use
  `docker compose logs` or run an alpine sidecar.
- Migrations run *before* the new api container takes traffic
  (`docker compose run --rm migrate` in `deploy.sh`). Schema-and-code
  changes ship atomically.
- Seed is idempotent — re-runs every deploy to keep the super-admin password
  in sync with `.env`. Don't break the idempotency.
- Next.js config has `output: 'standalone'` + `outputFileTracingRoot`.
  Don't remove these — the runtime image depends on them being on.

## 10. Open questions / followups parking lot

(Append here when something comes up that we deferred. Clear when resolved.)

- [ ] Domain + TLS (currently HTTP-only on the EC2 IP)
- [ ] CI/CD (currently manual `bash deploy/deploy.sh` on the box)
- [ ] Backups → S3 (currently local-disk only)
- [ ] L2 scope kickoff: multi-vendor, lead status workflow polish, channel integrations
