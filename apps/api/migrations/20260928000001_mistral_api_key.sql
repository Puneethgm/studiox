-- +goose Up
-- Mistral is used for Knowledge Base image OCR (mistral-ocr-latest), not the
-- chat-reply waterfall — single plaintext key, same convention as
-- gemini_api_key/groq_api_key/claude_api_key.
ALTER TABLE studios ADD COLUMN mistral_api_key TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE studios DROP COLUMN mistral_api_key;
