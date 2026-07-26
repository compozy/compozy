package cli

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
)

const openCommandKey = "open"

func newOpenCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   openCommandKey,
		Short: "Open the AGH web UI in the default browser",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			status, err := client.DaemonStatus(ctx)
			if err != nil {
				return fmt.Errorf("open: daemon is not running: %w", err)
			}
			if status.HTTPHost == "" || status.HTTPPort == 0 {
				return errors.New("open: daemon did not report a valid HTTP address")
			}

			url := fmt.Sprintf("http://%s:%d", status.HTTPHost, status.HTTPPort)
			return openBrowser(ctx, url)
		},
	}
}

func openBrowser(ctx context.Context, url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, openCommandKey, url)
	case "windows":
		cmd = exec.CommandContext(ctx, "cmd", "/c", "start", url)
	default:
		cmd = exec.CommandContext(ctx, "xdg-open", url)
	}
	return cmd.Start()
}
