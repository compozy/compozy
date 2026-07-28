package store

import (
	"fmt"
	"strings"
	"time"
)

// SessionLineage is the persisted parent/root metadata used for safe spawned sessions.
type SessionLineage struct {
	ParentSessionID  string                  `json:"parent_session_id,omitempty"`
	RootSessionID    string                  `json:"root_session_id,omitempty"`
	SpawnDepth       int                     `json:"spawn_depth"`
	SpawnRole        string                  `json:"spawn_role,omitempty"`
	TTLExpiresAt     *time.Time              `json:"ttl_expires_at,omitempty"`
	AutoStopOnParent bool                    `json:"auto_stop_on_parent"`
	SpawnBudget      SessionSpawnBudget      `json:"spawn_budget"`
	PermissionPolicy SessionPermissionPolicy `json:"permission_policy"`
}

// CloneSessionLineage returns a deep copy of lineage metadata.
func CloneSessionLineage(lineage *SessionLineage) *SessionLineage {
	if lineage == nil {
		return nil
	}
	cloned := *lineage
	if lineage.TTLExpiresAt != nil {
		ttl := lineage.TTLExpiresAt.UTC()
		cloned.TTLExpiresAt = &ttl
	}
	cloned.PermissionPolicy = NormalizeSessionPermissionPolicy(lineage.PermissionPolicy)
	return &cloned
}

// NormalizeSessionLineage returns lineage with trimmed identifiers and a root
// record for first-class manual sessions when no lineage was supplied.
func NormalizeSessionLineage(sessionID string, lineage *SessionLineage) *SessionLineage {
	normalized := SessionLineage{}
	if lineage != nil {
		normalized = *lineage
	}
	normalized.ParentSessionID = strings.TrimSpace(normalized.ParentSessionID)
	normalized.RootSessionID = strings.TrimSpace(normalized.RootSessionID)
	normalized.SpawnRole = strings.TrimSpace(normalized.SpawnRole)
	if normalized.TTLExpiresAt != nil {
		ttl := normalized.TTLExpiresAt.UTC()
		normalized.TTLExpiresAt = &ttl
	}
	normalized.PermissionPolicy = NormalizeSessionPermissionPolicy(normalized.PermissionPolicy)

	if normalized.ParentSessionID == "" && normalized.RootSessionID == "" {
		normalized.RootSessionID = strings.TrimSpace(sessionID)
	}
	return &normalized
}

// ValidateSessionLineage ensures lineage is structurally usable by spawn policy enforcement.
func ValidateSessionLineage(sessionID string, lineage *SessionLineage) error {
	if lineage == nil {
		return nil
	}
	normalized := NormalizeSessionLineage(sessionID, lineage)
	if normalized.SpawnDepth < 0 {
		return fmt.Errorf("store: session lineage spawn depth cannot be negative")
	}
	if err := validateSessionSpawnBudget(normalized.SpawnBudget); err != nil {
		return err
	}
	if err := validateSessionPermissionPolicy(normalized.PermissionPolicy); err != nil {
		return err
	}

	sessionID = strings.TrimSpace(sessionID)
	switch {
	case normalized.ParentSessionID == "":
		if normalized.SpawnDepth != 0 {
			return fmt.Errorf("store: root session lineage depth must be 0")
		}
		if normalized.RootSessionID != "" && sessionID != "" && normalized.RootSessionID != sessionID {
			return fmt.Errorf("store: root session lineage root must match session id")
		}
		if normalized.AutoStopOnParent {
			return fmt.Errorf("store: root session lineage cannot auto-stop on parent")
		}
	case normalized.RootSessionID == "":
		return fmt.Errorf("store: child session lineage root session id is required")
	case normalized.SpawnDepth == 0:
		return fmt.Errorf("store: child session lineage depth must be greater than 0")
	case normalized.ParentSessionID == sessionID:
		return fmt.Errorf("store: child session lineage parent cannot be the session itself")
	case normalized.RootSessionID == sessionID:
		return fmt.Errorf("store: child session lineage root cannot be the session itself")
	}
	return nil
}
