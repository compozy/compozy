package daemon

import (
	"context"

	core "github.com/compozy/agh/internal/api/core"

	toolspkg "github.com/compozy/agh/internal/tools"
)

func (n *daemonNativeTools) registryAvailability() toolspkg.NativeAvailabilityFunc {
	return func(context.Context, toolspkg.Scope) toolspkg.Availability {
		if n.registry() == nil {
			return toolspkg.Unavailable(toolspkg.ReasonDependencyMissing)
		}
		return toolspkg.Available()
	}
}

func (n *daemonNativeTools) dependencyAvailability(ready func() bool) toolspkg.NativeAvailabilityFunc {
	return func(context.Context, toolspkg.Scope) toolspkg.Availability {
		if ready == nil || !ready() {
			return toolspkg.Unavailable(toolspkg.ReasonDependencyMissing)
		}
		return toolspkg.Available()
	}
}

func (n *daemonNativeTools) registry() toolspkg.Registry {
	if n == nil || n.deps.Registry == nil {
		return nil
	}
	return n.deps.Registry()
}

type nativeToolDiagnosticRegistry interface {
	DiagnosticSearch(ctx context.Context, scope toolspkg.Scope, q toolspkg.SearchQuery) ([]toolspkg.ToolView, error)
	DiagnosticGet(ctx context.Context, scope toolspkg.Scope, id toolspkg.ToolID) (toolspkg.ToolView, error)
}

var _ nativeToolDiagnosticRegistry = (*toolspkg.RuntimeRegistry)(nil)

func unavailableToolDiagnosticRegistryError(toolID toolspkg.ToolID) *toolspkg.ToolError {
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeUnavailable,
		toolID,
		"tool diagnostic registry is unavailable",
		toolspkg.ErrToolUnavailable,
		toolspkg.ReasonDependencyMissing,
	)
}

func (n *daemonNativeTools) mcpAuthProvider() toolspkg.MCPAuthStatusProvider {
	if n == nil || n.deps.MCPAuth == nil {
		return nil
	}
	return n.deps.MCPAuth()
}

func (n *daemonNativeTools) automationManager() core.AutomationManager {
	if n == nil || n.deps == nil {
		return nil
	}
	if n.deps.Automation != nil {
		return n.deps.Automation
	}
	if n.deps.AutomationRuntime == nil {
		return nil
	}
	return n.deps.AutomationRuntime()
}
