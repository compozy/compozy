package windowmanager

import (
	"context"
	"fmt"
	"strings"
)

func (m *Manager) resolveLayoutResourceCommand(
	ctx context.Context,
	workspaceID WorkspaceID,
	payload Command,
	defaults Config,
) (Command, error) {
	command, ok := payload.(ArrangeLayoutCommand)
	if !ok || strings.TrimSpace(command.ResourceID) == "" {
		return payload, nil
	}
	if command.DesktopID != "" || len(command.WindowIDs) != 0 || command.Arrangement != "" ||
		command.Frame != (NormalizedRect{}) || command.GroupID != "" {
		return nil, fmt.Errorf(
			"layout resource cannot be combined with inline arrangement fields: %w",
			ErrInvalidCommand,
		)
	}
	document, err := m.layouts.Resolve(ctx, workspaceID, strings.TrimSpace(command.ResourceID))
	if err != nil {
		return nil, fmt.Errorf("resolve layout resource %q: %w", command.ResourceID, err)
	}
	validation, err := validateLayoutDocument(defaults, workspaceID, document)
	if err != nil {
		return nil, fmt.Errorf("validate layout resource %q: %w", command.ResourceID, err)
	}
	if !validation.Valid {
		return nil, &TopologyError{Diagnostics: append([]Diagnostic(nil), validation.Diagnostics...)}
	}
	return ReplaceLayoutCommand{Document: cloneLayoutDocument(document)}, nil
}
