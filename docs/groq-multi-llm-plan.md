# Groq Multi-LLM Integration Plan

## Goal
Replace Claude/Gemini as primary AI with Groq's Llama models.
Cheaper, faster, same Groq infrastructure for all models.
Gemini only as last-resort fallback.

---

## Model Waterfall (in order)

```
Incoming message
      │
      ▼
Llama 3.1 8B (Groq) ──fail──► Llama 3.3 70B (Groq) ──fail──► Gemini 2.5 Flash ──fail──► Fallback message
      │                               │                               │
   $0.0001/msg                   $0.001/msg                    rate-limited
   ~80% of traffic               ~15% of traffic               ~5% of traffic
```

**When to escalate from 8B → 70B:**
- Groq returns HTTP 429 (rate limit) or 5xx (server error)
- Response is empty or too short (< 20 chars)
- No escalation based on content — keep it simple

**When to escalate from 70B → Gemini:**
- Groq is completely down (both models fail)

**When to send fallback message:**
- All providers fail
- Message: *"Thank you for reaching out to [Studio Name]! Our team will get back to you as soon as possible."*

---

## Cost Estimate

| Messages/month | 8B only | With 70B fallback | Old (Claude Sonnet) |
|----------------|---------|-------------------|---------------------|
| 1,000          | $0.10   | $0.12             | $5.00               |
| 10,000         | $1.00   | $1.20             | $50.00              |
| 100,000        | $10.00  | $12.00            | $500.00             |

**~40x cheaper than Claude Sonnet.**

---

## What Needs to Be Built

### Step 1 — Groq Client (`apps/api/internal/integrations/groq/client.go`)

New package, similar to the existing Claude client.

```
Package: groq
Struct:  Client { url, key, httpClient }
Method:  GenerateReply(ctx, prompt, model) (string, error)
```

- Base URL: `https://api.groq.com/openai/v1/chat/completions`
- Auth header: `Authorization: Bearer <GROQ_API_KEY>`
- Request format: OpenAI-compatible JSON
  ```json
  {
    "model": "llama-3.1-8b-instant",
    "messages": [{"role": "user", "content": "<prompt>"}],
    "max_tokens": 512,
    "temperature": 0.7
  }
  ```
- Response: parse `choices[0].message.content`
- Retry: 2 attempts on 429/5xx with 1s backoff

Models to support:
- `llama-3.1-8b-instant` (fast, cheap — primary)
- `llama-3.3-70b-versatile` (better quality — fallback)

### Step 2 — Wire Groq into AI Worker (`apps/api/internal/messaging/ai_worker.go`)

Add `groq *groq.Client` field to `AIWorker` struct.

New reply generation order (replacing current logic at line ~461):

```go
// 1. Try Groq 8B
resp, err = w.groq.GenerateReply(ctx, prompt, "llama-3.1-8b-instant")
if err != nil || resp == "" {
    // 2. Try Groq 70B
    resp, err = w.groq.GenerateReply(ctx, prompt, "llama-3.3-70b-versatile")
}
if err != nil || resp == "" {
    // 3. Try Gemini (existing code)
    if apiKey != "" {
        resp, err = w.generateGeminiReply(ctx, apiKey, prompt)
    }
}
if err != nil || resp == "" {
    // 4. Fallback message
    enqueue fallback "Our team will get back to you..."
    return nil
}
```

Remove Claude from the chain entirely (or keep as optional 4th tier if needed).

### Step 3 — Config (`deploy/docker-compose.yml` + `deploy/.env`)

Add to `x-api-env` block:
```yaml
GROQ_API_KEY: ${GROQ_API_KEY:-}
```

Add to `deploy/.env`:
```
GROQ_API_KEY=gsk_xxxxxxxxxxxxxxxxxxxx
```

### Step 4 — Initialize Client in Server Startup (`apps/api/cmd/server/main.go`)

Find where Claude client is initialized, add Groq next to it:
```go
groqClient := groq.New(os.Getenv("GROQ_API_KEY"))
// pass groqClient to AIWorker constructor
```

---

## What Does NOT Change

- Decision tree logic — still runs first, before any LLM
- Auto-contact automation (booking flow) — no LLM involved
- RAG / knowledge base retrieval — still uses Gemini embeddings
- Gemini embedding API — only the generation step changes
- All existing WhatsApp/Meta webhook handling

---

## Files to Create/Modify

| File | Action |
|------|--------|
| `apps/api/internal/integrations/groq/client.go` | CREATE — Groq HTTP client |
| `apps/api/internal/messaging/ai_worker.go` | MODIFY — new provider waterfall |
| `apps/api/cmd/server/main.go` | MODIFY — init Groq client |
| `deploy/docker-compose.yml` | MODIFY — add GROQ_API_KEY env var |
| `deploy/.env` | MODIFY — set actual key (on EC2) |

---

## How to Get a Groq API Key

1. Go to **console.groq.com**
2. Sign up / log in
3. Go to **API Keys → Create API Key**
4. Copy the key (starts with `gsk_`)
5. Free tier: 14,400 requests/day on 8B, 1,000 req/day on 70B
6. Paid: $0.05/$0.08 per 1M tokens (8B), $0.59/$0.79 per 1M tokens (70B)

---

## Implementation Order

1. Get Groq API key
2. I build `groq/client.go`
3. I update `ai_worker.go` with new waterfall
4. I update server startup + docker-compose
5. Build binary → deploy to EC2
6. Test: send WhatsApp message → confirm Groq 8B replies
7. Force 8B to fail → confirm 70B takes over
8. Force both to fail → confirm Gemini fallback
9. Force all to fail → confirm fallback message

---

## Summary

| Before | After |
|--------|-------|
| Gemini (free tier, rate limits) → Claude (no credits) → silence | Groq 8B → Groq 70B → Gemini → fallback message |
| Unreliable | Always responds |
| $5-50/10K msgs | $1.20/10K msgs |
| Two separate providers to manage | One Groq account covers both models |
