package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	mcppkg "github.com/compozy/compozy/internal/mcp"
	"github.com/compozy/compozy/internal/toolmeta"
	"github.com/compozy/compozy/internal/version"
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
	var toolListThreshold int
	cmd := &cobra.Command{
		Use:    toolMCPKey,
		Short:  "Serve session-bound CompozyOS tools over MCP stdio",
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
				return fmt.Errorf("cli: ensure CompozyOS home: %w", err)
			}
			client, err := deps.newClient(runtime.Target)
			if err != nil {
				return err
			}
			hostedClient, ok := client.(mcppkg.HostedProxyClient)
			if !ok {
				return errors.New("cli: daemon client does not support hosted MCP")
			}
			return mcppkg.RunHostedProxy(cmd.Context(), hostedClient, mcppkg.HostedProxyOptions{
				SessionID:         sessionID,
				Nonce:             bindNonce,
				Stdin:             os.Stdin,
				Stdout:            os.Stdout,
				Stderr:            os.Stderr,
				Version:           version.Current().Version,
				ToolListThreshold: toolListThreshold,
			})
		},
	}
	cmd.Flags().StringVar(&sessionID, "session", "", "Session id to bind")
	cmd.Flags().StringVar(&bindNonce, "bind-nonce", "", "Hosted MCP bind nonce")
	cmd.Flags().
		IntVar(&toolListThreshold, toolmeta.HostedToolListThresholdFlag, 0, "MCP tools listed per server before deferral")
	return cmd
}
