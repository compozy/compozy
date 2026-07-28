package store

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// NetworkChannelEntry stores durable channel metadata for the operator-facing network workspace.
type NetworkChannelEntry struct {
	Channel           string
	WorkspaceID       string
	Purpose           string
	FanoutPolicy      string
	CoordinatorPeerID string
	CreatedBy         string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// NetworkChannelPatch describes a partial channel metadata update. Nil fields are left unchanged.
type NetworkChannelPatch struct {
	Purpose           *string
	FanoutPolicy      *string
	CoordinatorPeerID *string
	UpdatedAt         time.Time
}

// Apply returns the entry that would result from applying the patch.
func (p NetworkChannelPatch) Apply(entry NetworkChannelEntry) NetworkChannelEntry {
	if p.Purpose != nil {
		entry.Purpose = strings.TrimSpace(*p.Purpose)
	}
	if p.FanoutPolicy != nil {
		entry.FanoutPolicy = NormalizeNetworkFanoutPolicy(*p.FanoutPolicy)
	}
	if p.CoordinatorPeerID != nil {
		entry.CoordinatorPeerID = strings.TrimSpace(*p.CoordinatorPeerID)
	}
	if !p.UpdatedAt.IsZero() {
		entry.UpdatedAt = p.UpdatedAt.UTC()
	}
	return entry
}

// HasChanges reports whether the patch includes at least one mutable field.
func (p NetworkChannelPatch) HasChanges() bool {
	return p.Purpose != nil || p.FanoutPolicy != nil || p.CoordinatorPeerID != nil
}

// NetworkChannelRef identifies one workspace-qualified network channel.
type NetworkChannelRef struct {
	WorkspaceID string
	Channel     string
}

// Validate ensures the channel reference is workspace-qualified.
func (r NetworkChannelRef) Validate() error {
	if err := requireField(r.WorkspaceID, "network channel workspace_id"); err != nil {
		return err
	}
	return requireField(r.Channel, "network channel channel")
}

// Validate ensures the persisted channel metadata is complete.
func (e NetworkChannelEntry) Validate() error {
	if err := (NetworkChannelRef{WorkspaceID: e.WorkspaceID, Channel: e.Channel}).Validate(); err != nil {
		return err
	}
	if err := requireField(e.Purpose, "network channel purpose"); err != nil {
		return err
	}
	if err := ValidateNetworkChannelFanoutConfiguration(e.FanoutPolicy, e.CoordinatorPeerID); err != nil {
		return err
	}
	if strings.TrimSpace(e.CoordinatorPeerID) != "" {
		return validateNetworkPeerID(e.CoordinatorPeerID, "network channel coordinator_peer_id")
	}
	return nil
}

// ValidateNetworkChannelFanoutConfiguration checks policy and coordinator coupling.
func ValidateNetworkChannelFanoutConfiguration(policy string, coordinatorPeerID string) error {
	normalized := NormalizeNetworkFanoutPolicy(policy)
	if err := ValidateNetworkFanoutPolicy(normalized); err != nil {
		return err
	}
	coordinator := strings.TrimSpace(coordinatorPeerID)
	if normalized == NetworkFanoutPolicyCoordinator {
		if coordinator == "" {
			return errors.New("store: network channel coordinator_peer_id is required for coordinator fanout policy")
		}
		return nil
	}
	if coordinator != "" {
		return errors.New("store: network channel coordinator_peer_id requires coordinator fanout policy")
	}
	return nil
}

// NormalizeNetworkFanoutPolicy applies the default channel activation policy.
func NormalizeNetworkFanoutPolicy(policy string) string {
	trimmed := strings.TrimSpace(policy)
	if trimmed == "" {
		return NetworkFanoutPolicyCapabilityMatch
	}
	return trimmed
}

// ValidateNetworkFanoutPolicy checks one channel activation policy value.
func ValidateNetworkFanoutPolicy(policy string) error {
	switch NormalizeNetworkFanoutPolicy(policy) {
	case NetworkFanoutPolicyCapabilityMatch, NetworkFanoutPolicyCoordinator, NetworkFanoutPolicyAllMembers:
		return nil
	default:
		return fmt.Errorf("store: unsupported network fanout policy %q", policy)
	}
}

// NetworkChannelQuery filters persisted network channel metadata lookups.
type NetworkChannelQuery struct {
	Channel     string
	WorkspaceID string
	Limit       int
}

// Validate ensures the query uses sane bounds.
func (q NetworkChannelQuery) Validate() error {
	if err := requireField(q.WorkspaceID, "network channel query workspace_id"); err != nil {
		return err
	}
	return requirePositiveLimit(q.Limit, "network channel limit")
}
