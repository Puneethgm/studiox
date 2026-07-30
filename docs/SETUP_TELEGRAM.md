# Telegram — setup runbook

Unlike WhatsApp/Instagram/Messenger/X, Telegram has no shared platform app
and no App Review process. Each studio brings its own bot (free, created via
Telegram's @BotFather), and the platform registers a webhook for that bot
automatically when the studio admin connects it via the Channels page.

---

## Part 1 — Platform owner: one-time setup

The only prerequisite is that the API is reachable from the public internet
over HTTPS (Telegram will not deliver webhooks to `localhost` or plain HTTP
in production).

Add to `.env` (or `deploy/.env` on the EC2 box):

```bash
# Domain root, no /api suffix — nginx forwards /api/* through unchanged, so
# this should match whatever origin the browser hits (see PUBLIC_FORM_BASE_URL).
PUBLIC_API_BASE_URL=https://<your-host>
```

Then redeploy: `bash deploy/deploy.sh`.

### Local development

Telegram needs a real HTTPS URL to call, so `localhost:8080` alone won't
receive webhooks. Use a tunnel:

```bash
ngrok http 8080
# copy the https://xxxx.ngrok-free.app URL it prints
```

Set `PUBLIC_API_BASE_URL=https://xxxx.ngrok-free.app` in `.env` and restart
`make api` before connecting a bot locally. Without a tunnel, you can still
exercise everything except real inbound delivery (outbound sends and the
connect flow's `getMe`/`setWebhook` calls work fine over plain internet
access — only Telegram's callback to *your* server needs the tunnel).

---

## Part 2 — Per studio: connect a bot

1. **Create the bot.** In Telegram, message **@BotFather** → send `/newbot`
   → follow the prompts (choose a display name and a `_bot`-suffixed
   username). BotFather replies with a token that looks like
   `123456789:AAExampleTokenFromBotFather`.
2. In the studio's admin panel, go to **Channels → Telegram** and paste the
   token into **Bot Token**. Click **Connect Telegram**.
   - Behind the scenes this calls Telegram's `getMe` to validate the token,
     generates a random per-channel webhook secret, calls `setWebhook` with
     `{PUBLIC_API_BASE_URL}/api/v1/webhooks/telegram/<bot's numeric ID>`, and
     stores the token + secret encrypted (see
     `internal/messaging/channels/telegram.go`).
3. **Test it.** Open the bot's Telegram profile (`t.me/<bot_username>`,
   shown once connected) and send it a message from any Telegram account.
   It should appear in the studio's Inbox within a couple of seconds. Reply
   from the Inbox — it should arrive back in the Telegram app.

### Sharing the bot with customers

There's no "link a phone number" step like WhatsApp — customers reach the
bot by starting a chat with `t.me/<bot_username>` (put this link in bio,
QR code, etc.) or by clicking **Start** if you've shared it directly.

### Notes

- **No 24-hour messaging window** — unlike WhatsApp/Messenger/Instagram,
  Telegram bots can message any user who has ever started a conversation
  with them, at any time, for free. No template-message workaround needed.
- **Disconnecting** a bot (Channels page → Disconnect) does not delete the
  bot itself, only removes our record of it and lets a future `setWebhook`
  call from a re-connect take over cleanly.
- **Rotating the token**: if a bot token is compromised, revoke it via
  BotFather (`/revoke`) and reconnect with the new token — this repeats the
  `setWebhook` registration with a fresh per-channel secret automatically.
