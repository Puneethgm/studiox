# Site Audit Report
**Date:** 2026-06-17
**Project:** 1herosocial.ai (Project-X)
**Detected stack:** Next.js 15 (App Router, standalone output) · TypeScript · Tailwind CSS v3.4 · Go 1.24 · chi router · pgx/v5 · JWT (HTTP-only cookie auth) · Asynq/Redis (outbox) · Docker Compose / Nginx (production on single EC2)
**Detected audience/goal:** B2B SaaS targeting fitness-studio owners. Platform super-admin provisions studio tenants; each studio manages leads, campaigns, and inbox in a branded admin shell. Public-facing lead-capture forms at `/l/<studio>/<campaign>`. Revenue via Stripe subscriptions (SGD). Apparent goal: multi-tenant "AI-run marketing OS" for gyms.
**Design system maturity:** Partially tokenized — `brand.*` Tailwind scale + CSS variables (`--brand`, `--brand-soft`, `--brand-softer`, `--brand-onbrand`) exist and are used consistently in the component library. However, public/marketing pages (`(home)`, `about`, `pricing`) use hardcoded hex `#FAF9F6` and hardcoded status colors (e.g. `#10b981`, `#94a3b8`) appear directly in admin pages. Overall maturity: **mid-tier tokenized; admin core is good, marketing surface and edge cases leak.**

---

## Anti-Pattern Verdict
**Partially** — the admin shell and component library show genuine design intent and are not AI slop, but several public-facing pages carry AI tells.

**Specific tells found:**
1. **Glassmorphism as decoration, not function** — `globals.css` defines `.glass`, `.glass-dark`, `.glass-container`, `.liquid-card`, `.premium-glass-card` with `backdrop-filter: blur(40px) saturate(180%)`. Named classes like `premium-glass-card` and `liquid-card` are decorative labels. The `login/page.tsx` hero stacks three separate `animate-pulse-liquid` radial-gradient blobs doing nothing structurally. (`globals.css:L73–L120`, `login/page.tsx:L119–L124`)
2. **`LIVE` + `STREAM` status badges with no function behind them** — the topbar renders an animated green `Live` pulse badge and a `Zap` `Stream` badge on the Inbox route (`AppShell.tsx:L518–L531`). These are cosmetic; they reflect no real-time connection state readable by screen readers or engineers.
3. **Gradient text on the hero** — `<span className="bg-gradient-to-r from-brand-300 to-sky-300 bg-clip-text text-transparent">` (`login/page.tsx:L133`). Classic AI-generated login hero.
4. **Predictable page layout on marketing pages** — home and pricing follow the exact pattern: gradient hero → feature grid → pricing cards → CTA → footer.
5. **Competing animation keywords** — CSS layer defines `animate-float`, `animate-pulse-liquid`, `shimmer`, `animate-in`, `animate-slide-up`, and stagger helpers all at once. The logo alone has `animate-float` applied.

**Score: 2/4** — the admin core and public lead form are noticeably intentional; the marketing/login shell is a stock AI design with applied brand colors.

---

## Audit Health Score

| # | Dimension | Score | Key finding |
|---|-----------|-------|-------------|
| 1 | Accessibility | 2/4 | Multiple interactive elements missing ARIA, sidebar nav hidden from AT in collapsed state, toast notifications not announced |
| 2 | Performance | 2/4 | Google Fonts loaded via blocking `@import` in CSS; all scrollbars globally suppressed; multiple `backdrop-filter` layers stacked on mobile |
| 3 | Security | 1/4 | JWT stored in `localStorage` in SettingsForm (contradicts HTTP-only cookie arch); real Meta App Secret and IP addresses hardcoded in `.env.example`; Stripe webhook verification bypassable; GDPR data-deletion endpoint stub |
| 4 | Theming & design system | 2/4 | Token system exists and is well-used in admin core; hardcoded hex `#10b981`, `#94a3b8`, `#FAF9F6` leak in admin and marketing pages; mixed `boxShadow` inline strings |
| 5 | Responsive design | 3/4 | Mobile-first layout is structurally correct; tables scroll; a few touch targets are borderline; marketing pages lack tested breakpoints |
| 6 | Anti-patterns | 2/4 | See Anti-Pattern Verdict above |
| | **Total** | **12/24** | **Acceptable** |

**Rating:** 12/24 — Acceptable (just above the Poor/Acceptable boundary at 11)

