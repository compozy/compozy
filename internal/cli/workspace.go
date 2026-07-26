package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/spf13/cobra"
)

const (
	workspaceModelKey = "model"
)

const (
	workspaceAgentValue    = "Agent"
	workspaceCategoryValue = "Category"
	workspaceCreatedValue  = "Created"
	workspaceNameValue     = "Name"
	workspaceRootValue     = "Root"
	workspaceSandboxValue  = "Sandbox"
	workspaceSourceValue   = "Source"
	workspaceUpdatedValue  = "Updated"
	workspaceAgentNameKey  = "agent_name"
	workspaceCategoryKey   = "category"
	workspaceCreatedAtKey  = "created_at"
	workspaceFlagKey       = "flag"
	workspaceListKey       = "list"
	workspaceSourceKey     = "source"
)

func newWorkspaceCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   workspaceSkillSource,
		Short: "Manage registered workspaces",
	}

	cmd.AddCommand(newWorkspaceAddCommand(deps))
	cmd.AddCommand(newWorkspaceListCommand(deps))
	cmd.AddCommand(newWorkspaceInfoCommand(deps))
	cmd.AddCommand(newWorkspaceEditCommand(deps))
	cmd.AddCommand(newWorkspaceRemoveCommand(deps))
	return cmd
}

func newWorkspaceAddCommand(deps commandDeps) *cobra.Command {
	var (
		name         string
		addDirs      []string
		defaultAgent string
		sandboxRef   string
	)

	cmd := &cobra.Command{
		Use:   "add <path>",
		Short: "Register a workspace",
		Example: `  # Register a workspace with a stable name
  agh workspace add "$PWD" --name checkout-api

  # Include an additional directory and set a workspace default agent
  agh workspace add "$PWD" --name platform --add-dir "$PWD/docs" --default-agent architect`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			workspace, err := client.CreateWorkspace(cmd.Context(), WorkspaceCreateRequest{
				RootDir:      strings.TrimSpace(args[0]),
				Name:         strings.TrimSpace(name),
				AddDirs:      trimmedUniqueStrings(addDirs),
				DefaultAgent: strings.TrimSpace(defaultAgent),
				SandboxRef:   strings.TrimSpace(sandboxRef),
			})
			if err != nil {
				return err
			}

			return writeCommandOutput(cmd, workspaceRecordBundle(workspace))
		},
	}
	cmd.Flags().StringVar(&name, automationNameKey, "", "Optional workspace name")
	cmd.Flags().
		StringArrayVar(&addDirs, "add-dir", nil, "Additional directory to include (repeatable)")
	cmd.Flags().
		StringVar(&defaultAgent, "default-agent", "", "Default agent override for this workspace")
	cmd.Flags().
		StringVar(&sandboxRef, "sandbox", "", "Sandbox profile override for this workspace")
	return cmd
}

func newWorkspaceListCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   workspaceListKey,
		Short: "List registered workspaces",
		Example: `  # Show every registered workspace
  agh workspace list

  # Return workspace records as JSON
  agh workspace list --output json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			workspaces, err := client.ListWorkspaces(cmd.Context())
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, workspaceListBundle(workspaces))
		},
	}
}

func newWorkspaceInfoCommand(deps commandDeps) *cobra.Command {
	var workspaceRef string
	cmd := &cobra.Command{
		Use:   "info [name-or-id]",
		Short: "Show one workspace with resolved details",
		Example: `  # Show workspace paths, agents, and skills by name
  agh workspace info checkout-api

  # Resolve the current directory as a workspace
  agh workspace info

  # Emit resolved workspace details as JSON
  agh workspace info ws_1234 -o json`,
		Args: cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			resolved, err := resolveWorkspaceInfoRef(deps, args, workspaceRef)
			if err != nil {
				return err
			}
			detail, err := client.GetWorkspace(cmd.Context(), resolved.Ref)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, workspaceDetailBundle(detail, resolved.Source))
		},
	}
	cmd.Flags().
		StringVar(
			&workspaceRef,
			workspaceSkillSource,
			"",
			"Workspace root/name/id to inspect when no positional ref is supplied",
		)
	return cmd
}

type workspaceInfoRef struct {
	Ref    string
	Source string
}

func resolveWorkspaceInfoRef(deps commandDeps, args []string, workspaceFlag string) (workspaceInfoRef, error) {
	if len(args) > 0 {
		ref := strings.TrimSpace(args[0])
		if ref == "" {
			return workspaceInfoRef{}, errors.New("cli: workspace reference is required")
		}
		return workspaceInfoRef{Ref: ref, Source: "positional"}, nil
	}
	if trimmed := strings.TrimSpace(workspaceFlag); trimmed != "" {
		if isPathLikeWorkspaceRef(trimmed) {
			resolved, err := aghconfig.ResolvePath(trimmed)
			if err != nil {
				return workspaceInfoRef{}, err
			}
			return workspaceInfoRef{Ref: resolved, Source: workspaceFlagKey}, nil
		}
		return workspaceInfoRef{Ref: trimmed, Source: workspaceFlagKey}, nil
	}
	if trimmed := strings.TrimSpace(deps.getenv("AGH_WORKSPACE")); trimmed != "" {
		return workspaceInfoRef{Ref: trimmed, Source: configEnvKey}, nil
	}
	cwd, err := currentWorkingDirectory(deps)
	if err != nil {
		return workspaceInfoRef{}, err
	}
	return workspaceInfoRef{Ref: cwd, Source: "cwd"}, nil
}

func resolveCLIWorkspaceRouteRef(
	ctx context.Context,
	deps commandDeps,
	client DaemonClient,
	workspaceRef string,
) (string, error) {
	trimmed := strings.TrimSpace(workspaceRef)
	if trimmed == "" {
		trimmed = strings.TrimSpace(deps.getenv("AGH_WORKSPACE"))
	}
	if trimmed == "" {
		cwd, err := currentWorkingDirectory(deps)
		if err != nil {
			return "", err
		}
		trimmed = cwd
	}
	if !workspaceRefLooksLikePath(trimmed) {
		return trimmed, nil
	}
	detail, err := client.GetWorkspace(ctx, trimmed)
	if err != nil {
		return "", fmt.Errorf("cli: resolve workspace %q: %w", trimmed, err)
	}
	workspaceID := strings.TrimSpace(detail.Workspace.ID)
	if workspaceID == "" {
		return "", fmt.Errorf("cli: resolve workspace %q: missing workspace id", trimmed)
	}
	return workspaceID, nil
}

func isPathLikeWorkspaceRef(ref string) bool {
	return filepath.IsAbs(ref) ||
		strings.HasPrefix(ref, ".") ||
		strings.Contains(ref, string(filepath.Separator))
}

func workspaceEditFlagsChanged(cmd *cobra.Command) bool {
	return cmd.Flags().Changed(automationNameKey) ||
		cmd.Flags().Changed("add-dir") ||
		cmd.Flags().Changed("remove-dir") ||
		cmd.Flags().Changed("default-agent") ||
		cmd.Flags().Changed("sandbox")
}

func newWorkspaceEditCommand(deps commandDeps) *cobra.Command {
	var (
		name         string
		addDirs      []string
		removeDirs   []string
		defaultAgent string
		sandboxRef   string
	)

	cmd := &cobra.Command{
		Use:   "edit <name-or-id>",
		Short: "Edit a registered workspace",
		Example: `  # Rename a workspace
  agh workspace edit checkout-api --name checkout

  # Clear the workspace default agent
  agh workspace edit checkout-api --default-agent ""`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			if !workspaceEditFlagsChanged(cmd) {
				return errors.New("cli: at least one edit flag is required")
			}

			detail, err := client.GetWorkspace(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			request := WorkspaceUpdateRequest{}
			if cmd.Flags().Changed(automationNameKey) {
				trimmedName := strings.TrimSpace(name)
				if trimmedName == "" {
					return errors.New("cli: --name cannot be empty")
				}
				request.Name = &trimmedName
			}
			if cmd.Flags().Changed("add-dir") || cmd.Flags().Changed("remove-dir") {
				mergedDirs, err := mergeWorkspaceAddDirs(
					detail.Workspace.AddDirs,
					addDirs,
					removeDirs,
				)
				if err != nil {
					return err
				}
				request.AddDirs = &mergedDirs
			}
			if cmd.Flags().Changed("default-agent") {
				trimmedDefaultAgent := strings.TrimSpace(defaultAgent)
				request.DefaultAgent = &trimmedDefaultAgent
			}
			if cmd.Flags().Changed("sandbox") {
				trimmedSandbox := strings.TrimSpace(sandboxRef)
				request.SandboxRef = &trimmedSandbox
			}

			updated, err := client.UpdateWorkspace(cmd.Context(), detail.Workspace.ID, request)
			if err != nil {
				return err
			}

			return writeCommandOutput(cmd, workspaceRecordBundle(updated))
		},
	}
	cmd.Flags().StringVar(&name, automationNameKey, "", "Rename the workspace")
	cmd.Flags().
		StringArrayVar(&addDirs, "add-dir", nil, "Additional directory to include (repeatable)")
	cmd.Flags().
		StringArrayVar(&removeDirs, "remove-dir", nil, "Additional directory to remove (repeatable)")
	cmd.Flags().
		StringVar(&defaultAgent, "default-agent", "", "Override the workspace default agent (set empty to clear)")
	cmd.Flags().
		StringVar(&sandboxRef, "sandbox", "", "Override the workspace sandbox profile (set empty to clear)")
	return cmd
}

func newWorkspaceRemoveCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name-or-id>",
		Short: "Remove a workspace registration and stopped session history",
		Example: `  # Remove a workspace registration by name
  agh workspace remove checkout-api`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			detail, err := client.GetWorkspace(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if err := client.DeleteWorkspace(cmd.Context(), detail.Workspace.ID); err != nil {
				return err
			}

			return writeCommandOutput(cmd, workspaceRecordBundle(detail.Workspace))
		},
	}
}
