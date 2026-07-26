CREATE TABLE vault_secrets (
			ref             TEXT PRIMARY KEY,
			kind            TEXT NOT NULL DEFAULT '',
			encrypted_value TEXT NOT NULL,
			created_at      TEXT NOT NULL,
			updated_at      TEXT NOT NULL
		);

CREATE INDEX idx_vault_secrets_kind
			ON vault_secrets(kind);

CREATE INDEX idx_vault_secrets_updated_at
			ON vault_secrets(updated_at);
