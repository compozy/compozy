package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/compozy/compozy/internal/service"
)

func newServiceCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{Use: "service", Short: "Manage the persistent user daemon"}
	cmd.AddCommand(
		serviceVerb(deps, "install", "Install and start the user service", (*service.Manager).Install),
		serviceVerb(deps, "uninstall", "Stop and remove the user service", (*service.Manager).Uninstall),
		serviceVerb(deps, "status", "Show user service status", (*service.Manager).Status),
	)
	return cmd
}

func serviceVerb(
	deps commandDeps, name, short string, run func(*service.Manager, context.Context) (service.Result, error),
) *cobra.Command {
	return &cobra.Command{Use: name, Short: short, Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			paths, err := deps.resolveHome()
			if err != nil {
				return err
			}
			manager, err := service.New(paths, os.Args[0])
			if err != nil {
				return err
			}
			result, err := run(manager, cmd.Context())
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, outputBundle{jsonValue: result, human: func() (string, error) {
				switch {
				case result.Warning == service.NoManager:
					return "This computer has no supported user service manager.", nil
				case result.Running:
					return fmt.Sprintf("CompozyOS service is running (%s).", result.Manager), nil
				case result.Installed:
					return "CompozyOS service is installed and stopped.", nil
				default:
					return "CompozyOS service is not installed.", nil
				}
			}})
		}}
}
