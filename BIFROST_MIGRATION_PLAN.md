# Migrating to Bifrost — full cutover plan

Goal: replace the three hand-written provider clients (`integrations/groq`,
`integrations/gemini`, `integrations/claude`) and the duplicated Groq→Gemini→Claude
waterfall logic in `ai_worker.go` with a single [Bifrost](https://docs.getbifrost.ai/)
gateway — one self-hosted Go service, one OpenAI-compatible HTTP endpoint, with
fallback/retry handled by the gateway instead of copy-pasted Go code.

No code has been changed yet. This is the plan to review before anything is touched.

---

## Why Bifrost fits this codebase specifically

- **Go-native, self-hosted, single container** — `docker run`/`docker-compose`, no new
  language or managed dependency, consistent with how `apps/embeddings` is already run
  as a sidecar.
- **One OpenAI-compatible endpoint fronts Groq, Gemini, and Claude** — this directly
  replaces `groq/client.go`, `gemini/client.go`, `claude/client.go` and the `llm`
  package's hand-rolled `Provider`/`Resolver` abstraction (`provider.go`, `resolve.go`).
- **Fallbacks are per-request, not static config** — a request body carries
  `"model": "groq/qwen/qwen3.8-27b", "fallbacks": ["groq/openai/gpt-oss-120b", ...]`.
  That's a direct structural match for what `studio_ai_models`/`DefaultModels` already
  compute today for *model* fallback within one provider.
- **`x-bf-direct-key` header** — lets each request carry a raw provider key that bypasses
  Bifrost's own key pool entirely. This is a structural match for the current BYOK model
  (`studios.groq_api_key` / `gemini_api_key` / `claude_api_key`, read from Postgres per
  request) — confirmed as the chosen key strategy for this migration, no change to how
  studios manage their own keys.
- **⚠️ Correction after reading Bifrost's own docs**
  ([supported-headers](https://docs.getbifrost.ai/providers/supported-headers)):
  *"The direct key is used on the first provider in the chain, including its retries.
  Fallbacks use the gateway's configured keys."* This means `x-bf-direct-key` does
  **not** carry across a cross-*provider* fallback — if a studio's Groq call fails and
  Bifrost falls over to Gemini, that Gemini call would use Bifrost's own pool key, not
  the studio's own Gemini key. It **does** still cover same-provider model retries
  (e.g. Groq's qwen → gpt-oss-120b), since that's "the first provider ... including its
  retries." **Revised approach** (see Phase 4): the Go code keeps doing the
  *cross-provider* loop it does today (try Groq entirely, including its model fallback
  list, via one Bifrost call with that provider's direct key; if the whole provider
  fails, issue a second Bifrost call for the next provider with *its* direct key) —
  Bifrost replaces the 3 hand-written client packages and handles in-provider
  model-level retry/fallback, but cross-provider looping stays in Go, because that's
  the only way to keep each provider using that specific studio's own key.
- Fixes a real gap found in the earlier audit for free: `ClassifyIntent`/`ExpandQuery`/
  `RerankChunks` in `studios/embeddings.go` are currently **Gemini-only, no fallback** —
  once they go through Bifrost, they can carry the same fallback array as the main reply
  path with zero extra Go logic.

Sources: [Bifrost quickstart](https://docs.getbifrost.ai/quickstart/) ·
[Supported headers](https://docs.getbifrost.ai/providers/supported-headers) ·
[Retries & fallbacks](https://docs.getbifrost.ai/features/retries-and-fallbacks.md) ·
[Governance/virtual keys](https://www.getmaxim.ai/resources/governance)

---

## Current state (what's being replaced)

| Piece | File | Role |
|---|---|---|
| Groq client | `apps/api/internal/integrations/groq/client.go` | Fixed key at construction, `GenerateReply(ctx, prompt, model)` |
| Gemini client | `apps/api/internal/integrations/gemini/client.go` | Key passed per-call (BYOK), `Models` fallback list, used both for chat and for classify/expand/rerank |
| Claude client | `apps/api/internal/integrations/claude/client.go` | Fixed key/URL at construction, used for chat + `Analyze` (CRM doc parsing) |
| `llm.Provider`/`Resolver` | `apps/api/internal/integrations/llm/{provider,resolve,studio_models}.go` | Only used today by CRM-doc-parsing (`ai_task_configs`); holds `DefaultModels`, the per-studio model list logic we want to keep |
| Waterfall (reply) | `ai_worker.go:1433-1526` | Inline Groq→Gemini→Claude loop, duplicated in `llmWaterfall`/`llmWaterfallUncached` (`ai_worker.go:337-445`) for summarization/style |
| Gemini-only calls | `studios/embeddings.go:113` (classify), `:348` (expand), `:202` (rerank) | No fallback — single Gemini call, silent failure |
| Groq-only hardcoded | `decisiontree/service.go:32` | Bypasses `studio_ai_models`, uses `groq.Model8B` directly |
| Usage logging | `messaging/repo.go:2942` `LogLLMUsage` → `llm_usage_logs` | Metadata only (provider/model/latency/success/tokens), no prompt/response text — **keep this contract unchanged** |

---

## Target architecture

```
ai_worker.go / style_worker.go / embeddings.go / decisiontree/service.go / crm/parse.go
                              │
                              ▼
                 apps/api/internal/integrations/bifrost  (new package)
                              │  one HTTP client, OpenAI-compatible
                              ▼
                      Bifrost container (new service)
                              │  x-bf-direct-key per request
                              ▼
              Groq API  /  Gemini API  /  Claude API
```

- `groq/`, `gemini/`, `claude/` packages are deleted once every call site is migrated.
- `llm.Provider`/`Resolver`/`studio_models.go` logic (per-studio model list, defaults)
  is **kept** — it moves from "pick a client package and call it" to "build a Bifrost
  request body (model + fallbacks + direct-key header)".
- `LogLLMUsage` call sites stay exactly where they are — Bifrost's response still
  reports which provider/model actually served the request and the token counts.

---

## Migration phases (each independently shippable and testable)

### Phase 0 — Stand up Bifrost in dev, prove one call works — ✅ DONE

- Added a `bifrost` service to the root `docker-compose.yml`
  (`127.0.0.1:${BIFROST_PORT:-8090}:8080`), file-only config at
  `deploy/bifrost/config.json` (git-tracked; its runtime sqlite dbs are
  gitignored via `deploy/bifrost/.gitignore`), wired into `make db-up`/`make
  bifrost-logs`. No app code touched.
- Real provider names confirmed by running the actual binary (v2.3.2) against a
  candidate config: `groq`, `gemini`, `anthropic` (not `claude`) — all accepted with
  `"keys": []` and `"allow_direct_keys": true`, and `"model discovery skipped ...: no
  keys configured"` is a warning, not a startup failure, confirming direct-key-only
  providers (no platform pool key) are a supported, working setup.
- Verified end-to-end with a fake key — `curl` through `x-bf-direct-key: true` +
  `Authorization: Bearer <fake>` to `/v1/chat/completions` returned a real 401
  **from Groq itself** (`"key":"header-provided"`, real round-trip latency), proving
  the header plumbing works structurally without needing a real studio key yet.
- Verified the cross-provider-fallback risk from the docs empirically: adding
  `"fallbacks": ["gemini/gemini-flash-latest"]` to that same request did **not** fall
  through to Gemini on Groq's 401 — Bifrost returned the Groq error directly. (Didn't
  get to confirm what happens on a 5xx/timeout-triggered fallback with no Gemini key
  configured; expect a clean "no keys configured" failure given the empty `keys: []`,
  consistent with Decision #1 below, but flag as unverified until Phase 4.)
- **New finding, not in the original plan**: file-only config mode (`config_store.
  enabled: false`) disables Bifrost's own auth middleware entirely —
  confirmed from its boot log: `"auth middleware requires config store, skipping auth
  middleware initialization"`. Bifrost's HTTP API has **zero built-in authentication**
  in this mode. Fine for dev bound to `127.0.0.1`; **must never be host-published in
  prod** (Phase 7) — reachable only from the `api` container over the internal Docker
  network, same trust model as `embeddings`.
- **Exit criteria met**: real compose-deployed service, real round-trip to Groq's API
  confirmed via direct-key header.

### Phase 1 — New Go client package, used nowhere yet — ✅ DONE
- `apps/api/internal/integrations/bifrost/client.go`: `Client.Complete(ctx, Request)
  (Reply, error)`. `Request` carries `Provider`/`Model`/`Fallbacks` (same-provider
  only, per Phase 0's direct-key finding), `DirectKey`, `SystemPrompt`/`Prompt`,
  `MaxTokens`, `Temperature`. `Reply` adds a `ServedByModel` field (from Bifrost's
  `extra_fields.resolved_model_used`) so callers — and eventually `LogLLMUsage` — can
  record which model actually served a request even after an in-provider fallback.
- 9 unit tests against a local `httptest` server (same pattern as
  `integrations/embeddings/client_test.go`, the only existing precedent): direct-key
  headers sent/omitted correctly, `model`/`fallbacks` are provider-qualified
  (`"groq/qwen/qwen3.8-27b"`, not bare model names), reply/usage parsing, non-OK
  status handling, empty-choices handling, nil-receiver guard.
- Verified in a real Go 1.24 container (no local Go toolchain in this environment,
  matches `apps/api/go.mod`'s `go 1.24.0`): `go build`, `go vet`, and `go test -v` all
  pass, and `go build ./...` for the whole `apps/api` module still succeeds with the
  new package added — confirms nothing else broke and it's genuinely unused
  elsewhere yet (`grep` for `integrations/bifrost` outside its own directory: 0 hits).

### Phase 2 — Migrate the lowest-risk call site first: CRM doc parsing — ✅ DONE
- `llm/provider.go`: the 3 adapters (`claudeAdapter`/`geminiAdapter`/`groqAdapter`)
  collapsed into one `bifrostAdapter` calling `bifrost.Client.Complete`. `Provider`'s
  interface (`Analyze(ctx, systemPrompt, document)`) is unchanged, so `crm/parse.go`
  needed **zero changes** — confirmed by grep, it only ever depended on the interface.
  `maxTokens` is now fixed at 4096 for every provider (previously only Claude had an
  explicit cap at that value; Groq was hard-capped at 512 and Gemini had no cap) — a
  deliberate, documented improvement given this task can produce a large structured
  JSON mapping, not a silent side effect.
- `llm/resolve.go`: `Resolver` now holds a `*bifrost.Client` + the platform Claude API
  key (a raw string, not a constructed client) instead of 3 provider clients.
  `providerBifrostName` maps this app's `ProviderName` (`"claude"`/`"gemini"`/`"groq"`,
  matching `ai_task_configs`/`studio_ai_models` DB values — unchanged) to Bifrost's own
  provider id (`"anthropic"`/`"gemini"`/`"groq"`). Per-provider direct-key resolution
  logic (override key, else platform Claude key / platform Gemini setting / nothing for
  Groq) preserved exactly as before.
- `cmd/server/main.go`: added `bifrostServiceURL()` (same pattern as
  `embeddingsServiceURL()`, env `BIFROST_SERVICE_URL`, default
  `http://localhost:8090`), constructs one shared `bifrost.Client`, and the
  `llm.NewResolver(...)` call site updated. `claudeClient` itself is untouched and
  still used by `msgHandler`/`aiWorker` — those migrate in Phase 4.
- `llm/resolve_test.go` updated to assert against `*bifrostAdapter`'s `provider`/
  `directKey` fields directly (strictly more verification than the old
  type-per-provider assertions could do) plus a new subtest for the nil-Bifrost-client
  guard.
- **Verified for real, not just compiled**: brought up a fresh dev DB (`make db-up`,
  previous local Postgres volume was gone — ran all migrations + seeded the
  super-admin from scratch), then ran the actual DB-backed
  `TestResolveProvider_Integration` — all 5 subtests pass against real Postgres, not
  skipped. Also ran `go build ./...` and the **full** `go test ./...` for `apps/api`:
  everything passes except one pre-existing, unrelated failure
  (`TestChunkText/Long_text_simple_split` in `internal/studios`, about KB text
  chunking — confirmed via `git status`/`git diff` that neither this phase nor any
  other pending local change touched that file; flagging it here rather than silently
  ignoring it, but it is not this migration's regression).
- **Exit criteria met**: `ParseCRMDoc`'s only dependency (`Provider.Analyze`) is
  satisfied by `bifrostAdapter`, verified by a real integration test rather than a
  manual admin-tool click-through (equivalent coverage: the resolver is the entire
  piece that changed — `crm/parse.go` itself is untouched).

### Phase 3 — Migrate the Gemini-only calls (classify/expand/rerank)
- `studios/embeddings.go`: `ClassifyIntent`, `ExpandQuery`, `RerankChunks` switch from
  calling `gemini.Client` directly to the Bifrost client, **adding a fallback list for
  the first time** (e.g. Gemini → Groq) — a direct fix for the "no fallback" gap found
  in the earlier audit, and especially relevant given Gemini's confirmed month-long
  production outage.
- Test via the admin Test Chat tool (`test_chat.go`), which already exercises this
  exact code path without touching live customer messages.
- **Exit criteria**: Test Chat produces a classified/expanded/reranked result even when
  the configured Gemini key is deliberately broken (proves the new fallback works).

### Phase 4 — Migrate the main reply waterfall
- `ai_worker.go:1433-1526` (`handleMessage`) and `llmWaterfall`/`llmWaterfallUncached`
  (`ai_worker.go:337-445`, used by summarization and style-profile) keep their outer
  Groq→Gemini→Claude loop in Go (per the direct-key limitation above), but each
  provider's own attempt — including its model-level fallback list (e.g. Groq's
  qwen→gpt-oss-120b) — becomes **one Bifrost call** instead of one hand-written HTTP
  call per model. Net effect: 3 client packages collapse into 1, and in-provider
  model retry logic (previously duplicated per package) is handled by Bifrost once.
- Keep the existing post-generation guards unchanged (`looksLikeAIDoesNotKnow`,
  `stripMotivationQuestions`, `guardAgainstPlaceholderArtifact`, etc.) — those operate
  on the final text regardless of which provider served it.
- Keep the "all providers failed → silent drop" behavior for now (`ai_worker.go:1520-1523`)
  rather than changing product behavior as part of an infra migration — flag it as a
  separate follow-up (a customer-visible fallback message), not bundled into this cutover.
- **Exit criteria**: a full reply cycle in Test Chat and then one real studio in dev,
  with `llm_usage_logs` still recording provider/model/latency/tokens correctly for
  calls that went through Bifrost.

### Phase 5 — Migrate the remaining stragglers
- `style_worker.go` (already covered by Phase 4's shared waterfall functions — verify).
- `decisiontree/service.go:32` `SuggestKeywords` — currently hardcoded to
  `groq.Model8B`, bypassing `studio_ai_models` entirely. Migrating it to Bifrost is a
  good moment to also route it through the studio's actual configured model list
  instead of a hardcoded one (small behavior improvement, flag separately for sign-off
  since it changes current behavior, unlike the other phases which are infra-only).

### Phase 6 — Delete the old packages
- Once nothing imports `groq`, `gemini`, `claude` packages directly, delete them along
  with their now-unused tests. `llm/provider.go`'s adapter structs get simplified to a
  single Bifrost-backed implementation.

### Phase 7 — Prod rollout
- Add the same `bifrost` service to the EC2 prod compose stack
  (`/home/ubuntu/studiox/docker-compose.yml`, the "projectx" project — confirmed via
  nginx's `projectx_frontend` network membership as the one actually serving
  `1herosocial.ai`).
- Roll out studio-by-studio if possible (env flag or a per-studio toggle), starting
  with a low-traffic studio (**Trial K**, 38 leads, lowest real usage) before the
  highest-traffic one (**BFT Taman Jurong**, 117K LLM calls/30d).
- Watch `llm_usage_logs` provider/model/latency/failure numbers for the migrated
  studio for a few days before moving the next one.

---

## Decisions (resolved)

0. **Provider naming**: Bifrost's provider id for Claude is `anthropic`, not `claude`
   (confirmed by running the binary — `groq`/`gemini`/`anthropic` are the three
   accepted provider keys in `config.json`). The Go package stays named `claude/` (it's
   this app's own naming for Anthropic's API), but any Bifrost model string uses
   `anthropic/<model>`.
1. **Platform-level keys in Bifrost's `config.json`**: **none.** Matches today's exact
   behavior — no platform-wide Gemini/Claude key exists in dev or prod today
   (`platform_settings` has 0 rows in dev, only an unrelated `platform_plans` row in
   prod). Every provider entry in `config.json` is declared with `allow_direct_keys:
   true` and no `keys` pool; a studio with no key configured for a provider simply
   can't use it, same as today. If this ever needs a platform-wide fallback key, that's
   a deliberate follow-up, not part of this migration.
2. **Claude**: **keep it wired** as the third fallback leg. Zero cost to keep it
   configured (it's a no-op until a studio actually adds a key), and removing it would
   be a product-behavior change bundled into an infra migration — out of scope here.
3. **Config mode**: **file-based `config.json`**, `config_store.enabled: false`. This
   app already owns per-studio model bookkeeping in Postgres (`studio_ai_models`) — a
   second DB-backed config layer inside Bifrost would just be a duplicate, less-visible
   place for the same information to drift out of sync. File-based keeps Bifrost itself
   stateless and its config reviewable in git.
4. **Observability**: `llm_usage_logs` stays the **only** source of truth the app code
   reads and that `LogLLMUsage` writes to. Bifrost's own built-in dashboard can be left
   on for operational debugging (request traces, latency) but nothing in the app should
   read from it — avoids two systems ever disagreeing on "what happened."

---

## Rollback plan

Every phase up to and including Phase 5 leaves the old `groq`/`gemini`/`claude`
packages in the tree and compiling — nothing is deleted until Phase 6. If a migrated
call site misbehaves in prod, reverting that one call site's commit restores the old
direct-client code path immediately, with no dependency on anything else having moved.
Phase 6 (package deletion) should only happen after Phase 7 has been stable in prod for
a studio spanning at least a few days of real traffic.
