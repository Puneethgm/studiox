# AI Architecture — Project-X

This document is a code-verified map of every AI-related feature in the repo:
what model each path calls, how fallback works, what gets embedded and how,
and whether any evaluation/quality-scoring exists. Every claim below is
backed by a `file:line` citation from the actual source at the time of
writing. Anything that could not be confirmed by reading code is explicitly
flagged as **unverified** rather than guessed.

Note: this document describes the *current state of the code*, which has
moved well past the `L1` scope described in `docs/skills.md` (that file
describes the studio/campaign/lead-capture foundation; it does not mention
the AI messaging stack below, which already exists and is in active use per
recent commits).

---

## 1. Provider clients (the 4 LLM/OCR integrations)

| Provider | File | Model(s) | Endpoint | Auth source | Timeout | Retry |
|---|---|---|---|---|---|---|
| **Claude** | `apps/api/internal/integrations/claude/client.go` | `claude-haiku-4-5-20251001` (const `defaultModel`/`DefaultModel`, lines 15,19) | `https://api.anthropic.com/v1/messages` (line 14) | `x-api-key` header; platform env `CLAUDE_API_KEY`/`CLAUDE_API_URL` (`platform/config/config.go:249-250`) **or** per-studio `studio.ClaudeAPIKey` (DB col `claude_api_key`) | 20s (line 34) | None |
| **Gemini** | `apps/api/internal/integrations/gemini/client.go` | `Models = []string{"gemini-flash-latest", "gemini-flash-lite-latest"}` (line 29) | `https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s` (line 65) | API key passed per call (studio BYO key only — no platform fallback key, lines 5-8) | 15s/attempt (line 80) | 3 attempts, exponential backoff 500ms→1s→2s, on transport error/429/5xx (lines 76-116); `GenerateReply` also loops over both models in `Models` until one succeeds (lines 46-58) — a hardcoded model-level fallback independent of the studio-configurable one in §2 |
| **Groq** | `apps/api/internal/integrations/groq/client.go` | `Model8B = "qwen/qwen3.8-27b"`, `Model70B = "openai/gpt-oss-120b"` (lines 16-18) | `https://api.groq.com/openai/v1/chat/completions` (line 13) | `Authorization: Bearer` header; env `GROQ_API_KEY` (`config.go:253`) or per-studio `GroqAPIKey` | 30s (line 29) | 2 attempts, linear backoff (`attempt * 1s`) on transport error/429/5xx (lines 57-79) |
| **Mistral** | `apps/api/internal/integrations/mistral/client.go` | `mistral-ocr-latest` (line 21) — **OCR only**, not part of the chat waterfall | `https://api.mistral.ai/v1/ocr`, `/v1/models` (lines 19-20) | `Authorization: Bearer`, key passed per call; **no platform env var found** (unverified beyond grep) | 60s (line 29) | None |

No circuit breaker exists anywhere in the codebase — failures are logged, not tripped into an open state.

---

## 2. Model resolution & the provider fallback chain ("waterfall")

Two separate configuration mechanisms exist, both under `apps/api/internal/integrations/llm/`:

- **`ai_task_configs`** (platform-scoped, one provider+model per named admin "purpose", e.g. CRM doc parsing — see §7) — resolved by `Resolver.ResolveProvider` (`resolve.go:54-104`). No cross-provider fallback here: returns an error if the purpose is unconfigured (line 57-59) or the provider is unsupported (line 102).
- **`studio_ai_models`** (per-studio, per-provider, ordered list of enabled models) — `EnabledModelsForStudio` (`studio_models.go:168-183`) returns the studio's enabled models in `sort_order`, falling back to a hardcoded `DefaultModels` map (lines 37-41: Groq → `{Model8B, Model70B}`, Gemini → `gemini.Models`, Claude → `{claude.DefaultModel}`) if the studio has configured nothing.

**The actual chat-reply fallback chain** ("try Groq, then Gemini, then Claude") lives in the messaging layer, not the `llm` package: `apps/api/internal/messaging/ai_worker.go:307-377` (`llmWaterfallUncached`), duplicated inline for the live-reply path at lines 1387-1490. For each provider it tries, it loops through that provider's `EnabledModelsForStudio` list.

