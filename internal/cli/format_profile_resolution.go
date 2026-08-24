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

func writeAggregateReadResolutionFrame(cmd *cobra.Command) error {
	selection, ok := commandProfileReadSelection(cmd)
	if !ok || !selection.AllProfiles {
		return nil
	}
	if workspace, found := commandWorkspaceResolution(cmd); found {
		name := workspace.Detail.Workspace.Name
		if name == "" {
			name = workspace.ID
		}
		return writeJSONLineWithoutWorkspaceResolution(cmd, aggregateWorkspaceResolutionFrame{
			Kind: "workspace_resolution", Workspace: name, Source: workspace.Source,
		})
	}
	return writeJSONLineWithoutWorkspaceResolution(cmd, profileResolutionFrame{
		Kind: "profile_resolution", Profile: providerModelViewAll, Source: "aggregate",
	})
}

func writeProfileResolutionFrame(cmd *cobra.Command) error {
	resolution, ok := commandProfileResolution(cmd)
	if !ok {
		return nil
	}
	workspace := resolution.WorkspaceName
	if workspace == "" {
		workspace = resolution.WorkspaceID
	}
	err := writeJSONLineWithoutWorkspaceResolution(cmd, profileResolutionFrame{
		Kind: "profile_resolution", Profile: resolution.Profile.Name, Source: resolution.Source,
		Note: resolution.Note, Workspace: workspace,
	})
	return err
}

func writeProfileJSONLines[T any](cmd *cobra.Command, items []T) error {
	if err := writeProfileResolutionFrame(cmd); err != nil {
		return err
	}
	for _, item := range items {
		if err := writeJSONLineWithoutWorkspaceResolution(cmd, item); err != nil {
			return err
		}
	}
	return nil
}
