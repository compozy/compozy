package cli

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/spf13/cobra"
)

func newWorkspacesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "workspaces",
		Short:        "Manage daemon workspace registrations",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newWorkspacesListCommand(),
		newWorkspacesShowCommand(),
		newWorkspacesRegisterCommand(),
		newWorkspacesUnregisterCommand(),
		newWorkspacesResolveCommand(),
	)
	return cmd
}

func newWorkspacesListCommand() *cobra.Command {
	var outputFormat string
	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List registered workspaces",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			format, err := normalizeOperatorOutputFormat(outputFormat)
			if err != nil {
				return withExitCode(1, err)
			}

			ctx, stop := signalCommandContext(cmd)
			defer stop()

			client, err := newCLIDaemonBootstrap().ensure(ctx)
			if err != nil {
				return withExitCode(2, err)
			}
			workspaces, err := client.ListWorkspaces(ctx)
			if err != nil {
				return mapDaemonCommandError(err)
			}
			return writeWorkspaceListOutput(cmd, format, workspaces)
		},
	}
	cmd.Flags().StringVar(&outputFormat, "format", operatorOutputFormatText, "Output format: text or json")
	return cmd
}

func newWorkspacesShowCommand() *cobra.Command {
	var outputFormat string
	cmd := &cobra.Command{
		Use:          "show <id-or-path>",
		Short:        "Show one registered workspace",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := normalizeOperatorOutputFormat(outputFormat)
			if err != nil {
				return withExitCode(1, err)
			}

			ctx, stop := signalCommandContext(cmd)
			defer stop()

			client, err := newCLIDaemonBootstrap().ensure(ctx)
			if err != nil {
				return withExitCode(2, err)
			}
			ref, err := resolveWorkspaceRouteRef(ctx, client, strings.TrimSpace(args[0]))
			if err != nil {
				return mapDaemonCommandError(err)
			}
			workspace, err := client.GetWorkspace(ctx, ref)
			if err != nil {
				return mapDaemonCommandError(err)
			}
			return writeWorkspaceOutput(cmd, format, workspace)
		},
	}
	cmd.Flags().StringVar(&outputFormat, "format", operatorOutputFormatText, "Output format: text or json")
	return cmd
}

func newWorkspacesRegisterCommand() *cobra.Command {
	var outputFormat string
	var name string
	cmd := &cobra.Command{
		Use:          "register <path>",
		Short:        "Register a workspace explicitly",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := normalizeOperatorOutputFormat(outputFormat)
			if err != nil {
				return withExitCode(1, err)
			}

			ctx, stop := signalCommandContext(cmd)
			defer stop()

			client, err := newCLIDaemonBootstrap().ensure(ctx)
			if err != nil {
				return withExitCode(2, err)
			}
			result, err := client.RegisterWorkspace(ctx, strings.TrimSpace(args[0]), name)
			if err != nil {
				return mapDaemonCommandError(err)
			}
			return writeWorkspaceMutationOutput(cmd, format, "registered", result.Created, result.Workspace)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Operator-facing display name for the workspace")
	cmd.Flags().StringVar(&outputFormat, "format", operatorOutputFormatText, "Output format: text or json")
	return cmd
}

func newWorkspacesUnregisterCommand() *cobra.Command {
	var outputFormat string
	cmd := &cobra.Command{
		Use:          "unregister <id-or-path>",
		Short:        "Unregister a workspace when no active runs exist",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := normalizeOperatorOutputFormat(outputFormat)
			if err != nil {
				return withExitCode(1, err)
			}

			ctx, stop := signalCommandContext(cmd)
			defer stop()

			client, err := newCLIDaemonBootstrap().ensure(ctx)
			if err != nil {
				return withExitCode(2, err)
			}
			ref, err := resolveWorkspaceRouteRef(ctx, client, strings.TrimSpace(args[0]))
			if err != nil {
				return mapDaemonCommandError(err)
			}
			if err := client.DeleteWorkspace(ctx, ref); err != nil {
				return mapDaemonCommandError(err)
			}
			return writeWorkspaceDeletionOutput(cmd, format, ref)
		},
	}
	cmd.Flags().StringVar(&outputFormat, "format", operatorOutputFormatText, "Output format: text or json")
	return cmd
}

func newWorkspacesResolveCommand() *cobra.Command {
	var outputFormat string
	cmd := &cobra.Command{
		Use:          "resolve <path>",
		Short:        "Resolve or lazily register a workspace path",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := normalizeOperatorOutputFormat(outputFormat)
			if err != nil {
				return withExitCode(1, err)
			}

			ctx, stop := signalCommandContext(cmd)
			defer stop()

			client, err := newCLIDaemonBootstrap().ensure(ctx)
			if err != nil {
				return withExitCode(2, err)
			}
			workspace, err := client.ResolveWorkspace(ctx, strings.TrimSpace(args[0]))
			if err != nil {
				return mapDaemonCommandError(err)
			}
			return writeWorkspaceMutationOutput(cmd, format, "resolved", false, workspace)
		},
	}
	cmd.Flags().StringVar(&outputFormat, "format", operatorOutputFormatText, "Output format: text or json")
	return cmd
}