- **Fallthrough trigger**: `gerr != nil || strings.TrimSpace(reply.Text) == ""` — any error *or* an empty response text. There is no differentiation between error classes (timeout vs. 429 vs. auth failure vs. content filter) — all advance to the next model/provider identically.
- Every attempt (success or failure) is logged via `msgRepo.LogLLMUsage` (see §6, `llm_usage_logs`).
- Admins can reorder/enable/disable per-provider models via `PATCH /studios/{studioId}/ai-models/{provider}/order` (`ai_models_http.go:428`, backed by `SetStudioModelOrder`, `studio_models.go:128-146`).

### Studio-side AI model configuration (`ai_models_http.go`)

| Endpoint | Purpose |
|---|---|
| `GET .../ai-models` | list providers/models/keys for a studio (line 114) |
| `PUT .../ai-models/{provider}/key` | save an API key (line 190) |
| `POST .../ai-models/{provider}/test-key` | validate a key live (line 232) |
| `POST .../ai-models/{provider}/models` | add + test a model (line 313) |
| `PATCH .../ai-models/{provider}/models/{modelId}` | enable/disable a model (line 384) |
| `PATCH .../ai-models/{provider}/order` | reorder fallback priority (line 428) |
| `DELETE .../ai-models/{provider}/models/{modelId}` | remove a model (line 472) |

**Security note (factual, not an opinion):** per-studio provider API keys (`gemini_api_key`, `groq_api_key`, `claude_api_key`, `mistral_api_key`) are stored **in plaintext** in the `studios` table (comment `ai_models_http.go:40`; column map `repo.go:449-454`; `UpdateAIProviderKey`, `repo.go:460-474`).

---

## 3. Embeddings

**Current embedding model/provider is a local Python sidecar, not a cloud API.**

- `apps/api/internal/integrations/embeddings/client.go:1-6` — package comment states this service "replaced Gemini's embedContent API." No API key; only a base URL, `EMBEDDINGS_SERVICE_URL` (default `http://localhost:8001`, `cmd/backfill-embeddings/main.go:31-36`).
- Call shape: `POST <url>/embed` with `{text, is_query}` (`client.go:39-42,48-51,64`). `is_query` selects an **e5-family** `"query: "` / `"passage: "` prefix server-side (comment, lines 48-50).
- The exact model ID/name is **not present in the Go code** — only the e5-family hint. The Python service itself was not inspected in this pass (**unverified**).
- Vector dimension is pinned at the DB level to the *old* Gemini model: `embedding vector(768)` with comment "Gemini text-embedding-004 = 768 dims" (`migrations/20260604000001_studio_knowledge_chunks.sql:13`; same on `messages.embedding`, `migrations/20260624000001_rag_upgrade.sql:23-24`). The local service must itself produce 768-dim vectors to stay compatible — **not verified from the Go side**.

Gemini is still used elsewhere, but only for **generation**, not embeddings: `apps/api/internal/studios/embeddings.go` — `ClassifyIntent` (lines 112-133), `RerankChunks` (lines 201-227), `ExpandQuery` (lines 322-328), all hitting Gemini's `generateContent`.

### What gets embedded

| Source | Where | Function |
|---|---|---|
| Knowledge-base text + uploaded KB files | `studios/service.go:562-660` (`asyncSyncKnowledgeChunks`) | chunks at 800 chars / 150 overlap, then `EmbedPassage` per chunk (line 634), saved to `studio_knowledge_chunks` |
| OCR'd images | **Not embedded directly** — `knowledge_base_ocr.go` only extracts/returns text for admin review (see §8); embedding happens later, only if the admin saves the reviewed text through the normal KB save path |
| Customer inbound message (query) | `messaging/ai_worker.go:772` | `EmbedQuery` on the expanded query, for RAG retrieval |
| Customer inbound message (raw, async) | `messaging/ai_worker.go:783` | `EmbedPassage`, persisted via `SaveMessageEmbedding` for future semantic-history search |
| Staff reply bodies ("style learning" corpus) | `messaging/style_worker.go:226,266` | `EmbedPassage`, builds few-shot style-matching corpus |
| Admin test-chat query | `messaging/test_chat.go:92` | `EmbedQuery` |

