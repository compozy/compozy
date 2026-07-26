package bridges

import (
	"errors"
	"fmt"
	"strings"
)

// RoutingDimensions carries the platform-normalized identity values that may
// participate in a routing key, depending on the instance routing policy.
type RoutingDimensions struct {
	PeerID   string `json:"peer_id,omitempty"`
	ThreadID string `json:"thread_id,omitempty"`
	GroupID  string `json:"group_id,omitempty"`
}

// PlatformDimensionMapping documents how one adapter maps platform-native
// identity concepts onto AGH's canonical routing dimensions.
//
// Semantics:
// - `peer_id` identifies the direct conversation peer or primary counterparty.
// - `thread_id` identifies a sub-conversation nested under a peer or group.
// - `group_id` identifies a shared container such as a room, forum, bridge, or guild.
//
// Adapters should publish one mapping per platform so route inspection and
// cross-platform tooling can interpret `peer_id`, `thread_id`, and `group_id`
// consistently.
type PlatformDimensionMapping struct {
	Platform        string `json:"platform"`
	PeerIDConcept   string `json:"peer_id_concept,omitempty"`
	ThreadIDConcept string `json:"thread_id_concept,omitempty"`
	GroupIDConcept  string `json:"group_id_concept,omitempty"`
}

// Validate reports whether the contract identifies a platform and at least one
// mapped routing dimension concept.
func (m PlatformDimensionMapping) Validate() error {
	normalized := m.normalize()
	if err := requireField(normalized.Platform, "platform dimension mapping platform"); err != nil {
		return err
	}
	if normalized.PeerIDConcept == "" && normalized.ThreadIDConcept == "" && normalized.GroupIDConcept == "" {
		return errors.New("bridges: platform dimension mapping must describe at least one routing dimension")
	}
	return nil
}

func dimensionsFromRoutingKey(key RoutingKey) RoutingDimensions {
	normalized := key.normalize()
	return RoutingDimensions{
		PeerID:   normalized.PeerID,
		ThreadID: normalized.ThreadID,
		GroupID:  normalized.GroupID,
	}
}

func (m PlatformDimensionMapping) normalize() PlatformDimensionMapping {
	return PlatformDimensionMapping{
		Platform:        strings.TrimSpace(m.Platform),
		PeerIDConcept:   strings.TrimSpace(m.PeerIDConcept),
		ThreadIDConcept: strings.TrimSpace(m.ThreadIDConcept),
		GroupIDConcept:  strings.TrimSpace(m.GroupIDConcept),
	}
}

func validateRoutingKeyBase(instance BridgeInstance, key RoutingKey) error {
	normalizedInstance := instance.normalize()
	normalizedKey := key.normalize()

	if normalizedKey.BridgeInstanceID != "" && normalizedKey.BridgeInstanceID != normalizedInstance.ID {
		return fmt.Errorf(
			"bridges: routing key bridge instance id %q does not match instance %q",
			normalizedKey.BridgeInstanceID,
			normalizedInstance.ID,
		)
	}
	if normalizedKey.Scope != "" && normalizedKey.Scope != normalizedInstance.Scope {
		return fmt.Errorf(
			"bridges: routing key scope %q does not match instance scope %q",
			normalizedKey.Scope,
			normalizedInstance.Scope,
		)
	}
	if normalizedKey.WorkspaceID != "" && normalizedKey.WorkspaceID != normalizedInstance.WorkspaceID {
		return fmt.Errorf(
			"bridges: routing key workspace id %q does not match instance workspace id %q",
			normalizedKey.WorkspaceID,
			normalizedInstance.WorkspaceID,
		)
	}

	return nil
}
