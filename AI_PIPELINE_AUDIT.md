# AI Reply Pipeline — Full Audit

Investigation of Project-X's AI reply pipeline: LLM call sites, reply flow, decision
trees, model routing, prompts, retrieval, guardrails, and logging/data. Findings are
split into **local/dev DB** (code-grounded, DB on `localhost:5436`) and **production**
(real customer data, queried 2026-10-19 via SSH to the prod host). All claims are
file:line-cited or backed by a literal SQL query; anything that couldn't be verified is
called out explicitly rather than guessed.

---

## Executive summary

- **Groq is the only provider actually serving replies.** All reply-path generation,
  summarization, and style-profile calls go through a Groq → Gemini → Claude waterfall,
  but **Gemini has had zero successful calls in production since 2026-09-17**, and while
  it was active it failed **98.6% of the time** (117,243 failures / 1,685 successes,
  mostly a generic `"all Gemini models failed"` error). No studio has a Claude key.
  Intent classification, query expansion, and KB reranking are **Gemini-only with no
  fallback**, so these features have been silently dead for a studio's worth of traffic
  for over a month with no alerting.
- **In the local dev environment, the Groq account itself is billing-suspended**
  (`organization_delinquent`), driving an 85% failure rate there. When every provider
  fails, the reply is **silently dropped** — no customer-visible message, just a server
  log warning.
- **No prompt/response text is stored anywhere**, dev or prod — `llm_usage_logs` only
  has token counts and metadata. There is no replay tooling.
- **Real customers hit a confirmed language gap**: a live production `knowledge_gaps`
  row is literally *"Kya hum hindi me baat kar skte hai?"* ("Can we talk in Hindi?") —
  the code has no language detection or Hinglish handling at all.
- **Staff-takeover race condition**: a manual reply sent during the 6-second inbound
  debounce window does not stop the AI from independently generating and sending its
  own reply ~20+ seconds later.
- **`ai_suggestions`** (a table shaped exactly like an accept/edit/reject staff-review
  flow) has **0 rows in both dev and production** and is never referenced in code — a
  dead/orphaned feature, not a dev artifact.
- Architecture is otherwise reasonably sound: studio isolation is airtight everywhere
  checked (KB, style examples, semantic history, answer cache keys), decision trees
  correctly take priority over AI, and Redis/embeddings outages degrade gracefully
  rather than breaking the reply path.

---

## 1. Every LLM call site

