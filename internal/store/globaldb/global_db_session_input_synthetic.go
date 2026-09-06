package globaldb

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

func encodeSyntheticQueuePrompt(prompt *store.SessionInputSyntheticPrompt) (sql.NullString, error) {
	if prompt == nil {
		return sql.NullString{}, nil
	}
	encoded, err := json.Marshal(prompt)
	if err != nil {
		return sql.NullString{}, fmt.Errorf("store: encode synthetic queued prompt: %w", err)
	}
	return sql.NullString{String: string(encoded), Valid: true}, nil
}

func decodeSyntheticQueuePrompt(raw sql.NullString, entry *store.SessionInputQueueEntry) error {
	if !raw.Valid {
		return nil
	}
	if err := json.Unmarshal([]byte(raw.String), &entry.SyntheticPrompt); err != nil {
		return fmt.Errorf("store: decode synthetic queued prompt: %w", err)
	}
	return nil
}
