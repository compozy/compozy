package bridges

import (
	"errors"
	"fmt"
	"strings"
	"time"

	bridgecontract "github.com/compozy/agh/internal/bridges/contract"
)

// RoutingKey is the canonical identity used to resolve bridge traffic to one ACP session.
type RoutingKey struct {
	Scope            Scope  `json:"scope"`
	WorkspaceID      string `json:"workspace_id,omitempty"`
	BridgeInstanceID string `json:"bridge_instance_id"`
	PeerID           string `json:"peer_id,omitempty"`
	ThreadID         string `json:"thread_id,omitempty"`
	GroupID          string `json:"group_id,omitempty"`
}

// BuildRoutingKey constructs the canonical routing key for one instance using
// the instance's fixed base identity and policy-selected routing dimensions.
func BuildRoutingKey(instance BridgeInstance, dims RoutingDimensions) (RoutingKey, error) {
	key, err := bridgecontract.BuildRoutingKey(
		BridgeInstanceToContract(instance),
		bridgecontract.RoutingDimensions{
			PeerID: dims.PeerID, ThreadID: dims.ThreadID, GroupID: dims.GroupID,
		},
	)
	if err != nil {
		return RoutingKey{}, err
	}
	return routingKeyFromContract(key), nil
}

// CanonicalizeRoutingKey rebuilds the supplied routing key under the instance's
// routing policy and validates that any supplied base identity matches.
func CanonicalizeRoutingKey(instance BridgeInstance, key RoutingKey) (RoutingKey, error) {
	if err := validateRoutingKeyBase(instance, key); err != nil {
		return RoutingKey{}, err
	}
	return BuildRoutingKey(instance, dimensionsFromRoutingKey(key))
}

// Validate reports whether the routing key carries the required base identity.
func (k RoutingKey) Validate() error {
	return routingKeyToContract(k).Validate()
}

// Serialize returns the stable serialized representation used for routing-key hashing.
func (k RoutingKey) Serialize() (string, error) {
	return routingKeyToContract(k).Serialize()
}

// Hash returns the stable SHA-256 hash for the serialized routing key.
func (k RoutingKey) Hash() (string, error) {
	return routingKeyToContract(k).Hash()
}

// BridgeRoute persists the canonical routing-key to ACP-session mapping.
type BridgeRoute struct {
	RoutingKeyHash   string    `json:"routing_key_hash"`
	Scope            Scope     `json:"scope"`
	WorkspaceID      string    `json:"workspace_id,omitempty"`
	BridgeInstanceID string    `json:"bridge_instance_id"`
	PeerID           string    `json:"peer_id,omitempty"`
	ThreadID         string    `json:"thread_id,omitempty"`
	GroupID          string    `json:"group_id,omitempty"`
	SessionID        string    `json:"session_id"`
	AgentName        string    `json:"agent_name"`
	LastActivityAt   time.Time `json:"last_activity_at"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// RoutingKey returns the canonical routing key represented by the route.
func (r BridgeRoute) RoutingKey() RoutingKey {
	normalized := r.normalize()
	return RoutingKey{
		Scope:            normalized.Scope,
		WorkspaceID:      normalized.WorkspaceID,
		BridgeInstanceID: normalized.BridgeInstanceID,
		PeerID:           normalized.PeerID,
		ThreadID:         normalized.ThreadID,
		GroupID:          normalized.GroupID,
	}
}

// Validate reports whether the persisted route is complete and internally consistent.
func (r BridgeRoute) Validate() error {
	normalized := r.normalize()
	if err := normalized.RoutingKey().Validate(); err != nil {
		return err
	}
	if err := requireField(normalized.SessionID, "bridge route session id"); err != nil {
		return err
	}
	if err := requireField(normalized.AgentName, "bridge route agent name"); err != nil {
		return err
	}
	if strings.TrimSpace(normalized.RoutingKeyHash) != "" {
		hash, err := normalized.RoutingKey().Hash()
		if err != nil {
			return err
		}
		if normalized.RoutingKeyHash != hash {
			return errors.New("bridges: routing key hash does not match route identity")
		}
	}
	return nil
}

// Canonicalize normalizes the route and fills the routing-key hash when missing.
func (r BridgeRoute) Canonicalize() (BridgeRoute, error) {
	normalized := r.normalize()
	if err := normalized.Validate(); err != nil {
		return BridgeRoute{}, err
	}
	if normalized.RoutingKeyHash == "" {
		hash, err := normalized.RoutingKey().Hash()
		if err != nil {
			return BridgeRoute{}, err
		}
		normalized.RoutingKeyHash = hash
	}
	return normalized, nil
}

// CanonicalizeRoute rebuilds the supplied route identity under the instance's
// routing policy and computes the expected routing-key hash.
func CanonicalizeRoute(instance BridgeInstance, route BridgeRoute) (BridgeRoute, error) {
	normalizedRoute := route.normalize()
	if err := requireField(normalizedRoute.BridgeInstanceID, "bridge route bridge instance id"); err != nil {
		return BridgeRoute{}, err
	}

	key, err := CanonicalizeRoutingKey(instance, normalizedRoute.RoutingKey())
	if err != nil {
		return BridgeRoute{}, err
	}

	canonical := normalizedRoute
	canonical.Scope = key.Scope
	canonical.WorkspaceID = key.WorkspaceID
	canonical.BridgeInstanceID = key.BridgeInstanceID
	canonical.PeerID = key.PeerID
	canonical.ThreadID = key.ThreadID
	canonical.GroupID = key.GroupID

	expectedHash, err := canonical.RoutingKey().Hash()
	if err != nil {
		return BridgeRoute{}, err
	}
	if canonical.RoutingKeyHash != "" && canonical.RoutingKeyHash != expectedHash {
		return BridgeRoute{}, fmt.Errorf(
			"bridges: routing key hash %q does not match canonical hash %q",
			canonical.RoutingKeyHash,
			expectedHash,
		)
	}
	canonical.RoutingKeyHash = expectedHash

	if err := canonical.Validate(); err != nil {
		return BridgeRoute{}, err
	}

	return canonical, nil
}

func (k RoutingKey) normalize() RoutingKey {
	return routingKeyFromContract(bridgecontract.NormalizeRoutingKey(routingKeyToContract(k)))
}

func (r BridgeRoute) normalize() BridgeRoute {
	normalized := r
	normalized.RoutingKeyHash = strings.TrimSpace(normalized.RoutingKeyHash)
	normalized.Scope = normalized.Scope.Normalize()
	normalized.WorkspaceID = strings.TrimSpace(normalized.WorkspaceID)
	normalized.BridgeInstanceID = strings.TrimSpace(normalized.BridgeInstanceID)
	normalized.PeerID = strings.TrimSpace(normalized.PeerID)
	normalized.ThreadID = strings.TrimSpace(normalized.ThreadID)
	normalized.GroupID = strings.TrimSpace(normalized.GroupID)
	normalized.SessionID = strings.TrimSpace(normalized.SessionID)
	normalized.AgentName = strings.TrimSpace(normalized.AgentName)
	return normalized
}
