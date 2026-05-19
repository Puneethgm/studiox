# Test Guide: AI Auto-Replies on WhatsApp & Messenger

## Quick Start (5 minutes)

### 1. Start the System

```bash
cd /home/puneeth-g-m/Downloads/Infaira
make dev
```

Wait for both servers to start:
```
[api] {"msg":"api listening","addr":"localhost:8080"}
[web] ▲ Next.js 15.0.3 ready
```

### 2. Login to Admin Dashboard

- URL: http://localhost:3000
- Email: `admin@example.com`
- Password: `ChangeMeNow!2026`

### 3. Connect WhatsApp or Messenger

Go to: **Admin → Studios → {Your Studio} → Channels**

Click **"Connect WhatsApp"** or **"Connect Messenger"** and add your credentials.

### 4. Create a Campaign

Go to: **Admin → Studios → {Your Studio} → Campaigns → New Campaign**

Fill in:
- Campaign name: "Test Campaign"
- Fitness plans: Any
- Active: Yes

### 5. Send a Test Message

Send a message to your WhatsApp/Messenger number from your personal phone:
```
"Hi! I'm interested in your yoga class"
```

### 6. Watch It Happen

✅ **Within 1-3 seconds**, you'll see:

**WhatsApp/Messenger:**
```
Auto-reply from studio:
"Hey! That's awesome! 🎉 We offer classes at 9am, 6pm, and weekends.
Which time works best for you?"
```

**Dashboard - Real-time:**
- New lead appears in the inbox
- Status auto-updates: **New → Contacted**
- Lead appears in pipeline under "Contacted" column

---

## Detailed Test Scenarios

### Test 1: Positive Sentiment

**Send:**
```
"Yes! I'd love to book a trial"
```

**Expected AI Response (within 2 seconds):**
```
Something like: "Perfect! Which day and time work best for you?"
```

**Expected Pipeline Change:**
```
New → Contacted (if first message)
OR
Contacted → Trial Booked (if already contacted)
```

**Check Logs:**
```bash
# In terminal running `make api`:
# Look for these lines:
"sentiment analyzed" ... "sentiment":1, "confidence":0.95
"ai response generated" ... "response_len":...
"lead status auto-updated" ... "from":"new", "to":"contacted"
```

---

### Test 2: Negative Sentiment

**Send:**
```
"No thanks, too expensive"
```

**Expected AI Response:**
```
Something like: "I understand. Feel free to reach out anytime!"
```

**Expected Pipeline Change:**
```
New → Dropped
(Lead marked as not interested)
```

**Check Logs:**
```bash
"sentiment analyzed" ... "sentiment":-1, "confidence":0.88
"lead status auto-updated" ... "from":"new", "to":"dropped"
```

---

### Test 3: Neutral/Question

**Send:**
```
"What are your pricing options?"
```

**Expected AI Response:**
```
Something like: "We have flexible plans starting at $29/month.
Would you like to try a free trial first?"
```

**Expected Pipeline Change:**
```
New → Contacted (because conversation started)
(Waits for clear yes/no)
```

**Check Logs:**
```bash
"sentiment analyzed" ... "sentiment":0, "confidence":0.50
"lead status auto-updated" ... "from":"new", "to":"contacted"
```

---

### Test 4: Multi-turn Conversation

**Message 1:**
```
Client: "Tell me about morning classes"
AI: "We have 6am and 9am yoga classes. Interested?"
Status: New → Contacted
```

**Message 2:**
```
Client: "Yes! Sign me up for 9am"
AI: "Great! Your trial is set for tomorrow 9am."
Status: Contacted → Trial Booked
```

**Message 3:**
```
Client: "Awesome! See you tomorrow!"
AI: "Can't wait! See you then! 💪"
Status: Trial Booked → Member
```

---

## Dashboard Verification

### Check In Real-Time

1. **Open two windows:**
   - Window 1: WhatsApp/Messenger conversation
   - Window 2: Admin dashboard

2. **Go to Pipeline view:**
   http://localhost:3000/admin/studios/{studioId}/pipeline

3. **Send a message on WhatsApp:**
   ```
   "Hi! Interested in fitness"
   ```

4. **Watch the dashboard:**
   - New card appears in "New" column
   - (within 2 seconds) Card moves to "Contacted" column
   - Lead count updates automatically

### Check Message History

