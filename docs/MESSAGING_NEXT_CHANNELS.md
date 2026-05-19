# Messaging — Next Channels (Instagram / Messenger / X)

> **Status:** Phase A (WhatsApp via Meta Cloud API) is shipped. This doc is
> the implementation plan for adding **Instagram DMs**, **Facebook
> Messenger**, and **X (Twitter) DMs**. Pick this up once WhatsApp is
> fully tested in production with at least one real studio.
>
> Read [`SETUP_META_WHATSAPP.md`](SETUP_META_WHATSAPP.md) and §"Messaging" in
> [`skills.md`](skills.md) first — they describe the architecture every
> channel below sits on.

---

## What carries over from WhatsApp (don't rebuild)

The Phase A foundation already supports all 4 channels at the data layer.
**None of the following needs to change** when adding IG / Messenger / X:

- Schema: `channel_accounts.kind` enum already includes
  `whatsapp_meta`, `instagram_meta`, `messenger_meta`, `x_dm`. The
  `contact_identities.kind` enum includes `phone`, `email`, `ig_psid`,
  `fb_psid`, `x_id`.
- The `messages` / `conversations` / `outbound_jobs` tables — channel-agnostic.
- `internal/messaging/service.go` use cases (`HandleInbound...`, `EnqueueReply`,
  `MarkRead`, etc.) — only inbound parsing is per-channel.
- `internal/messaging/worker.go` — switches on `channel.Kind` to pick a
  `Sender`; just add cases.
- `internal/messaging/events.go` events bus — same.
- The Inbox UI (`/admin/studios/[id]/inbox`) — `ChannelAvatar` already
  recognises all four kinds; just wire the data.
- The Channels page tabs (`ChannelTabs.tsx`) — the placeholders for IG,
  Messenger, X already exist. Flip a tab's `status` from `'coming_soon'` to
  `'available'` and provide the connect form.
- Token encryption (`internal/platform/secrets`) — same key, same flow.

**What IS per-channel** (the real work below):
- Webhook payload parser (each platform has its own JSON shape).
- `Sender` implementation (each platform has its own send endpoint + auth).
- Connect form (each platform exposes different ids the studio admin pastes).
- Scope/permission set in Meta App Review (or X Developer Portal).

---

## Instagram DMs

