# Google Sheets — service-account setup

The platform pushes every captured lead to a Google Sheet via the **outbox
pattern**: the lead is committed to Postgres + an `outbox` row in the same
transaction, then a background worker drains the outbox into Sheets. If Sheets
is down or unconfigured, leads still land in the platform — the worker retries
with exponential backoff until it succeeds.

## One-time provisioning (5–10 min)

### 1. Create / pick a Google Cloud project

1. Open the [Google Cloud Console](https://console.cloud.google.com/).
2. Create a new project (e.g. `projectx-prod`) or pick an existing one.

### 2. Enable the Sheets API

1. In the Cloud Console search bar, go to **APIs & Services → Library**.
2. Search for **Google Sheets API** and click **Enable**.

### 3. Create a service account

1. **APIs & Services → Credentials → + Create Credentials → Service account**.
2. Name it `projectx-sheets-writer`. Skip the optional grants and click **Done**.
3. On the new service account row, open the **Keys** tab → **Add key →
   Create new key → JSON**. A `*.json` file downloads — this is the credential.

### 4. Save the credentials in the repo

Move the downloaded JSON to `secrets/google-credentials.json`:

```bash
mkdir -p secrets
mv ~/Downloads/projectx-prod-*.json secrets/google-credentials.json
```

The `secrets/` directory is gitignored. Never commit this file.

### 5. Create the destination spreadsheet

1. Create a fresh Google Sheet (any name — e.g. `Project-X Leads`).
2. Rename the first tab to `Leads`. (Or set `GOOGLE_SHEETS_TAB=YourTabName`
   in `.env` to point elsewhere.)
3. **Share** the sheet with the service account email
   (e.g. `projectx-sheets-writer@projectx-prod.iam.gserviceaccount.com`,
   visible in the Cloud Console). Give it **Editor** access.
4. Copy the spreadsheet ID from the URL — the segment between `/d/` and `/edit`:

   ```
   https://docs.google.com/spreadsheets/d/1AbcDEFghIJklmNOPqrsTUVwxyz1234567890/edit
                                          └────────────── this part ──────────────┘
   ```

### 6. Update `.env`

```bash
GOOGLE_CREDENTIALS_PATH=secrets/google-credentials.json
GOOGLE_SHEETS_ID=1AbcDEFghIJklmNOPqrsTUVwxyz1234567890
GOOGLE_SHEETS_TAB=Leads
```

### 7. Restart the API

```bash
make api
```

On boot you should see `sheets_enabled=true`. The worker writes a header row to
the sheet on first run, then appends a row per lead within ~5 seconds of each
submission.

## Verifying

1. Submit a lead through any campaign's `/l/<slug>` form.
2. The lead immediately appears in the admin **Leads** table.
3. Within ~5 seconds the same lead appears as a new row in your Sheet.
4. To confirm queueing works: stop the API, submit a lead through the public
   form (it'll fail since the API is down). Now break Sheets credentials in
   `.env`, restart, submit another lead — it'll land in the admin table; the
   sheets worker logs a warning and the row sits in the `outbox` table with
   `status=pending`. Fix credentials, restart, and the worker drains it.

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| `sheets init failed` on boot | Credentials path wrong, or service account lacks access to the sheet (re-share). |
| Rows never appear | Spreadsheet ID is wrong, or tab name doesn't match `GOOGLE_SHEETS_TAB`. |
| `403 The caller does not have permission` | Sheet not shared with the service account email — check step 5. |
| Headers wrong | The worker only writes headers when row 1 is empty. Clear row 1 and restart, or write them manually to match `internal/integrations/sheets/client.go:Header`. |

## Multi-tenant note

Currently **one Google Sheet receives leads from every studio** on the
platform. Studio columns (`Studio`, `Studio Slug`) are at the front of every
row so you can filter/group by studio in Sheets.

Per-studio Sheets ("each studio gets their own spreadsheet") is a planned L2
upgrade — it'll move `GOOGLE_SHEETS_ID` from `.env` onto the `studios` table
so each studio can configure their own destination.
