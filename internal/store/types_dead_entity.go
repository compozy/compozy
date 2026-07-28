package store

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const maxDeadEntityReasonBytes = 512

// DeadEntityKind identifies one workspace-scoped external runtime family.
type DeadEntityKind string

const (
	DeadEntityKindExtension  DeadEntityKind = "extension"
	DeadEntityKindBridge     DeadEntityKind = "bridge"
	DeadEntityKindMCPSidecar DeadEntityKind = "mcp_sidecar"
)

var ErrInvalidDeadEntity = errors.New("store: invalid dead entity")

// DeadEntityKey identifies one workspace-scoped external runtime.
type DeadEntityKey struct {
	WorkspaceID string
	Kind        DeadEntityKind
	EntityID    string
}

// Normalize returns the canonical dead-entity key.
func (k DeadEntityKey) Normalize() DeadEntityKey {
	return DeadEntityKey{
		WorkspaceID: strings.TrimSpace(k.WorkspaceID),
		Kind:        DeadEntityKind(strings.TrimSpace(string(k.Kind))),
		EntityID:    strings.TrimSpace(k.EntityID),
	}
}

// Validate ensures the key is workspace-scoped and uses a closed kind.
func (k DeadEntityKey) Validate() error {
	normalized := k.Normalize()
	if normalized.WorkspaceID == "" {
		return fmt.Errorf("%w: workspace_id is required", ErrInvalidDeadEntity)
	}
	if normalized.EntityID == "" {
		return fmt.Errorf("%w: entity_id is required", ErrInvalidDeadEntity)
	}
	switch normalized.Kind {
	case DeadEntityKindExtension, DeadEntityKindBridge, DeadEntityKindMCPSidecar:
		return nil
	default:
		return fmt.Errorf("%w: unsupported kind %q", ErrInvalidDeadEntity, normalized.Kind)
	}
}

// DeadEntity is one durable confirmed-dead external runtime.
type DeadEntity struct {
	DeadEntityKey
	Reason   string
	MarkedAt time.Time
}

// Normalize returns the canonical durable row.
func (e DeadEntity) Normalize() DeadEntity {
	normalized := e
	normalized.DeadEntityKey = e.DeadEntityKey.Normalize()
	normalized.Reason = strings.TrimSpace(e.Reason)
	if !e.MarkedAt.IsZero() {
		normalized.MarkedAt = e.MarkedAt.UTC()
	}
	return normalized
}

// Validate ensures the durable row is complete and bounded.
func (e DeadEntity) Validate() error {
	normalized := e.Normalize()
	if err := normalized.DeadEntityKey.Validate(); err != nil {
		return err
	}
	if normalized.Reason == "" {
		return fmt.Errorf("%w: reason is required", ErrInvalidDeadEntity)
	}
	if len(normalized.Reason) > maxDeadEntityReasonBytes {
		return fmt.Errorf(
			"%w: reason exceeds %d bytes",
			ErrInvalidDeadEntity,
			maxDeadEntityReasonBytes,
		)
	}
	if normalized.MarkedAt.IsZero() {
		return fmt.Errorf("%w: marked_at is required", ErrInvalidDeadEntity)
	}
	return nil
}
