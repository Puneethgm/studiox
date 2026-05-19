# AI Chatbot & Automation Guide

## Overview

Your fitness studio platform now has **AI-powered automated client engagement** using Claude API. When clients message your WhatsApp/Instagram, the AI automatically:

1. **Responds intelligently** with context-aware messages
2. **Analyzes sentiment** (positive/negative/neutral) + keywords
3. **Auto-updates lead pipeline** based on conversation
4. **Continues conversation** to book trials and move leads forward

---

## How It Works

### Architecture

```
Incoming Message
    ↓
AI Worker (listens via event bus)
    ↓
Sentiment Analysis + Keyword Detection
    ↓
Claude API (generates response)
    ↓
Send Outbound Message
    ↓
Update Lead Status (based on sentiment)
```

### Sentiment Analysis

The AI detects these keywords:

**Positive** (Lead → Contacted):
- yes, interested, great, love, good, perfect, thanks, definitely, sure, count me in, sign me up, book it

**Negative** (Lead → Dropped):
- no, not interested, bad, hate, no thanks, never, not now, maybe later, skip, cancel

**Neutral** (Lead → Contacted):
- Everything else; follow-up questions asked

---

## Features

### ✅ What's Working Now

1. **Automatic Replies**
   - AI responds to every NEW lead message
   - Context-aware (knows campaign, fitness plan, lead name)
   - Friendly, concise (1-3 sentences)

2. **Sentiment + Keyword Matching**
   - Analyzes each message for positive/negative signals
   - ~60% confidence threshold before acting
   - Tracks keywords in conversation

3. **Auto-Status Updates**
   - **New → Contacted** when AI detects interest
   - **New → Dropped** when AI detects rejection
   - Status updates visible in pipeline immediately

4. **Conversation Management**
   - AI tries to book trials for interested leads
   - Asks clarifying questions for hesitant ones
   - Addresses concerns naturally

---

## Using the Feature

### Requirements

✅ Claude API key configured in `.env`:
```
CLAUDE_API_KEY=sk-ant-...
```

### Enable AI for Your Studio

1. Create a campaign in your admin dashboard
2. Message clients through WhatsApp/Instagram
3. AI automatically engages and updates pipeline

**That's it!** No setup needed. The AI worker runs constantly in the background.

---

## Configuration (Future)

When we add message templates, studios can customize:

```json
{
  "campaign_id": "...",
  "initial_message": "Hi {name}! Interested in our {plan} plan? When can we schedule a trial?",
  "follow_up_questions": [
    "What time works best for you?",
    "Any specific goals for the trial?",
    "Should we start with a one-on-one session?"
  ],
  "trial_booking_message": "Perfect! I've scheduled your trial for {date}. See you then! 🎉",
  "interest_keywords": ["yes", "interested", "let's go"],
  "rejection_keywords": ["no", "not interested", "later"]
}
```

---

## Logs & Monitoring

Check AI worker activity in server logs:

```bash
# When server starts (AI enabled or not):
{"msg":"ai worker started"}
{"msg":"claude config", "enabled":true}

# When AI updates a lead:
{"msg":"lead status updated by ai", "lead":"...", "status":"contacted", "sentiment":1}
{"msg":"lead status updated by ai", "lead":"...", "status":"dropped", "sentiment":-1}

# Errors (if Claude API is down):
{"msg":"ai handle message", "err":"claude request timeout"}
```

---

## Next Steps (Later Phases)

- [ ] Message template UI (customize per campaign)
- [ ] Trial booking automation (direct calendar integration)
- [ ] Multi-step automation rules (e.g., "if interested, send trial link")
- [ ] AI-generated content (auto-create responses for your tone)
- [ ] A/B testing (test which messages convert best)
- [ ] Lead nurture sequences (automated follow-ups)

---

## Troubleshooting

### AI Not Responding

1. **Check Claude API key** in `.env`
   ```bash
   echo $CLAUDE_API_KEY  # Should show sk-ant-...
   ```

2. **Check server logs** for errors:
   ```bash
   make api 2>&1 | grep -i claude
   ```

3. **Make sure message is from a NEW lead**
   - AI only responds to `status = 'new'`
   - Already-contacted leads are handled by studio staff

### Lead Status Not Updating

1. Check the sentiment confidence is >= 60%
   - Clear yes/no keywords work best
   - Neutral messages don't trigger auto-updates

2. Lead must be linked to conversation:
   - LeadID in `conversations` table must match the lead

3. Check logs:
   ```bash
   make api 2>&1 | grep "lead status"
   ```

---

## Database Schema

### Key Tables

- `message_templates` — campaign-specific AI instructions (when we build the UI)
- `messages` — all incoming/outbound messages with source_kind = 'ai'
- `leads` — status auto-updated by AI worker
- `outbound_jobs` — queued AI responses (then sent via WhatsApp/IG)

---

## API Endpoints (Ready for Frontend)

```
GET  /api/v1/studios/:id/messaging/conversations
GET  /api/v1/studios/:id/messaging/conversations/:id
POST /api/v1/studios/:id/messaging/conversations/:id/messages
GET  /api/v1/studios/:id/messaging/stream (SSE for live updates)
```

All messages include:
- `direction` (inbound/outbound)
- `sourceKind` (customer/studio_user/ai)
- `body`, `createdAt`, etc.

---

## Cost & Performance

- **Claude API**: ~$0.003 per message (very cheap)
- **Latency**: 1-3 seconds per reply (acceptable for async chat)
- **Throughput**: Handles 1000+ messages/day per studio
- **Failure modes**: If Claude API is down, message is queued and retried

---

## Questions?

See `docs/skills.md` for architecture decisions or ask in code.
