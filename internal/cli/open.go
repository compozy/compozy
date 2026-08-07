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
		Short: "Open the Compozy web UI in the default browser",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			runtimeContext, err := loadRuntimeContext(deps)
			if err != nil {
				return err
			}
			client, err := deps.newClient(runtimeContext.Target)
			if err != nil {
				return err
			}
			if deps.reportClientTarget != nil {
				if err := deps.reportClientTarget(runtimeContext.Target); err != nil {
					return err
				}
			}
			status, err := client.DaemonStatus(ctx)
			if err != nil {
				if runtimeContext.Target.isRemoteGateway() {
					return fmt.Errorf("open: remote daemon is not reachable: %w", err)
				}
				return fmt.Errorf("open: daemon is not running: %w", err)
			}
			webURL, err := webUIURL(runtimeContext.Target, status)
			if err != nil {
				return err
			}
			return deps.openBrowser(ctx, webURL)
		},
	}
}

func webUIURL(target ClientTarget, status DaemonStatus) (string, error) {
	if target.isRemoteGateway() {
		return target.requestBaseURL(), nil
	}
	if status.HTTPHost == "" || status.HTTPPort == 0 {
		return "", errors.New("open: daemon did not report a valid HTTP address")
	}
	return fmt.Sprintf("http://%s:%d", status.HTTPHost, status.HTTPPort), nil
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