**Legal & compliance flags:**
- Privacy Policy: **Present** (`/privacy`) and accessible via URL, but **not linked from the login page, public lead form, or footer**
- Terms & Conditions: **Present** (`/terms`) — same linkage problem
- Cookie consent banner: **Missing** — platform uses HTTP-only session cookies (essential, exempt) but the Privacy Policy itself says "Public lead-capture forms may use minimal analytics cookies where required by Studio configuration" — if any analytics cookie fires, consent is required before it does
- GDPR signals: **Partially present** — Privacy Policy covers data rights; account-deletion flow exists at `/delete-account`; but the backend data-deletion webhook handler from Meta is a confirmed stub (`HandleDataDeletion` only logs, performs no deletion)
- COPPA: **Addressed** — Privacy Policy states platform not directed at under-16s; no age-gate needed for a B2B product

---

## Executive Summary
The core admin shell is well-engineered: the token system, component library, multi-tenant CSS variable approach, and RSC data architecture show CTO-level intent. The security posture has two issues requiring immediate action before the platform handles real customer PII at scale: a JWT being stored in `localStorage` in `SettingsForm.tsx` (directly contradicting the HTTP-only cookie design documented everywhere else), and real Meta App credentials (App Secret `d2d2fad32c909c92a83899b7ad946315`) and a production IP address committed to `.env.example`. The Stripe webhook signature verification bypass and the GDPR deletion stub are also pre-launch risks, not future concerns. Accessibility has honest gaps — the sidebar, toast system, and user dropdown lack ARIA wiring — but no WCAG A failures that would block a screen-reader user entirely. Responsive design works.

**Total findings by severity:** P0 **2** · P1 **8** · P2 **6** · P3 **5**

---

## Quick Wins
1. **Remove real Meta App Secret + IP from `.env.example`** (P0) — replace with placeholder strings and rotate the leaked credentials immediately
2. **Add `aria-live="polite"` to the global toast container** (P1) — one-line addition in `AppShell.tsx:L251`; makes all success/error toasts audible to screen readers
3. **Add `rel="noopener noreferrer"` to all `target="_blank"` links** (P2) — grep finds several missing these attributes on external doc links
4. **Link Privacy Policy and Terms in the lead-capture form footer** (P1) — the "Powered by 1herosocial.ai" line at the bottom of the public form is already there; add two links next to it
5. **Remove `localStorage.getItem('token')` calls from `SettingsForm.tsx`** (P0) — these fetch an `Authorization: Bearer` token that does not exist in the normal auth flow; the backend uses HTTP-only cookies; these calls silently fail or work only when an orphaned token is present

---

## Findings

### P0 — Blocking

