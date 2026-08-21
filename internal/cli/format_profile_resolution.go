package cli

import "github.com/spf13/cobra"

type profileResolutionFrame struct {
	Kind      string `json:"kind"`
	Profile   string `json:"profile"`
	Source    string `json:"source"`
	Note      string `json:"note,omitempty"`
	Workspace string `json:"workspace,omitempty"`
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
