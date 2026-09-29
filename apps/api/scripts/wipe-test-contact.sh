#!/usr/bin/env bash
# Wipes a single contact's lead + conversation + messages + contact identity
# for one studio, for clean retesting. Refuses if the lead has a real Stripe
# subscription (member_sold=true or a non-empty stripe_subscription_id) —
# delete that manually if you really mean to.
#
# Usage:
#   ./wipe-test-contact.sh <studio_id> <phone_or_name_pattern>
#
# Examples:
#   ./wipe-test-contact.sh 1259cf9a-3798-42d4-85d2-14b83eb6c570 puneethgm21
#   ./wipe-test-contact.sh 1259cf9a-3798-42d4-85d2-14b83eb6c570 917483974512

set -euo pipefail

STUDIO_ID="${1:?Usage: $0 <studio_id> <phone_or_name_pattern>}"
PATTERN="${2:?Usage: $0 <studio_id> <phone_or_name_pattern>}"

: "${PGHOST:=localhost}"
: "${PGPORT:=5436}"
: "${PGUSER:=projectx}"
: "${PGPASSWORD:=projectx_dev}"
: "${PGDATABASE:=projectx}"
export PGPASSWORD

psql -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDATABASE" -v ON_ERROR_STOP=1 <<SQL
DO \$\$
DECLARE
  r RECORD;
BEGIN
  FOR r IN
    SELECT ci.id AS contact_identity_id, ci.lead_id, l.member_sold, l.stripe_subscription_id
    FROM contact_identities ci
    LEFT JOIN leads l ON l.id = ci.lead_id
    WHERE ci.studio_id = '$STUDIO_ID'
      AND (ci.display_name ILIKE '%$PATTERN%' OR ci.value ILIKE '%$PATTERN%')
  LOOP
    IF r.member_sold IS TRUE OR COALESCE(r.stripe_subscription_id, '') <> '' THEN
      RAISE EXCEPTION 'contact_identity % has real payment data (member_sold=%, stripe_subscription_id=%) — refusing to auto-delete, remove it manually', r.contact_identity_id, r.member_sold, r.stripe_subscription_id;
    END IF;

    DELETE FROM conversations WHERE contact_identity_id = r.contact_identity_id;
    IF r.lead_id IS NOT NULL THEN
      DELETE FROM leads WHERE id = r.lead_id;
    END IF;
    DELETE FROM contact_identities WHERE id = r.contact_identity_id;
    RAISE NOTICE 'wiped contact_identity %', r.contact_identity_id;
  END LOOP;
END \$\$;
SQL
