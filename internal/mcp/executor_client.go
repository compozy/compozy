package mcp

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	"github.com/compozy/compozy/internal/mcp/securehttp"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/vault"
	"github.com/compozy/compozy/internal/version"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

type managedMCPClient struct {
	session *mcpsdk.ClientSession
	cleanup func() error
}

const maxAuthorizationChallengeDrainBytes = int64(64 << 10)

func (e *CallExecutor) openClient(
	ctx context.Context,
	resolved ResolvedServer,
	authorizationHeader string,
) (*managedMCPClient, error) {
	server := resolved.Server
	transport := server.EffectiveTransport()
	switch transport {
	case compozyconfig.MCPServerTransportStdio:
		launch, err := e.newMCPStdioLaunch(ctx, server, resolved.Target)
		if err != nil {
			return nil, err
		}
		session, err := e.connectClient(ctx, &mcpsdk.CommandTransport{Command: launch.command})
		if err != nil {
			return nil, errors.Join(err, launch.cleanup())
		}
		return &managedMCPClient{session: session, cleanup: launch.cleanup}, nil
	case compozyconfig.MCPServerTransportHTTP:
		session, err := e.connectClient(ctx, &mcpsdk.StreamableClientTransport{
			Endpoint:             strings.TrimSpace(server.URL),
			HTTPClient:           e.httpClientWithAuthorization(resolved, authorizationHeader),
			DisableStandaloneSSE: true,
			MaxRetries:           -1,
		})
		if err != nil {
			return nil, err
		}
		return &managedMCPClient{session: session}, nil
	default:
		return nil, toolspkg.NewValidationError(
			"mcp_server.transport",
			toolspkg.ReasonMCPUnreachable,
			"unsupported mcp transport",
		)
	}
}

func (e *CallExecutor) connectClient(ctx context.Context, transport mcpsdk.Transport) (*mcpsdk.ClientSession, error) {
	client := mcpsdk.NewClient(
		&mcpsdk.Implementation{Name: "compozy", Version: version.Current().Version},
		&mcpsdk.ClientOptions{
			Capabilities:   &mcpsdk.ClientCapabilities{},
			MultiRoundTrip: &mcpsdk.MultiRoundTripOptions{Disabled: true},
		},
	)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, normalizeMCPError("", err)
	}
	if result := session.InitializeResult(); result == nil || !supportedProtocolVersion(result.ProtocolVersion) {
		version := ""
		if result != nil {
			version = result.ProtocolVersion
		}
		return nil, errors.Join(
			&UnsupportedProtocolVersionError{Version: version},
			closeMCPClient(&managedMCPClient{session: session}),
		)
	}
	return session, nil
}

func closeMCPClient(client *managedMCPClient) error {
	if client == nil {
		return nil
	}
	var err error
	if client.session != nil {
		if closeErr := client.session.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("mcp: close client session: %w", closeErr))
		}
	}
	if client.cleanup != nil {
		if cleanupErr := client.cleanup(); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("mcp: clean up client resources: %w", cleanupErr))
		}
	}
	return err
}

func joinMCPClientCleanup(err error, client *managedMCPClient) error {
	return errors.Join(err, closeMCPClient(client))
}

