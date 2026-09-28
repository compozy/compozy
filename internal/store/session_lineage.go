package store

import (
	"fmt"
	"strings"
	"time"
)

// LineageKind is the typed relation between a session and its parent.
type LineageKind string

const (
	// LineageKindRoot marks a session without a relation kind (root sessions).
	LineageKindRoot LineageKind = ""
	// LineageKindProvenance marks a user session created with an explicit parent.
	LineageKindProvenance LineageKind = "provenance"
	// LineageKindSpawn marks a governed spawned or spawn-role session.
	LineageKindSpawn LineageKind = "spawn"
	// LineageKindContinue marks a session continued from another session.
	LineageKindContinue LineageKind = "continue"
	// LineageKindFork marks a session forked from another session.
	LineageKindFork LineageKind = "fork"
	// LineageKindRecovery marks a parented session created to recover the parent.
	LineageKindRecovery LineageKind = "recovery"
)

// LineageKindValues returns every persisted lineage kind in canonical order.
func LineageKindValues() []string {
	return []string{
		string(LineageKindRoot),
		string(LineageKindProvenance),
		string(LineageKindSpawn),
		string(LineageKindContinue),
		string(LineageKindFork),
		string(LineageKindRecovery),
	}
}

// Valid reports whether kind is one of the persisted lineage kinds.
func (k LineageKind) Valid() bool {
	switch k {
	case LineageKindRoot, LineageKindProvenance, LineageKindSpawn,
		LineageKindContinue, LineageKindFork, LineageKindRecovery:
		return true
	default:
		return false
	}
}

// Derived reports whether kind is written by the continue/fork derive path.
func (k LineageKind) Derived() bool {
	return k == LineageKindContinue || k == LineageKindFork
}

// SessionLineage is the persisted parent/root metadata used for safe spawned sessions.
type SessionLineage struct {
	ParentSessionID  string                  `json:"parent_session_id,omitempty"`
	RootSessionID    string                  `json:"root_session_id,omitempty"`
	SpawnDepth       int                     `json:"spawn_depth"`
	SpawnRole        string                  `json:"spawn_role,omitempty"`
	Kind             LineageKind             `json:"kind,omitempty"`
	OriginMessageID  string                  `json:"origin_message_id,omitempty"`
	OriginAgentName  string                  `json:"origin_agent_name,omitempty"`
	TTLExpiresAt     *time.Time              `json:"ttl_expires_at,omitempty"`
	AutoStopOnParent bool                    `json:"auto_stop_on_parent"`
	NotifyCreator    bool                    `json:"notify_creator"`
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
	normalized.Kind = LineageKind(strings.TrimSpace(string(normalized.Kind)))
	normalized.OriginMessageID = strings.TrimSpace(normalized.OriginMessageID)
	normalized.OriginAgentName = strings.TrimSpace(normalized.OriginAgentName)
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
	return validateSessionLineageKind(normalized)
}

// ValidateSessionLineageForType checks the lineage kind against the owning session type.
func ValidateSessionLineageForType(sessionType string, lineage *SessionLineage) error {
	if lineage == nil {
		return nil
	}
	kind := LineageKind(strings.TrimSpace(string(lineage.Kind)))
	if NormalizeSessionType(sessionType) == sessionTypeSpawned && kind != LineageKindSpawn {
		return fmt.Errorf("store: spawned session lineage kind must be %q, got %q", LineageKindSpawn, kind)
	}
	return nil
}

func validateSessionLineageKind(lineage *SessionLineage) error {
	if !lineage.Kind.Valid() {
		return fmt.Errorf("store: unsupported session lineage kind %q", lineage.Kind)
	}
	hasParent := lineage.ParentSessionID != ""
	switch lineage.Kind {
	case LineageKindRoot:
		if hasParent {
			return fmt.Errorf("store: parented session lineage requires a kind")
		}
	case LineageKindProvenance, LineageKindRecovery:
		if !hasParent {
			return fmt.Errorf("store: %s session lineage requires a parent session id", lineage.Kind)
		}
	case LineageKindContinue, LineageKindFork:
		if !hasParent {
			return fmt.Errorf("store: %s session lineage requires a parent session id", lineage.Kind)
		}
		if lineage.OriginAgentName == "" {
			return fmt.Errorf("store: %s session lineage requires an origin agent name", lineage.Kind)
		}
	case LineageKindSpawn:
	}
	if lineage.OriginMessageID != "" && lineage.Kind != LineageKindFork {
		return fmt.Errorf("store: only fork session lineage may carry an origin message id")
	}
	if lineage.OriginAgentName != "" && !lineage.Kind.Derived() {
		return fmt.Errorf("store: only continue or fork session lineage may carry an origin agent name")
	}
	return nil
}

// UpgradeSessionLineageKind fills Kind for lineage written before kinds existed, by
// the same rule as the catalog backfill: spawn when the session type is spawned or a
// spawn role is set; provenance when only a parent is set; root otherwise. It is
// idempotent, preserves every other field, and reports whether it changed anything.
func UpgradeSessionLineageKind(sessionType string, lineage *SessionLineage) bool {
	if lineage == nil || strings.TrimSpace(string(lineage.Kind)) != "" {
		return false
	}
	switch {
	case NormalizeSessionType(sessionType) == sessionTypeSpawned || strings.TrimSpace(lineage.SpawnRole) != "":
		lineage.Kind = LineageKindSpawn
	case strings.TrimSpace(lineage.ParentSessionID) != "":
		lineage.Kind = LineageKindProvenance
	default:
		return false
	}
	return true
}
