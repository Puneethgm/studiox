# Telegram QR-login (personal account) — setup runbook

This is the second Telegram integration — see [`SETUP_TELEGRAM.md`](SETUP_TELEGRAM.md)
for the bot-token one first if you haven't already, and read `docs/skills.md`
§8 for how the two differ. This one links a real personal or business
Telegram account by scanning a QR code (like Telegram Desktop's own login),
so the studio's existing chats and contacts show up in the Inbox — a bot
can't do that.

Architecturally this is the Telegram equivalent of the `whatsapp_web`
channel: a small Node microservice (`apps/tg-web`, using `teleproto` — the
maintained fork of GramJS) holds the MTProto session, and the Go API proxies
to it. See `docs/skills.md` §8 for the invariants.

---

## Part 1 — Platform owner: one-time setup

### 1. Get API credentials

Unlike the bot integration, QR login needs a platform-wide **API ID + API
hash** (this is how Telegram's MTProto protocol identifies which app is
connecting, separate from any individual account):

1. Go to <https://my.telegram.org/auth>, log in with any Telegram-registered
   phone number (this is just to create the app credentials — it is **not**
   the account that will show up in any studio's inbox).
2. **API development tools** → fill in an app name/short name (anything) →
   **Create application**.
3. Copy `api_id` and `api_hash`.

### 2. Add to `.env` (or `deploy/.env` on the EC2 box)

```bash
TELEGRAM_API_ID=<api_id from step 1>
TELEGRAM_API_HASH=<api_hash from step 1>

# Reuses the same encryption key and internal-service key as everything else:
# TOKEN_ENCRYPTION_KEY, INTERNAL_API_KEY — already set if wa-web is running.
```

In production this also needs the `tg-web` container running — it's already
wired into `deploy/docker-compose.yml`. Redeploy: `bash deploy/deploy.sh`.

### 3. Local development

```bash
# One-time
cd apps/tg-web && npm install

# Run alongside the API (separate terminal — NOT started by `make dev`,
# since most contributors won't have TELEGRAM_API_ID/HASH set and a
# crash-looping tg-web would be noisy for everyone else):
make tg-web
```

---

## Part 2 — Per studio: connect via QR

1. In the studio's admin panel, go to **Channels → Telegram (QR)**.
2. Click **Show QR Code**.
3. On the phone with the Telegram account you want to link: **Settings →
   Devices → Link Desktop Device** → scan the code.
4. If the account has two-factor authentication (a "cloud password") turned
   on, you'll be prompted for it right in the same panel — Telegram's QR
   login flow requires it as a second step, same as it would linking a real
   desktop client.
5. Once connected, optionally click **Import chat history** to pull in
   recent direct-message conversations (group chats are skipped — this
   channel is for 1:1 customer conversations, matching how the inbox is
   scoped everywhere else).

### What's different from the bot channel

| | Bot (`telegram`) | QR-linked (`telegram_mtproto`) |
|---|---|---|
| Setup | Paste a BotFather token | Scan a QR code |
| Customers reach you via | `t.me/<bot_username>` | Your existing phone number / username |
| Sees existing chat history | No | Yes (via "Import chat history") |
| Messaging window | None — send anytime | None — send anytime |
| Platform-wide credential needed | No | Yes (`TELEGRAM_API_ID`/`HASH`, one-time) |

Both can be connected on the same studio simultaneously if useful (e.g. a
bot for automated/marketing-style messages, the QR account for the number
customers already know).

### Troubleshooting: "Show QR Code" just says "Starting Telegram session…" forever

This means tg-web is stuck (or was stuck, before it timed out) trying to
establish the raw MTProto connection to Telegram's servers — this is a
different, lower-level connection than normal HTTPS, so it can fail even
when everything else about the deploy looks fine. `_startSession` in
`apps/tg-web/src/sessions.js` now times out after 20s and reports a clear
error instead of hanging silently, so:

1. **Check tg-web's own logs first** (`docker compose logs -f tg-web` in
   production, or the terminal running `make tg-web` locally) — it logs
   `tg-web: connecting to Telegram...` right before the handshake and either
   `tg-web: connected to Telegram MTProto layer` or a specific error
   afterward (e.g. `API ID invalid`, a timeout).
2. **Most common cause: `TELEGRAM_API_ID`/`TELEGRAM_API_HASH` are blank,
   swapped, or copy-pasted with extra whitespace.** Re-check them against
   what my.telegram.org shows for your app — `API ID invalid` in the logs
   confirms this directly.
3. **Less common: outbound network restrictions.** Cloud firewalls/security
   groups that only allow standard HTTP(S) egress can block the raw TCP
   connection MTProto needs. If the logs show the 20s timeout firing rather
   than a Telegram-side error, this is the likely cause — check that the
   host tg-web runs on allows outbound traffic to Telegram's datacenter IP
   ranges, not just `api.telegram.org` over HTTPS (that's a different,
   HTTPS-based endpoint the bot integration uses — MTProto doesn't go
   through it).

### Known limitations (v1)

- **No automatic "session revoked" detection.** If the linked account is
  logged out from another Telegram client (or the QR is manually
  unlinked from **Settings → Devices**), tg-web doesn't proactively notice —
  it'll surface as a failed send, or self-heal on the next tg-web restart
  (which re-checks authorization before reporting "connected"). Use the
  **Disconnect** button in the Channels page as the reliable way to unlink.
- **History import is best-effort**, limited to the 50 most recent direct
  chats and 200 messages per chat (same bounds as the WhatsApp Web
  integration, for the same reason — keeping the import fast and the Go
  API's write volume bounded).
- **2FA password is entered once per login**, not stored — if the session
  is later revoked and needs re-linking, the password prompt happens again.
