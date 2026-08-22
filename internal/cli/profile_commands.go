package cli

import (
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

func newProfileCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{Use: "profile", Short: "Manage profiles"}
	cmd.AddCommand(
		newProfileListCommand(deps), newProfileCurrentCommand(deps), newProfileUseCommand(deps),
		newProfileCreateCommand(deps), newProfileUpdateCommand(deps), newProfileRenameCommand(deps),
		newProfileArchiveCommand(deps), newProfileUnarchiveCommand(deps), newProfileDeleteCommand(deps),
		newProfileOpsCommand(deps),
	)
	return cmd
}

func newProfileListCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: "List profiles",
		RunE: func(cmd *cobra.Command, _ []string) error {
			profiles, client, err := profileResolutionClientFromDeps(deps)
			if err != nil {
				return err
			}
			resolution, err := resolveCommandProfile(cmd.Context(), cmd, deps, profiles, client)
			if err != nil {
				return err
			}
			items, err := profiles.ListProfiles(cmd.Context())
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, profileListBundle(items, resolution.Profile.Name))
		},
	}
}

func newProfileCurrentCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use: "current", Short: "Show the active profile",
		RunE: func(cmd *cobra.Command, _ []string) error {
			profiles, client, err := profileResolutionClientFromDeps(deps)
			if err != nil {
				return err
			}
			resolution, err := resolveCommandProfile(cmd.Context(), cmd, deps, profiles, client)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, profileCurrentBundle(resolution))
		},
	}
}

func newProfileUseCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use: "use <name>", Short: "Remember the active profile", Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			profiles, client, err := profileClientFromDeps(deps)
			if err != nil {
				return err
			}
			workspace, hasWorkspace, err := resolveProfileWorkspaceLens(cmd.Context(), cmd, deps, client)
			if err != nil {
				return err
			}
			selection, err := profiles.PutProfileSelection(
				cmd.Context(), profileSelectionLens(workspace, hasWorkspace, strings.TrimSpace(args[0])),
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, profileUseBundle(selection, workspace, hasWorkspace))
		},
	}
}

func newProfileCreateCommand(deps commandDeps) *cobra.Command {
	var color, icon, emoji string
	cmd := &cobra.Command{
		Use: "create <name>", Short: "Create and activate a profile", Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			profiles, client, err := profileClientFromDeps(deps)
			if err != nil {
				return err
			}
			workspace, hasWorkspace, err := resolveProfileWorkspaceLens(cmd.Context(), cmd, deps, client)
			if err != nil {
				return err
			}
			name := strings.TrimSpace(args[0])
			selection := profileSelectionLens(workspace, hasWorkspace, name)
			created, err := profiles.CreateProfile(cmd.Context(), contract.CreateProfileRequest{
				Name: name, Color: strings.TrimSpace(color), Icon: strings.TrimSpace(icon),
				Emoji: strings.TrimSpace(emoji), Activate: &selection,
			})
			if err != nil {
				return err
			}
			recordProfileResolution(cmd, profileResolution{
				Profile: created, Source: profileResolutionRemembered,
				WorkspaceID: workspace.ID, WorkspaceName: workspace.Detail.Workspace.Name,
			})
			return writeCommandOutput(cmd, profileMutationBundle(created, "Created profile "+created.Name+" — now active."))
		},
	}
	cmd.Flags().StringVar(&color, "color", "", "Profile color as #rrggbb")
	cmd.Flags().StringVar(&icon, "icon", "", "Profile icon name")
	cmd.Flags().StringVar(&emoji, "emoji", "", "Profile emoji")
	cmd.MarkFlagsMutuallyExclusive("icon", "emoji")
	return cmd
}

func newProfileUpdateCommand(deps commandDeps) *cobra.Command {
	var color, icon, emoji string
	cmd := &cobra.Command{
		Use: "update <name>", Short: "Update profile identity", Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("color") && !cmd.Flags().Changed("icon") && !cmd.Flags().Changed("emoji") {
				return errors.New("cli: profile update requires --color, --icon, or --emoji")
			}
			profiles, _, err := profileClientFromDeps(deps)
			if err != nil {
				return err
			}
			request := contract.UpdateProfileRequest{}
			if cmd.Flags().Changed("color") {
				request.Color = stringPointer(strings.TrimSpace(color))
			}
			if cmd.Flags().Changed("icon") {
				request.Icon = stringPointer(strings.TrimSpace(icon))
			}
			if cmd.Flags().Changed("emoji") {
				request.Emoji = stringPointer(strings.TrimSpace(emoji))
			}
			updated, err := profiles.UpdateProfile(cmd.Context(), args[0], request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, profileMutationBundle(updated, "Updated "+updated.Name+"."))
		},
	}
	cmd.Flags().StringVar(&color, "color", "", "Profile color as #rrggbb")
	cmd.Flags().StringVar(&icon, "icon", "", "Profile icon name")
	cmd.Flags().StringVar(&emoji, "emoji", "", "Profile emoji")
	cmd.MarkFlagsMutuallyExclusive("icon", "emoji")
	return cmd
}

func newProfileOpsCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use: "ops", Short: "List profile lifecycle operations",
		RunE: func(cmd *cobra.Command, _ []string) error {
			profiles, _, err := profileClientFromDeps(deps)
			if err != nil {
				return err
			}
			operations, err := profiles.ListProfileOperations(cmd.Context())
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, profileOperationsBundle(operations))
		},
	}
	cmd.AddCommand(&cobra.Command{
		Use: "retry <op-id>", Short: "Retry a failed profile lifecycle operation", Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			profiles, _, err := profileClientFromDeps(deps)
			if err != nil {
				return err
			}
			operation, err := profiles.RetryProfileOperation(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, profileOperationBundle(operation))
		},
	})
	return cmd
}

func stringPointer(value string) *string { return &value }
