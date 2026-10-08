package session

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

// ErrRecoveryPersistence prevents admission against an incompletely recovered session inventory.
var ErrRecoveryPersistence = errors.New("session: recovery persistence failed")

// RecoveryMetadataSkipReason refuses unsupported lifecycle metadata before recovery can normalize or persist it.
func RecoveryMetadataSkipReason(meta *store.SessionMeta) string {
	if meta == nil {
		return ""
	}
	sessionType := Type(strings.TrimSpace(meta.SessionType))
	if sessionType != "" && normalizeSessionType(sessionType) != sessionType {
		return "unknown_session_type"
	}
	if meta.Lineage != nil {
		role := strings.TrimSpace(meta.Lineage.SpawnRole)
		if slices.Contains(toolspkg.RetiredInternalSpawnRoles, role) {
			return "retired_spawn_role"
		}
	}
	return ""
}

func (m *Manager) persistRecoveryCatalog(ctx context.Context, meta *store.SessionMeta) error {
	if m.sessionCatalog == nil {
		return nil
	}
	before, err := m.durableSessionInfoForLifecycleTransition(ctx, meta.ID)
	if err != nil {
		return err
	}
	after := sessionInfoFromMeta(meta)
	if before.State == after.State && before.StopReason == after.StopReason &&
		before.StopDetail == after.StopDetail && before.StopEscalated == after.StopEscalated &&
		before.StopVerificationFailed == after.StopVerificationFailed &&
		before.ACPSessionID == after.ACPSessionID && sessionFailureEqual(before.Failure, after.Failure) &&
		sessionLivenessEqual(before.Liveness, after.Liveness) {
		return nil
	}
	if err := m.hydrateSessionInfoAttention(ctx, after); err != nil {
		return err
	}
	update := sessionCatalogStateUpdate(after)
	update.ACPSessionID = new(after.ACPSessionID)
	update.AttentionTransition = BadgeForInfo(before) != BadgeForInfo(after)
	if err := m.sessionCatalog.UpdateSessionState(ctx, update); err != nil {
		return fmt.Errorf("session: project recovered state for %q: %w", meta.ID, err)
	}
	if err := m.hydrateSessionInfoAttention(ctx, after); err != nil {
		return err
	}
	m.publishLifecycleAttentionTransition(ctx, before, after)
	return nil
}
