# 🔗 Google Sheets Integration - Complete Setup Guide

## Overview

Your leads will **automatically sync to Google Sheets** within 5 seconds of being created. This guide walks you through the 10-minute setup.

---

## ⏱️ Step-by-Step Setup (10 minutes)

### STEP 1: Create Google Cloud Project (2 min)

1. Go to **[Google Cloud Console](https://console.cloud.google.com/)**
2. Click **"Select a Project"** at the top
3. Click **"NEW PROJECT"**
4. Enter name: `Infaira-Fitness` or any name
5. Click **CREATE**
6. Wait for it to be created (~1 minute)

✅ **You now have a Google Cloud project**

---

### STEP 2: Enable Google Sheets API (2 min)

1. In the same Cloud Console, search for **"Sheets API"** in the search bar
2. Click **Google Sheets API** from results
3. Click **ENABLE** (blue button)
4. Wait a few seconds for it to enable

✅ **Google Sheets API is now enabled**

---

### STEP 3: Create Service Account (2 min)

1. In Cloud Console, go to **APIs & Services → Credentials** (left sidebar)
2. Click **+ CREATE CREDENTIALS** (blue button at top)
3. Select **Service Account**
4. Fill in:
   - Service account name: `infaira-sheets-writer`
   - Description: (optional, leave blank)
5. Click **CREATE AND CONTINUE**
6. In "Grant this service account access to project" → Click **CONTINUE**
7. Skip "Grant users access..." → Click **DONE**

✅ **Service account created**

---

### STEP 4: Create and Download JSON Key (1 min)

1. You're now on the Service Accounts page
2. Click the service account you just created (`infaira-sheets-writer`)
3. Go to **KEYS** tab
4. Click **ADD KEY → Create new key**
5. Select **JSON** (already selected)
6. Click **CREATE**

📥 **A JSON file downloads automatically** (usually in `~/Downloads/`)

Example filename: `infaira-fitness-abc123def456.json`

✅ **Keep this file safe - it's your credentials**

---

### STEP 5: Save Credentials in Your Project (1 min)

```bash
# Move the downloaded JSON to your project:
cd /home/puneeth-g-m/Downloads/Infaira

# Create secrets directory if it doesn't exist:
mkdir -p secrets

# Move the JSON file (replace with your actual filename):
mv ~/Downloads/infaira-fitness-*.json secrets/google-credentials.json

# Verify it's there:
ls -la secrets/google-credentials.json
# Should show the file exists
```

✅ **Credentials saved**

---

### STEP 6: Create a Google Sheet (1 min)

1. Go to **[Google Sheets](https://sheets.google.com/)**
2. Click **+ Blank** to create new sheet
3. Name it: `Infaira Leads Tracker` (or any name)
4. It automatically creates a sheet with tab named "Sheet1"
5. Right-click the "Sheet1" tab at bottom → **Rename**
6. Change to: `Leads`
7. Press Enter

✅ **Google Sheet created and renamed**

---

### STEP 7: Share Sheet with Service Account (2 min)

Now you need to give the service account permission to write to your sheet.

**Get the service account email:**

1. Go back to **[Google Cloud Console](https://console.cloud.google.com/)**
2. Go to **APIs & Services → Service Accounts**
3. Click on `infaira-sheets-writer` service account
4. Copy the email address (looks like: `infaira-sheets-writer@infaira-fitness-123456.iam.gserviceaccount.com`)

**Share the sheet with this email:**

1. Go back to your Google Sheet (`Infaira Leads Tracker`)
2. Click **Share** button (top right)
3. Paste the service account email into "Add people or groups"
4. In the dropdown, select **Editor** (not Viewer)
5. Uncheck "Notify people"
6. Click **Share**

✅ **Service account now has Editor access to your sheet**

---

### STEP 8: Get Your Spreadsheet ID (1 min)

1. You're in your Google Sheet (`Infaira Leads Tracker`)
2. Look at the URL in your browser:
   ```
   https://docs.google.com/spreadsheets/d/1AbCdEfGhIjKlMnOpQrStUvWxYz123456/edit#gid=0
   ```
3. Copy the long ID between `/d/` and `/edit`:
   ```
   1AbCdEfGhIjKlMnOpQrStUvWxYz123456
   ```

✅ **You have your Spreadsheet ID**

---

### STEP 9: Update .env File (1 min)

Edit your `.env` file and add/update these lines:

```bash
# Google Sheets Integration
GOOGLE_CREDENTIALS_PATH=secrets/google-credentials.json
GOOGLE_SHEETS_ID=1AbCdEfGhIjKlMnOpQrStUvWxYz123456
GOOGLE_SHEETS_TAB=Leads
```

⚠️ **Important:** Replace `1AbCdEfGhIjKlMnOpQrStUvWxYz123456` with YOUR actual spreadsheet ID

✅ **.env updated**

---

### STEP 10: Test It (1 min)

```bash
# Restart the API
make dev

# In the logs, you should see:
# {"msg":"sheets init success"} or {"msg":"sheets_enabled":true}
```

If you see errors like `sheets init failed`, check:
- File exists: `ls secrets/google-credentials.json`
- Sheet ID is correct in `.env`
- Service account has Editor access to sheet

✅ **Setup complete!**

---

## 🧪 Verify It Works

### Method 1: Create a Lead via Admin

1. Go to **Admin → Studios → {Your Studio} → Campaigns**
2. Click on any campaign
3. Copy the public form URL: `/l/{studio-slug}/{campaign-slug}`
4. Open it in another browser/incognito
5. Fill in and submit the form with:
   - Name: "Test User"
   - Email: "test@example.com"
   - Phone: "+1234567890"
   - Any other fields
6. Submit

### Method 2: Check Results

**In Admin Dashboard (instant):**
- Go to **Leads** → Should see your test lead immediately

**In Google Sheet (within 5 seconds):**
- Refresh your sheet
- Should see a new row with:
  - Studio name
  - Lead name ("Test User")
  - Email
  - Phone
  - Campaign
  - Status
  - Timestamp

✅ **If you see the lead in both places, it's working!**

---

## 📊 What Gets Synced

Every time a lead is created, these details go to Google Sheets:

```
Studio         | Studio name
Studio Slug    | studio-slug
Campaign       | campaign-name  
Campaign ID    | {UUID}
Name           | Lead's name
Email          | Lead's email
Phone          | Lead's phone
Fitness Plan   | Selected plan
Goals          | Lead's goals
Source         | How they found you
Status         | Pipeline status (new/contacted/etc)
Notes          | Any notes from studio
Created At     | Timestamp
```

---

## 🔄 How It Works (Behind the Scenes)

```
Lead created via form
        ↓
Saved to database
        ↓
Outbox row created (same transaction)
        ↓
Sheets worker picks it up (every 5 seconds)
        ↓
Sends to Google Sheets
        ↓
Row appears in sheet
```

**If Google Sheets is down:**
- Lead still saves to database ✅
- Outbox keeps retrying automatically
- When Sheets comes back, row syncs automatically
- **Zero data loss**

---

## ⚙️ Configuration Options

### Change Sheet Tab Name

If you want leads in a different tab (not "Leads"):

1. In Google Sheet, create/use a tab named "YourTabName"
2. Update `.env`:
   ```bash
   GOOGLE_SHEETS_TAB=YourTabName
   ```

### Multiple Studios (Future)

Currently: **One Google Sheet receives ALL studio leads**

Soon: Each studio can have their own sheet (planned upgrade)

---

## 🐛 Troubleshooting

### Issue: "sheets init failed" in logs

**Solution:**
1. Check file exists:
   ```bash
   ls -la secrets/google-credentials.json
   ```
2. Check .env has correct path:
   ```bash
   grep GOOGLE_CREDENTIALS_PATH .env
   ```
3. Restart API:
   ```bash
   make api
   ```

### Issue: Leads don't appear in sheet

**Check 1: Spreadsheet ID correct?**
```bash
# Get it from URL:
https://docs.google.com/spreadsheets/d/1AbCdEf.../edit
#                                          └─ copy this
```

**Check 2: Service account has access?**
1. Go to Google Sheet
2. Click Share
3. Should see: `infaira-sheets-writer@...` with Editor access

**Check 3: Tab name matches .env?**
```bash
# If your tab is "Leads", should have:
GOOGLE_SHEETS_TAB=Leads
```

### Issue: "403 The caller does not have permission"

**Solution:** Share the sheet with service account again:
1. Google Sheet → Share
2. Paste: `infaira-sheets-writer@infaira-fitness-...`
3. Select: **Editor**
4. Click Share

---

## 📝 Example: What Your Sheet Looks Like

```
Studio | Studio Slug | Campaign      | Name      | Email           | Phone        | Status    | Created At
Yoga Co| yoga-co     | Summer Classes| John Doe  | john@example.com| +1234567890 | contacted | 2026-05-19
Yoga Co| yoga-co     | Summer Classes| Jane Smith| jane@example.com| +0987654321 | new       | 2026-05-19
Gym Pro| gym-pro     | Spring Promo  | Mike Liu  | mike@example.com| +1111111111 | dropped   | 2026-05-19
```

---

## ✨ Pro Tips

1. **Use Google Sheets features** - Sort, filter, add formulas, create charts
2. **Backup your data** - Download as CSV regularly
3. **Create formulas** - Count leads by status, calculate conversion rate
4. **Share with team** - Give non-technical team members view access
5. **Monitor status** - Check logs: `make api 2>&1 | grep sheets`

---

## 🎉 Summary

✅ **Google Sheets integration is now:**
- ✅ Connected
- ✅ Syncing all new leads
- ✅ Automatic (no manual work)
- ✅ Real-time (within 5 seconds)
- ✅ Reliable (auto-retries if failed)

**All leads now appear in TWO places:**
1. **Admin Dashboard** (for team management)
2. **Google Sheet** (for analysis/reporting)

---

## Next Steps

1. Test with a few real leads
2. Share sheet with your team
3. Create formulas/charts for analytics
4. Monitor logs to ensure it's working

Questions? Check `docs/SETUP_GOOGLE_SHEETS.md` for technical details.