Not found: leads or FAQs embedded as standalone objects — FAQ-like content only enters via knowledge-base chunks.

### Storage & retrieval

- **pgvector** extension (`migrations/20260604000001_studio_knowledge_chunks.sql:3`).
- `studio_knowledge_chunks.embedding vector(768)` with an HNSW cosine index (`migrations/20260624000001_rag_upgrade.sql:6-9`), plus a generated `content_tsv` + GIN index for hybrid BM25/semantic search (lines 12-19).
- `messages.embedding vector(768)` with its own HNSW cosine index (lines 23-28), plus `intent`, `sentiment`, `sentiment_confidence` columns (lines 31-33).
- Similarity query: pgvector cosine-distance operator `<=>`, similarity computed as `1 - (embedding <=> $x::vector)` (`studios/repo.go:671,706,746,1199`; `messaging/repo.go:1192-1242` `SearchSemanticHistory`, and line 1282-1298).
- An older migration (`20260507000001_messaging.sql:13-20,275-276`) shows pgvector being enabled conditionally with an `embedding_text TEXT` fallback for environments without pgvector — predates the 768-dim schema above.

### `cmd/backfill-embeddings`

One-shot operator CLI (`go run ./cmd/backfill-embeddings`) run after the Gemini→local-service embedding migration, because mixing vectors from two different models in one table silently produces wrong similarity rankings (header comment, lines 1-6). It:
1. Connects to the embeddings service and sanity-checks connectivity (lines 65-68).
2. Re-embeds **every** row in `studio_knowledge_chunks` (line 72) and updates in place.
3. Re-embeds only `messages` rows that **already have** a (stale) embedding and non-empty body (`WHERE embedding IS NOT NULL AND body != ''`, line 112) — leaves never-embedded messages to the live async workers.
4. Safe to re-run; logs progress every 50 rows, exits non-zero if rows were skipped (lines 99-150).

---

## 4. Messaging AI pipeline (the core conversational AI)

All files under `apps/api/internal/messaging/`.

### 4.1 `ai_worker.go` — the live auto-reply engine

- **Trigger**: event-driven, not polled. `AIWorker.Run` subscribes per-studio to an in-process event bus (lines 77-119); `listenStudio` dispatches `handleMessage` on `EvtMessageReceived` (lines 121-160).
- **Gating** before a reply is generated (`handleMessage`, line 446): must be customer-inbound (454-457), conversation must exist with `AIEnabled=true` (497-500), no DND/stop-keyword (509-524), and no active hand-off to the "auto-contact" bot flow (526-560).
- **Prompt construction** (`buildPrompt`, ~lines 1565-1860): a hardcoded system role ("warm, professional sales assistant for a fitness studio" + rules, 1568-1574), grounded current date/timezone (1576-1627), program-schedule math (1596-1636), knowledge-base RAG chunks (1658-1666), membership plans (1674-1682), lead context — name/status/plan interest (1684-1737), the studio's distilled "style profile" + few-shot examples (1796-1827), semantic/older-history matches (1829-1847), and the recent conversation transcript (1849+).
- **Model**: the Groq→Gemini→Claude waterfall from §2, using the studio's configured `EnabledModelsForStudio` list (not hardcoded).
- **Response handling**: **fully automatic — not a human-reviewed draft.** The reply is written straight to `outbound_jobs` (`EnqueueOutbound`, `ScheduledFor: now+20s`, lines 1514-1521), then `EvtOutboundJobEnqueued` is published. There is no approval step in this path.
- **Keyword-based intent override** (independent of any LLM): phrases like "become a member"/"sign me up" force `intent = "ready_to_buy"` *before* LLM classification runs, specifically so it still works when Gemini isn't configured (lines 720-730, 763-766).

### 4.2 `style_worker.go` — tone/style distillation

