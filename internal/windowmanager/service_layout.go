package windowmanager

import (
	"context"
	"errors"
	"fmt"
)

// ExportLayout returns a history-free declarative document.
func (m *Manager) ExportLayout(ctx context.Context, workspaceID WorkspaceID) (LayoutDocument, error) {
	snapshot, err := m.Snapshot(ctx, workspaceID)
	if err != nil {
		return LayoutDocument{}, err
	}
	return LayoutDocument{
		Version:     SnapshotVersion,
		WorkspaceID: workspaceID,
		Desktops:    cloneDesktops(snapshot.Desktops),
		Windows:     cloneWindows(snapshot.Windows),
		Overrides:   cloneWorkspaceConfig(snapshot.Overrides),
	}, nil
}

// ValidateLayout checks a raw document without normalization or writes.
func (m *Manager) ValidateLayout(
	ctx context.Context,
	workspaceID WorkspaceID,
	document LayoutDocument,
) (Validation, error) {
	if err := m.resolveWorkspace(ctx, workspaceID); err != nil {
		return Validation{}, err
	}
	defaults, err := m.resolveWorkspaceDefaults(ctx, workspaceID)
	if err != nil {
		return Validation{}, err
	}
	return validateLayoutDocument(defaults, workspaceID, document)
}

func validateLayoutDocument(
	defaults Config,
	workspaceID WorkspaceID,
	document LayoutDocument,
) (Validation, error) {
	if document.WorkspaceID != workspaceID {
		return Validation{
			Diagnostics: []Diagnostic{
				{
					Code:    "topology.workspace_mismatch",
					Path:    "workspace_id",
					Message: "layout workspace does not match request",
				},
			},
		}, nil
	}
	candidate := Snapshot{
		Version:     document.Version,
		WorkspaceID: document.WorkspaceID,
		Desktops:    cloneDesktops(document.Desktops),
		Windows:     cloneWindows(document.Windows),
		Overrides:   cloneWorkspaceConfig(document.Overrides),
	}
	if _, err := effectiveConfig(defaults, candidate.Overrides); err != nil {
		return Validation{
			Diagnostics: []Diagnostic{{Code: "config.invalid", Path: "overrides", Message: err.Error()}},
		}, nil
	}
	if err := ValidateSnapshot(candidate); err != nil {
		var topologyErr *TopologyError
		if errors.As(err, &topologyErr) {
			return Validation{Diagnostics: append([]Diagnostic(nil), topologyErr.Diagnostics...)}, nil
		}
		return Validation{}, fmt.Errorf("validate layout document: %w", err)
	}
	return Validation{Valid: true}, nil
}

// ReplaceLayout applies a raw layout through the normal semantic command pipeline.
func (m *Manager) ReplaceLayout(ctx context.Context, request ReplaceLayoutRequest) (Result, error) {
	return m.Execute(
		ctx,
		CommandRequest{
			WorkspaceID:      request.WorkspaceID,
			CommandID:        CommandLayoutReplace,
			ExpectedRevision: request.ExpectedRevision,
			ClientID:         clonePointer(request.ClientID),
			Actor:            request.Actor,
			Origin:           request.Origin,
			Payload:          ReplaceLayoutCommand{Document: cloneLayoutDocument(request.Document)},
		},
	)
}

// LayoutResources lists declarative resources visible to a workspace.
func (m *Manager) LayoutResources(ctx context.Context, workspaceID WorkspaceID) ([]LayoutResource, error) {
	if err := m.resolveWorkspace(ctx, workspaceID); err != nil {
		return nil, err
	}
	resources, err := m.layouts.List(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list layout resources: %w", err)
	}
	cloned := make([]LayoutResource, len(resources))
	for index, resource := range resources {
		cloned[index] = CloneLayoutResource(resource)
	}
	return cloned, nil
}