func writeWorkspaceOutput(cmd *cobra.Command, format string, workspace apicore.Workspace) error {
	if format == operatorOutputFormatJSON {
		if err := writeOperatorJSON(cmd.OutOrStdout(), map[string]any{"workspace": workspace}); err != nil {
			return withExitCode(2, fmt.Errorf("write workspace json: %w", err))
		}
		return nil
	}

	_, err := fmt.Fprintf(
		cmd.OutOrStdout(),
		"id: %s\nname: %s\nroot_dir: %s\ncreated_at: %s\nupdated_at: %s\n",
		workspace.ID,
		workspace.Name,
		workspace.RootDir,
		workspace.CreatedAt.Format(time.RFC3339Nano),
		workspace.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return withExitCode(2, fmt.Errorf("write workspace output: %w", err))
	}
	return nil
}

func writeWorkspaceListOutput(cmd *cobra.Command, format string, workspaces []apicore.Workspace) error {
	if format == operatorOutputFormatJSON {
		if err := writeOperatorJSON(cmd.OutOrStdout(), map[string]any{"workspaces": workspaces}); err != nil {
			return withExitCode(2, fmt.Errorf("write workspace list json: %w", err))
		}
		return nil
	}

	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "workspace_count: %d\n", len(workspaces)); err != nil {
		return withExitCode(2, fmt.Errorf("write workspace list header: %w", err))
	}
	if len(workspaces) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(cmd.OutOrStdout(), "id\tname\troot_dir"); err != nil {
		return withExitCode(2, fmt.Errorf("write workspace list header row: %w", err))
	}
	for _, workspace := range workspaces {
		if _, err := fmt.Fprintf(
			cmd.OutOrStdout(),
			"%s\t%s\t%s\n",
			workspace.ID,
			workspace.Name,
			workspace.RootDir,
		); err != nil {
			return withExitCode(2, fmt.Errorf("write workspace list row: %w", err))
		}
	}
	return nil
}

func writeWorkspaceMutationOutput(
	cmd *cobra.Command,
	format string,
	action string,
	created bool,
	workspace apicore.Workspace,
) error {
	if format == operatorOutputFormatJSON {
		payload := map[string]any{
			"action":    action,
			"created":   created,
			"workspace": workspace,
		}
		if err := writeOperatorJSON(cmd.OutOrStdout(), payload); err != nil {
			return withExitCode(2, fmt.Errorf("write workspace mutation json: %w", err))
		}
		return nil
	}

	status := action
	if action == "registered" && !created {
		status = "already registered"
	}
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", status, workspace.RootDir); err != nil {
		return withExitCode(2, fmt.Errorf("write workspace mutation output: %w", err))
	}
	return writeWorkspaceOutput(cmd, format, workspace)
}

func writeWorkspaceDeletionOutput(cmd *cobra.Command, format string, ref string) error {
	if format == operatorOutputFormatJSON {
		if err := writeOperatorJSON(cmd.OutOrStdout(), map[string]any{
			"action":        "unregistered",
			"workspace_ref": ref,
		}); err != nil {
			return withExitCode(2, fmt.Errorf("write workspace deletion json: %w", err))
		}
		return nil
	}
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "unregistered: %s\n", ref); err != nil {
		return withExitCode(2, fmt.Errorf("write workspace deletion output: %w", err))
	}
	return nil
}

func resolveWorkspaceRouteRef(ctx context.Context, client daemonCommandClient, ref string) (string, error) {
	trimmedRef := strings.TrimSpace(ref)
	if trimmedRef == "" {
		return "", apicore.NewProblem(
			http.StatusBadRequest,
			"workspace_ref_required",
			"workspace ref is required",
			nil,
			nil,
		)
	}
	if !looksLikeWorkspacePath(trimmedRef) {
		return trimmedRef, nil
	}

	rootDir, err := discoverWorkspaceRootFrom(ctx, trimmedRef)
	if err != nil {
		return "", err
	}

	workspaces, err := client.ListWorkspaces(ctx)
	if err != nil {
		return "", err
	}

	cleanRoot := filepath.Clean(rootDir)
	for _, workspace := range workspaces {
		if filepath.Clean(workspace.RootDir) == cleanRoot {
			return workspace.ID, nil
		}
	}

	return "", apicore.NewProblem(
		http.StatusNotFound,
		"workspace_not_found",
		fmt.Sprintf("workspace %q is not registered", trimmedRef),
		map[string]any{"workspace": cleanRoot},
		nil,
	)
}

func looksLikeWorkspacePath(ref string) bool {
	trimmedRef := strings.TrimSpace(ref)
	if trimmedRef == "" {
		return false
	}
	return filepath.IsAbs(trimmedRef) ||
		strings.HasPrefix(trimmedRef, ".") ||
		strings.Contains(trimmedRef, string(filepath.Separator)) ||
		strings.Contains(trimmedRef, "/")
}
