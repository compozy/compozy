-- +goose Up
-- +goose StatementBegin
ALTER TABLE api_keys 
ADD COLUMN fingerprint BYTEA;

-- Populate fingerprint for existing keys (SHA-256 of the bcrypt hash)
-- This is a one-time operation for existing data
UPDATE api_keys 
SET fingerprint = sha256(hash);

-- Now make it NOT NULL after populating
ALTER TABLE api_keys 
ALTER COLUMN fingerprint SET NOT NULL;

-- Create unique index for O(1) lookups
CREATE UNIQUE INDEX idx_api_keys_fingerprint ON api_keys(fingerprint);

-- Add comment for documentation
COMMENT ON COLUMN api_keys.fingerprint IS 'SHA-256 hash of the bcrypt hash of the API key for O(1) lookups before bcrypt comparison';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_api_keys_fingerprint;
ALTER TABLE api_keys DROP COLUMN fingerprint;
-- +goose StatementEnd
