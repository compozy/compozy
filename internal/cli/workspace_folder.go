package cli

import (
	"errors"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

const (
	workspaceFolderWorkspacesLabel = "Workspaces"
	workspaceRootContainerLabel    = "(root)"
	workspaceLayoutPositionKey     = "position"
)

func newWorkspaceFolderCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   sessionFolderFlagName,
		Short: "Group workspaces into root folders",
		Long: "Manage the profile's root workspace folders shown in All projects. " +
			"Folders are one level deep and hold workspaces only. " +
			"Deleting a folder returns its workspaces to the root; no workspace or session is touched.",
	}
	cmd.AddCommand(
		newWorkspaceFolderListCommand(deps),
		newWorkspaceFolderCreateCommand(deps),
		newWorkspaceFolderRenameCommand(deps),
		newWorkspaceFolderDeleteCommand(deps),
		newWorkspaceFolderReorderCommand(deps),
	)
	return cmd
}

func newWorkspaceFolderListCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   agentListKey,
		Short: "List workspace folders and placements in manual order",
		Example: `  # Show the profile's workspace folders and manual order
  compozy workspace folder list`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := workspaceLayoutClient(deps)
			if err != nil {
				return err
			}
			response, err := client.WorkspaceLayout(cmd.Context())
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, workspaceLayoutBundle(response))
		},
	}
}

func newWorkspaceFolderCreateCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "create <name>",
		Short: "Create a root workspace folder",
		Example: `  # Create a folder at the top of All projects
  compozy workspace folder create "Clients"`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := workspaceLayoutClient(deps)
			if err != nil {
				return err
			}
			response, err := client.CreateWorkspaceFolder(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, workspaceFolderBundle(response.Folder))
		},
	}
}

func newWorkspaceFolderRenameCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "rename <folder-id> <name>",
		Short: "Rename a workspace folder",
		Example: `  # Rename a folder
  compozy workspace folder rename wfold-1234 "Archive"`,
		Args: exactTwoNonBlankArgs(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := workspaceLayoutClient(deps)
			if err != nil {
				return err
			}
			response, err := client.RenameWorkspaceFolder(cmd.Context(), args[0], args[1])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, workspaceFolderBundle(response.Folder))
		},
	}
}

func newWorkspaceFolderDeleteCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <folder-id>",
		Short: "Delete a workspace folder; its workspaces return to the root",
		Example: `  # Delete a folder without touching its workspaces
  compozy workspace folder delete wfold-1234`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := workspaceLayoutClient(deps)
			if err != nil {
				return err
			}
			response, err := client.DeleteWorkspaceFolder(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, workspaceLayoutBundle(response))
		},
	}
}

func newWorkspaceFolderReorderCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "reorder <folder-id>...",
		Short: "Set the complete workspace folder order",
		Example: `  # List every folder once, top first
  compozy workspace folder reorder wfold-b wfold-a`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := workspaceLayoutClient(deps)
			if err != nil {
				return err
			}
			response, err := client.ReorderWorkspaceFolders(cmd.Context(), args)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, workspaceLayoutBundle(response))
		},
	}
}

func newWorkspaceMoveCommand(deps commandDeps) *cobra.Command {
	var folderID string
	cmd := &cobra.Command{
		Use:   "move <name-or-id>",
		Short: "Move a workspace into a root folder, or back to the root",
		Long: "Move a workspace (or @user for the User scope) to the end of a folder, or to the end of the root. " +
			"A workspace already there keeps its position; use reorder to change it.",
		Example: `  # Move a workspace into a folder
  compozy workspace move checkout-api --folder wfold-1234

  # Return a workspace to the root
  compozy workspace move checkout-api`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := workspaceLayoutClient(deps)
			if err != nil {
				return err
			}
			response, err := client.MoveWorkspace(cmd.Context(), contract.MoveWorkspaceRequest{
				WorkspaceID: args[0], FolderID: strings.TrimSpace(folderID),
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, workspaceLayoutBundle(response))
		},
	}
	cmd.Flags().StringVar(&folderID, sessionFolderFlagName, "", "Target folder ID (omit for the root)")
	return cmd
}

