package resource

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/compozy/compozy/cli/api"
	"github.com/compozy/compozy/cli/cmd"
	"github.com/compozy/compozy/cli/helpers"
	"github.com/spf13/cobra"
)

var allowedStrategies = map[string]struct{}{"seed_only": {}, "overwrite_conflicts": {}}

type CommandConfig struct {
	Use              string
	Short            string
	Long             string
	ExportPath       string
	ImportPath       string
	SupportsStrategy bool
}

func NewCommand(cfg *CommandConfig) *cobra.Command {
	root := &cobra.Command{Use: cfg.Use, Short: cfg.Short, Long: cfg.Long}
	root.AddCommand(newExportCommand(cfg), newImportCommand(cfg))
	return root
}

func newExportCommand(cfg *CommandConfig) *cobra.Command {
	c := &cobra.Command{
		Use:   "export",
		Short: fmt.Sprintf("Export %s to YAML", cfg.Use),
		RunE:  func(cmdObj *cobra.Command, _ []string) error { return executePost(cmdObj, cfg.ExportPath, "") },
	}
	helpers.AddGlobalFlags(c)
	return c
}

func newImportCommand(cfg *CommandConfig) *cobra.Command {
	c := &cobra.Command{
		Use:   "import",
		Short: fmt.Sprintf("Import %s from YAML", cfg.Use),
		RunE: func(cmdObj *cobra.Command, _ []string) error {
			strategy := ""
			if cfg.SupportsStrategy {
				s := cmdObj.Flags().Lookup("strategy")
				value := "seed_only"
				if s != nil {
					value = s.Value.String()
				}
				normalized, err := normalizeStrategy(value)
				if err != nil {
					return err
				}
				strategy = normalized
			}
			return executePost(cmdObj, cfg.ImportPath, strategy)
		},
	}
	if cfg.SupportsStrategy {
		c.Flags().String("strategy", "seed_only", "Import strategy: seed_only|overwrite_conflicts")
	}
	helpers.AddGlobalFlags(c)
	return c
}

func normalizeStrategy(value string) (string, error) {
	v := strings.TrimSpace(strings.ToLower(value))
	if v == "" {
		return "seed_only", nil
	}
	if _, ok := allowedStrategies[v]; !ok {
		return "", fmt.Errorf("invalid --strategy: %q (allowed: seed_only|overwrite_conflicts)", value)
	}
	return v, nil
}

func executePost(cmdObj *cobra.Command, basePath, strategy string) error {
	return cmd.ExecuteCommand(cmdObj, cmd.ExecutorOptions{RequireAuth: true}, cmd.ModeHandlers{
		JSON: func(ctx context.Context, _ *cobra.Command, executor *cmd.CommandExecutor, _ []string) error {
			return callResourcePostEndpoint(ctx, executor, basePath, strategy)
		},
		TUI: func(ctx context.Context, _ *cobra.Command, executor *cmd.CommandExecutor, _ []string) error {
			return callResourcePostEndpoint(ctx, executor, basePath, strategy)
		},
	}, nil)
}

func callResourcePostEndpoint(ctx context.Context, executor *cmd.CommandExecutor, basePath, strategy string) error {
	client := executor.GetAuthClient()
	if client == nil {
		return fmt.Errorf("auth client not initialized")
	}
	path := basePath
	if strategy != "" {
		u, err := url.Parse(basePath)
		if err != nil {
			return fmt.Errorf("invalid resource path: %w", err)
		}
		q := u.Query()
		q.Set("strategy", strategy)
		u.RawQuery = q.Encode()
		path = u.String()
	}
	env, err := api.CallPOSTDecode(ctx, client, path, nil)
	if err != nil {
		return err
	}
	if env.Message != "" {
		fmt.Println(env.Message)
	}
	if dataMap, ok := env.Data.(map[string]any); ok {
		for k, v := range dataMap {
			fmt.Printf("%s: %v\n", k, v)
		}
		return nil
	}
	if env.Data != nil {
		fmt.Printf("data: %v\n", env.Data)
	}
	return nil
}
