package session

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/workspaceaccess"
)

// Spawn permission rules name why one requested atom was refused.
const (
	// SpawnPermissionRuleSubset refuses an atom the parent ceiling does not grant.
	SpawnPermissionRuleSubset = "subset_of_parent"
	// SpawnPermissionRuleBlankAtom refuses an empty permission atom.
	SpawnPermissionRuleBlankAtom = "blank_atom"
	// SpawnPermissionRuleUnknownToolset refuses a toolset name the catalog cannot expand.
	SpawnPermissionRuleUnknownToolset = "known_toolset"
)

const spawnPermissionAllowedPreview = 8

// SpawnPermissionError reports the one field, value, and rule that refused a spawn
// permission request. It matches ErrSpawnPermissionDenied.
type SpawnPermissionError struct {
	// Field is the request field, such as tools, toolsets, or workspace_paths.
	Field string
	// Value is the refused atom.
	Value string
	// Rule is one of the SpawnPermissionRule* constants.
	Rule string
	// Allowed is the parent's ceiling for Field; empty means the parent grants none.
	Allowed []string
}

// Error names the field, value, and rule without reformatting the request.
func (e *SpawnPermissionError) Error() string {
	if e == nil {
		return ErrSpawnPermissionDenied.Error()
	}
	switch e.Rule {
	case SpawnPermissionRuleBlankAtom:
		return fmt.Sprintf("%s: %s includes a blank permission atom", ErrSpawnPermissionDenied, e.Field)
	case SpawnPermissionRuleUnknownToolset:
		return fmt.Sprintf("%s: %s value %q is not a known toolset", ErrSpawnPermissionDenied, e.Field, e.Value)
	default:
		return fmt.Sprintf(
			"%s: %s value %q widens parent permissions (rule %s; parent grants %s)",
			ErrSpawnPermissionDenied,
			e.Field,
			e.Value,
			e.Rule,
			e.allowedSummary(),
		)
	}
}

// Is lets callers branch on ErrSpawnPermissionDenied.
func (e *SpawnPermissionError) Is(target error) bool {
	return target == ErrSpawnPermissionDenied
}

// Recovery is the caller-facing next step for this refusal.
func (e *SpawnPermissionError) Recovery() string {
	if e == nil {
		return ""
	}
	switch {
	case e.Field == "workspace_paths":
		return "workspace_paths only records a narrowing atom; it does not move the child's checkout. " +
			"Bind a managed worktree with worktree or new_worktree instead, or omit workspace_paths."
	case e.Rule == SpawnPermissionRuleUnknownToolset:
		return "Use a toolset ID listed by compozy__tool_search or compozy__tool_list."
	default:
		return "Omit " + e.Field + " (or set inherit_permissions) to keep the parent's grant, " +
			"or request only values the parent grants."
	}
}

func (e *SpawnPermissionError) allowedSummary() string {
	if len(e.Allowed) == 0 {
		return "none"
	}
	if len(e.Allowed) <= spawnPermissionAllowedPreview {
		return "[" + strings.Join(e.Allowed, ", ") + "]"
	}
	return fmt.Sprintf(
		"[%s, ... %d more]",
		strings.Join(e.Allowed[:spawnPermissionAllowedPreview], ", "),
		len(e.Allowed)-spawnPermissionAllowedPreview,
	)
}

// Spawn limits name the live-child cap a SpawnLimitError refers to.
const (
	SpawnLimitMaxChildren           = "max_children"
	SpawnLimitMaxActivePerWorkspace = "max_active_per_workspace"
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

func spawnWorkspaceAccessDenied(targetWorkspaceID string, parentWorkspaceID string) error {
	return fmt.Errorf(
		"%w: child workspace %q is outside parent workspace %q: %s",
		ErrSpawnPermissionDenied,
		targetWorkspaceID,
		parentWorkspaceID,
		workspaceaccess.DenialHint,
	)
}
