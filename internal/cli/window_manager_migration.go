package cli

import (
	"context"
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/windowmanager"
	"github.com/spf13/cobra"
)

type windowManagerMigrationClient interface {
	WindowManagerMigration(context.Context, string, string, bool) (windowmanager.MigrationStatus, error)
}

func (c *daemonClient) WindowManagerMigration(ctx context.Context, workspace, action string, replace bool) (windowmanager.MigrationStatus, error) {
	var status windowmanager.MigrationStatus
	method := http.MethodGet
	var body any
	if action != "" {
		method = http.MethodPost
		body = contract.WindowManagerMigrationRequest{Action: action, Replace: replace}
	}
	err := c.doJSON(ctx, method, windowManagerClientPath(workspace)+"/migration", nil, body, &status)
	return status, err
}

func newLayoutMigrationCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{Use: "migration", Short: "Inspect or recover the layout upgrade", Args: cobra.NoArgs}
	for _, action := range []string{"status", "ack", "retry"} {
		var workspace string
		var replace bool
		child := &cobra.Command{
			Use: action, Short: map[string]string{"status": "Show the layout migration status", "ack": "Acknowledge the layout migration notice", "retry": "Retry the preserved layout migration"}[action], Args: cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				client, workspace, err := windowManagerClientAndWorkspace(cmd, deps, workspace)
				if err != nil {
					return err
				}
				migrations, ok := client.(windowManagerMigrationClient)
				if !ok {
					return errors.New("cli: window-manager migration client is unavailable")
				}
				operation := action
				if operation == "status" {
					operation = ""
				}
				status, err := migrations.WindowManagerMigration(cmd.Context(), workspace, operation, replace)
				if err != nil {
					return err
				}
				return writeCommandOutput(cmd, windowManagerJSONBundle("window_manager_migration", "Layout migration", status))
			},
		}
		child.Flags().StringVar(&workspace, windowManagerWorkspaceFlag, "", "Override workspace (ID, name, or path)")
		if action == "retry" {
			child.Flags().BoolVar(&replace, "replace", false, "Replace a layout changed after the failed migration")
		}
		cmd.AddCommand(child)
	}
	return cmd
}
