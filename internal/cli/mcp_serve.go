package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/compozy/agh/internal/diagnostics"
	mcppkg "github.com/compozy/agh/internal/mcp"
	"github.com/spf13/cobra"
)

const (
	mcpServeTransportStdio = "stdio"
	mcpServeTransportHTTP  = "http"
	// #nosec G101 -- this is an environment variable name, not a credential.
	mcpServeTokenEnv = "AGH_MCP_SERVE_TOKEN"
)

type mcpServeOptions struct {
	Workspace string
	Transport string
	Listen    string
	TokenEnv  string
	Stdin     io.Reader
	Stdout    io.Writer
	Stderr    io.Writer
}

func newMCPServeCommand(deps commandDeps) *cobra.Command {
	opts := mcpServeOptions{}
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve the workspace-bound AGH Host API over MCP",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts.Stdin = cmd.InOrStdin()
			opts.Stdout = cmd.OutOrStdout()
			opts.Stderr = cmd.ErrOrStderr()
			return deps.runMCPServe(cmd.Context(), opts)
		},
	}
	cmd.Flags().StringVar(&opts.Workspace, "workspace", "", "Workspace ID, name, or path to bind")
	cmd.Flags().StringVar(&opts.Transport, "transport", mcpServeTransportStdio, "MCP transport: stdio or http")
	cmd.Flags().StringVar(&opts.Listen, "listen", "", "Loopback host:port for HTTP transport")
	cmd.Flags().StringVar(
		&opts.TokenEnv,
		"token-env",
		mcpServeTokenEnv,
		"Environment variable containing the HTTP bearer token",
	)
	mustMarkFlagRequired(cmd, "workspace")
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

	if transport == mcpServeTransportStdio {
		return mcppkg.ServeStdio(
			ctx,
			invoker,
			opts.Workspace,
			opts.Stdin,
			opts.Stdout,
			opts.Stderr,
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
