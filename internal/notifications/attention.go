package notifications

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
)

// ErrAttentionSnapshotUnavailable requires a fresh notification read before retrying.
var ErrAttentionSnapshotUnavailable = errors.New(
	"notifications: snapshot expired or unavailable; refresh notifications",
)

// AttentionScope binds receipts to the operator and snapshots to their selected population.
type AttentionScope struct {
	ProfileID  string
	ActorKind  string
	ActorID    string
	Population string
}

func (s AttentionScope) Validate() error {
	if s.ProfileID == "" || s.ActorKind == "" || s.ActorID == "" || s.Population == "" {
		return errors.New("notifications: profile, actor and population are required")
	}
	return nil
}

// AttentionStore persists exact occurrence receipts without changing source state.
type AttentionStore interface {
	CaptureAttentionSnapshot(context.Context, AttentionScope, []string) (string, []string, error)
	AcknowledgeAttentionSnapshot(context.Context, AttentionScope, string, string) error
}

// AttentionIdentity preserves source boundaries even when identifiers contain separators.
func AttentionIdentity(parts ...string) string {
	// A string slice is always JSON encodable.
	raw, _ := json.Marshal(parts)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// AttentionSnapshotIdentity deduplicates unchanged snapshots across polling clients.
func AttentionSnapshotIdentity(scope AttentionScope, occurrences []string) (string, []string) {
	ids := slices.Clone(occurrences)
	slices.Sort(ids)
	ids = slices.Compact(ids)
	parts := append([]string{scope.ProfileID, scope.ActorKind, scope.ActorID, scope.Population}, ids...)
	return AttentionIdentity(parts...), ids
}
