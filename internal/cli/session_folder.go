package cli

import (
	"errors"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

const sessionFolderFlagName = "folder"

func newSessionFolderCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "folder",
		Short: "Organize sessions into folders",
		Long: "Manage the session folders of the current workspace and profile. " +
			"Deleting a folder returns its sessions to the workspace root; no session is deleted.",
	}
	cmd.AddCommand(
		newSessionFolderListCommand(deps),
		newSessionFolderCreateCommand(deps),
		newSessionFolderRenameCommand(deps),
		newSessionFolderDeleteCommand(deps),
		newSessionFolderReorderCommand(deps),
	)
	return cmd
}

func newSessionFolderListCommand(deps commandDeps) *cobra.Command {
	return withSessionLayoutWorkspaceFlag(&cobra.Command{
		Use:   "list",
		Short: "List session folders in manual order",
		Example: `  # List folders of the current workspace
  compozy session folder list --workspace checkout-api`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspace, err := sessionLayoutClientFromDeps(cmd, deps)
			if err != nil {
				return err
			}
			response, err := client.ListSessionFolders(cmd.Context(), workspace)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionFoldersBundle(response.Folders))
		},
	})
}

func newSessionFolderCreateCommand(deps commandDeps) *cobra.Command {
	return withSessionLayoutWorkspaceFlag(&cobra.Command{
		Use:   "create <name>",
		Short: "Create a session folder",
		Example: `  # Create a folder at the top of the list
  compozy session folder create "Release"`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, workspace, err := sessionLayoutClientFromDeps(cmd, deps)
			if err != nil {
				return err
			}
			response, err := client.CreateSessionFolder(cmd.Context(), workspace, args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionFolderBundle(response.Folder))
		},
	})
}

func newSessionFolderRenameCommand(deps commandDeps) *cobra.Command {
	return withSessionLayoutWorkspaceFlag(&cobra.Command{
		Use:   "rename <folder-id> <name>",
		Short: "Rename a session folder",
		Example: `  # Rename a folder
  compozy session folder rename sfold-1234 "Shipped"`,
		Args: exactTwoNonBlankArgs(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, workspace, err := sessionLayoutClientFromDeps(cmd, deps)
			if err != nil {
				return err
			}
			response, err := client.RenameSessionFolder(cmd.Context(), workspace, args[0], args[1])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionFolderBundle(response.Folder))
		},
	})
}

func newSessionFolderDeleteCommand(deps commandDeps) *cobra.Command {
	return withSessionLayoutWorkspaceFlag(&cobra.Command{
		Use:   "delete <folder-id>",
		Short: "Delete a session folder; its sessions return to the workspace root",
		Example: `  # Delete a folder without deleting its sessions
  compozy session folder delete sfold-1234`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, workspace, err := sessionLayoutClientFromDeps(cmd, deps)
			if err != nil {
				return err
			}
			response, err := client.DeleteSessionFolder(cmd.Context(), workspace, args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionFoldersBundle(response.Folders))
		},
	})
}

func newSessionFolderReorderCommand(deps commandDeps) *cobra.Command {
	return withSessionLayoutWorkspaceFlag(&cobra.Command{
		Use:   "reorder <folder-id>...",
		Short: "Set the complete folder order",
		Example: `  # List every folder once, top first
  compozy session folder reorder sfold-b sfold-a`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, workspace, err := sessionLayoutClientFromDeps(cmd, deps)
			if err != nil {
				return err
			}
			response, err := client.ReorderSessionFolders(cmd.Context(), workspace, args)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionFoldersBundle(response.Folders))
		},
	})
}

func newSessionMoveCommand(deps commandDeps) *cobra.Command {
	var folderID string
	cmd := &cobra.Command{
		Use:   "move <id>",
		Short: "Move a session into a folder, or back to the workspace root",
		Example: `  # Move a session into a folder
  compozy session move sess_1234 --folder sfold-1234

  # Return a session to the workspace root
  compozy session move sess_1234`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := sessionLayoutClient(deps)
			if err != nil {
				return err
			}
			response, err := client.MoveSession(cmd.Context(), args[0], strings.TrimSpace(folderID))
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionFoldersBundle(response.Folders))
		},
	}
	cmd.Flags().StringVar(&folderID, sessionFolderFlagName, "", "Target folder ID (omit for the workspace root)")
	return cmd
}