- Runs as an **independent background worker**, not inline per-message. Ticks every 5 minutes (`stylePollInterval`, line 26); rebuilds a studio's "communication style" profile once ≥20 new staff-authored replies have accumulated (`styleRefreshThreshold`, line 29), sampling up to 100 recent replies (line 32), via the same Groq→Gemini→Claude waterfall (line 340).
- The resulting profile (tone, phrasing, emoji use, pricing-objection handling) is a stable anchor injected once per refresh into `buildPrompt` — distinct from the per-reply few-shot retrieval (`SearchStyleExamples`) done live inside `ai_worker.go`.
- Also embeds staff replies for that retrieval corpus (`embedStaffReply`/`embedUnembeddedStaffReplies`, lines 201-274) and listens for `EvtMessageSent`/`EvtWAWebBackfillDone` (lines 108-131).

### 4.3 `answer_cache.go` — LLM response caching

- Caches the final answer text + which provider/model served it, keyed `studio:<studioID>:ai:<kind>:<sha256(prompt)>` (lines 35-45) — namespaced by `kind` (`"incoming_reply"`, `"test_chat"`, `"style_profile"`, `"summary"`).
- Redis-backed via a fixed TTL passed at construction (lines 26,31); a nil cache (no Redis configured) is always a no-op miss (lines 20-23,50-53,69-71).
- Explicit purpose per the doc comment: avoid "paying for and waiting on another round through Groq/Gemini/Claude" for an identical prompt — cost + latency savings (lines 14-19).
- **Unverified**: no active invalidation (e.g. on knowledge-base update) was found beyond TTL expiry.

### 4.4 `class_escalation.go` — human hand-off logic + KPI

"Escalation" = disabling AI auto-reply for a conversation and handing it to a human staff member (`EscalateAndNotify`, `service.go:2565-2582` — sets `ai_enabled=false` + `escalated_at`/`escalated_reason`, best-effort emails the studio).

Deterministic (non-LLM) triggers:
- Class-action requests the AI can't fulfill (book/cancel/reschedule + class noun) (lines 44-50).
- A class question combined with weak/no knowledge-base coverage (`kbCoversQuestion`, deterministic keyword-overlap, no embeddings/LLM, lines 212-234).
- A complaint/negative-feedback phrase list (lines 67-94).
- The model's own reply admitting it doesn't know (`looksLikeAIDoesNotKnow`) — a provider-agnostic fallback for studios without Gemini (lines 143-171).
- "Knowledge gap" hand-off (`escalateKnowledgeGap`, lines 124-141), which also logs to a "Needs Answers" admin tab.

One LLM-classification trigger (low-confidence Gemini intent classification) is referenced in a comment (lines 159-162) but its implementation lives in `ai_worker.go` outside the sections read for this doc — **flagged as not independently line-verified**.

A second, explicit non-LLM "ready_to_buy" fallback lives here too: `looksReadyToBuy` (lines 239-258) — a plain substring match against purchase-intent phrases, documented as covering the gap because ready_to_buy classification normally needs Gemini "and simply never runs without it" (comment, lines 239-242). Both this and the `ai_worker.go` keyword override above exist because of a real incident where a Gemini-less studio's bot fabricated a payment-link promise with no actual escalation (comment, `ai_worker.go:1148-1152`).

**Escalation KPI**: `GetEscalationStats` (`repo.go:814-828`) counts currently-escalated vs. resolved conversations, exposed at `GET /api/v1/studios/{studioId}/messaging/escalations/stats` (`http.go:1452-1470`), documented as a "Dashboard KPI snapshot."

### 4.5 `llm_stats.go` — operational telemetry (not a quality eval — see §6)

Every waterfall attempt is logged via `LogLLMUsage` to `llm_usage_logs` (provider, model, latency_ms, success, tokens_in, tokens_out, created_at). `GetLLMStats` (lines 47-127) aggregates the last 30 days by provider/model/day and per-studio, exposed at `GET /api/v1/admin/llm-stats`, super-admin only (lines 36-46,129-133). This measures volume/success-rate/latency/tokens — **not** answer correctness or relevance.

### 4.6 `test_chat.go` — admin sandbox

