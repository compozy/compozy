package core

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

func (h *BaseHandlers) terminalCapabilities(
	ctx context.Context,
	workspaceID string,
) (terminalpkg.Capabilities, error) {
	workspaceKind := terminalpkg.WorkspaceKindLocal
	if h != nil && h.Workspaces != nil {
		workspace, err := h.Workspaces.Get(ctx, workspaceID)
		if err != nil {
			return terminalpkg.Capabilities{}, fmt.Errorf("terminal: resolve workspace capabilities: %w", err)
		}
		if strings.TrimSpace(workspace.SandboxRef) != "" {
			workspaceKind = "sandbox"
		}
	}
	return terminalpkg.ResolveCapabilities(runtime.GOOS, workspaceKind), nil
}
