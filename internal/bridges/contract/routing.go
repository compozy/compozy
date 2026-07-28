package contract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
)

// RoutingDimensions carries platform-normalized routing identity.
type RoutingDimensions struct {
	PeerID   string `json:"peer_id,omitempty"`
	ThreadID string `json:"thread_id,omitempty"`
	GroupID  string `json:"group_id,omitempty"`
}

// RoutingKey is the canonical identity used to resolve bridge traffic.
type RoutingKey struct {
	Scope            Scope  `json:"scope"`
	WorkspaceID      string `json:"workspace_id,omitempty"`
	BridgeInstanceID string `json:"bridge_instance_id"`
	PeerID           string `json:"peer_id,omitempty"`
	ThreadID         string `json:"thread_id,omitempty"`
	GroupID          string `json:"group_id,omitempty"`
}

// NormalizeRoutingKey returns the canonical wire routing identity.
func NormalizeRoutingKey(key RoutingKey) RoutingKey {
	return key.normalize()
}

// BuildRoutingKey constructs a key from an instance and selected dimensions.
func BuildRoutingKey(instance BridgeInstance, dims RoutingDimensions) (RoutingKey, error) {
	normalizedInstance := instance.normalize()
	if err := normalizedInstance.Validate(); err != nil {
		return RoutingKey{}, err
	}
	normalizedDims := dims.normalize()
	if err := validateRoutingDimensions(normalizedInstance.RoutingPolicy, normalizedDims); err != nil {
		return RoutingKey{}, err
	}
	key := RoutingKey{
		Scope:            normalizedInstance.Scope,
		WorkspaceID:      normalizedInstance.WorkspaceID,
		BridgeInstanceID: normalizedInstance.ID,
	}
	if normalizedInstance.RoutingPolicy.IncludePeer {
		key.PeerID = normalizedDims.PeerID
	}
	if normalizedInstance.RoutingPolicy.IncludeThread {
		key.ThreadID = normalizedDims.ThreadID
	}
	if normalizedInstance.RoutingPolicy.IncludeGroup {
		key.GroupID = normalizedDims.GroupID
	}
	return key.normalize(), nil
}

// Validate reports whether the key carries its required base identity.
func (k RoutingKey) Validate() error {
	normalized := k.normalize()
	if err := ValidateScopeWorkspaceID(normalized.Scope, normalized.WorkspaceID); err != nil {
		return err
	}
	return requireField(normalized.BridgeInstanceID, "routing key bridge instance id")
}

// Serialize returns the stable representation used for routing-key hashing.
func (k RoutingKey) Serialize() (string, error) {
	normalized := k.normalize()
	if err := normalized.Validate(); err != nil {
		return "", err
	}
	payload, err := json.Marshal(struct {
		Scope            Scope  `json:"scope"`
		WorkspaceID      string `json:"workspace_id"`
		BridgeInstanceID string `json:"bridge_instance_id"`
		PeerID           string `json:"peer_id"`
		ThreadID         string `json:"thread_id"`
		GroupID          string `json:"group_id"`
	}{
		Scope:            normalized.Scope,
		WorkspaceID:      normalized.WorkspaceID,
		BridgeInstanceID: normalized.BridgeInstanceID,
		PeerID:           normalized.PeerID,
		ThreadID:         normalized.ThreadID,
		GroupID:          normalized.GroupID,
	})
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

// Hash returns the stable SHA-256 hash of the serialized key.
func (k RoutingKey) Hash() (string, error) {
	serialized, err := k.Serialize()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(serialized))
	return hex.EncodeToString(sum[:]), nil
}

func (k RoutingKey) normalize() RoutingKey {
	k.Scope = k.Scope.Normalize()
	k.WorkspaceID = strings.TrimSpace(k.WorkspaceID)
	k.BridgeInstanceID = strings.TrimSpace(k.BridgeInstanceID)
	k.PeerID = strings.TrimSpace(k.PeerID)
	k.ThreadID = strings.TrimSpace(k.ThreadID)
	k.GroupID = strings.TrimSpace(k.GroupID)
	return k
}

func (d RoutingDimensions) normalize() RoutingDimensions {
	return RoutingDimensions{
		PeerID: strings.TrimSpace(d.PeerID), ThreadID: strings.TrimSpace(d.ThreadID),
		GroupID: strings.TrimSpace(d.GroupID),
	}
}

func validateRoutingDimensions(policy RoutingPolicy, dims RoutingDimensions) error {
	if err := policy.Validate(); err != nil {
		return err
	}
	if policy.IncludePeer && dims.PeerID == "" {
		return errors.New("bridges: routing policy requires peer id")
	}
	if policy.IncludeThread && dims.ThreadID == "" {
		return errors.New("bridges: routing policy requires thread id")
	}
	if policy.IncludeGroup && dims.GroupID == "" {
		return errors.New("bridges: routing policy requires group id")
	}
	return nil
}
