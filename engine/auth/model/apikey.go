package model

import (
	"database/sql"
	"time"

	"github.com/compozy/compozy/engine/core"
)

// APIKey represents an API key for authentication
type APIKey struct {
	ID          core.ID      `db:"id,pk"`
	UserID      core.ID      `db:"user_id"`
	Hash        []byte       `db:"hash"`        // bcrypt-hashed key
	Fingerprint []byte       `db:"fingerprint"` // SHA-256 hash for O(1) lookups
	Prefix      string       `db:"prefix"`      // cpzy_
	CreatedAt   time.Time    `db:"created_at"`
	LastUsed    sql.NullTime `db:"last_used"`
}