func newSessionReorderCommand(deps commandDeps) *cobra.Command {
	var folderID string
	cmd := &cobra.Command{
		Use:   "reorder <id>...",
		Short: "Pin a manual session order in a folder or the workspace root",
		Long: "Pin the listed sessions, in order, at the top of one folder (or the workspace root). " +
			"Sessions already there but not listed keep their order after them. " +
			"List sessions with --sort manual to see this order.",
		Example: `  # Order two sessions at the top of the workspace root
  compozy session reorder sess_b sess_a

  # Order sessions inside a folder
  compozy session reorder sess_c sess_a --folder sfold-1234`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, workspace, err := sessionLayoutClientFromDeps(cmd, deps)
			if err != nil {
				return err
			}
			response, err := client.ReorderSessions(cmd.Context(), workspace, contract.ReorderSessionsRequest{
				FolderID: strings.TrimSpace(folderID), SessionIDs: args,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionFoldersBundle(response.Folders))
		},
	}
	cmd.Flags().StringVar(&folderID, sessionFolderFlagName, "", "Folder ID (omit for the workspace root)")
	return withSessionLayoutWorkspaceFlag(cmd)
}

func withSessionLayoutWorkspaceFlag(cmd *cobra.Command) *cobra.Command {
	cmd.Flags().String(workspaceFlagName, "", "Workspace ID, name, or path (defaults to the current directory)")
	return cmd
}

func sessionLayoutClient(deps commandDeps) (SessionLayoutClient, error) {
	client, err := clientFromDeps(deps)
	if err != nil {
		return nil, err
	}
	layoutClient, ok := client.(SessionLayoutClient)
	if !ok {
		return nil, errors.New("cli: session layout client is unavailable")
	}
	return layoutClient, nil
}

func sessionLayoutClientFromDeps(cmd *cobra.Command, deps commandDeps) (SessionLayoutClient, string, error) {
	client, err := clientFromDeps(deps)
	if err != nil {
		return nil, "", err
	}
	layoutClient, ok := client.(SessionLayoutClient)
	if !ok {
		return nil, "", errors.New("cli: session layout client is unavailable")
	}
	workspace, err := resolveWorkspaceFlagOverride(cmd, deps, client, true)
	if err != nil {
		return nil, "", err
	}
	if strings.TrimSpace(workspace) == "" {
		return nil, "", errors.New("cli: session folders need a workspace")
	}
	return layoutClient, workspace, nil
}

func sessionFolderBundle(folder contract.SessionFolderPayload) outputBundle {
	return outputBundle{
		jsonValue: folder,
		human: func() (string, error) {
			return renderHumanSection("Session Folder", []keyValue{
				{Label: "ID", Value: stringOrDash(folder.ID)},
				{Label: "Name", Value: stringOrDash(folder.Name)},
				{Label: "Position", Value: strconv.Itoa(folder.Position)},
				{Label: "Sessions", Value: strconv.Itoa(folder.SessionCount)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"session_folder",
				[]string{"id", "name", "position", "session_count"},
				[]string{folder.ID, folder.Name, strconv.Itoa(folder.Position), strconv.Itoa(folder.SessionCount)},
			), nil
		},
	}
}

func sessionFoldersBundle(folders []contract.SessionFolderPayload) outputBundle {
	if folders == nil {
		folders = []contract.SessionFolderPayload{}
	}
	row := func(folder contract.SessionFolderPayload) []string {
		return []string{folder.ID, folder.Name, strconv.Itoa(folder.Position), strconv.Itoa(folder.SessionCount)}
	}
	return listBundle(
		contract.SessionFoldersResponse{Folders: folders},
		folders,
		"Session Folders",
		[]string{"ID", "Name", "Position", "Sessions"},
		"session_folders",
		[]string{"id", "name", "position", "session_count"},
		row,
		row,
	)
}
