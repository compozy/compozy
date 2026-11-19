package agentinstance

import (
	"context"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

type Caller struct {
	SessionID, AgentName, ProfileID, InstanceID string
	InstanceDepth                               int
	PermissionPolicy                            store.SessionPermissionPolicy
	Grant                                       string
}

type CallerReader interface {
	InstanceCaller(context.Context, string) (Caller, error)
}

type Authority struct{ Callers CallerReader }

// AuthorizeMemory requires a daemon-resolved instance lineage for the named agent.
func (a Authority) AuthorizeMemory(ctx context.Context, sessionID, agentName, profileID string) error {
	if a.Callers == nil || sessionID == "" {
		return ErrForbidden
	}
	caller, err := a.Callers.InstanceCaller(ctx, sessionID)
	if err != nil {
		return err
	}
	if caller.InstanceID == "" || caller.AgentName != agentName || caller.ProfileID != profileID {
		return ErrForbidden
	}
	return nil
}

func (a Authority) Authorize(
	ctx context.Context,
	actor workspaceaccess.ActorRef,
	instance Instance,
	verb string,
) error {
	if actor.Operator {
		return nil
	}
	if actor.Kind != workspaceaccess.ActorAgentSession || actor.SessionID == "" || a.Callers == nil {
		return ErrForbidden
	}
	caller, err := a.Callers.InstanceCaller(ctx, actor.SessionID)
	if err != nil {
		return err
	}
	if verb == "peers" && caller.InstanceID != "" && caller.ProfileID == instance.ProfileID {
		return nil // The catalog owner still applies workspace access before pagination.
	}
	if caller.InstanceID != instance.ID || caller.AgentName != instance.AgentName ||
		caller.ProfileID != instance.ProfileID {
		return ErrForbidden
	}
	switch verb {
	case "list",
		"reports",
		"skill_propose",
		"skill_reviews",
		"occurrence",
		"pause",
		"archive",
		"ensure_main",
		"check",
		"check_dry_run",
		"checks_off":
		return nil
	case "resume":
		if !OperatorOnlyResume(instance.PauseReason) {
			return nil
		}
	}
	return ErrForbidden
}

func IntersectPermissions(a, b store.SessionPermissionPolicy) store.SessionPermissionPolicy {
	a = store.NormalizeSessionPermissionPolicy(a)
	b = store.NormalizeSessionPermissionPolicy(b)
	mode := store.PermissionList
	if a.MCPServersMode == store.PermissionInherit && b.MCPServersMode == store.PermissionInherit {
		mode = store.PermissionInherit
	}
	toolsMode := store.PermissionList
	if a.ToolsMode == store.PermissionInherit && b.ToolsMode == store.PermissionInherit {
		toolsMode = store.PermissionInherit
	}
	servers := intersectAtoms(a.MCPServers, b.MCPServers)
	if a.MCPServersMode == store.PermissionInherit {
		servers = b.MCPServers
	}
	if b.MCPServersMode == store.PermissionInherit {
		servers = a.MCPServers
	}
	ceiling := a.AncestorMCPCeiling
	if b.AncestorMCPCeiling != nil {
		ceiling = store.IntersectMCPCeiling(ceiling, *b.AncestorMCPCeiling)
	}
	return store.SessionPermissionPolicy{
		MCPServersMode: mode, ToolsMode: toolsMode, AncestorMCPCeiling: ceiling,
		Tools:          intersectTools(a, b),
		Skills:         intersectAtoms(a.Skills, b.Skills),
		MCPServers:     servers,
		WorkspacePaths: intersectAtoms(a.WorkspacePaths, b.WorkspacePaths),
	}
}

// Inherited external tools are open; stored native tools remain a concrete ceiling.
func intersectTools(a, b store.SessionPermissionPolicy) []string {
	result := intersectAtoms(a.Tools, b.Tools)
	for _, pair := range [][2]store.SessionPermissionPolicy{{a, b}, {b, a}} {
		if pair[0].ToolsMode != store.PermissionInherit {
			continue
		}
		for _, id := range pair[1].Tools {
			if !strings.HasPrefix(id, "compozy__") && !slices.Contains(result, id) {
				result = append(result, id)
			}
		}
	}
	return store.NormalizeSessionPermissionPolicy(store.SessionPermissionPolicy{Tools: result}).Tools
}

func intersectAtoms(a, b []string) []string {
	result := make([]string, 0)
	for _, atom := range a {
		if slices.Contains(b, atom) {
			result = append(result, atom)
		}
	}
	return result
}

const (
	pauseReasonManual                = "manual"
	pauseReasonDefinitionMissing     = "definition_missing"
	pauseReasonDefinitionNotAlwaysOn = "definition_not_always_on"
)

type budgetResumeAppKey struct{}

// WithAppBudgetResume marks application credentials while preserving existing
// lifecycle authority for pauses unrelated to the daily budget.
func WithAppBudgetResume(ctx context.Context) context.Context {
	return context.WithValue(ctx, budgetResumeAppKey{}, true)
}

// HasDailyBudgetResumeAuthority reserves budget resume for an operator without app credentials.
func HasDailyBudgetResumeAuthority(ctx context.Context, operator bool) bool {
	app, _ := ctx.Value(budgetResumeAppKey{}).(bool)
	return operator && !app
}

// OperatorOnlyResume keeps safety and unknown pauses under operator control.
func OperatorOnlyResume(reason string) bool {
	return reason != pauseReasonDefinitionMissing && reason != pauseReasonDefinitionNotAlwaysOn
}
