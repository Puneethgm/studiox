-- +goose Up
-- Captures the external member/barcode ID from a Glofox member export (the "BarcodeID"
-- column) when leads are bulk-imported from one, so it's kept on the lead instead of
-- being silently dropped — lets staff cross-reference back to the Glofox record.
ALTER TABLE leads ADD COLUMN glofox_member_id text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE leads DROP COLUMN glofox_member_id;