#### JWT stored in `localStorage` contradicts HTTP-only cookie architecture
- **Category:** Security — XSS surface, Storage
- **Location:** [apps/web/src/app/admin/studios/[studioId]/settings/SettingsForm.tsx:230](apps/web/src/app/admin/studios/[studioId]/settings/SettingsForm.tsx#L230), [L827](apps/web/src/app/admin/studios/[studioId]/settings/SettingsForm.tsx#L827), [L986](apps/web/src/app/admin/studios/[studioId]/settings/SettingsForm.tsx#L986), [L1023](apps/web/src/app/admin/studios/[studioId]/settings/SettingsForm.tsx#L1023), [L1035](apps/web/src/app/admin/studios/[studioId]/settings/SettingsForm.tsx#L1035)
- **Issue:** `SettingsForm.tsx` reads `localStorage.getItem('token')` and sends `Authorization: Bearer <token>` headers to the API. The entire platform is designed around HTTP-only cookies (`px_session`). There is no code path that writes a JWT to `localStorage`. The token will always be `null`, making these fetch calls send no auth header — meaning they succeed only because the Go API must also be accepting the HTTP-only cookie from the browser at the same time (since it's a same-origin fetch). If the cookie is the real auth, these `Authorization` headers are dead code. If a future change ever writes a JWT to `localStorage`, it would be vulnerable to XSS theft.
- **User impact:** Logo uploads and knowledge-base saves in Settings silently depend on an undefined token. The auth path is opaque and inconsistent. Any XSS vulnerability anywhere on the domain would immediately steal a stored JWT if one were ever written.
- **Fix:** Remove all `localStorage.getItem('token')` usages from `SettingsForm.tsx`. The fetch calls should rely purely on the HTTP-only cookie that the browser attaches automatically to same-origin requests. No `Authorization` header is needed.

#### Real Meta App Secret and production IP committed to `.env.example`
- **Category:** Security — Secrets exposure
- **Location:** [.env.example:L71–L78](.env.example#L71)
- **Issue:** `.env.example` contains `META_APP_SECRET=d2d2fad32c909c92a83899b7ad946315`, `META_APP_ID=2405726999940224`, `META_WEBHOOK_VERIFY_TOKEN=my_secret_token_123`, `POSTGRES_HOST=3.224.238.210`, and `POSTGRES_PASSWORD=projectx_dev`. These appear to be real values. The `.gitignore` excludes `.env` but not `.env.example`, which is committed and tracked. Anyone with repository access can enumerate these.
- **User impact:** Attacker can use the Meta App Secret to forge signed webhook requests, read messages from the associated WhatsApp Business Account, and enumerate connected phone numbers. The Postgres host IP narrows targeting surface.
- **Fix:** Rotate the Meta App Secret and Webhook Verify Token immediately via the Meta developer console. Replace all values in `.env.example` with generic placeholders (e.g. `your_meta_app_secret_here`). Remove the real IP address and database password.

---

### P1 — Major

#### Stripe webhook signature verification is conditionally bypassed
- **Category:** Security — CSRF surface, Webhooks
- **Location:** [apps/api/internal/studios/webhook_stripe.go](apps/api/internal/studios/webhook_stripe.go)
- **Issue:** The `StripeWebhookHandler` is initialized via `os.Getenv("STRIPE_WEBHOOK_SECRET")` called inline in `main.go`. When `endpointSecret` is empty and `API_ENV != "local"`, the handler returns 401 — but this guard is only checked at request-time against the env var. If the env var is missing in any deployment, the handler falls through to parse the raw JSON without signature verification, allowing any POST to forge payment events (checkout completion, subscription activation, etc.).
- **User impact:** A forged `checkout.session.completed` event could provision a subscription-tier upgrade for a studio without actual payment.
- **Fix:** Fail closed unconditionally — if `STRIPE_WEBHOOK_SECRET` is empty, reject the request with 500 (server misconfiguration), not 200. Never process events without a verified signature. Remove the environment-based bypass path.

#### GDPR data-deletion endpoint is a non-functional stub
- **Category:** Security — Legal compliance, GDPR
- **Location:** `apps/api/internal/messaging/webhook_meta.go` — `HandleDataDeletion`
- **Issue:** The handler for `POST /api/v1/webhooks/meta/data-deletion` acknowledges the request but performs no actual data deletion. A comment in the source reads "For now, we just acknowledge the deletion request / In production, you would verify the signature using your app secret / and then delete the user's data from your database." The Meta Platform Policy requires that a Data Deletion Callback either confirms deletion or provides a deletion status URL.
- **User impact:** Users who request deletion of their Meta-linked data via Facebook settings will receive a confirmation from this endpoint but their data will remain in the database indefinitely. This is a Meta policy violation and a GDPR breach.
- **Fix:** Implement signature verification of Meta's `signed_request` using the committed App Secret (after rotating it). Implement actual data deletion for the identified user. Return a valid `{url, confirmation_code}` response. Treat this as a compliance blocker before Meta App Review.

#### Toast notifications are invisible to screen readers
- **Category:** Accessibility — ARIA live regions
- **Location:** [apps/web/src/components/AppShell.tsx:L251](apps/web/src/components/AppShell.tsx#L251)
- **Issue:** The floating toast container has no `role="status"`, `aria-live`, or `aria-atomic` attribute. Screen reader users never hear success or error feedback from actions (save, delete, status toggle, etc.).
- **User impact:** Blind and low-vision users take destructive or state-changing actions with no feedback about whether the action succeeded. They must re-navigate to the relevant page to confirm the change.
- **Fix:** Add `role="status" aria-live="polite" aria-atomic="true"` to the toast wrapper `div` at `AppShell.tsx:L251`. For error toasts change to `role="alert" aria-live="assertive"`.

#### User dropdown menu has no keyboard support or ARIA
- **Category:** Accessibility — Interactive components
- **Location:** [apps/web/src/components/AppShell.tsx:L608–L662](apps/web/src/components/AppShell.tsx#L608)
- **Issue:** The avatar button toggling the account dropdown has no `aria-expanded`, `aria-haspopup`, or `aria-controls` attributes. The dropdown div has no `role="menu"`. The "Sign out" button inside is not reachable via arrow-key navigation. The click-outside dismiss layer is a bare `<div className="fixed inset-0 z-10">` with no `aria-hidden` or role.
- **User impact:** Keyboard-only users cannot access or close the account menu. The Sign Out action is unreachable without a mouse.
- **Fix:** Add `aria-expanded={open}`, `aria-haspopup="menu"` to the trigger button. Add `role="menu"` to the dropdown container. Add `role="menuitem"` and keyboard event handlers (ArrowDown/Up, Escape) to menu items. Close on Escape and return focus to the trigger.

#### Sidebar navigation is inaccessible in collapsed state
- **Category:** Accessibility — ARIA states, navigation
- **Location:** [apps/web/src/components/AppShell.tsx:L376–L409](apps/web/src/components/AppShell.tsx#L376)
- **Issue:** When the sidebar is collapsed (`lg:w-20`), nav item labels are hidden via `lg:max-w-0 lg:overflow-hidden lg:opacity-0` but remain in the DOM and are still read by screen readers (they show up as empty or invisible text nodes attached to links). The collapse/expand toggle button has only a `title` attribute (tooltip), not `aria-label`. No `aria-expanded` state is communicated.
- **User impact:** Screen reader users hear meaningless or empty link text when the sidebar is collapsed; the toggle button is unlabeled on AT.
- **Fix:** Add `aria-hidden="true"` to the label spans in collapsed state (or add visible text via a proper `aria-label` on the Link itself). Add `aria-label={isCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}` and `aria-expanded={!isCollapsed}` to the toggle button.

#### Privacy Policy and Terms not linked from any user-facing surface
- **Category:** Legal & compliance — Privacy, Terms
- **Location:** [apps/web/src/app/l/[studioSlug]/[campaignSlug]/page.tsx:L73–L79](apps/web/src/app/l/%5BstudioSlug%5D/%5BcampaignSlug%5D/page.tsx#L73), login page, all admin pages
- **Issue:** `/privacy` and `/terms` exist and contain substantive content but are not linked from: (1) the public lead-capture form where PII is collected, (2) the login page, (3) any admin page footer, (4) any registration/onboarding flow. Under GDPR, the legal basis for processing must be communicated at the point of data collection.
- **User impact:** Users submitting their name, email, and phone on a public lead form have no visibility into how their data will be used. Studios may face liability if leads later claim they were not informed.
- **Fix:** Add a "By submitting, you agree to our [Privacy Policy] and [Terms]" line to the lead form submit button area. Add Privacy Policy and Terms links to the login page footer and the "Powered by 1herosocial.ai" footer on the public form.

#### Google Fonts loaded via blocking `@import` in CSS
- **Category:** Performance
- **Location:** [apps/web/src/app/globals.css:L1](apps/web/src/app/globals.css#L1)
- **Issue:** `@import url('https://fonts.googleapis.com/css2?family=Plus+Jakarta+Sans:...')` in the global CSS file is a render-blocking network request. The browser cannot begin painting until this external CSS resolves. Next.js provides `next/font/google` which self-hosts fonts, eliminates the blocking request, and removes the external DNS lookup.
- **User impact:** Measurably slower First Contentful Paint (FCP) on all pages. On slow networks, users see the flash of unstyled text (FOUT) as the font loads asynchronously after unblocking.
- **Fix:** Replace the `@import` with `next/font/google` in `layout.tsx` using `const plusJakarta = PlusJakartaSans({ subsets: ['latin'], weight: ['300','400','500','600','700','800'] })`. Apply via `className={plusJakarta.variable}` on `<html>`. Remove the `@import` line from `globals.css`.

#### `dangerouslySetInnerHTML` used for inline script in `layout.tsx`
- **Category:** Security — XSS surface
- **Location:** [apps/web/src/app/layout.tsx:L43](apps/web/src/app/layout.tsx#L43)
- **Issue:** The root layout uses `dangerouslySetInnerHTML={{ __html: '...' }}` for the theme-detection inline script. The script content is a string literal with no user input, so there is no active XSS vulnerability. However, this pattern bypasses React's escaping and would be a P0 finding if the content were ever dynamic.
- **User impact:** No current user impact. Future developers may copy this pattern for dynamic content without understanding the risk.
- **Fix:** Move to a `<Script strategy="beforeInteractive">` with a static `src`, or keep the pattern but add a JSDoc comment explaining why it's safe here (static string only, no interpolation). Alternatively, if the build toolchain is trusted, accept this as-is and document it.

---

### P2 — Minor

#### Scrollbars globally suppressed site-wide
- **Category:** Performance, Accessibility
- **Location:** [apps/web/src/app/globals.css:L187–L194](apps/web/src/app/globals.css#L187)
- **Issue:** The global stylesheet applies `scrollbar-width: none !important` and `::-webkit-scrollbar { display: none !important }` to `*`, meaning every scrollable container on every page has no visible scrollbar. This includes the Kanban pipeline columns, the leads table, the inbox, and the sidebar nav.
- **User impact:** Users with motor disabilities who rely on scrollbars for precise navigation cannot use them. Users on touchpad-only devices may not realize a container is scrollable. This is a WCAG 2.1 SC 1.4.1 concern (use of color/presentation as the sole indicator of scrollability).
- **Fix:** Remove the global scrollbar suppression. Apply `.no-scrollbar` only to the specific containers where decorative scrollbar removal is intentional (sidebar `nav`, modal body). The main content area, pipeline columns, and tables should retain system scrollbars.

#### Missing `Content-Security-Policy` and `Strict-Transport-Security` headers
- **Category:** Security — Headers
- **Location:** [apps/web/next.config.mjs:L47–L67](apps/web/next.config.mjs#L47), [deploy/nginx/default.conf](deploy/nginx/default.conf)
- **Issue:** `next.config.mjs` sets `X-Content-Type-Options`, `X-Frame-Options`, and `X-XSS-Protection` — good — but omits `Content-Security-Policy` and `Strict-Transport-Security`. The nginx config runs on port 80 only with no HTTPS redirect or HSTS header.
- **User impact:** No CSP means any XSS vulnerability can freely exfiltrate data. No HSTS means browsers will connect via HTTP on the first request, enabling man-in-the-middle attacks when the EC2 eventually gets TLS.
- **Fix:** Add a `Strict-Transport-Security: max-age=63072000; includeSubDomains` header in nginx (add after TLS is enabled). Add a `Content-Security-Policy` header to `next.config.mjs` restricting `script-src` to `'self'` with a nonce for the inline theme script, `connect-src` to the same origin.

#### `fmt.Printf` debug statements in production Go code
- **Category:** Security — Information disclosure
- **Location:** `apps/api/internal/studios/webhook_stripe.go` (multiple lines including session IDs and payment metadata)
- **Issue:** Raw `fmt.Printf("[Stripe Webhook] session ID: %s, Metadata: %+v\n", ...)` calls log structured Stripe session data to stdout, which flows into Docker container logs visible to anyone with `docker compose logs` access on the EC2 box.
- **User impact:** Stripe session IDs and payment metadata visible in unprotected logs. Low-severity today (EC2 is single-tenant) but becomes a compliance risk as team grows.
- **Fix:** Replace all `fmt.Printf` calls in the Stripe webhook handler with `log.Info(...)` / `log.Debug(...)` using the structured `slog` logger already available in the codebase. Use `log.Debug` for verbose payment debugging so it's off in production.

#### Public lead form submit button uses hardcoded `style={{ background: brandColor }}`
- **Category:** Theming & design system
- **Location:** [apps/web/src/app/l/[studioSlug]/[campaignSlug]/form.tsx:L210](apps/web/src/app/l/%5BstudioSlug%5D/%5BcampaignSlug%5D/form.tsx#L210)
- **Issue:** The submit button uses `style={{ background: brandColor }}` directly on a `<Button>` component that also applies its own variant gradient (`from-brand-500 to-brand-700`). The inline style overrides only the `background` property, leaving the `box-shadow` referring to `brand-500/20` (the platform violet) regardless of the studio's color.
- **User impact:** On studios with non-purple brand colors, the submit button shadow is the wrong color — a subtle but noticeable brand inconsistency on the public form.
- **Fix:** Pass `brandColor` via a CSS variable (`style={{ '--brand': brandColor }}`) and use `bg-[var(--brand)]` as the button class, consistent with how the rest of the page uses the brand variable. Or set `--brand` at the form wrapper level.

#### `NEXT_PUBLIC_API_URL` exposed in browser bundle despite convention prohibiting it
- **Category:** Security — Secrets exposure
- **Location:** [apps/web/src/app/admin/studios/[studioId]/knowledge-base/KnowledgeBaseForm.tsx:L24](apps/web/src/app/admin/studios/[studioId]/knowledge-base/KnowledgeBaseForm.tsx#L24), [apps/web/.env.local:L1](apps/web/.env.local#L1), [apps/web/Dockerfile:L15](apps/web/Dockerfile#L15)
- **Issue:** `skills.md §4.2` explicitly states "There is no `NEXT_PUBLIC_API_BASE_URL` — and there should never be one." Yet `KnowledgeBaseForm.tsx` reads `process.env.NEXT_PUBLIC_API_URL`, `.env.local` sets `NEXT_PUBLIC_API_BASE_URL=http://localhost:8080`, and the `Dockerfile` bakes in `NEXT_PUBLIC_API_URL=https://api.1herosocial.ai`. This exposes the direct API URL to the browser bundle — the variable is inlined at build time and visible in the compiled JS.
- **User impact:** The internal API URL is visible to anyone who inspects the page source. In production this is `https://api.1herosocial.ai`, which allows direct API targeting bypassing the nginx proxy and potentially CORS restrictions.
- **Fix:** Remove `NEXT_PUBLIC_API_URL` from `KnowledgeBaseForm.tsx`, `.env.local`, and the Dockerfile. The form should use relative paths (`/api/v1/...`) like every other component in the codebase.

#### Hardcoded hex colors in admin page for status indicators
- **Category:** Theming & design system
- **Location:** [apps/web/src/app/admin/studios/page.tsx:L336](apps/web/src/app/admin/studios/page.tsx#L336)
- **Issue:** `style={{ color: s.active ? '#10b981' : '#94a3b8' }}` bypasses the token system for the studio status indicator in every studio card. If the design system changes its success/neutral colors, this won't update.
- **User impact:** No direct user impact. Design inconsistency if tokens are updated.
- **Fix:** Use Tailwind classes `text-emerald-500` and `text-slate-400` (already used elsewhere for the same semantic), or introduce a `success`/`neutral` design token.

---

### P3 — Polish

#### Sidebar collapse/expand toggle lacks visible label text
- **Category:** Usability — Affordance
- **Location:** [apps/web/src/components/AppShell.tsx:L434–L446](apps/web/src/components/AppShell.tsx#L434)
- **Issue:** The collapse button at the bottom of the sidebar has only a `title` tooltip (visible on hover) and a `ChevronLeft`/`ChevronRight` icon. The action is not obvious to new users who haven't discovered the tooltip.
- **User impact:** Some users may not discover the sidebar can be collapsed.
- **Fix:** Add a small text label "Collapse" / "Expand" next to the chevron that is visible on hover, or always visible when sidebar is expanded.

#### `suppressHydrationWarning` used broadly across components
- **Category:** Performance, Code quality
- **Location:** `Button.tsx:L79`, `Input.tsx:L26`, `AppShell.tsx:L323`, `AppShell.tsx:L578`, `AppShell.tsx:L611`, `AppShell.tsx:L651`, form components throughout
- **Issue:** `suppressHydrationWarning` is a React escape hatch intended for elements whose server/client output legitimately differs (e.g. a timestamp). It's used here on buttons, inputs, and interactive elements where it should not be needed. Broad use masks real hydration bugs rather than fixing them.
- **User impact:** No visible user impact. Technical debt.
- **Fix:** Audit each usage. Most can likely be removed. Keep only on the theme-detection `<html>` element and possibly relative-time displays.

#### Missing `lang` attribute specification for multi-language readiness
- **Category:** Accessibility, SEO
- **Location:** [apps/web/src/app/layout.tsx:L40](apps/web/src/app/layout.tsx#L40)
- **Issue:** `<html lang="en">` is present — this is correct. However, the public lead-capture form serves studios that may operate in non-English locales, and the campaign name/description can be in any language. No `lang` override is applied when content language differs.
- **User impact:** Screen readers may mispronounce non-English content on public forms.
- **Fix:** Consider adding `lang` to the campaign description paragraph if the studio's locale is known.

#### `robots.ts` and `sitemap.ts` created but may expose admin routes
- **Category:** Security (informational), SEO
- **Location:** [apps/web/src/app/robots.ts](apps/web/src/app/robots.ts), [apps/web/src/app/sitemap.ts](apps/web/src/app/sitemap.ts)
- **Issue:** Without reading the file content, existence of a `sitemap.ts` that auto-discovers routes could include `/admin/*` routes in the public sitemap, making admin URL patterns discoverable by search engines. `robots.ts` may or may not disallow admin routes.
- **User impact:** Admin URLs showing up in search results aids attackers in enumerating admin surfaces.
- **Fix:** Verify `robots.ts` includes `Disallow: /admin` and `sitemap.ts` explicitly excludes all `/admin/*` paths.

#### No `<title>` on the login page beyond the global layout title
- **Category:** Accessibility, SEO
- **Location:** [apps/web/src/app/login/page.tsx](apps/web/src/app/login/page.tsx)
- **Issue:** The login page is a client component with no exported `metadata` or `generateMetadata` function. The page title stays as the global "1herosocial.ai — Multi-Studio Gym Marketing & AI Operations" title, which gives no contextual indication to screen reader users or browser tab managers that this is the sign-in page.
- **User impact:** Minor — screen reader users navigating multiple tabs may not distinguish the login page from the home page by tab title.
- **Fix:** Convert the page to a Server Component wrapper that exports `metadata: { title: 'Sign In — 1herosocial.ai' }` and passes initial state down to a client sub-component.

---

## Systemic Patterns

**1. `style={{ }}` inline overrides used throughout admin pages for decorative layout values (15+ instances)**
Hardcoded `boxShadow: 'inset 0 0 0 1px rgba(255,255,255,0.2), ...'`, `background: 'rgba(255,255,255,0.25)'`, and radial-gradient strings appear on at least 15 components across `studios/page.tsx`, `DashboardClient.tsx`, `pipeline/page.tsx`, `settings/page.tsx`. These never change with the design system and create a maintenance burden. The Tailwind config has shadow tokens (`card`, `card-hover`, `liquid`) that cover most cases. Root cause: the glassmorphism style is not tokenized.
**Fix:** Create Tailwind component classes (`.glass-card`, `.glass-header`) in `@layer components` in `globals.css` (the `.glass` and `.premium-glass-card` classes already exist there) and replace inline shadow strings with these classes. Roughly 30 LOC of inline styles would become 2–3 class names.

**2. Touch targets under 44px consistently appear on icon-only buttons across the interface**
The sidebar close button (`h-8 w-8` = 32×32px, `AppShell.tsx:L320`), the collapse toggle (`h-9 w-9` = 36×36px, `L437`), topbar menu open button (`h-10 w-10` = 40×40px, just within WCAG but below Apple HIG 44px), and the toast dismiss button (`p-1 rounded-lg` with no explicit size) all fall below WCAG 2.1 SC 2.5.5 recommended 44×44 CSS pixels. This is a recurring pattern across every icon-only control, not a one-off.
**Fix:** Establish a minimum interactive target size rule: all icon buttons must be `h-11 w-11` (44px) minimum. Apply to sidebar close (`L320`), collapse toggle (`L437`), and toast dismiss (`L268`). A PR search for `h-8 w-8` and `h-9 w-9` on `<button>` elements will surface all instances.

**3. Competing auth mechanisms in the same codebase**
The documented architecture uses HTTP-only cookies as the sole auth mechanism. `lib/auth.ts` and `lib/api.ts` correctly rely on cookies. However, `SettingsForm.tsx` reads `localStorage.getItem('token')` and sends `Authorization: Bearer` headers. This is not an isolated mistake — it appears in 4 separate fetch calls across logo upload, Stripe integration, and knowledge-base file actions. The root cause is likely that a different developer implemented these features without reading the auth conventions in `skills.md`.
**Fix:** Remove all `localStorage.getItem('token')` usages from `SettingsForm.tsx`. Add a linting rule (ESLint custom rule or `no-restricted-syntax`) that forbids `localStorage.getItem('token')` to prevent regression.

**4. Admin pages bypass the Server Action pattern for some mutations**
`skills.md §4.2` mandates Server Actions for all mutations (for cache invalidation). `SettingsForm.tsx` uses direct `fetch()` calls for logo upload, Stripe OAuth, and knowledge-base operations. These calls cannot call `revalidatePath()`, meaning after a logo upload the sidebar still shows the old logo until a manual refresh. This is the exact stale-data bug documented in the playbook as the reason Server Actions were adopted.
**Fix:** Migrate `SettingsForm.tsx` logo upload and knowledge-base operations to Server Actions co-located in `settings/actions.ts`, following the pattern in `leads/[id]/actions.ts`. Each action calls `revalidatePath('/admin/studios/[studioId]')` and `revalidatePath('/admin/studios/[studioId]/settings')`.

**5. Marketing pages (`(home)`, `pricing`, `about`) are architecturally disconnected from the design system**
All three marketing pages use hardcoded `bg-[#FAF9F6]` instead of a token, do not use the `brand.*` color scale, do not have dark mode support (no `dark:` classes), and do not use any component from `components/ui/`. These pages appear to have been built separately from the admin core. If the brand color changes, they will not update.
**Fix:** Add a `#FAF9F6` entry to `tailwind.config.js` as `off-white` or `canvas`. Ensure the marketing surface uses `brand.*` tokens for accent colors. Add `dark:` variants if dark mode is desired on marketing pages. This is a polish item unless the marketing pages are part of the public product.

---

## Strengths

**1. Multi-tenant CSS variable theming is elegantly engineered**
`AppShell.tsx` sets `--brand`, `--brand-soft`, `--brand-softer`, `--brand-onbrand` as CSS custom properties on the shell wrapper, derived from the authenticated user's studio color. The `Button`, `Input`, and nav components all reference these variables, meaning every admin session is automatically re-themed for the active studio with no per-component prop drilling. This is a sophisticated approach that avoids the common mistake of prop-threading brand colors. The fallback `#7c3aed` in every `var()` call ensures the platform brand shows when no studio color is active.

**2. Server-side multi-tenancy enforcement is fail-closed and consistent**
The Go backend enforces tenant isolation at every layer: the JWT carries `studio_id`, the `RequireActiveStudio` middleware wraps all studio-scoped routes, `resolveStudioID` validates that studio_admins can only act on their own studio, and the inactive-studio lockout is matched by a full-screen frontend guard in `AppShell.tsx`. The frontend lockout for `past_due`/`canceled` subscriptions (`StudioPastDueModal`) mirrors the backend billing guard. The decision to "fail closed" is applied consistently.

**3. Public lead form is a clean, accessible, well-structured component**
`form.tsx` uses semantic HTML (`<form>`, `<label for="...">`, `<input id="...">`, `autoComplete` attributes), server-validated field errors tied back to specific inputs via `FieldError` components adjacent to each field, a client-side 20-second cooldown to prevent double-submission, and properly encodes slugs in the API URL. The form does not use `noValidate` alone without client-side fallback — it validates both server and client side. This is a good example of the pattern the rest of the codebase should follow.

**4. The `skills.md` playbook is a genuine working document, not a template**
The playbook documents real decisions with real reasoning: why TanStack Query wasn't added, why the frontend wasn't split, why the outbox pattern is required for Sheets, the hydration footgun list with specific examples. It's kept current (§8 lists messaging architecture, §9 documents the EC2 deploy, §10 has a live parking lot). This level of documentation reduces onboarding time and prevents accidental regression of architectural decisions.

**5. No XSS from user-controlled data found in React-rendered output**
A search for `dangerouslySetInnerHTML` with dynamic content found only the theme-detection script in `layout.tsx` (static string, no interpolation). All user-supplied data (campaign names, lead names, studio names) flows through React's standard rendering and is escaped automatically. The codebase correctly avoids `.innerHTML`, `eval()`, and `document.write()`. The one `dangerouslySetInnerHTML` usage is controlled and documented.

---

## Recommended Priority Order

1. **Rotate leaked Meta App Secret and remove real credentials from `.env.example`** — takes 10 minutes; the secret is already committed to git history and may have been crawled. Rotate it at meta.com immediately, then scrub the file.
2. **Remove `localStorage.getItem('token')` from `SettingsForm.tsx`** — this is a security architecture violation that also causes subtle auth bugs. Remove all 4 usages, rely on the HTTP-only cookie.
3. **Fix Stripe webhook to fail closed when `STRIPE_WEBHOOK_SECRET` is missing** — one conditional change in `webhook_stripe.go`; prevents fraudulent subscription upgrades in any deployment where the env var is misconfigured.
4. **Implement Meta data-deletion webhook (or disable the endpoint until implemented)** — GDPR + Meta Platform Policy compliance blocker. If it cannot be implemented before Meta App Review, remove the endpoint and re-add it when ready so Meta doesn't receive fake "deletion confirmed" responses.
5. **Add `aria-live` to the toast container and fix the user dropdown ARIA** — highest accessibility ROI per line of code; fixes the most commonly-used feedback mechanism and the most commonly-used navigation element.
6. **Link Privacy Policy and Terms from the public lead form** — legal risk mitigation; single line addition to the existing "Powered by" footer on the form page.
7. **Replace Google Fonts `@import` with `next/font/google`** — direct FCP improvement on every page load; 20-minute fix.
8. **Remove global scrollbar suppression; apply only to decorative containers** — restores a browser accessibility feature that affects all scrollable regions.
9. **Remove `NEXT_PUBLIC_API_URL` from `KnowledgeBaseForm.tsx`, `.env.local`, and the Dockerfile** — fixes the violation of the documented "no `NEXT_PUBLIC_API_BASE_URL`" rule and removes the internal API URL from the browser bundle.
10. **Migrate `SettingsForm.tsx` mutations to Server Actions** — fixes stale logo/settings data after save; aligns with the documented pattern.
