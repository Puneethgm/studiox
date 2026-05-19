# scripts/leads

Standalone Go tool for seeding and progressing leads on a Project-X campaign.
Lives outside `apps/` and has its own `go.mod` — no shared imports with the
API. Reads `.env` at the repo root for `POSTGRES_*` vars and connects directly
to Postgres.

> Seeded leads are tagged `source='seed'` and **bypass the outbox**, so they
> are NOT pushed to Google Sheets. This keeps your real Sheets clean.

## Quick start

```bash
cd scripts/leads

# 1. Seed 80 leads spread across the funnel
go run . populate --studio=fawaz --campaign=spring-season-onki --count=80

# 2. Walk 15 of them forward (new → contacted → trial → member|dropped)
go run . progress --studio=fawaz --campaign=spring-season-onki --count=15

# 3. Wipe seeded data (real submissions are untouched)
go run . reset --studio=fawaz --campaign=spring-season-onki
```

## Commands

### `populate`
Inserts N leads with realistic names, emails, phones, plans, and goals.
Status mix is weighted (`new 40 / contacted 25 / trial_booked 15 / member 12
/ dropped 8`) so the inbox immediately looks like a working funnel. Each lead
gets a `created_at` randomly placed in the last 30 days, with a 25% bias
toward "this week" so the dashboard has fresh activity.

Flags: `--studio=<slug>` (req) · `--campaign=<slug>` (req) · `--count=80`

### `progress`
Picks the oldest-updated non-terminal leads (status ∈ {new, contacted,
trial_booked}) and randomly advances them along the funnel. Roughly:

| From | 60% | 20% | rest |
|---|---|---|---|
| new | → contacted | → dropped | stay |
| contacted | → trial_booked | → dropped | stay |
| trial_booked | → member | → dropped | stay |

Each advancement appends a contextual note so the lead detail page reads
like a real CRM trail.

Flags: `--studio=<slug>` (req) · `--campaign=<slug>` (req) · `--count=20`

### `reset`
Deletes every `source='seed'` lead on the campaign. Real submissions
(`source='public_form'`) are untouched.

Flags: `--studio=<slug>` (req) · `--campaign=<slug>` (req)

## Customizing

- Status weights and the transition matrix live in `data.go` (`buckets`,
  `nextStatus`).
- Name / email / goal pools are in `data.go` — extend freely.
- The script does direct SQL inserts (skipping the outbox). If you want
  seeded leads to flow into Sheets, change `INSERT INTO leads ...` in
  `main.go` to also write a corresponding `outbox` row.
