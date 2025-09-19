package admin

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/cli/api"
	"github.com/compozy/compozy/cli/cmd"
	"github.com/compozy/compozy/cli/helpers"
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
			return cmd.ExecuteCommand(cobraCmd, cmd.ExecutorOptions{RequireAuth: true}, cmd.ModeHandlers{
				JSON: func(ctx context.Context, _ *cobra.Command, executor *cmd.CommandExecutor, _ []string) error {
					return callAdminEndpoint(ctx, executor, "/admin/export-yaml")
				},
				TUI: func(ctx context.Context, _ *cobra.Command, executor *cmd.CommandExecutor, _ []string) error {
					return callAdminEndpoint(ctx, executor, "/admin/export-yaml")
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
			if strategy != "" {
				path = fmt.Sprintf("%s?strategy=%s", path, strategy)
			}
			return cmd.ExecuteCommand(cobraCmd, cmd.ExecutorOptions{RequireAuth: true}, cmd.ModeHandlers{
				JSON: func(ctx context.Context, _ *cobra.Command, executor *cmd.CommandExecutor, _ []string) error {
					return callAdminEndpoint(ctx, executor, path)
				},
				TUI: func(ctx context.Context, _ *cobra.Command, executor *cmd.CommandExecutor, _ []string) error {
					return callAdminEndpoint(ctx, executor, path)
				},
			}, nil)
		},
	}
	c.Flags().StringVar(&strategy, "strategy", "seed_only", "Import strategy: seed_only|overwrite_conflicts")
	helpers.AddGlobalFlags(c)
	return c
}

func callAdminEndpoint(ctx context.Context, executor *cmd.CommandExecutor, path string) error {
	client := executor.GetAuthClient()
	if client == nil {
		return fmt.Errorf("auth client not initialized")
	}
	env, err := api.CallGETDecode(ctx, client, path)
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
