package notifications

import (
	"fmt"
	"unicode/utf8"
)

// ScopeKind identifies the durable ownership scope of a notification cursor.
type ScopeKind string

const (
	// ScopeKindGlobal identifies a daemon-wide cursor with no workspace binding.
	ScopeKindGlobal ScopeKind = "global"
	// ScopeKindWorkspace identifies a cursor owned by one workspace.
	ScopeKindWorkspace ScopeKind = "workspace"
)

// ScopeRef keeps a cursor's scope separate from every opaque identity component.
type ScopeRef struct {
	Kind        ScopeKind `json:"kind"`
	WorkspaceID string    `json:"workspace_id,omitempty"`
}

// Normalize preserves the exact scope components supplied by the caller.
//
// Scope values take part in durable cursor identity, so normalizing whitespace
// or case here could merge two independently owned cursors. Validate instead
// accepts only the two canonical scope kinds.
func (s ScopeRef) Normalize() ScopeRef {
	return s
}

// Validate reports whether a scope has one unambiguous ownership binding.
func (s ScopeRef) Validate() error {
	if err := validateNotificationIdentityUTF8(string(s.Kind), "scope kind"); err != nil {
		return err
	}
	if err := validateNotificationIdentityUTF8(s.WorkspaceID, "workspace id"); err != nil {
		return err
	}
	switch s.Kind {
	case ScopeKindGlobal:
		if s.WorkspaceID != "" {
			return fmt.Errorf(
				"%w: workspace id must be empty for global scope",
				ErrInvalidCursor,
			)
		}
	case ScopeKindWorkspace:
		if s.WorkspaceID == "" {
			return fmt.Errorf(
				"%w: workspace id is required for workspace scope",
				ErrInvalidCursor,
			)
		}
	default:
		return fmt.Errorf(
			"%w: scope kind must be %q or %q",
			ErrInvalidCursor,
			ScopeKindGlobal,
			ScopeKindWorkspace,
		)
	}
	return nil
}

func validateNotificationIdentityUTF8(value string, field string) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("%w: %s must be valid UTF-8", ErrInvalidCursor, field)
	}
	return nil
}