func (e *CallExecutor) httpClientWithAuthorization(
	resolved ResolvedServer,
	authorizationHeader string,
) *http.Client {
	client := e.httpClientForServer(resolved)
	if resolved.Server.Auth.Enabled() {
		client.CheckRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	client.Transport = authorizationRoundTripper{next: transport, header: authorizationHeader}
	return client
}

func (e *CallExecutor) httpClientForServer(resolved ResolvedServer) *http.Client {
	if strings.TrimSpace(resolved.Server.CatalogEntry) != "" {
		return securehttp.NewClient(securehttp.WithTimeout(e.timeout)).HTTPClient()
	}
	return e.httpClientForCall()
}

type authorizationRoundTripper struct {
	next   http.RoundTripper
	header string
}

func (t authorizationRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	cloned := request.Clone(request.Context())
	if t.header != "" {
		cloned.Header.Set("Authorization", t.header)
	}
	response, err := t.next.RoundTrip(cloned)
	if err != nil {
		return nil, err
	}
	if authErr := authorizationChallengeError(response); authErr != nil {
		return nil, errors.Join(authErr, drainAndCloseAuthorizationChallenge(response.Body))
	}
	return response, nil
}

func drainAndCloseAuthorizationChallenge(body io.ReadCloser) error {
	if body == nil {
		return nil
	}
	var cleanupErr error
	if _, err := io.Copy(io.Discard, io.LimitReader(body, maxAuthorizationChallengeDrainBytes)); err != nil {
		cleanupErr = fmt.Errorf("mcp: drain authorization challenge response body: %w", err)
	}
	if err := body.Close(); err != nil {
		cleanupErr = errors.Join(
			cleanupErr,
			fmt.Errorf("mcp: close authorization challenge response body: %w", err),
		)
	}
	return cleanupErr
}

func authorizationChallengeError(response *http.Response) error {
	if response == nil {
		return nil
	}
	if response.StatusCode == http.StatusUnauthorized {
		return ErrAuthorizationRequired
	}
	return insufficientScopeError(response)
}

func insufficientScopeError(response *http.Response) error {
	if response == nil || response.StatusCode != http.StatusForbidden {
		return nil
	}
	challenges, err := oauthex.ParseWWWAuthenticate(response.Header.Values("WWW-Authenticate"))
	if err != nil {
		return nil
	}
	for _, challenge := range challenges {
		if challenge.Scheme != "bearer" || challenge.Params["error"] != "insufficient_scope" {
			continue
		}
		return &InsufficientScopeError{Scopes: trimStrings(strings.Fields(challenge.Params["scope"]))}
	}
	return nil
}

func (e *CallExecutor) httpClientForCall() *http.Client {
	cloned := *e.httpClient
	if cloned.Timeout <= 0 || cloned.Timeout > e.timeout {
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

func mcpStdioCommandWithExactEnv(ctx context.Context, command string, env []string, args []string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Env = append([]string(nil), env...)
	return cmd
}

type mcpStdioLaunch struct {
	command *exec.Cmd
	cleanup func() error
}

func (e *CallExecutor) newMCPStdioLaunch(
	ctx context.Context,
	server compozyconfig.MCPServer,
	target mcpauth.Target,
) (mcpStdioLaunch, error) {
	env, err := e.mcpServerEnv(ctx, server)
	if err != nil {
		return mcpStdioLaunch{}, err
	}
	home, err := mcpStdioHome(target)
	if err != nil {
		return mcpStdioLaunch{}, err
	}
	cleanup := func() error {
		return os.RemoveAll(home)
	}
	env = append(env,
		"HOME="+home,
		"XDG_CONFIG_HOME="+filepath.Join(home, "config"),
		"XDG_CACHE_HOME="+filepath.Join(home, "cache"),
		"npm_config_cache="+filepath.Join(home, "npm-cache"),
		"UV_CACHE_DIR="+filepath.Join(home, "uv-cache"),
	)
	command := mcpStdioCommandWithExactEnv(ctx, strings.TrimSpace(server.Command), env, trimStrings(server.Args))
	return mcpStdioLaunch{command: command, cleanup: cleanup}, nil
}

func mcpStdioHome(target mcpauth.Target) (string, error) {
	key, err := target.Key()
	if err != nil {
		return "", fmt.Errorf("mcp: derive stdio target identity: %w", err)
	}
	prefix := mcpStdioHomePrefix(key)
	home, err := os.MkdirTemp("", prefix)
	if err != nil {
		return "", fmt.Errorf("mcp: create isolated stdio home: %w", err)
	}
	for _, path := range []string{
		filepath.Join(home, "config"),
		filepath.Join(home, "cache"),
		filepath.Join(home, "npm-cache"),
		filepath.Join(home, "uv-cache"),
	} {
		if err := os.Mkdir(path, 0o700); err != nil {
			return "", errors.Join(
				fmt.Errorf("mcp: create isolated stdio directory %q: %w", path, err),
				os.RemoveAll(home),
			)
		}
	}
	return home, nil
}

func mcpStdioHomePrefix(targetKey string) string {
	digest := sha256.Sum256([]byte(targetKey))
	return fmt.Sprintf("compozy-mcp-%x-", digest[:8])
}

func mcpStdioBaseEnv() map[string]string {
	keys := []string{
		"PATH",
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

func (e *CallExecutor) mcpServerEnv(ctx context.Context, server compozyconfig.MCPServer) ([]string, error) {
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
			registerMCPRequestSecret(ctx, value)
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
			registerMCPRequestSecret(ctx, value)
			return value, nil
		}
	}
	return "", fmt.Errorf("%w: %s", vault.ErrUnsupportedSecretRef, normalized)
}
