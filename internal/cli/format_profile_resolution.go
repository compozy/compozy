package cli

import "github.com/spf13/cobra"

type profileResolutionFrame struct {
	Kind      string `json:"kind"`
	Profile   string `json:"profile"`
	Source    string `json:"source"`
	Note      string `json:"note,omitempty"`
	Workspace string `json:"workspace,omitempty"`
}

type aggregateWorkspaceResolutionFrame struct {
	Kind      string `json:"kind"`
	Workspace string `json:"workspace"`
	Source    string `json:"source"`
}

func writeAggregateReadResolutionFrame(cmd *cobra.Command) (bool, error) {
	selection, ok := commandProfileReadSelection(cmd)
	if !ok || !selection.AllProfiles {
		return false, nil
	}
	if workspace, found := commandWorkspaceResolution(cmd); found {
		name := workspace.Detail.Workspace.Name
		if name == "" {
			name = workspace.ID
		}
		return true, writeJSONLineWithoutWorkspaceResolution(cmd, aggregateWorkspaceResolutionFrame{
			Kind: "workspace_resolution", Workspace: name, Source: workspace.Source,
		})
	}
	return true, writeJSONLineWithoutWorkspaceResolution(cmd, profileResolutionFrame{
		Kind: "profile_resolution", Profile: "all", Source: "aggregate",
	})
}

func writeProfileResolutionFrame(cmd *cobra.Command) (bool, error) {
	resolution, ok := commandProfileResolution(cmd)
	if !ok {
		return false, nil
	}
	workspace := resolution.WorkspaceName
	if workspace == "" {
		workspace = resolution.WorkspaceID
	}
	err := writeJSONLineWithoutWorkspaceResolution(cmd, profileResolutionFrame{
		Kind: "profile_resolution", Profile: resolution.Profile.Name, Source: resolution.Source,
		Note: resolution.Note, Workspace: workspace,
	})
	return true, err
}

func writeProfileJSONLines[T any](cmd *cobra.Command, items []T) error {
	if _, err := writeProfileResolutionFrame(cmd); err != nil {
		return err
	}
	for _, item := range items {
		if err := writeJSONLineWithoutWorkspaceResolution(cmd, item); err != nil {
			return err
		}
	}
	return nil
}
