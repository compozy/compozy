package attention

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
)

// ErrSnapshotUnavailable requires a fresh notification read before retrying.
var ErrSnapshotUnavailable = errors.New(
	"notifications: snapshot expired or unavailable; refresh notifications",
)

// Scope binds receipts to the operator and snapshots to their selected population.
type Scope struct {
	ProfileID  string
	ActorKind  string
	ActorID    string
	Population string
}

func (s Scope) Validate() error {
	if s.ProfileID == "" || s.ActorKind == "" || s.ActorID == "" || s.Population == "" {
		return errors.New("notifications: profile, actor and population are required")
	}
	return nil
}

// Store persists exact occurrence receipts without changing source state.
type Store interface {
	CaptureAttentionSnapshot(context.Context, Scope, []string) (string, []string, error)
	AcknowledgeAttentionSnapshot(context.Context, Scope, string, string) error
}

// Identity preserves source boundaries even when identifiers contain separators.
func Identity(parts ...string) string {
	raw, err := json.Marshal(parts)
	if err != nil {
		// []string has no unsupported values, cycles or custom marshalers.
		panic("invariant: encode attention identity: " + err.Error())
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// SnapshotIdentity deduplicates unchanged snapshots across polling clients.
func SnapshotIdentity(scope Scope, occurrences []string) (string, []string) {
	ids := slices.Clone(occurrences)
	slices.Sort(ids)
	ids = slices.Compact(ids)
	parts := append([]string{scope.ProfileID, scope.ActorKind, scope.ActorID, scope.Population}, ids...)
	return Identity(parts...), ids
}
