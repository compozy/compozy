// Package extensioninput defines the persistence boundary for non-secret extension inputs.
package extensioninput

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var ErrConflict = errors.New("extension inputs changed since preparation")

// Value is one install/update input: exactly one of Value or VaultRef must be supplied.
type Value struct {
	Value    json.RawMessage `json:"value,omitempty"`
	VaultRef *string         `json:"vault_ref,omitempty"`
}

// Instance addresses one exact cell; values do not cross profile or workspace boundaries.
type Instance struct {
	Extension   string
	ProfileID   string
	WorkspaceID string
}

// Record retains inactive values for lossless manifest updates.
type Record struct {
	Type      string
	Value     json.RawMessage
	Active    bool
	UpdatedAt time.Time
}

// Mutation carries rollback before-images; nil Before expects no row and nil After removes the row.
type Mutation struct {
	InputID string
	Before  *Record
	After   *Record
}

// Store reads one instance and applies a complete mutation batch atomically.
type Store interface {
	List(context.Context, Instance) (map[string]Record, error)
	Apply(context.Context, Instance, []Mutation) error
}
