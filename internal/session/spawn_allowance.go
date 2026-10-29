package session

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

// minSpawnedGovernedDepth is the shallowest governed depth a spawned session can have.
const minSpawnedGovernedDepth = 1

// ChildSpawnAllowance is the spawn budget a session holds for its own governed children.
type ChildSpawnAllowance struct {
	// Budget is the effective budget the spawn engine enforces for this session's children.
	Budget store.SessionSpawnBudget
	// GovernedDepth counts only spawned-type ancestors, unlike the display lineage depth.
	GovernedDepth int
	// Allowed reports whether the session's depth still admits a governed child.
	Allowed bool
	// BlockedReason explains a false Allowed in agent-readable terms.
	BlockedReason string
}

// ChildSpawnAllowance resolves a session's child-spawn allowance with the same
// ancestor walk the spawn engine uses. The session itself need not be registered yet;
// only its spawned-type ancestors are looked up.
func (m *Manager) ChildSpawnAllowance(ctx context.Context, info *Info) (ChildSpawnAllowance, error) {
	governance, err := m.spawnGovernanceForParent(ctx, info)
	if err != nil {
		return ChildSpawnAllowance{}, err
	}
	return childSpawnAllowance(info.ID, LineageSpawnBudget(info.Lineage), governance.depth), nil
}

// ChildSpawnAllowanceFor is the lookup-free allowance: user, coordinator, and
// provenance-linked sessions sit at governed depth 0, and a spawned session is
// assumed to sit at the shallowest spawned depth.
func ChildSpawnAllowanceFor(sessionType Type, budget store.SessionSpawnBudget) ChildSpawnAllowance {
	depth := 0
	if normalizeSessionType(sessionType) == SessionTypeSpawned {
		depth = minSpawnedGovernedDepth
	}
	return childSpawnAllowance("", budget, depth)
}

func childSpawnAllowance(sessionID string, budget store.SessionSpawnBudget, depth int) ChildSpawnAllowance {
	allowance := ChildSpawnAllowance{
		Budget:        store.EffectiveSessionSpawnBudget(budget),
		GovernedDepth: depth,
		Allowed:       true,
	}
	if childDepth := depth + 1; childDepth > allowance.Budget.MaxDepth {
		refusal := &SpawnDepthError{Parent: sessionID, ChildDepth: childDepth, Max: allowance.Budget.MaxDepth}
		allowance.Allowed = false
		allowance.BlockedReason = fmt.Sprintf("%s: %s", SpawnLimitMaxDepth, refusal.Recovery())
	}
	return allowance
}
