package mcp

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/diagnostics"
	toolspkg "github.com/compozy/agh/internal/tools"
	"github.com/compozy/agh/internal/vault"
	mcpclient "github.com/mark3labs/mcp-go/client"
	mcptransport "github.com/mark3labs/mcp-go/client/transport"
	mcpsdk "github.com/mark3labs/mcp-go/mcp"
)

func (e *CallExecutor) openClient(ctx context.Context, resolved ResolvedServer) (*mcpclient.Client, error) {
	server := resolved.Server
	transport := server.EffectiveTransport()
	switch transport {
	case aghconfig.MCPServerTransportStdio:
		env, err := e.mcpServerEnv(ctx, server)
		if err != nil {
			return nil, err
		}
		client, err := mcpclient.NewStdioMCPClientWithOptions(
			strings.TrimSpace(server.Command),
			env,
			trimStrings(server.Args),
			mcptransport.WithCommandFunc(mcpStdioCommandWithExactEnv),
		)
		if err != nil {
			return nil, normalizeMCPError("", err)
		}
		if err := client.Start(ctx); err != nil {
			return nil, normalizeMCPStartError(client, err)
		}
		return client, nil
	case aghconfig.MCPServerTransportHTTP:
		options := []mcptransport.StreamableHTTPCOption{
			mcptransport.WithHTTPBasicClient(e.httpClientForCall()),
		}
		if server.Auth.Enabled() {
			options = append(options, mcptransport.WithHTTPHeaderFunc(e.authHeaderFunc(resolved)))
		}
		client, err := mcpclient.NewStreamableHttpClient(strings.TrimSpace(server.URL), options...)
		if err != nil {
			return nil, normalizeMCPError("", err)
		}
		if err := client.Start(ctx); err != nil {
			return nil, normalizeMCPStartError(client, err)
		}
		return client, nil
	case aghconfig.MCPServerTransportSSE:
		options := []mcptransport.ClientOption{
			mcptransport.WithHTTPClient(e.httpClientForCall()),
			mcptransport.WithEndpointTimeout(e.timeout),
			mcptransport.WithResponseTimeout(e.timeout),
		}
		if server.Auth.Enabled() {
			options = append(options, mcptransport.WithHeaderFunc(e.authHeaderFunc(resolved)))
		}
		client, err := mcpclient.NewSSEMCPClient(strings.TrimSpace(server.URL), options...)
		if err != nil {
			return nil, normalizeMCPError("", err)
		}
		if err := client.Start(ctx); err != nil {
			return nil, normalizeMCPStartError(client, err)
		}
		return client, nil
	default:
		return nil, toolspkg.NewValidationError(
			"mcp_server.transport",
			toolspkg.ReasonMCPUnreachable,
			"unsupported mcp transport",
		)
	}
}

func initializeClient(ctx context.Context, client *mcpclient.Client) error {
	_, err := client.Initialize(ctx, mcpsdk.InitializeRequest{
		Params: mcpsdk.InitializeParams{
			ProtocolVersion: mcpsdk.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcpsdk.Implementation{
				Name:    "agh",
				Version: "0.0.0",
			},
			Capabilities: mcpsdk.ClientCapabilities{},
		},
	})
	return err
}

func normalizeMCPStartError(client *mcpclient.Client, err error) error {
	if closeErr := client.Close(); closeErr != nil {
		err = fmt.Errorf("%w; close MCP client after start failure: %v", err, closeErr)
	}
	return normalizeMCPError("", err)
}

func closeMCPClient(client *mcpclient.Client) {
	if client == nil {
		return
	}
	if err := client.Close(); err != nil {
		// The operation already completed; callers have no useful recovery action for transport close failure.
		return
	}
}

func (e *CallExecutor) httpClientForCall() *http.Client {
	cloned := *e.httpClient
	if cloned.Timeout <= 0 {
		cloned.Timeout = e.timeout
	}
	return &cloned
}

func mcpServerEnv(env map[string]string) []string {
	merged := mcpStdioBaseEnv()
	for key, value := range env {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		merged[trimmedKey] = value
	}
	keys := make([]string, 0, len(merged))
	for key := range merged {
		if strings.TrimSpace(key) != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	values := make([]string, 0, len(keys))
	for _, key := range keys {
		values = append(values, strings.TrimSpace(key)+"="+merged[key])
	}
	return values
}

func mcpStdioCommandWithExactEnv(ctx context.Context, command string, env []string, args []string) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Env = append([]string(nil), env...)
	return cmd, nil
}

func mcpStdioBaseEnv() map[string]string {
	keys := []string{
		"PATH",
		"HOME",
		"TMPDIR",
		"TMP",
		"TEMP",
		"SystemRoot",
		"WINDIR",
		"COMSPEC",
		"PATHEXT",
		"SSL_CERT_FILE",
		"SSL_CERT_DIR",
	}
	base := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := os.LookupEnv(key); ok {
			base[key] = value
		}
	}
	return base
}

func (e *CallExecutor) mcpServerEnv(ctx context.Context, server aghconfig.MCPServer) ([]string, error) {
	env := cloneStringMap(server.Env)
	if len(server.SecretEnv) > 0 {
		if env == nil {
			env = make(map[string]string, len(server.SecretEnv))
		}
		for key, ref := range server.SecretEnv {
			trimmedKey := strings.TrimSpace(key)
			if trimmedKey == "" {
				continue
			}
			value, err := e.resolveSecretRef(ctx, ref)
			if err != nil {
				return nil, fmt.Errorf("mcp: resolve secret_env %s for server %q: %w", trimmedKey, server.Name, err)
			}
			diagnostics.RegisterDynamicSecret(value)
			env[trimmedKey] = value
		}
	}
	return mcpServerEnv(env), nil
}

func (e *CallExecutor) resolveSecretRef(ctx context.Context, ref string) (string, error) {
	normalized := vault.NormalizeRef(ref)
	if normalized == "" {
		return "", nil
	}
	if e != nil && e.secretResolver != nil {
		value, err := e.secretResolver.ResolveRef(ctx, normalized)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(value) != "" {
			diagnostics.RegisterDynamicSecret(value)
		}
		return value, nil
	}
	if vault.IsEnvRef(normalized) {
		envName, err := vault.EnvNameFromRef(normalized)
		if err != nil {
			return "", err
		}
		if e != nil && e.lookupSecret != nil {
			value := e.lookupSecret(envName)
			if strings.TrimSpace(value) == "" {
				return "", fmt.Errorf("%w: env:%s", vault.ErrMissingSecret, envName)
			}
			diagnostics.RegisterDynamicSecret(value)
			return value, nil
		}
	}
	return "", fmt.Errorf("%w: %s", vault.ErrUnsupportedSecretRef, normalized)
}
