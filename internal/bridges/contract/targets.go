package contract

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// BridgeTargetType identifies a provider target family.
type BridgeTargetType string

const (
	BridgeTargetTypeChannel BridgeTargetType = "channel"
	BridgeTargetTypeUser    BridgeTargetType = "user"
	BridgeTargetTypeRoom    BridgeTargetType = "room"
	BridgeTargetTypeThread  BridgeTargetType = "thread"
	BridgeTargetTypeGroup   BridgeTargetType = "group"
)

// Normalize returns the canonical target type.
func (t BridgeTargetType) Normalize() BridgeTargetType {
	return BridgeTargetType(strings.ToLower(strings.TrimSpace(string(t))))
}

// Validate reports whether the target type is supported.
func (t BridgeTargetType) Validate() error {
	switch t.Normalize() {
	case BridgeTargetTypeChannel, BridgeTargetTypeUser, BridgeTargetTypeRoom,
		BridgeTargetTypeThread, BridgeTargetTypeGroup:
		return nil
	case "":
		return errors.New("bridges: bridge target type is required")
	default:
		return fmt.Errorf("bridges: unsupported bridge target type %q", strings.TrimSpace(string(t)))
	}
}

// BridgeTargetSnapshotRequest selects an instance for target enumeration.
type BridgeTargetSnapshotRequest struct {
	BridgeInstanceID string `json:"bridge_instance_id"`
}

// Validate reports whether the request identifies an instance.
func (r BridgeTargetSnapshotRequest) Validate() error {
	return requireField(strings.TrimSpace(r.BridgeInstanceID), "bridge target snapshot bridge instance id")
}

// BridgeTargetSnapshotResponse carries adapter-enumerated targets.
type BridgeTargetSnapshotResponse struct {
	Targets []BridgeTargetSnapshot `json:"targets"`
}

// BridgeTargetSnapshot is adapter-provided target data.
type BridgeTargetSnapshot struct {
	CanonicalRoute string           `json:"canonical_route"`
	DisplayName    string           `json:"display_name"`
	TargetType     BridgeTargetType `json:"target_type"`
	Qualifier      string           `json:"qualifier,omitempty"`
	Capabilities   []string         `json:"capabilities,omitempty"`
	LastSeenAt     time.Time        `json:"last_seen_at,omitzero"`
}

// Validate reports whether the snapshot has a stable provider identity.
func (s BridgeTargetSnapshot) Validate() error {
	if err := requireField(strings.TrimSpace(s.CanonicalRoute), "bridge target canonical route"); err != nil {
		return err
	}
	name := strings.TrimSpace(s.DisplayName)
	if err := requireField(name, "bridge target display name"); err != nil {
		return err
	}
	if err := s.TargetType.Normalize().Validate(); err != nil {
		return err
	}
	if NormalizeBridgeTargetName(name) == "" {
		return errors.New("bridges: bridge target normalized name is required")
	}
	return nil
}

// NormalizeBridgeTargetName canonicalizes display names for lookup.
func NormalizeBridgeTargetName(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimLeft(trimmed, "#@")
	return strings.ToLower(strings.TrimSpace(trimmed))
}

// NormalizeBridgeTargetQualifier canonicalizes workspace and guild qualifiers.
func NormalizeBridgeTargetQualifier(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimLeft(trimmed, "#@")
	return strings.ToLower(strings.TrimSpace(trimmed))
}