`AIWorker.TestChat` (lines 63-153) is an explicit no-DB-write sandbox ("no database writes and no dependency on a real conversation/lead", lines 53-62), used by the Knowledge Base admin page's test-chat drawer. It runs the identical pipeline as a live reply — classify intent, expand query, hybrid-search KB chunks, rerank, retrieve style examples, build prompt, run the waterfall (cache kind `"test_chat"`) — but returns `(reply, sources, err)` with nothing persisted. Exposed at `POST /api/v1/studios/{studioId}/knowledge-base/test-chat`.

### 4.7 Which inbound channels trigger the AI vs. store-only

| Source | Behavior |
|---|---|
| WA-Web live inbound | `EvtMessageReceived` if not from staff → **triggers AI** |
| Telegram-Web live inbound | same split → **triggers AI** |
| Twilio SMS/WhatsApp | always `EvtMessageReceived` → **triggers AI** |
| X/Twitter DM | always `EvtMessageReceived` → **triggers AI** |
| Meta (WhatsApp Cloud/Messenger/Instagram) | also publishes `EvtMessageReceived` → **triggers AI** (noted for completeness; not in the original file list reviewed) |
| WA-Web / TG-Web **backfill** (historical import) | store-only; no `EvtMessageReceived` seen at the one site inspected (`service.go:1735-1753`) — instead triggers `EvtWAWebBackfillDone`, which runs a *summarization* pass, not a reply (`ai_worker.go:148-156`). **Not exhaustively verified** across every backfill code path. |

---

## 5. Decision tree engine (`apps/api/internal/decisiontree`)

Rule-based conversation-flow control per studio, scoped to the lead's pipeline status. `TraverseActiveTree` (`service.go:444`) picks the best active tree for the lead's status and walks it per message, carrying `currentNodeID` to continue mid-flow across turns (lines 438-464).

- **Matching is 100% rule-based, no LLM at runtime.** `traverseNodes` (lines 481-522) matches `ConditionKeyword`/`ConditionIntent`/`ConditionSentiment`/`ConditionLeadStatus` via local heuristics: `detectIntent` (keyword-table, lines 586-602/575-583) and `detectSentiment` (positive/negative word lists, lines 551-572). `matchKeyword` does whole-word plus Levenshtein fuzzy-typo matching (lines 631-725).
- **Nodes**: `ConditionType` (keyword/intent/sentiment/leadStatus/default, validated lines 773-775), `ConditionValue`, `ReplyTemplate`, `Action` (reply/escalate_human/book_trial/send_link, lines 776-778); `ActionChangeStatus` can also move the lead's pipeline status.
- **The one LLM usage in this package**: `SuggestKeywords` (lines 32-80) — an admin-authoring helper that asks Groq to suggest trigger keywords for a node being edited, falling back to a non-LLM word-frequency heuristic `extractKeywordsLocally` (lines 103-170) whenever no Groq key is set or the call/JSON-parse fails (lines 33-36,48-50,59-61,76-78) — this fallback is never surfaced as an error.
- `http.go` exposes CRUD, bulk import/export, a `simulate` dry-run endpoint (lines 525-548), and `suggestKeywords` (lines 456-469).

---

## 6. Evals — explicitly searched, **not found**

A repo-wide search (excluding `node_modules`/`.next`/vendor/`.git`) for `eval|benchmark|golden|regression|scoring|accuracy|ground_truth|testset|test_set|llm_stats|judge` turned up only: third-party vendor code inside a Python venv (`apps/embeddings/venv/.../sentence_transformers/evaluation/*`), unrelated product copy ("lead scoring", "sentiment scoring" in `init.md`/`docs/PLATFORM_GUIDE.md`), and Go identifiers that coincidentally match the substring (`WriteValidationError`, `twilioSignatureValid`). None of these is an eval harness.