| # | Call | Trigger | Path | Model | Settings | On failure |
|---|---|---|---|---|---|---|
| 1 | **Reply generation** (`ai_worker.go:504`, prompt at `:1656`) | Inbound msg, after 6s debounce, after all deterministic shortcuts miss | Reply (sync) | Groq→Gemini→Claude waterfall | Groq: temp 0.7, max_tokens 512; Gemini: no config (API defaults); Claude: max_tokens 512 | Short/empty reply → next model tried; all fail → **silent drop, no message sent** |
| 2 | **Summarization** (`ai_worker.go:254`) | After WA-Web backfill, or every 5 msgs once history >15 | Background | Same waterfall | Same as above | Empty → summary not stored, falls back to raw window |
| 3 | **Intent classification** (`embeddings.go:113`) | Every inbound msg (if Gemini key resolvable), parallel w/ #4 | Reply (sync) | **Gemini only, no fallback** | temp 0.0, max_tokens 80, JSON via prompt instruction (no real JSON mode) | Error → `intent="unknown"`, silent |
| 4 | **Query expansion** (`embeddings.go:348`) | Same as #3, parallel | Reply (sync) | Gemini only, no fallback | temp 0.0, max_tokens 120 | Error → returns original query unchanged, silent |
| 5 | **KB chunk reranking** (`embeddings.go:202`) | After hybrid KB retrieval | Reply (sync) | Gemini only, no fallback | temp 0.0, max_tokens 50 | Error → truncates unranked results, silent |
| 6 | **Style-profile distillation** (`style_worker.go:306`) | Every 5 min, if studio has ≥20 new staff replies | Background | Same waterfall | Same as #1 | Error → profile stays stale, logged only |
| 7 | **Decision-tree keyword suggestion** (`decisiontree/service.go:32`) | Admin editing a tree node | Admin tool, not customer-facing | **Groq only, hardcoded model**, bypasses studio config | Same as Groq default | Falls back to local word-frequency heuristic, never errors |
| 8 | **CRM doc parsing** (`crm/parse.go:54`) | Admin uploads API doc | Admin tool | Whatever `ai_task_configs` names — **0 rows configured in both dev and prod, feature is dead** | Claude: max_tokens 4096 | Hard error returned to caller (only call site that doesn't swallow errors) |
| 9 | **OCR** (`knowledge_base_ocr.go:36`) | Admin uploads image to KB | Admin tool | Mistral `mistral-ocr-latest`, no fallback | N/A (OCR endpoint) | Missing key → 400; error → 500/502, visible to admin |
| 10 | **Test Chat** | Admin manually testing | Admin tool | Reuses the exact live pipeline (#1, #3-5) | Same | Same |

Program-schedule parsing is **pure regex**, not an LLM call (deliberately, to avoid
semantic search picking the wrong week, `program_schedule.go:46-50`).

**Embeddings** (`apps/embeddings/main.py`): a local sentence-transformers sidecar
(`intfloat/multilingual-e5-base`), not a generative call — pure vector production for RAG.

---

## 2. End-to-end reply flow

**Pipeline**: webhook → `EvtMessageReceived` → **6-second debounce**
(`inboundDebounceWindow`, `ai_worker.go:68` — not 20, see below) → `handleMessage` runs a
long chain of deterministic gates (AI-enabled check, DND, auto-contact stage, greeting,
"talk to human" phrase, complaint keywords, **decision tree**, class-action request) →
if nothing short-circuits: classify+expand (parallel) → embed → KB retrieval/rerank/
history/style (parallel) → confidence gates → build prompt → LLM waterfall → output
guards → enqueue.

- **LLM calls per reply**: 0 (tree/escalation/greeting) to up to 4 (classify + expand +
  rerank + 1 waterfall call) on the full AI path.
- **Multiple messages in a row**: the 6s debounce timer resets on each new message and
  fires once quiet, producing exactly one reply covering the whole burst.
- **Staff replies during the delay — real gap found**: `EnqueueReply` cancels any
  *already-enqueued* AI/automation job (`service.go:2043` → `repo.go:2058-2068`), but
  nothing cancels the debounce timer itself, and the post-debounce check
  (`ai_worker.go:605-617`) only looks for prior `automation`/`ai` replies, not
  `studio_user`. **A staff reply during the 6s debounce window does not stop the AI from
  independently generating and sending its own reply ~20+ seconds later.** Once the AI
  *has* enqueued its reply, a staff reply in that window does correctly cancel it.
- **The "20 seconds"**: a hardcoded pre-send delay on the final enqueued reply
  (`ScheduledFor: now+20s`, `ai_worker.go:1571`, "so it doesn't feel instant/robotic") —
  separate from, and inconsistent with, the studio-configurable delay every other reply
  path (tree, greeting, escalation handoffs) uses.
- **Latency, local dev DB** (n=23 real AI replies, 30 days): p50 ≈ 24.4s, p95 ≈ 27.4s.
- **Latency, production** (n=393 real AI replies, 30 days): **p50 ≈ 25.6s, p95 ≈
  267s** — a far worse real-world tail than dev showed, consistent with queueing/backlog
  effects at scale.

---

## 3. Decision trees

- **Trees take priority** — checked before AI (`ai_worker.go:975`), and any matched
  action branch returns before the LLM waterfall is ever reached.
- **Adoption**:
  - Local dev DB: 1 of 18 studios (≈5.6%).
  - **Production: 5 of 6 studios (≈83%)** — the dev DB badly understates real adoption.
- **Tree vs AI in practice**:
  - Dev (30 days): 4 tree-answered vs 14 genuine LLM replies.
  - **Prod (30 days)**: `source_ref='decision_tree'` (incl. named variants) ≈ 14 replies
    vs `groq:qwen/qwen3.8-27b` 310 replies — AI free-form generation dominates even
    where trees exist, because trees only catch specific matched phrasings.

---

## 4. Models

- **Fallback order**: Groq (per-studio enabled models, default
  `{qwen/qwen3.8-27b, gpt-oss-120b}`) → Gemini (`gemini-flash-latest,
  gemini-flash-lite-latest`) → Claude (`claude-haiku-4-5`). First non-empty reply wins;
  every attempt logged to `llm_usage_logs`.
- **Live routing config**:
  - Dev: `ai_task_configs` 0 rows, `platform_settings` 0 rows, `studio_ai_models` 1 row
    (the dev owner's own studio) — 17 of 18 studios run on hardcoded defaults with no
    key for Gemini/Claude, and only 1 studio has a Groq key.
  - **Prod**: `ai_task_configs` 0 rows (CRM-doc-parsing feature is dead here too).
    `platform_settings` has exactly **1 row** (`platform_plans` — not an LLM key, so
    there is **no platform-wide LLM fallback key** in prod either). `studio_ai_models`
    has rows for only 4 of 6 studios (the other 2 run on hardcoded defaults). Key
    coverage across all 6 studios: **Gemini 2/6, Groq 5/6, Claude 0/6**.
- **30-day provider/model split**:

  Dev (Groq only, zero Gemini/Claude calls ever logged):
  | provider/model | count | avg latency | failure rate |
  |---|---|---|---|
  | groq/qwen3.8-27b | 1,442 | 268ms | 74% |
  | groq/gpt-oss-120b | 1,028 | 83ms | **100%** |

  Root cause: the dev environment's Groq account is billing-suspended
  (`organization_delinquent`).

  **Production** (real scale):
  | provider/model | count | avg latency | p50 | p95 | failures |
  |---|---|---|---|---|---|
  | groq/qwen3.8-27b | 243,206 | 253ms | 155ms | 363ms | 816 (0.3%) |
  | groq/gpt-oss-120b | 475 | 644ms | 635ms | 906ms | 0 |
  | groq/gpt-oss-20b | 15 | 329ms | 335ms | 409ms | 0 |

  **Production, all-time** (not just 30 days): `groq` 442,445 calls (2026-07-18 →
  ongoing), `gemini` 118,928 calls — but **all between 2026-08-07 and 2026-09-17, then
  zero ever since**. Of those 118,928 Gemini calls, 117,243 failed (98.6%), nearly all
  with the generic error `"all Gemini models failed"`.
- **Thinking/reasoning-tag stripping**: **does not exist anywhere in the codebase.**
  Both Qwen and gpt-oss are reasoning-capable model families; if either emits `<think>`
  preambles via Groq, it passes straight through to the customer unfiltered
  (`groq/client.go:84-105` takes `choices[0].message.content` verbatim). No evidence
  either way that it has actually happened (stored reply bodies weren't inspected for
  literal `<think>` text), but there is no guard against it.
- **Studios with no Gemini key**: dev 18/18 (100%); **prod 4/6 (67%)**.
- **Cost per reply / per studio**: not computable anywhere — no pricing data is stored,
  only token counts (`tokens_in`/`tokens_out`).

---

## 5. Prompts

- All prompts are **Go string literals / `fmt.Sprintf` templates in code** — no external
  template files, no prompt-config system. Locations: `ai_worker.go:1656` (main reply,
  reused by Test Chat), `ai_worker.go:271` (summarization), `style_worker.go:327`
  (style profile), `embeddings.go:121,218,352` (classify/rerank/expand),
  `decisiontree/service.go:38` (keyword suggestion), `crm/parse.go:111` (CRM doc parsing).
- **History windowing**: last 15 messages verbatim (`aiHistoryWindow`, `ai_worker.go:302`),
  plus a rolling summary of everything older (refreshed every 5 messages, capped at
  loading 500), plus up to 3 semantic-similarity matches (score ≥0.75) over the
  conversation's own older messages — three separate mechanisms for history to resurface.
- **Prices/plans**: fetched live per-request from the studio's own `ListActivePlans`
  query (`ai_worker.go:636`) — never cached/stale, never cross-studio.
- **Cross-studio leakage**: **none found** — every retrieval call (KB, style examples,
  semantic history, answer cache keys) is explicitly `studio_id`-scoped. Answer cache
  keys are `studio:%s:ai:%s:%x` (`answer_cache.go:42-45`), so even a byte-identical
  prompt for two different studios can never share a cache hit.
- **Hinglish/multi-language handling**: **none exists in code.** No language
  detection, no explicit instruction. **Confirmed as a real production problem**: a live
  `knowledge_gaps` row is *"Kya hum hindi me baat kar skte hai?"* and another is *"Can
  you answer me in any other language?"* — real customers hitting this gap, not a
  theoretical concern.
- **Testing**: no eval harness, no golden/snapshot tests of actual prompt output text,
  no LLM-as-judge. Only unit tests of discrete gating functions
  (`class_escalation_test.go`, `worker_table_format_test.go`, etc.). Manual review
  happens via the admin Test Chat tool, not automated testing.

---

## 6. Retrieval

- **Embedding model**: `intfloat/multilingual-e5-base`, **768-dim**, **512-token limit**
  (silently truncates beyond that — `SentenceTransformer` default, never overridden).
- **Chunking**: semantic (paragraph→sentence packing, `embeddings.go:22-76`),
  **800 chars, 150 overlap** (`studios/service.go:637,645`).
- **k funnel** (`repo.go:902-960`, called with `limit=8`): vector search k=40 → keyword/
  BM25 k=40 → RRF fusion k=8 → Gemini rerank k=4 → **4 chunks** into the final prompt.
  `kbConfident := len(reranked) >= 2` is a count-based gate, not a similarity threshold.
- **Index**: pgvector **HNSW** (`m=16, ef_construction=64`,
  `migrations/20260624000001_rag_upgrade.sql:5-8`), cosine distance (`<=>` operator).
  `ef_search` is never tuned anywhere — runs on pgvector's default.
- **studio_id filter**: applied **pre-filter**, same SQL statement as the vector search
  (`WHERE studio_id = $1 ... ORDER BY embedding <=> $2::vector`) — no cross-studio
  exposure, confirmed at the query level.
- **Minimum similarity threshold**: **none** for KB retrieval — relevance is gated by
  Gemini's rerank judgment and the count-based confidence flag above, not a numeric
  distance cutoff. (Unrelated thresholds do exist for semantic message history, ≥0.75,
  and style examples, ≥0.6.)
- **Re-embedding on KB edit**: yes, edit-triggered (`service.go:588`, async goroutine,
  not a cron), full re-chunk with content-hash dedup reuse of unchanged chunks, atomic
  replace-all on success (`repo.go:762-789`) — but a failed sync leaves the previous
  (now-stale) chunk set in place with no retry.
- **Row counts**: `studio_knowledge_chunks` has **0 rows in dev** (no studio ever synced
  a KB there) vs **8,763 rows in production** — real KB content genuinely exists and is
  searched in prod; the dev-only findings on dimension/threshold were code-verified, not
  data-verified, and production confirms the table is actually in active use.

---

## 7. Guardrails and dependencies

- **Escalation triggers** (all route through `Service.EscalateAndNotify`,
  `service.go:2630`): explicit human-request phrase, complaint keywords, classifier-
  flagged complaint intent, decision-tree escalate node, class-booking-action request (AI
  can't book), low KB confidence, zero KB topical overlap, AI's own reply reading as "I
  don't know," plus a few billing-specific triggers. **Resolution is manual-only** — no
  automatic re-enable of AI after escalation anywhere in the messaging/leads packages.
- **Real escalation counts**:
  - Dev (all-time, ~10 days of data): `decision_tree: 4`, `explicit_handoff_request: 1`.
    All other trigger types: zero, ever. `knowledge_gaps` had 0 rows.
  - **Prod (30 days)**: `class_request_handoff: 17`, `no_kb_coverage_handoff: 11`,
    `explicit_handoff_request: 2`. **Currently escalated conversations: 4 of 4,459**
    (≈0.09%). `knowledge_gaps`: **12 real rows** (see §5 for language-gap examples).
- **Input checks**: **no prompt-injection or abuse filtering exists** anywhere —
  customer message text is interpolated into the prompt with no sanitization and no
  "ignore embedded instructions" guard.
- **Output checks**: targeted string/regex guards only (`ai_worker.go:1528-1559`) — strip
  motivational questions, time/name greetings, and an unfilled `{{placeholder}}`
  detector that substitutes a safe fallback string. No general toxicity/safety filter.
- **Reply caps**: no length cap found anywhere. There **is** a per-studio daily rate cap
  on automation/AI WhatsApp sends (`whatsapp_daily_message_limit`, 0=unlimited,
  enforced in `worker.go:350-358`) — manual staff replies are explicitly exempt.
- **If every model fails**: **nothing is sent to the customer.** `ai_worker.go:1520-1523`
  logs a warning and returns `nil` — silent drop, confirmed actively happening in the dev
  environment during its Groq billing outage, and a live risk in production given the
  month-long Gemini outage removing one leg of the fallback chain for 2 of 6 studios.
- **AI auto-resuming after escalation**: no, manual-only in both environments.
- **Answer cache (Redis) hit rate**: **not instrumented at all** — no counter, no log
  entry; a cache hit bypasses `LogLLMUsage` entirely (`ai_worker.go:1427-1431`), so hits
  are invisible even indirectly. Genuinely unknowable from existing data, not just "low."
- **Redis down**: fails open — treated as a cache miss (`answer_cache.go:50-77`), never
  blocks or breaks a reply.
- **Embeddings service down**: retries twice (1s, 2s backoff), then skips the whole RAG
  block and falls back to raw (un-retrieved) KB text; doesn't crash the reply, though
  quality degrades and the "no topical overlap" gate may escalate instead of guessing.

---

## 8. Logging and data

- **Full prompt/response storage: no, confirmed in both environments.**
  `llm_usage_logs` schema (identical in dev and prod) only has:
  `id, studio_id, provider, model, latency_ms, success, error_msg, created_at,
  tokens_in, tokens_out, created_by, updated_by` — never the actual prompt or response
  text, for any call, anywhere.
- **Replay tooling**: none exists — confirmed by an explicit code comment
  (`http.go:1679`): *"we don't replay history server-side at L1 — clients re-fetch on
  reconnect."*
- **30-day export**:
  - Dev: 2,470 rows, single studio ("Puneeth.G.M," the owner's own test studio), 10-day
    actual span, bot-like timestamp clustering — this is dev/test traffic, not
    production.
  - **Prod: 243,696 rows in 30 days** (442,445 all-time for Groq alone) across real
    customer traffic on 6 studios.
- **"Bad" conversations**:
  - Dev: real count is 1 — a trivial test-chat correction ("hi" → "hi good thing uare
    back"). No escalation or knowledge-gap data existed to build a meaningful list.
  - **Prod: 12 real `knowledge_gaps` rows** exist (see sample below) — genuine unanswered
    customer questions, though still short of the 20-30 requested.

  Sample production knowledge gaps (question previews, no customer identifiers):
  ```
  status     times_asked  question
  open       1            Maybe Tuesday or Wednesday?
  open       1            Want to know a little more about offers
  open       1            Can you answer me in any other language?
  open       1            Kya hum hindi me baag kar skte hai?
  open       1            Why?
  dismissed  1            Can I talk with any person?
  open       1            Yes I would like direction
  open       1            hello?
  open       1            I wanna lose weight
  open       1            what's the weather like today
  open       1            Tell me which plan does what
  open       1            Is there any discount or offers?
  ```

- **Staff edits/overrides of AI replies**: **not recorded anywhere in the live send
  path, in either environment.** `ai_suggestions` (has exactly the right columns for an
  accept/edit/reject flow — `status`, `reviewed_by`, `reviewed_at`, `sent_message_id`)
  has **0 rows in dev and 0 rows in production**, and is never referenced in code — a
  confirmed dead/orphaned feature, not a dev-only gap. The only correction mechanism
  (`ai_feedback_corrections`) is scoped to the admin Test Chat sandbox (1 row in both
  environments, trivial test content), not live customer messages.
- **Auto-reply rate / escalation rate / conversion**:
  - Dev: auto-reply ≈2.0%, escalation 0% — describes one dev studio's test session, not
    production reality.
  - **Prod**: message mix (30 days) — inbound/customer 45,294; outbound/studio_user
    43,266; outbound/automation 1,013; outbound/ai 467. **Auto-reply rate ≈ 3.3%**
    ((1,013+467) / 44,746 outbound). **Escalation rate ≈ 0.09%** (4 of 4,459
    conversations currently escalated).
  - **AI-vs-non-AI lead conversion (prod, real sample)**: leads whose conversation had an
    AI reply converted to trial/member at **14.4%** (16/111); leads with no AI reply
    converted at **18.3%** (338/1,845). Real, meaningfully-sized data — but
    **correlational, not causal**: AI-touched conversations may skew toward a different
    mix of leads (e.g. harder questions, off-hours contacts) rather than AI itself
    lowering conversion. Worth investigating further, not treating as proof.

---

## Per-studio breakdown (production, real data, queried 2026-10-19)

All 6 studios in production, with key config, usage, KB, escalation, and conversion
figures. `BFT Test` has zero activity across the board — an unused test studio, not a
real customer, kept here for completeness.

| Studio | Gemini key | Groq key | Claude key | Trees | LLM calls (30d) | LLM fail (30d) | KB chunks | KB gaps | Leads | Conversations | Escalated now |
|---|---|---|---|---|---|---|---|---|---|---|---|
| BFT Jalan Ampang | — | ✓ | — | 1 | 46,205 | 6 | 72 | 1 | 520 | 1,293 | 0 |
| BFT Taman Jurong | ✓ | ✓ | — | 3 | 117,342 | 4 | 414 | 1 | 1,419 | 1,560 | 2 |
| BFT Test | — | — | — | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| FS8 Tanjong Pagar | — | ✓ | — | 2 | 77,174 | 480 | 60 | 3 | 1,128 | 1,530 | 3 |
| Trial K | — | ✓ | — | 1 | 2,632 | 0 | 3 | 7 | 38 | 60 | 0 |
| trail | ✓ | ✓ | — | 1 | 0 | 0 | 8,214 | 0 | 11 | 16 | 0 |

Message mix per studio (30 days, all-time direction/source split):

| Studio | Inbound (customer) | Outbound (AI) | Outbound (automation) | Outbound (staff) |
|---|---|---|---|---|
| BFT Jalan Ampang | 8,942 | 50 | 5 | 17,446 |
| BFT Taman Jurong | 20,933 | 86 | 401 | 12,027 |
| BFT Test | 0 | 0 | 0 | 0 |
| FS8 Tanjong Pagar | 13,819 | 295 | 563 | 13,168 |
| Trial K | 1,091 | 30 | 6 | 485 |
| trail | 512 | 8 | 38 | 141 |

Lead-to-trial/member conversion per studio (leads linked to a conversation):

| Studio | Trial/member | Leads w/ conversation | Conversion |
|---|---|---|---|
| BFT Jalan Ampang | 2 | 155 | 1.3% |
| BFT Taman Jurong | 340 | 741 | **45.9%** |
| FS8 Tanjong Pagar | 9 | 1,013 | 0.9% |
| Trial K | 0 | 36 | 0% |
| trail | 3 | 11 | 27.3% |

**Notable per-studio findings:**
- **BFT Taman Jurong is the only studio with any real AI-model diversity and the only
  one carrying real conversion success** (45.9%, driven mostly by human staff replies —
  12,027 of its outbound is `studio_user`, only 86 is `ai`). It's also the only
  active-Gemini studio with meaningful KB content (414 chunks) and the heaviest LLM
  caller (117K calls/30d) — but note from §4 that Gemini itself has been failing/dead
  since Sep 17, so its "has Gemini key" status doesn't mean Gemini is actually working.
- **FS8 Tanjong Pagar has by far the highest LLM failure count (480 of 77,174, ≈0.6%)**
  and the highest live escalation count (3) and knowledge-gap count (3) — the worst
  combination of reliability + unanswered-question volume of any active studio, and its
  conversion rate (0.9%) is the lowest of any studio with real lead volume.
  **BFT Jalan Ampang's conversion (1.3%) is comparably low** despite near-zero LLM
  failures, suggesting the conversion gap there isn't primarily an AI-reliability issue.
- **"trail" has 8,214 KB chunks — by far the largest knowledge base of any studio** —
  but only 11 leads and 0 LLM calls in the last 30 days; looks like a KB-heavy pilot/demo
  studio rather than one with live AI traffic.
- **Trial K has the highest knowledge-gap rate relative to its size** (7 gaps on only 38
  leads / 60 conversations) — proportionally the most unanswered-question volume of any
  studio, despite the lowest LLM call count among active studios.
- No studio has a Claude key — Claude is fully unused in production, confirming it only
  ever serves as the waterfall's unreachable third fallback.

---

## Caveats

- Local dev DB claims about live data (sample rows, table population) reflect a
  single-studio seed/test dataset, not production traffic — flagged inline above
  wherever it matters.
- Production queries avoided pulling raw customer message content or PII; only
  aggregates, schema, and short knowledge-gap question previews were retrieved.
- Could not confirm whether an environment-variable-only Claude/Gemini key exists
  outside the DB-backed config tables in either environment — the `llm_usage_logs`
  evidence (zero Claude calls ever, Gemini calls stopping entirely on 2026-09-17) is
  consistent with no such key existing, but isn't absolute proof.
- `<think>`/reasoning-tag leakage is confirmed absent at the *code* level (no stripping
  logic exists); whether it has actually appeared in a sent customer reply was not
  verified against stored message bodies.
