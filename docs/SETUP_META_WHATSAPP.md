# Meta WhatsApp Cloud API — setup runbook

This is the platform-side setup (one-time, done by you as the platform owner)
plus the per-studio setup (done by each studio admin via the Channels page).

Direct Meta integration — no Twilio, no BSP markup. One Meta App on the
platform, many studios bring their own WABA + access token.

---

## Part 1 — Platform owner: one-time Meta App setup

### 1. Create the Meta App
1. Go to <https://developers.facebook.com/apps>.
2. Click **Create App** → choose **Business** → name it (e.g. `1herosocial.ai-prod`).
3. Add a contact email and create.
4. From the App Dashboard, click **Add product** → **WhatsApp** → **Set up**.

You'll land on the WhatsApp → Getting Started page with a free **test phone
number**, a **WABA**, a **Phone Number ID**, and a 24-hour temporary token.
Keep this tab open.

### 2. Capture two app-wide values
From **App Settings → Basic**:
- `META_APP_ID` — the App ID
- `META_APP_SECRET` — click "Show" and copy

### 3. Pick a webhook verify token
Make up a long random string (anything alphanumeric, ~32+ chars). This is the
secret Meta uses during the GET handshake.

```bash
openssl rand -hex 32
```

Save it as `META_WEBHOOK_VERIFY_TOKEN`.

### 4. Configure the webhook in Meta
WhatsApp → Configuration → Webhook:
- **Callback URL**: `https://<your-host>/api/v1/webhooks/meta/whatsapp`
  - For the EC2 box: `http://13.250.175.113/api/v1/webhooks/meta/whatsapp` (Meta accepts HTTP for the test number; HTTPS required for production)
- **Verify Token**: paste the `META_WEBHOOK_VERIFY_TOKEN` from step 3.
- Click **Verify and save**. Meta sends a GET to the URL; our server echoes
  the challenge if the token matches. If it doesn't verify, check that the
  API is running and the URL is reachable from the public internet.

After saving, click **Manage** under "Webhook fields" → subscribe to
**`messages`**. (You can subscribe more later; `messages` covers inbound
messages + delivery statuses.)

### 5. Add these to `.env`
On the EC2 box:

```bash
ssh -i studiox.pem ubuntu@13.250.175.113
cd studiox
nano deploy/.env
```

Append:
```bash
META_APP_ID=<from step 2>
META_APP_SECRET=<from step 2>
META_WEBHOOK_VERIFY_TOKEN=<from step 3>
META_GRAPH_API_VERSION=v21.0

# 32-byte AES key (one-time, do not change later — would un-decryptable
# every studio's stored token).
TOKEN_ENCRYPTION_KEY=<output of `openssl rand -base64 32`>
```

Then redeploy: `bash deploy/deploy.sh`.

---

## Part 2 — Per studio: connect WhatsApp

The studio admin needs three values from their WhatsApp Cloud API setup
(under your Meta App):

| Value | Where |
|---|---|
| **WABA ID** | WhatsApp → API Setup → "WhatsApp Business Account ID" near the top |
| **Phone Number ID** | WhatsApp → API Setup → "From" dropdown, the long numeric id |
| **Access Token** | WhatsApp → API Setup → "Temporary access token" (24h, fine for dev) OR a **Permanent System User token** (production — see below) |

### Permanent System User token (production)
The 24-hour token is fine for the test number. For real studios:
1. Meta Business Settings → System Users → **Add** → name it
   `<studio-name> WhatsApp` → Admin role.
2. Click **Generate new token** → select your Meta App → check
   `whatsapp_business_management` and `whatsapp_business_messaging` →
   Generate.
3. Copy the token (you only see it once).
4. Assign the WABA to the system user: Business Settings → Accounts →
   WhatsApp Accounts → select WABA → **Add People** → pick the system user.

### Connect in the platform
1. Sign in to the platform as the studio admin.
2. Sidebar → **Channels** → **Connect WhatsApp**.
3. Paste the four values (WABA ID, Phone Number ID, display phone, access
   token) → **Connect**.
4. The channel appears with status **Active**. Token is encrypted at rest
   (AES-256-GCM with `TOKEN_ENCRYPTION_KEY`).

### Subscribe the WABA to your app
For the test number this happens automatically. For real WABAs you may need
to subscribe manually:
- Meta Business Manager → Business Settings → Accounts → WhatsApp Accounts
  → select the WABA → **Apps** → confirm your app is subscribed.

This is what tells Meta to send webhooks for that WABA's messages to your
app's webhook URL.

---

## Verifying end-to-end

1. From your phone, send a WhatsApp message to the studio's connected number.
2. Within ~1s the conversation appears in the **Inbox** (no refresh needed
   thanks to SSE).
3. Type a reply → click Send → message goes through `outbound_jobs` → worker
   dispatches to Meta → status ticks update (✓ → ✓✓ → ✓✓ blue when read).

If something doesn't arrive:
- Check the API logs: `docker compose -f deploy/docker-compose.yml logs -f api`
- Look for `meta_webhook` entries on inbound, `messaging_worker` on outbound.
- Common causes:
  - `bad signature` → `META_APP_SECRET` mismatch
  - `verify mismatch` → `META_WEBHOOK_VERIFY_TOKEN` mismatch
  - No webhook fires at all → WABA not subscribed to the app, or webhook URL
    not reachable from Meta's servers (firewall / port 80 closed)

---

## Costs

- **Meta WhatsApp**: free for the first 1000 service conversations per
  WABA per month. Then $0.005–$0.10 per conversation depending on country
  and conversation type. See <https://developers.facebook.com/docs/whatsapp/pricing>.
- **No platform markup** — we go direct, no BSP fees.
- **Test number** is fully free.

---

## What's intentionally NOT here yet

- **Embedded Signup** — Meta's hosted onboarding flow, replaces manual
  WABA + token paste. Phase B work; needs Meta App Review for the relevant
  scopes (`whatsapp_business_management`, `whatsapp_business_messaging`).
- **WhatsApp Templates** — required to message users *outside* the 24-hour
  window. Phase D (automations) work; templates need Meta approval before
  use.
- **Media uploads** (sending images/videos) — phase B. Webhook *receives*
  media metadata today; we just don't render or send media yet.
- **Instagram DMs / FB Messenger** — same Meta Graph API, different
  webhook subscription field. Phase B addition.