- No `eval/`, `evals/`, or `benchmarks/` directory exists outside the Python venv. The root `Makefile` has zero eval-related targets.
- `llm_stats.go` (§4.5) is operational telemetry only — "success" means the API call didn't error, not that the output was judged good.
- Test files: `claude/`, `gemini/`, `groq/`, `mistral/` packages have **zero** `*_test.go` files. `llm/` has `resolve_test.go`, `studio_models_test.go`. `embeddings/client_test.go` only checks that `is_query` is set correctly against a mocked server — not retrieval quality. `messaging/*_test.go` (~27 files) are standard unit tests with mocked HTTP/DB; `class_escalation_test.go`'s `TestIsClassActionRequest` uses real prod customer messages as table-test cases, but tests the deterministic keyword classifier, not LLM output.

**Conclusion: there is usage/latency/cost-adjacent telemetry for LLM calls, but no golden-dataset eval suite, no prompt-regression testing, no LLM-as-judge, and no human-feedback or correctness scoring anywhere in the repo.**

---

## 7. CRM integrations — AI-assisted doc parsing

File: `apps/api/internal/integrations/crm/parse.go`, UI: `apps/web/src/app/admin/crm-integrations/AiSettingsCard.tsx`.

- One admin-configurable AI task, `crm_doc_parsing` (`parse.go:14`), lets a super-admin pick which provider/model (Claude, Gemini, or Groq) handles it, plus an optional per-task API-key override, via the generic `ai_task_configs` GET/PUT infra from §2 (`http.go:356-403`).
- `ParseCRMDoc` (lines 54-109) sends an uploaded CRM API doc (capped at 300KB) to `provider.Analyze(...)` — a real LLM call, not regex. The system prompt (lines 111-141) asks the model to map the CRM's endpoints onto a fixed 6-operation catalog (`create_lead`, `register_user`, `purchase_membership`, `find_membership_plan`, `get_member`, `list_bookings` — `operations.go:29-65`) and detect base URL/auth scheme, returning strict JSON (parsed with `json.Unmarshal` after stripping markdown fences, lines 146-158).
- Result is stored as a **draft** provider + operations, left unreviewed until a super-admin activates it (lines 69-106).
- `AiSettingsCard.tsx` exposes a provider dropdown, a model dropdown populated from `LLM_MODELS[provider]`, a password-masked optional API-key override ("leave blank to use the platform's key"), and a read-only summary badge.
- **Unverified / out of scope for this pass**: full route wiring in `crm/http.go` and `actions.ts` beyond the AI-settings portion; whether any other CRM code path calls an LLM.

---

## 8. Knowledge-base OCR

File: `apps/api/internal/studios/knowledge_base_ocr.go` (94 lines, read in full).

- Engine: **Mistral OCR**, via the studio's own stored Mistral API key (`mistral.New().OCRImage(...)`, line 87) — not a shared platform key.
- Accepted inputs: images only — `.jpg`, `.jpeg`, `.png`, `.webp` (line 18), max 20MB (line 43). **No PDF/scanned-document support.**
- Endpoint: `POST /api/v1/studios/{studioId}/knowledge-base/ocr`.
- Flow: per the handler's own doc comment (line 24), this endpoint **only extracts and returns text** (`{"text": text}`, line 93) for the admin to review/edit. It explicitly does **not** save to the knowledge base or trigger embedding itself — that only happens if the admin submits the reviewed text through the normal KB save path, which runs `asyncSyncKnowledgeChunks` (§3).

---

## 9. Open items flagged as unverified (do not treat as fact without further digging)

- The exact model name/ID and vector dimension guarantee of the local embeddings Python service (only inferable from an e5-prefix code comment and the DB's 768-dim schema; the Python service itself wasn't read).
- Whether any dollar-cost tracking exists anywhere for LLM usage (only raw token counts were found in `llm_usage_logs`).
- The full implementation of the "classifier-driven low-confidence escalation" referenced only in a comment in `class_escalation.go:159-162`.
- Whether `AnswerCache` has any invalidation path beyond Redis TTL expiry.
- Full behavior of WA-Web/TG-Web backfill handlers beyond the one call site inspected — specifically whether any backfill path could inadvertently trigger a live AI reply.
- Whether a `MISTRAL_` platform-level env var exists anywhere outside the per-studio key column (grepped, not found, but not exhaustively).
