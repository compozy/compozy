package admin

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/compozy/compozy/cli/api"
	"github.com/compozy/compozy/cli/cmd"
	"github.com/compozy/compozy/cli/helpers"
	"github.com/compozy/compozy/cli/tui/models"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	cmdRoot := &cobra.Command{
		Use:   "admin",
		Short: "Admin operations (export/import YAML)",
		RunE: func(c *cobra.Command, _ []string) error {
			return c.Help()
		},
	}
	cmdRoot.AddCommand(exportCmd(), importCmd())
	return cmdRoot
}

func exportCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "export",
		Short: "Export store to YAML in project directories",
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			path := "/admin/export-yaml"
			return cmd.ExecuteCommand(cobraCmd, cmd.ExecutorOptions{RequireAuth: true}, cmd.ModeHandlers{
				JSON: func(ctx context.Context, _ *cobra.Command, executor *cmd.CommandExecutor, _ []string) error {
					return callAdminPostEndpoint(ctx, executor, path)
				},
				TUI: func(ctx context.Context, _ *cobra.Command, executor *cmd.CommandExecutor, _ []string) error {
					return callAdminPostEndpoint(ctx, executor, path)
				},
			}, nil)
		},
	}
	helpers.AddGlobalFlags(c)
	return c
}

func importCmd() *cobra.Command {
	var strategy string
	c := &cobra.Command{
		Use:   "import",
		Short: "Import YAML from project directories into store",
		RunE: func(cobraCmd *cobra.Command, _ []string) error {
			path := "/admin/import-yaml"
			if s := strings.TrimSpace(strings.ToLower(strategy)); s != "" {
				switch s {
				case "seed_only", "overwrite_conflicts":
					path = fmt.Sprintf("%s?strategy=%s", path, url.QueryEscape(s))
				default:
					return fmt.Errorf("invalid --strategy: %q (allowed: seed_only|overwrite_conflicts)", strategy)
				}
			}
			return cmd.ExecuteCommand(cobraCmd, cmd.ExecutorOptions{RequireAuth: true}, cmd.ModeHandlers{
				JSON: func(ctx context.Context, _ *cobra.Command, executor *cmd.CommandExecutor, _ []string) error {
					return callAdminPostEndpoint(ctx, executor, path)
				},
				TUI: func(ctx context.Context, _ *cobra.Command, executor *cmd.CommandExecutor, _ []string) error {
					return callAdminPostEndpoint(ctx, executor, path)
				},
			}, nil)
		},
	}
	c.Flags().StringVar(&strategy, "strategy", "seed_only", "Import strategy: seed_only|overwrite_conflicts")
	helpers.AddGlobalFlags(c)
	return c
}

func callAdminPostEndpoint(ctx context.Context, executor *cmd.CommandExecutor, path string) error {
	client := executor.GetAuthClient()
	if client == nil {
		return fmt.Errorf("auth client not initialized")
	}
	var (
		env *models.APIResponse
		err error
	)
	env, err = api.CallPOSTDecode(ctx, client, path, nil)
	if err != nil {
		return err
	}
	if env.Message != "" {
		fmt.Println(env.Message)
	}
	if m, ok := env.Data.(map[string]any); ok {
		for k, v := range m {
			fmt.Printf("%s: %v\n", k, v)
		}
	}
	return nil
}
