package terminal

import (
	"context"
	"fmt"
	"runtime"
	"strings"
)

const WorkspaceKindLocal = "local"

// ResolveCapabilities reports what one platform/workspace pairing can run.
func ResolveCapabilities(_ string, workspaceKind string) Capabilities {
	return Capabilities{Interactive: workspaceKind == "" || workspaceKind == WorkspaceKindLocal}
}

// Capabilities resolves the authoritative runtime capability for one workspace.
func (m *Service) Capabilities(ctx context.Context, workspaceID string) (Capabilities, error) {
	if err := requestContextError(ctx, "resolve capabilities"); err != nil {
		return Capabilities{}, err
	}
	if m == nil || m.workspaces == nil {
		return Capabilities{}, fmt.Errorf(
			"terminal workspace capabilities are unavailable: %w",
			ErrServiceUnavailable,
		)
	}
	workspace, err := m.workspaces.Resolve(ctx, workspaceID)
	if err != nil {
		return Capabilities{}, fmt.Errorf("terminal: resolve workspace capabilities: %w", err)
	}
	kind := WorkspaceKindLocal
	if strings.TrimSpace(workspace.SandboxRef) != "" {
		kind = "sandbox"
	}
	return ResolveCapabilities(runtime.GOOS, kind), nil
}

// RecordingAvailable derives recording support from interactive availability.
func RecordingAvailable(capabilities Capabilities) bool { return capabilities.Interactive }
