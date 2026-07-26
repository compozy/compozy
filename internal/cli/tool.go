package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	mcppkg "github.com/compozy/agh/internal/mcp"
	"github.com/compozy/agh/internal/version"
	"github.com/spf13/cobra"
)

const (
	toolMCPKey  = "mcp"
	toolToolKey = "tool"
)

func newToolCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   toolToolKey,
		Short: "Inspect and invoke registry tools",
	}
	cmd.AddCommand(newToolListCommand(deps))
	cmd.AddCommand(newToolSearchCommand(deps))
	cmd.AddCommand(newToolInfoCommand(deps))
	cmd.AddCommand(newToolApproveCommand(deps))
	cmd.AddCommand(newToolApprovalsCommand(deps))
	cmd.AddCommand(newToolInvokeCommand(deps))
	cmd.AddCommand(newToolArtifactCommand(deps))
	cmd.AddCommand(newToolMCPCommand(deps))
	return cmd
}

func newToolMCPCommand(deps commandDeps) *cobra.Command {
	var sessionID string
	var bindNonce string
	cmd := &cobra.Command{
		Use:    toolMCPKey,
		Short:  "Serve session-bound AGH tools over MCP stdio",
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			sessionID = strings.TrimSpace(sessionID)
			if sessionID == "" {
				return mcppkg.ErrHostedSessionRequired
			}
			bindNonce = strings.TrimSpace(bindNonce)
			if bindNonce == "" {
				return mcppkg.ErrHostedNonceRequired
			}
			runtime, err := loadRuntimeContext(deps)
			if err != nil {
				return err
			}
			if err := deps.ensureHome(runtime.HomePaths); err != nil {
				return fmt.Errorf("cli: ensure AGH home: %w", err)
			}
			client, err := deps.newClient(runtime.Config.Daemon.Socket)
			if err != nil {
				return err
			}
			hostedClient, ok := client.(mcppkg.HostedProxyClient)
			if !ok {
				return errors.New("cli: daemon client does not support hosted MCP")
			}
			return mcppkg.RunHostedProxy(cmd.Context(), hostedClient, mcppkg.HostedProxyOptions{
				SessionID: sessionID,
				Nonce:     bindNonce,
				Stdin:     os.Stdin,
				Stdout:    os.Stdout,
				Stderr:    os.Stderr,
				Version:   version.Current().Version,
			})
		},
	}
	cmd.Flags().StringVar(&sessionID, "session", "", "Session id to bind")
	cmd.Flags().StringVar(&bindNonce, "bind-nonce", "", "Hosted MCP bind nonce")
	return cmd
}