### Scopes / prerequisites
- Already inside our existing Meta App.
- App Review for:
  - `instagram_basic`
  - `instagram_manage_messages`
  - `pages_show_list` (to find the IG account's parent FB Page)
- Each studio's IG account must be **Business or Creator** type and **linked
  to a Facebook Page**. Personal IG accounts cannot use the API. Document
  this clearly on the connect form.
- Webhook subscription field: `messages` on the **`instagram`** webhook
  product (separate subscription from WhatsApp's `messages`).

### Backend work

| File | Change |
|---|---|
| `internal/messaging/channels/meta_instagram.go` (new) | New `MetaInstagram` Sender. POST `/{ig-user-id}/messages` with `recipient: {id: psid}` payload. ~80 lines, mirrors `meta_whatsapp.go`. |
| `internal/messaging/channels/meta_instagram.go` (same file) | Webhook payload parser. Different shape than WhatsApp — see [Messenger Platform webhook reference](https://developers.facebook.com/docs/messenger-platform/webhooks). Top-level `entry[].messaging[]` with `sender.id` (PSID), `recipient.id`, `message.text`, `message.attachments`. |
| `internal/messaging/webhook_meta.go` | Add a second route handler `ReceiveInstagram` that parses IG payload (object: `"instagram"`). Or fold both into one handler that switches on the top-level `object` field — the file already has the verify + signature plumbing. |
| `internal/messaging/service.go` | Add `HandleInboundInstagramMessage(ctx, pageID, psid, ...)`. Identity is `IdentityIGPSID`, value is the PSID. |
| `internal/messaging/worker.go` | Add `case KindInstagramMeta: sender = w.instagram` in the dispatch switch. |
| `cmd/server/main.go` | Construct `channels.NewMetaInstagram(...)`, pass to `NewOutboundWorker`. |

### Frontend work

| File | Change |
|---|---|
| `app/admin/studios/[studioId]/channels/ConnectInstagram.tsx` (new) | Form fields: **IG Business Account ID**, **Page Access Token** (from FB Page). Same encrypted-token flow as WhatsApp. |
| `app/admin/studios/[studioId]/channels/ChannelTabs.tsx` | Flip the `instagram_meta` tab's `status` to `'available'`. Render `<ConnectInstagram />` when `kind === 'instagram_meta'`. |
| `app/admin/studios/[studioId]/inbox/InboxLive.tsx` | Already handles IG via `CHANNEL_BADGE['instagram_meta']`. Nothing to change. |

### Gotchas
- **24-hour rule**: same as WhatsApp — outbound only allowed within 24h of
  the last inbound, OR using a [`HUMAN_AGENT` message tag](https://developers.facebook.com/docs/messenger-platform/send-messages#supported-tags).
- **PSIDs are app-scoped**: the same Instagram user is a different PSID in
  every Meta App. If we ever change apps, identity stitching needs care.
- **Story replies and mentions** arrive as different webhook events
  (`message_reactions`, `messaging_postbacks`, etc.) — we ignore those at
  L1 but log the field so we can revisit.

### Effort
~1.5 days dev. App Review approval is the gating item (1–2 weeks; submit
early).

---

## Facebook Messenger

### Scopes / prerequisites
- Same Meta App. App Review for:
  - `pages_messaging`
  - `pages_show_list`
  - `pages_read_engagement`
- Each studio connects a **Facebook Page** (not a personal profile).
- Webhook field: `messages` on the **`messenger`** product (yet another
  subscription).

### Backend work

The good news: Instagram and Messenger share **almost the same payload
shape and send endpoint**. If you implemented IG already, Messenger is
mostly copy-paste.

| File | Change |
|---|---|
| `internal/messaging/channels/meta_messenger.go` (new) | `MetaMessenger` Sender. POST `/v21.0/me/messages?access_token=...` (or `/{page-id}/messages`) with same recipient/message shape as IG. Likely ~50 lines if extending IG. |
| `internal/messaging/webhook_meta.go` | Handle `object: "page"` payloads. |
| `internal/messaging/service.go` | `HandleInboundMessengerMessage(ctx, pageID, psid, ...)`. Identity is `IdentityFBPSID`. |
| `internal/messaging/worker.go` | `case KindMessengerMeta:`. |

### Frontend work

| File | Change |
|---|---|
| `ConnectMessenger.tsx` (new) | Form: **Facebook Page ID**, **Page Access Token**. |
| `ChannelTabs.tsx` | Flip `messenger_meta` to `'available'`. |

### Gotchas
- Same 24-hour rule + message tags as IG.
- The `messaging_postbacks` event for buttons in messages is potentially
  useful for automations — design the automation engine to consume those
  later. (Not needed for L1 inbox.)
- **One token per Page** — if a studio has multiple FB Pages, that's
  multiple `channel_accounts` rows.

### Effort
~1 day if IG landed first. App Review can be batched with IG (same
submission window).

---

## X (Twitter) DMs

### Prerequisites
- **Separate** developer ecosystem from Meta. New X Developer account,
  new app, new keys.
- [X API pricing](https://docs.x.com/x-api/getting-started/pricing) since Feb
  2026: pay-per-use. Roughly **$0.01 per outbound DM**, **$0.005 per
  inbound read**. No free tier for new dev accounts.
- OAuth 2.0 with PKCE — different auth flow than Meta. Studio admin signs
  in to X to grant scopes (`dm.read`, `dm.write`, `tweet.read`).
- Webhooks via [Account Activity API](https://developer.x.com/en/docs/twitter-api/enterprise/account-activity-api/overview)
  — currently only available on Enterprise/Pro tiers, not Basic. Confirm
  current state before committing.

### Backend work

| File | Change |
|---|---|
| `internal/messaging/channels/x_dm.go` (new) | `XDirectMessages` Sender. POST `/2/dm_conversations/with/:participant_id/messages` with bearer token. ~100 lines (more boilerplate than Meta because of OAuth refresh). |
| `internal/oauth/x.go` (new — outside `messaging/`) | Full OAuth 2.0 PKCE flow. Generate code verifier + challenge, redirect, callback handler that exchanges code for token, stores via `messaging.Repo.CreateChannel`. |
| `internal/messaging/webhook_x.go` (new) | Verify CRC challenge (X uses HMAC of CRC token, different from Meta's signature scheme). Parse Account Activity payload. |
| `internal/messaging/service.go` | `HandleInboundXDM(...)`. Identity is `IdentityXID`. |
| `internal/messaging/worker.go` | `case KindXDM:`. Token refresh logic — X tokens expire in ~2h, refresh tokens last 6 months. Add a refresh path before send. |
| `cmd/server/main.go` | Wire X OAuth routes (`/oauth/x/start`, `/oauth/x/callback`). |

### Frontend work

| File | Change |
|---|---|
| `ConnectX.tsx` (new) | Single button "Connect X" → redirects to `/api/v1/oauth/x/start` → X consent screen → callback. No paste-in-tokens — OAuth handles it. Different UX than the Meta forms. |
| `ChannelTabs.tsx` | Flip `x_dm` to `'available'`. |
| `lib/types.ts` | Possibly add a `tokenExpiresAt` on `ChannelAccount` and surface a "Reconnect" CTA when the refresh token is also expired (~6 months out). |

### Gotchas
- **Cost monitoring**: every send costs money. Add a per-studio rate limit
  + monthly cap (default $10) before turning this on for studios. Otherwise
  one runaway automation = surprise bill.
- **Webhook tier requirement**: confirm Account Activity API is available on
  the tier the platform is paying for before committing to webhooks.
  Polling fallback is unpleasant but possible (`/2/dm_events` every N
  seconds — costs reads, but predictable).
- **Token refresh** is the most fragile part. Schedule a daily job that
  refreshes any token within 24h of expiry; if the refresh fails (user
  revoked), mark the channel as `error` so the inbox flags it.

### Effort
~3–4 days dev, mostly OAuth + token refresh + cost controls.
Substantially more friction than the Meta channels. Worth deferring until
volume justifies it.

---

## Suggested ordering

1. **Validate WhatsApp first** with at least one real studio for a couple
   of weeks. Look for: token-refresh edge cases, webhook reliability,
   inbox UX issues, retry behavior under network blips. Fix the
   foundation before fanning out.
2. **Submit Meta App Review** for `instagram_*` and `pages_messaging`
   scopes in one batch — saves a review cycle.
3. **Ship Instagram + Messenger together** while review is pending. Test
   with a sandbox app/page; flip the live scopes when approval lands.
4. **Defer X** until either:
   - A studio specifically asks for it, AND
   - Per-studio cost cap UX is in place

---

## Definition of done (per channel)

For each channel, the same checklist applies:

- [ ] Sender adapter sends a text message end-to-end against a real account.
- [ ] Webhook parser handles inbound text + at least one media type.
- [ ] Identity stitching round-trips: same person on this channel + WhatsApp
      shows two `contact_identities` rows linked to one `lead`.
- [ ] Inbox UI renders the new channel's avatar badge and treats it
      indistinguishably from WhatsApp threads.
- [ ] ChannelTabs shows correct connect form, count chip, and connected
      account list.
- [ ] Outbound retries with backoff on transient failures; permanent errors
      mark the channel `status='error'` with a useful `last_error`.
- [ ] `docs/SETUP_META_WHATSAPP.md` either gets a new section or a
      sibling doc (e.g., `SETUP_META_INSTAGRAM.md`) describing the
      per-studio onboarding for that channel.
- [ ] `docs/skills.md` updated with any new architectural invariants.

---

## Don't forget when ready

- The **events bus** publishes `message.received` / `message.sent` for
  every channel — automations (phase D) and AI agent (phase E) will see
  them automatically. No per-channel wiring needed in those phases.
- **Templates** (WhatsApp-only concept today) — when adding IG/Messenger
  outside-24h messaging, those use **message tags**, not pre-approved
  templates. The `message_templates` table has a `channel_kinds` array
  for this reason.
- **Cost dashboards**: by the time we have 3+ channels live, add a
  per-studio per-channel message-count chart to the Settings page.
  Reach for the existing `messages` table — `GROUP BY studio_id,
  date_trunc('day', sent_at), studio_kind` does it.
