package session

import (
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

// Spawn limits name the cap a SpawnLimitError or SpawnDepthError refers to.
const (
	SpawnLimitMaxChildren           = "max_children"
	SpawnLimitMaxActivePerWorkspace = "max_active_per_workspace"
	SpawnLimitMaxDepth              = "max_depth"
)

// Coordinator config keys own the coordinator session's spawn budget; every other
// session uses the store defaults.
const (
	coordinatorMaxChildrenConfigKey = "roles.coordinator.max_children"
	coordinatorMaxActiveConfigKey   = "roles.coordinator.max_active_sessions_per_workspace"

	spawnSubagentRecovery           = "use compozy__subagent_delegate for answers your current turn waits on."
	spawnDefaultMaxChildrenRecovery = "This session uses the default per-session cap; " +
		"roles.coordinator.max_children applies only to coordinator sessions."
)

// SpawnLimitError reports a spawn refused by a live-child cap. It matches ErrSpawnLimitExceeded.
// Only live (starting, active, or stopping) governed children count; stopped children and
// subagents do not hold a slot.
type SpawnLimitError struct {
	// Limit is SpawnLimitMaxChildren or SpawnLimitMaxActivePerWorkspace.
	Limit string
	Max   int
	Live  int
	// Scope is the parent session ID or the target workspace ID the cap applies to.
	Scope string
	// ConfigKey names the config key that sets Max, or is empty when the session's own
	// stored budget or a daemon default sets it.
	ConfigKey string
	// DefaultCap reports that Max is the daemon default because the stored cap is unset.
	DefaultCap bool
}

// Error names the cap, its value, and the live count.
func (e *SpawnLimitError) Error() string {
	if e == nil {
		return ErrSpawnLimitExceeded.Error()
	}
	return fmt.Sprintf(
		"%s: %s %d reached for %q with %d live spawned sessions (stopped sessions and subagents do not count)",
		ErrSpawnLimitExceeded,
		e.Limit,
		e.Max,
		e.Scope,
		e.Live,
	)
}

// Is lets callers branch on ErrSpawnLimitExceeded.
func (e *SpawnLimitError) Is(target error) bool {
	return target == ErrSpawnLimitExceeded
}

// Recovery names the next step and, when the cap is configurable, the config key.
func (e *SpawnLimitError) Recovery() string {
	if e == nil {
		return ""
	}
	recovery := "Stop a live child (compozy__session_stop) or wait for one to stop, then retry; " +
		spawnSubagentRecovery
	if e.Limit == SpawnLimitMaxActivePerWorkspace {
		recovery = "Stop a live spawned session of this session tree in this workspace or wait for one to stop, " +
			"then retry; " + spawnSubagentRecovery
	}
	switch {
	case e.ConfigKey != "":
		return recovery + " The cap is set by " + e.ConfigKey + "."
	case e.Limit == SpawnLimitMaxChildren && e.DefaultCap:
		return recovery + " " + spawnDefaultMaxChildrenRecovery
	default:
		return recovery
	}
}

// SpawnDepthError reports a spawn refused because the child would exceed the
// governed spawn depth. It matches ErrSpawnLimitExceeded.
type SpawnDepthError struct {
	// Parent is the session that requested the spawn.
	Parent string
	// ChildDepth is the governed depth the refused child would have had.
	ChildDepth int
	Max        int
}

// Error names the depth cap, the would-be child depth, and the parent.
func (e *SpawnDepthError) Error() string {
	if e == nil {
		return ErrSpawnLimitExceeded.Error()
	}
	return fmt.Sprintf(
		"%s: %s %d reached for %q: a child would sit at governed depth %d "+
			"(live-child count is not the cause; subagents have no depth cap)",
		ErrSpawnLimitExceeded,
		SpawnLimitMaxDepth,
		e.Max,
		e.Parent,
		e.ChildDepth,
	)
}

// Is lets callers branch on ErrSpawnLimitExceeded.
func (e *SpawnDepthError) Is(target error) bool {
	return target == ErrSpawnLimitExceeded
}

// Recovery points at the paths that remain open at the depth cap.
func (e *SpawnDepthError) Recovery() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf(
		"Governed children of this session would exceed %s %d (child depth %d). Delegate with "+
			"compozy__subagent_delegate, which has no depth cap, or ask the root session to spawn the child.",
		SpawnLimitMaxDepth,
		e.Max,
		e.ChildDepth,
	)
}

// LineageSpawnBudget returns the stored spawn budget of a possibly nil lineage.
func LineageSpawnBudget(lineage *store.SessionLineage) store.SessionSpawnBudget {
	if lineage == nil {
		return store.SessionSpawnBudget{}
	}
	return lineage.SpawnBudget
}

// spawnBudgetFromCoordinator reports whether a parent's budget was written from
// roles.coordinator config.
func spawnBudgetFromCoordinator(parent *Info) bool {
	if parent == nil {
		return false
	}
	if normalizeSessionType(parent.Type) == SessionTypeCoordinator {
		return true
	}
	return parent.Lineage != nil && isCoordinatorSpawnRole(parent.Lineage.SpawnRole)
}
