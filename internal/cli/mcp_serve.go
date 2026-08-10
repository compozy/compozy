package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/compozy/compozy/internal/diagnostics"
	mcppkg "github.com/compozy/compozy/internal/mcp"
	"github.com/spf13/cobra"
)

const (
	mcpServeTransportStdio = "stdio"
	mcpServeTransportHTTP  = "http"
	// #nosec G101 -- this is an environment variable name, not a credential.
	mcpServeTokenEnv = "COMPOZY_MCP_SERVE_TOKEN"
)

type mcpServeOptions struct {
	Workspace string
	Transport string
	Listen    string
	TokenEnv  string
	Stdin     io.Reader
	Stdout    io.Writer
}

func newMCPServeCommand(deps commandDeps) *cobra.Command {
	opts := mcpServeOptions{}
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve the workspace-bound CompozyOS Host API over MCP",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts.Stdin = cmd.InOrStdin()
			opts.Stdout = cmd.OutOrStdout()
			return deps.runMCPServe(cmd.Context(), opts)
		},
	}
	cmd.Flags().StringVar(&opts.Workspace, "workspace", "", "Override workspace binding (ID, name, or path)")
	cmd.Flags().StringVar(&opts.Transport, "transport", mcpServeTransportStdio, "MCP transport: stdio or http")
	cmd.Flags().StringVar(&opts.Listen, "listen", "", "Loopback host:port for HTTP transport")
	cmd.Flags().StringVar(
		&opts.TokenEnv,
		"token-env",
		mcpServeTokenEnv,
		"Environment variable containing the HTTP bearer token",
	)
	return cmd
}

func runMCPServe(ctx context.Context, deps commandDeps, opts mcpServeOptions) error {
	transport := strings.ToLower(strings.TrimSpace(opts.Transport))
	if transport != mcpServeTransportStdio && transport != mcpServeTransportHTTP {
		return fmt.Errorf("cli: unsupported MCP transport %q", opts.Transport)
	}
	if transport == mcpServeTransportStdio && strings.TrimSpace(opts.Listen) != "" {
		return errors.New("cli: --listen is only valid with --transport http")
	}
	if transport == mcpServeTransportHTTP && strings.TrimSpace(opts.Listen) == "" {
		return errors.New("cli: --listen is required with --transport http")
	}

	client, running, err := daemonClientIfRunning(ctx, deps)
	if err != nil {
		return err
	}
	if !running {
		return errors.New("cli: daemon is not running")
	}
	invoker, ok := client.(mcppkg.HostAPIInvoker)
	if !ok {
		return errors.New("cli: daemon client does not support MCP Host API relay")
	}
	resolution, err := resolveCommandWorkspace(
		ctx,
		nil,
		deps,
		client,
		workspaceResolutionRequest{FlagRef: opts.Workspace},
	)
	if err != nil {
		return err
	}
	opts.Workspace = resolution.ID

	if transport == mcpServeTransportStdio {
		return mcppkg.ServeStdio(
			ctx,
			invoker,
			opts.Workspace,
			opts.Stdin,
			opts.Stdout,
		)
	}
	tokenEnv := strings.TrimSpace(opts.TokenEnv)
	if tokenEnv == "" {
		return errors.New("cli: --token-env is required with --transport http")
	}
	token := strings.TrimSpace(deps.getenv(tokenEnv))
	if token == "" {
		return fmt.Errorf("cli: MCP bearer token environment variable %q is empty", tokenEnv)
	}
	cleanupSecret := diagnostics.RegisterDynamicSecret(token)
	defer cleanupSecret()
	return mcppkg.ServeHTTP(
		ctx,
		invoker,
		opts.Workspace,
		opts.Listen,
		token,
		slog.Default(),
	)
}