func newWorkspaceReorderCommand(deps commandDeps) *cobra.Command {
	var folderID string
	cmd := &cobra.Command{
		Use:   "reorder <name-or-id>...",
		Short: "Pin a manual workspace order in a root folder or the root",
		Long: "Pin the listed workspaces (or @user), in order, at the top of one folder or the root. " +
			"Workspaces already there but not listed keep their order after them.",
		Example: `  # Order two workspaces at the top of the root
  compozy workspace reorder web-app checkout-api

  # Order workspaces inside a folder
  compozy workspace reorder checkout-api billing --folder wfold-1234`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := workspaceLayoutClient(deps)
			if err != nil {
				return err
			}
			response, err := client.ReorderWorkspaces(cmd.Context(), contract.ReorderWorkspacesRequest{
				FolderID: strings.TrimSpace(folderID), WorkspaceIDs: args,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, workspaceLayoutBundle(response))
		},
	}
	cmd.Flags().StringVar(&folderID, sessionFolderFlagName, "", "Folder ID (omit for the root)")
	return cmd
}

func workspaceLayoutClient(deps commandDeps) (WorkspaceLayoutClient, error) {
	client, err := clientFromDeps(deps)
	if err != nil {
		return nil, err
	}
	layoutClient, ok := client.(WorkspaceLayoutClient)
	if !ok {
		return nil, errors.New("cli: workspace layout client is unavailable")
	}
	return layoutClient, nil
}

func workspaceFolderBundle(folder contract.WorkspaceFolderPayload) outputBundle {
	return outputBundle{
		jsonValue: folder,
		human: func() (string, error) {
			return renderHumanSection("Workspace Folder", []keyValue{
				{Label: "ID", Value: stringOrDash(folder.ID)},
				{Label: automationNameValue, Value: stringOrDash(folder.Name)},
				{Label: sessionFolderPositionLabel, Value: strconv.Itoa(folder.Position)},
				{Label: workspaceFolderWorkspacesLabel, Value: strconv.Itoa(folder.WorkspaceCount)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"workspace_folder",
				[]string{"id", automationNameKey, workspaceLayoutPositionKey, "workspace_count"},
				[]string{folder.ID, folder.Name, strconv.Itoa(folder.Position), strconv.Itoa(folder.WorkspaceCount)},
			), nil
		},
	}
}

// workspaceLayoutBundle renders folders first, then placements by container.
func workspaceLayoutBundle(layout contract.WorkspaceLayoutResponse) outputBundle {
	if layout.Folders == nil {
		layout.Folders = []contract.WorkspaceFolderPayload{}
	}
	if layout.Placements == nil {
		layout.Placements = []contract.WorkspacePlacementPayload{}
	}
	names := make(map[string]string, len(layout.Folders))
	for _, folder := range layout.Folders {
		names[folder.ID] = folder.Name
	}
	type row struct {
		kind, id, name, container, position string
	}
	rows := make([]row, 0, len(layout.Folders)+len(layout.Placements))
	for _, folder := range layout.Folders {
		rows = append(rows, row{sessionFolderFlagName, folder.ID, folder.Name, workspaceRootContainerLabel,
			strconv.Itoa(folder.Position)})
	}
	for _, placement := range layout.Placements {
		container := workspaceRootContainerLabel
		if placement.FolderID != "" {
			container = stringOrDash(names[placement.FolderID])
		}
		rows = append(rows, row{"workspace", placement.WorkspaceID, "", container, strconv.Itoa(placement.Position)})
	}
	render := func(r row) []string { return []string{r.kind, r.id, stringOrDash(r.name), r.container, r.position} }
	return listBundle(
		layout,
		rows,
		"Workspace Layout",
		[]string{"Kind", "ID", automationNameValue, "Container", sessionFolderPositionLabel},
		"workspace_layout",
		[]string{cliKindKey, "id", automationNameKey, "container", workspaceLayoutPositionKey},
		render,
		render,
	)
}
