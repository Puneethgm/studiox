-- +goose Up
-- platform-security-hardening task 0.2: secrets become write-only. The
-- API previously decrypted a stored key just to show its last 4
-- characters (keySuffix()). A non-reversible fingerprint lets an admin
-- tell two keys apart without any character of either being derivable
-- from the response.
ALTER TABLE platform_ai_keys RENAME COLUMN last_four TO key_fingerprint;

ALTER TABLE studios ADD COLUMN IF NOT EXISTS mistral_api_key_fingerprint TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE platform_ai_keys RENAME COLUMN key_fingerprint TO last_four;
ALTER TABLE studios DROP COLUMN IF EXISTS mistral_api_key_fingerprint;
