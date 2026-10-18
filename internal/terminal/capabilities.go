package terminal

import (
	"context"
	"fmt"
)

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
	_, err := m.workspaces.Resolve(ctx, workspaceID)
	if err != nil {
		return Capabilities{}, fmt.Errorf("terminal: resolve workspace capabilities: %w", err)
	}
	return Capabilities{Interactive: true}, nil
}

// RecordingAvailable derives recording support from interactive availability.
func RecordingAvailable(capabilities Capabilities) bool { return capabilities.Interactive }
