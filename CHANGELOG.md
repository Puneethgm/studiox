# Changelog

A running record of what's been built, where it actually lives right now, and what's
still needed to get it live. Kept up to date as work lands — check **Status** before
assuming something is already working for your users.

**Status key:**
- 🟢 **Live on EC2** — deployed and actually running for real traffic.
- 🟡 **Committed, not deployed** — safe in git (`git log`), but EC2 hasn't pulled/rebuilt it.
- 🔴 **Local only** — on this machine's disk, not committed. Lost if not pushed.

**As of 2026-10-07, 21:35 IST:**
- Local `HEAD`: `36f9ddb`. Origin `main`: also `36f9ddb` (in sync).
- EC2: `d971fc5` — **3 commits behind origin**, *and* has 9 files hand-patched directly
  on the box (not through git) from earlier same-day fixes. See
  [EC2 sync status](#ec2-sync-status-read-this-before-assuming-something-is-live) below
  before trusting anything is "live" without checking first.

---

## 2026-10-07

### 🔴 AI won't improvise a booking it can't actually do
A reply to our own "1. Book a Trial / 2. Become a Member" menu (just "1") was never
handled deterministically when the conversation had no lead attached — it went
straight to the free-form AI, which invented a fake scheduling conversation ("which
day and time works best?") with nothing behind it. Now intercepted before the AI is
ever called, and routed into the same trial-payment-link flow (see below) — which
escalates to a human if it genuinely can't be processed.
**Files:** `apps/api/internal/messaging/ai_worker.go`, `class_escalation.go`,
`class_escalation_test.go` · The "2. Become a Member" case had the identical gap —
fixed the same day, below.

### 🔴 "2. Become a Member" — the same leadless gap, fixed
Same bug as above, for the other menu option: a leadless conversation replying "2"
also fell through to the free-form AI. Unlike trial, there's no leadless membership
checkout to send (those links are built per-lead), so this escalates to a human
instead of guessing. Building a real leadless membership checkout (a new page not
keyed by lead ID, a new Stripe path) is bigger scope and wasn't attempted.
**Files:** `apps/api/internal/messaging/ai_worker.go`, `class_escalation.go`
(`parseMenuChoice`), `class_escalation_test.go`

### 🔴 Membership checkout now escalates too when it can't actually be processed
Same principle as the trial plan, applied to membership: if a customer picks a
membership plan and either (a) the studio hasn't configured Stripe at all, or
(b) Stripe itself fails to create the checkout session, the old behaviour sent a
generic "our team will reach out" message and left it there — nobody was ever
told. Both cases now escalate to the Escalation tab with the plan name, exactly
like the Trial-plan-inactive and Stripe-not-configured cases already did for
trials.
**Files:** `apps/api/internal/messaging/service.go` (`buildPlanCheckoutBody`
now takes a `convID` and calls the existing `escalateForManualMembership` on
both failure paths), `plan_checkout_escalation_test.go` (NEW)

(Already confirmed working, no change needed: a daily send limit of 48 against
100+ contacts never marks the unsent remainder as "failed" — they just stay
`pending` and get picked up automatically on later days/ticks. Also already
correct: plans that exist in the database but are switched off are excluded by
`ListActivePlans`' `is_active = true` filter exactly like having zero plans,
so that path escalates too.)

### 🔴 Broadcast campaign no longer shows "Completed" before every message is actually sent
Real bug, found while checking the point above: once the *last* recipient in a
big broadcast (say, 100+ contacts against a 48/day limit) got handed an
outbound job, the campaign immediately flipped to "Completed" — even though
that last batch was often still sitting unsent in the queue, not yet actually
delivered. "Completed" was only checking "has every recipient been queued",
not "has every recipient's message actually gone out or permanently failed".
Now it also checks each recipient's real outbound-job status and stays
"Sending" until every single one has reached a terminal state (sent or
dead-lettered) — matching what the per-recipient detail popup already showed
correctly.
**Files:** `apps/api/internal/messaging/broadcast_repo.go`
(`AdvanceBroadcastCampaignProgress`), `broadcast_progress_test.go` (NEW)

### 🔴 Leads export to Excel
A new "Export" button on the Leads page (next to Import) downloads an .xlsx
of exactly the leads currently filtered/searched on screen — same columns as
the table, plus Glofox Member ID. Backend paginates past the API's 200-row
page cap internally so large filtered lists export in full, not just one page.
**Files:** `apps/api/internal/leads/http.go` (`exportLeads`, shared
`parseListLeadsFilter`), `apps/web/src/app/admin/studios/[studioId]/leads/page.tsx`

### 🔴 Glofox member-export import — real data bug, found and fixed
A studio uploaded their actual Glofox "member export" (First/Last Name,
Membership Name, Membership Plan, Membership Expiry Date, Email Consent, ...
— a different, more detailed format than the simpler one built earlier) and
every single lead came in broken: the **name showed the membership plan**
("Presale - Stage 2 Foundation", or "Staff" for anyone on a staff
membership), and **phone/email were blank** for all 145 rows. Root cause: the
fuzzy header-matching in `mapHeaders` matches on loose substrings ("name",
"email", "contact"), and this file has OTHER columns that coincidentally
contain those words too — "Membership Name" silently overwrote the real name
mapping, "Email Consent" overwrote the real Email column, "Last Contacted"
overwrote the real Phone column — because whichever matching column came
*last* in the header row won.
Fixed two ways: the matcher no longer lets a later column overwrite one
already claimed, and the name-building logic now always prefers First/Last
Name over the ambiguous generic "name" field when both exist. Also: this
format has no Active/Trial/Inactive status column at all, so every lead
landed as "New" regardless of their real status — now inferred from the
membership itself (a "Presale" plan → trial-stage; a future Membership
Expiry Date → member; expired or missing → dropped).
The 145 leads from the broken test import (studio `t4refds`, a disposable
test studio, 2026-10-07) were deleted at the user's request — none had a
real purchase attached — ready for a clean re-upload.
**Files:** `apps/api/internal/leads/service.go` (`mapHeaders`, `ImportLeads`,
`parseGlofoxExpiryDate`), `glofox_import_test.go` (built from the real
header row and two real rows, including the "Staff" case)

### 🔴 Real bug found while fixing the above: every import auto-messaged every lead, regardless of status
Separate, more serious bug surfaced while testing the Glofox import fix:
**every single imported lead — including ones that resolved to Member or
Trial Booked — was automatically handed to the AI auto-contact worker**,
which immediately WhatsApps the generic "want to book a trial / become a
member?" opener. There was no setting to control this at all. Importing a
real Glofox member list would have immediately messaged every active paying
member with a new-lead sales pitch.
Fixed: a new "Enable AI reply for these leads" checkbox on the Import dialog,
**off by default**. Even when turned on, only a lead that actually resolves
to status "new" gets auto-contacted — an imported Member/Trial
Booked/Dropped lead never does, no matter what the checkbox says, since that
opener makes no sense for someone who isn't a fresh, unconverted lead.
Verified against the real `outbox` table (not just inferred) that no
auto-contact row is created for a gated-out lead.
**Files (superseded by the redesign below — see that entry for the final
shape):** `apps/api/internal/leads/{service,http}.go`,
`apps/web/.../leads/ImportLeadsButton.tsx`

### 🔴 "Enable AI" redesigned: a standalone, explicit control, not an import checkbox
Reworked per feedback right after landing the above: the AI-enable checkbox
on the Import dialog was removed entirely. Import now **never** auto-contacts
anyone, full stop — no flag, no exceptions. In its place, the Leads page has
a new **"Enable AI"** button next to the "Live" badge: pick a lead status
from a dropdown (New, Contacted, Trial booked, Member, Dropped, Paused), and
every *current* lead at that status — imported or not, old or new — gets
connected to the AI auto-contact worker right then, with a confirmation step
first since it immediately sends real WhatsApp messages. A lead already
contacted is skipped (safe to run more than once without double-messaging),
and only leads genuinely at the chosen status are touched.
**Files:** `apps/api/internal/leads/repo.go`
(`EnqueueAutoContactForLeadsByStatus`), `service.go`
(`EnableAutoContactForLeadsByStatus`), `http.go` (`POST
.../leads/auto-contact/enable`), `glofox_import_test.go`
(`TestImportLeads_NeverAutoContacts`, `TestEnableAutoContactForLeadsByStatus`),
`apps/web/.../leads/{EnableAIButton.tsx (NEW),ImportLeadsButton.tsx,actions.ts,page.tsx}`

### 🔴 Pipeline "Cold" column — real bug, found while investigating a report
The Cold Leads column and the Paused column shared one grid slot — Cold only
rendered when the studio had zero paused leads; any studio with even one
paused lead made Cold disappear from the Pipeline entirely, regardless of
how many leads were actually cold. Now both render as separate, permanent
columns. (Checked against real data for the studio that reported this: Cold
was correctly empty there for an unrelated reason — those leads were
bulk-imported directly with advanced statuses and have zero conversations,
so there's nothing yet for the Cold scanner to evaluate.)
**Files:** `apps/web/.../pipeline/PipelineBoard.tsx`

### 🔴 "Enable AI" now supports multiple statuses at once
Follow-up request: the status picker was a single dropdown; now it's a
checkbox grid — check any combination (e.g. New + Contacted) and Continue
connects all of them in one confirmation, instead of running the action
once per status.
**Files:** `apps/api/internal/leads/{repo,service,http}.go`
(`EnableAutoContactForLeadsByStatuses`, also fixed a real NULL-scan crash:
the bulk query didn't `COALESCE` nullable `first_name`/`last_name`, which
would have failed on any real lead missing one), `glofox_import_test.go`
(`TestEnableAutoContactForLeadsByStatuses_Multiple`),
`apps/web/.../leads/{EnableAIButton.tsx,actions.ts}`

### 🔴 Real bug, found from a live chat: AI forgot context mid-conversation in long chats
Reported directly from a real conversation (Himmat Singh): partway through an
otherwise normal, continuous WhatsApp thread, the AI replied *"Apologies for
the mix-up — I don't have access to your previous chat history here... could
you remind me of your main fitness goals?"* — re-asking something the
customer had already told it a few messages earlier in the very same chat.
Root cause (confirmed against the real database for that exact
conversation): the AI's prompt only ever loads the **last 15 messages** as
its working context. The platform already has a summarization mechanism
(`ai_context_summary`) meant to cover everything outside that window — but
it only ever ran once, right after a WhatsApp-Web chat-history *import*, never
during an ordinary live conversation as it naturally grows. Once a real chat
passed ~15-20 messages, anything said earlier (including stated fitness
goals) silently fell out of the AI's context with nothing to replace it.
Fixed: a rolling summary now keeps itself updated for any live conversation
as it grows — the first time it crosses the 15-message window, and every 5
messages after that — covering everything older than the recent window, so
a long chat never again just drops what was said earlier.
**Files:** `apps/api/internal/messaging/ai_worker.go`
(`ensureRollingSummary`, `shouldRefreshRollingSummary`,
`summarizeMessagesInto` — refactored out of the existing
`summarizeConversation` so both paths share one implementation),
`rolling_summary_test.go` (NEW)

### 🔴 A manual reply now cancels any pending Manual Action / AI follow-up for that lead
Real gap, confirmed by tracing the code: if a staff member replied to a lead
manually in the Inbox, any already-scheduled "Manual Actions" job or queued
AI follow-up nudge for that same lead was completely unaffected — it would
still fire later and land on top of the reply, unannounced. Fixed: sending a
manual reply now cancels any other still-pending automated/AI job for that
conversation. It never touches the reply's own job, or any other genuine
staff-sent message.
One real limit this can't fix: if staff message a lead from their own
personal phone number — not the one connected to this platform — there's no
way for the platform to ever know that happened, so a scheduled Manual
Action would still fire in that case. That would need a manual "mark as
contacted" step, which doesn't exist; flagged to the user, not built, since
it wasn't asked for.
**Files:** `apps/api/internal/messaging/repo.go`
(`CancelPendingAutomatedJobsForConversation`), `service.go` (`EnqueueReply`),
`cancel_automated_on_manual_reply_test.go` (NEW)

### 🔴 AI no longer re-greets by name on every single reply in an active chat
Real complaint, from a live test conversation: the AI opened two replies in a
row — 4 minutes apart, mid-exchange — each with "Hi Puneeth! ..." Now it only
greets the lead by name again when genuinely resuming after a real pause
(more than 2 hours of silence); otherwise it goes straight into the answer,
the way a person would mid-conversation.
**Files:** `apps/api/internal/messaging/ai_worker.go` (`buildPrompt`),
`class_escalation.go` (`conversationResumedAfterGap`),
`class_escalation_test.go`

### 🔴 Re-greeting fix, round 2: the prompt instruction alone wasn't enough
You tested the first fix and it was still saying "Hi Puneeth" on every
reply. Root cause: the prompt instruction (previous entry) is just a
sentence in a long prompt — models (especially the fast Groq ones this
platform defaults to) don't reliably obey a buried stylistic rule like that,
the same reason `stripTimeGreeting` already exists as a code-level backup
for "don't open with Good morning". Added the equivalent backup for names:
`stripNameGreeting` now deterministically removes a leading "Hi
{name}!"/"Hey {name},"/"Hello {name}" from the AI's actual reply text
whenever `conversationResumedAfterGap` says this is still an active
exchange — not just asking the model nicely. Verified the server picked up
the new binary (build timestamp matches the edit, confirmed via `air`).
**Files:** `apps/api/internal/messaging/ai_worker.go` (`stripNameGreeting`,
wired into the same post-processing step as `stripTimeGreeting`),
`class_escalation_test.go`

### 🔴 Real bug found debugging the decision tree: a pricing question wrongly confirmed a trial booking
This is why the decision tree looked like it wasn't firing — it wasn't the
tree's fault. Traced step by step with the real database: sending "How much
is the trial? What's the price for the $68 package?" to a lead got
misread by `detectOptionChoice` as the customer *confirming* they want the
trial (because it bare-matched the word "trial" with no question-mark
guard, unlike the equivalent, already-correct logic elsewhere in this
codebase for the same kind of check) — instantly flipping the lead's status
and scheduling a trial follow-up, from what was actually just a pricing
question. The customer's real question then fell through to the generic
AI, which had no memory of what was asked. Fixed: a bare keyword match
(trial/member/ready/yes/no/later) now requires the message to actually not
be a question, same bar the rest of the platform already applies.
Also hardened while investigating: the goroutine that runs the AI reply for
every inbound message had no panic recovery — any unhandled panic anywhere
in that code path would silently kill message processing with zero log
line, zero reply, and zero trace. Added `recover()` with full logging, so a
future bug shows up as one logged error instead of a message vanishing
with no explanation (a symptom briefly suspected during this
investigation, though not confirmed as the cause here).
**Files:** `apps/api/internal/messaging/ai_worker.go`
(`detectOptionChoice`, `isLikelyQuestion`, `listenStudio`'s goroutine),
`class_escalation_test.go`

### 🔴 Two more instances of the same question-misread bug, found by proactively scanning for it
After fixing `detectOptionChoice`, searched the rest of the messaging code for the same
bug class (a bare keyword match on raw customer text, no check for whether it's a
question, driving a consequential action) and found two more real ones:
- **A genuine question could trigger a real Stripe payment link.** Several of the
  "customer wants to pay" trigger phrases ("how to pay", "how do I pay", "payment link")
  are themselves question-shaped — "How do I pay, do you take Apple Pay?" was read as a
  purchase confirmation and would have sent a real checkout link unprompted.
- **The menu-choice shortcut had the identical bug `parseMenuChoice` was built to avoid**,
  just on its own bare "trial"/"member" match: after the bot sends its "1. Book a Trial /
  2. Become a Member" menu, a customer asking "What does the trial actually include?"
  was read as picking option 1.
Both now require the message to not be a question before a bare keyword counts, same
rule as the earlier fix. Deliberately left `isComplaintOrNegativeFeedback` alone — a
false positive there just escalates to a human (the intentional safe fallback this
whole session has been building toward), and real complaints are often phrased as
rhetorical questions, so gating it the same way would make it miss genuine ones.
**Files:** `apps/api/internal/messaging/ai_worker.go`, `class_escalation.go`
(`isExplicitPurchaseIntent`, `parseMenuChoice`), `class_escalation_test.go`

### 🔴 Cold-lead scanner cadence: 15 min → 2 hours
Per explicit request — the background job that recomputes which
conversations are "Cold" (see the Pipeline bug above) now runs every 2 hours
instead of every 15 minutes. It was already running continuously, not just
once, before this change — just slower now.
**Files:** `apps/api/internal/messaging/cold_scanner.go`

### 🔴 Real purchases now escalate to a human
Trial/membership payment confirmed → lead updated, receipt sent — but nobody on
staff was ever told. Now escalates to the Escalation tab with the plan name
("Customer became a Member — Plan: Pro") and emails the studio, for all 5 payment
confirmation paths (2 trial, 3 membership).
**Files:** `apps/api/internal/studios/webhook_stripe.go`, `webhook_stripe_test.go`

### 🔴 "Can't process this booking" now actually escalates
`SendTrialPaymentLink` already escalated when a lead's Trial plan was inactive — but
only in that one specific, lead-attached case. A conversation with no lead, or
Stripe simply not configured for the studio, silently sent a holding message with no
one notified. Both gaps closed, same escalation pattern.
**Files:** `apps/api/internal/messaging/service.go`

### 🔴 AI stopped repeating "what are your fitness goals?" on every reply
The qualifying-goals question is meant to be asked once; a prompt default was
asking it again on every single turn for any studio without a Gemini key (most of
them). Now checks conversation history first — only asks if it hasn't already.
**Files:** `apps/api/internal/messaging/ai_worker.go`, `class_escalation.go` (shared
`motivationPhrases`), `class_escalation_test.go`

### 🔴 "If it's not in the knowledge base, don't answer" — now a hard rule, not a guess
Previously the only "don't guess" check depended on Gemini's classifier running at
all — so a studio with only a Groq key had **no** such check, and the AI would just
answer (or admit it didn't know but keep chatting) for anything. New deterministic,
provider-agnostic pre-check compares the question against the exact text the model
is about to see; if there's no real overlap, it escalates **before any model is
called** — Groq, Gemini or Claude, doesn't matter which.
Also fixed a real bug found while building this: "do u" (WhatsApp shorthand) wasn't
recognized as a question at all, only "do you" — so short, casual phrasing slipped
through unescalated.
**Files:** `apps/api/internal/messaging/ai_worker.go`, `class_escalation.go`,
`class_escalation_test.go`

### 🔴 "Needs Answers" tab — a feedback loop for the above
When the AI escalates because it doesn't know something, the exact question now
gets logged and shown on **Knowledge Base → Needs Answers**, with a badge count.
Answer it once there and it's appended to a shared, studio-wide knowledge document
and re-embedded immediately — the next customer who asks something similar gets it
from the knowledge base, no further escalation needed. Repeat asks of the same open
question just bump a counter instead of piling up duplicates.
**Files:** `apps/api/internal/studios/{domain,repo,service,http}.go`,
`knowledge_gaps_test.go`, migration `20261007090001_knowledge_gaps.sql`,
`apps/web/.../knowledge-base/{NeedsAnswersTab.tsx,KnowledgeBaseForm.tsx,actions.ts}`

### 🔴 Glofox member Excel can be uploaded straight into Leads
A Glofox member export (`BarcodeID, Client Name, Status, Membership Tier, Phone,
Email Address, ...`) can now be uploaded as-is under **Leads → Import** — no
reformatting. `Client Name`/`Membership Tier`/`BarcodeID` map automatically; Glofox's
own Active/Trial/Inactive status values translate to this platform's lead statuses;
the Barcode ID is kept on the lead (new `glofox_member_id` field, shown on the lead
detail page).
**Files:** `apps/api/internal/leads/{domain,repo,service,http}.go`,
`glofox_import_test.go`, migration `20261007100001_lead_glofox_member_id.sql`,
`apps/web/src/lib/types.ts`, `apps/web/.../leads/[id]/page.tsx`

### 🔴 Logo upload in Settings — fixed
Two bugs stacked on top of each other: the upload sent the file under the wrong
form field (`image` instead of `file`, so every upload 400'd), and even if that were
fixed, the success handler read the wrong response key (`url` instead of
`mediaUrl`), so it would've saved "undefined" as the logo. Same two bugs existed a
second time in the Booking Page's Hero Image uploader. Both fixed in both places.
**Files:** `apps/web/.../settings/{SettingsSections.tsx,SettingsForm.tsx}`

---

### 🔴 Inbox timestamps now match WhatsApp Web's style
Conversation list used to show an ever-ticking relative counter ("2m", "3h",
"5d") for every chat, with no distinction for today vs. older days. Now: a
bare time for anything today, "Yesterday", the weekday name for the rest of
the past week, and a short date beyond that — same convention WhatsApp Web
uses. Also added day-divider labels ("Today" / "Yesterday" / weekday / date)
inside an open conversation, separating messages sent on different days —
there was no such grouping before, just a flat scroll of messages.
**Files:** `apps/web/src/lib/datetime.ts` (`formatChatListTimestamp`,
`formatDayDivider`, `dayKey`), `apps/web/.../inbox/InboxLive.tsx`

## 2026-10-06

### 🔴 Several external Google Sheets per studio
A studio can now import leads from multiple Google Sheets (or multiple tabs of one),
each with its own tab, column mapping and auto-contact settings, instead of exactly
one. Import progress is now tracked per studio too (previously two studios importing
the same sheet would have silently shared — and corrupted — one position).
**Files:** `apps/api/internal/leads/*`, `apps/api/internal/integrations/sheets/inbound/worker.go`,
migration `20261006110001_external_sheets_multiple.sql`,
`apps/web/.../settings/{SettingsSections.tsx,actions.ts}`

### 🔴 Webhook signatures actually verified
Twilio (SMS), X (DMs) and Meta's data-deletion callback accepted **any** request
with no signature check at all — anyone who found the URL could inject fake inbound
messages. All three now verify their signatures (403/401/400 on failure) using the
real algorithm each provider documents, checked against independently-computed test
vectors, not just read from a doc.
**Files:** `apps/api/internal/messaging/webhook_{twilio,x,meta}.go`,
`webhook_signatures_test.go`, `apps/api/cmd/server/main.go`

### 🔴 Random one-time passwords, not a shared `password123`
Every new teammate/studio-admin used to get the identical fixed password
`password123` until they changed it — anyone who learned a new teammate's email
could log in during that window. Now a random 16-character password, unique per
account, emailed once. The forced first-login reset screen was updated to ask for
that temporary password (it used to hard-code the old shared one).
**Files:** `apps/api/internal/identity/*`, `apps/api/internal/studios/service.go`,
`apps/api/internal/platform/mail/mail.go`, `apps/web/src/components/AppShell.tsx`,
`temp_password_test.go`

### 🟡 Platform-level settings locked to super-admins; studio deletion locked to studio admins
*(Committed as `36f9ddb`, on origin, not yet on EC2.)*
Any logged-in studio user could reach `/me/studios/global/...` and overwrite
platform-wide Stripe keys or pricing plans — not just their own studio's settings.
Separately, any staff member (not just an admin) who knew a studio's contact email
could delete the entire studio. Both locked down.
**Files:** `apps/api/internal/studios/http.go`, `middleware_global_test.go`

### 🟢 Broadcast daily-limit accounting, longer network retry, auto-requeue
*(Hand-patched directly onto EC2 same day; not yet committed to git — see
[EC2 sync status](#ec2-sync-status-read-this-before-assuming-something-is-live).)*
- A broadcast only subtracted **sent** messages from the daily WhatsApp limit, not
  **queued** ones — with sends paced a minute apart, this queued ~2.5x the real
  limit before anyone noticed (124 queued against a limit of 48).
- Network/DNS failures (the exact kind that caused a stuck broadcast on prod) used
  to exhaust retries in about a minute; now retried for ~85 minutes before giving up.
- A broadcast recipient whose message died for a temporary reason now automatically
  goes back into the queue (up to 5 times) instead of staying failed forever —
  the campaign keeps working through its list on its own, day after day.
- Manual Actions and the broadcast recipient list now show a real reason
  ("Not on WhatsApp", "Network problem, will retry") instead of just "Failed".
**Files:** `apps/api/internal/messaging/{repo,broadcast_repo,broadcast_worker,worker}.go`,
`worker_transient_test.go`, migration `20261006100001_broadcast_recipient_retry.sql`,
`apps/web/src/lib/jobFailureReason.ts`, `apps/web/.../inbox/{InboxLive,BroadcastsPanel}.tsx`

---

## 2026-10-03 and earlier (for reference — already covered in `docs/PLATFORM_GUIDE.md`)

- 🟢 Per-studio outbound send isolation (one studio's dead channel can no longer
  block every other studio's messages).
- 🟢 Glofox first-session message made opt-in per studio, capped, recency-windowed
  — it used to message every member who'd *ever* attended, the first time it ran.
- 🟢 Leads page no longer crashes for a staff role that has Leads but not Campaigns
  permission.
- 🟡 Root `README.md` and `docs/PLATFORM_GUIDE.md` added (committed `843b993`, on
  origin, not yet on EC2 — harmless, it's documentation only).

---

## EC2 sync status (read this before assuming something is "live")

EC2 is currently a **mix of three different states** layered on top of each other:

1. **Base: commit `d971fc5`** — everything through "Leads page permission fix", Oct 3.
2. **Plus 9 files hand-patched directly on the box** (not through git) the same day
   as the broadcast fixes above — `repo.go`, `worker.go`, `broadcast_repo.go`,
   `broadcast_worker.go`, `worker_transient_test.go`, `jobFailureReason.ts`,
   `InboxLive.tsx`, `BroadcastsPanel.tsx`, and the retry-requeue migration. These
   work, and are live, but **git on this machine has no record that matches what's
   actually running on EC2** until they're committed.
3. **Missing entirely:** the Glofox first-session opt-in fix (`a1221c8`), the studio
   security fix (`36f9ddb`), the docs, and **every single thing listed under
   2026-10-06 and 2026-10-07 above** except the broadcast fixes in (2).

**To get EC2 fully caught up and back in sync with git** (recommended before
trusting "is X live?" again):
```bash
# 1. On this machine — commit everything currently sitting uncommitted:
cd ~/Desktop/studiox
git add -A
git commit -m "fix: AI escalation guardrails, knowledge gaps, Glofox import, webhook signatures, random passwords, logo upload, multi-sheet import"
git push origin main

# 2. On EC2 — discard the hand-patched files (git now has the same fix, properly),
#    then pull and redeploy everything in one go:
ssh -i ~/Downloads/1herosocialai1.pem ubuntu@13.63.245.166
cd ~/studiox
git checkout -- apps/api/internal/messaging apps/web/src/lib/jobFailureReason.ts \
  "apps/web/src/app/admin/studios/[studioId]/inbox"
git clean -fd apps/api/internal/messaging apps/web/src/lib
git pull
bash deploy/deploy.sh   # builds api+web, runs all pending migrations, restarts everything
```
After that, EC2's `git log -1` should read the same commit as this machine's, and
every 🔴/🟡 item above becomes 🟢.

**Untracked, not part of any fix (leave alone):** three `Members Report...xlsx`
files and a `Glofox - members 30 Sept.csv` sitting in the repo root — these are your
own data files, not code. `docs/findings.txt` is a stray copy of an earlier chat
message and can be deleted.