1. Go to: **Admin → Studios → {studioId} → Inbox**
2. You should see:
   - Your message (blue, incoming)
   - AI response (gray, outgoing, marked as "ai")
   - Timestamp and channel

---

## Debugging

### If AI Doesn't Reply

**Step 1: Check server logs**
```bash
# Terminal running `make api`
# Should see:
"ai response generated"

# If NOT seeing that, check:
"skipping ai reply" + reason
```

**Step 2: Check channel type**
```bash
# Only WhatsApp and Messenger get AI replies
# Not Instagram or X

# In logs, you'll see:
"skipping ai reply" ... "reason": "not whatsapp or messenger"
```

**Step 3: Check Claude API key**
```bash
echo $CLAUDE_API_KEY
# Must be: sk-ant-...
# If empty, set it:
export CLAUDE_API_KEY=sk-ant-...
```

**Step 4: Check database connection**
```bash
# Make sure Postgres is running:
docker ps | grep postgres
# Should show: projectx-postgres

# If not:
make db-up
```

---

### If Status Doesn't Update

**Check: Is confidence high enough?**
```bash
# Need ≥ 60% confidence
# Clear keywords get 85-95%
# Vague messages might be 40%

# In logs:
"sentiment analyzed" ... "confidence":0.95  ✅ Will update
"sentiment analyzed" ... "confidence":0.45  ❌ Won't update
```

**Check: Is lead in database?**
```bash
# In database:
SELECT * FROM conversations WHERE id = '...';
# lead_id must NOT be null
```

**Check: Are all services running?**
```bash
# Need:
✅ API running (port 8080)
✅ Database running (port 5434)
✅ Web running (port 3000)

# Check with:
ss -tlnp | grep -E "8080|5434|3000"
```

---

## Performance Benchmarks

### Expected Timings

| Operation | Time | Notes |
|-----------|------|-------|
| Message received | 0ms | Webhook fires |
| AI worker picks it up | 30-100ms | Event bus |
| Sentiment analysis | 10ms | Local keyword matching |
| Claude API call | 800-1500ms | Network latency to Anthropic |
| Response enqueue | 50ms | Database write |
| Total | **1-3 seconds** | User sees response |

### Throughput Test

**Send 10 messages in rapid succession:**
```
Message 1: "Hi"
Message 2: "How are you"
Message 3: "Tell me about plans"
... 
Message 10: "I'm interested!"
```

**Expected:**
- All 10 get replies
- Each gets unique response (not templated)
- Replies arrive in order
- No lost messages

---

## Success Criteria

✅ **Test Passes If:**

```
1. Within 2 seconds of sending a message on WhatsApp/Messenger,
   you receive an AI reply
   
2. The reply is relevant to your message
   (mentions your plan, is friendly, asks follow-up)
   
3. The dashboard updates in real-time
   (lead appears, status changes automatically)
   
4. Logs show: "ai response generated" and "lead status auto-updated"

5. Positive messages move leads forward:
   New → Contacted → Trial Booked → Member
   
6. Negative messages drop leads:
   New/Contacted → Dropped
   
7. Instagram/X messages are skipped:
   (No AI reply, only manual)
```

---

## Common Issues & Fixes

| Issue | Cause | Fix |
|-------|-------|-----|
| No reply for 10+ seconds | Claude API slow | Normal, can timeout to 20s |
| Same reply every time | Not Claude, code issue | Restart API |
| Reply sent to wrong chat | Message routing | Check channel_id in logs |
| Status not updating | Low confidence | Need clear yes/no words |
| Instagram got AI reply | Channel check failed | Update AI worker (shouldn't happen) |
| Dashboard frozen | Frontend issue | Refresh page, check console logs |

---

## Pro Tips

1. **Test with real conversations** - Use realistic language from your leads
2. **Monitor response quality** - Read first 5-10 AI replies
3. **Adjust keywords** - If AI is too aggressive, we can tune sentiment thresholds
4. **Watch the logs** - `make api 2>&1 | grep "ai "` shows everything
5. **Check database** - `docker exec projectx-postgres psql -U projectx -d projectx -c "SELECT * FROM messages"`

---

## Next: Full Integration

Once tested locally, you can:

1. **Deploy to production** (same code, just different server)
2. **Monitor real conversations** (metrics dashboards)
3. **Adjust AI behavior** (update prompts in code)
4. **Customize per campaign** (when templates UI is built)

---

Enjoy your AI chatbot! 🤖
