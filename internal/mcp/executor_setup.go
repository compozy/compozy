package mcp

import (
	"context"
	"fmt"
	"net/http"
	"time"

	mcpauth "github.com/compozy/agh/internal/mcp/auth"
	toolspkg "github.com/compozy/agh/internal/tools"
)

const defaultCallTimeout = 30 * time.Second

type authService interface {
	Status(ctx context.Context, cfg mcpauth.ServerConfig) (mcpauth.Status, error)
	Refresh(ctx context.Context, cfg mcpauth.ServerConfig) (mcpauth.Status, error)
}

type secretRefResolver interface {
	ResolveRef(ctx context.Context, ref string) (string, error)
}

// CallExecutor lists and calls configured MCP servers through mcp-go clients.
type CallExecutor struct {
	servers        ServerResolver
	tokenStore     mcpauth.TokenStore
	auth           authService
	lookupSecret   func(string) string
	secretResolver secretRefResolver
	httpClient     *http.Client
	authGeneration *mcpauth.MutationGeneration
	timeout        time.Duration
}

var _ toolspkg.MCPCallExecutor = (*CallExecutor)(nil)
var _ toolspkg.MCPAuthStatusProvider = (*CallExecutor)(nil)

// CallExecutorOption configures the daemon-owned MCP executor.
type CallExecutorOption func(*CallExecutor)

// WithTokenStore allows remote MCP authorization headers to be injected inside internal/mcp.
func WithTokenStore(store mcpauth.TokenStore) CallExecutorOption {
	return func(executor *CallExecutor) {
		executor.tokenStore = store
	}
}

// WithAuthMutationGeneration coordinates refresh with settings-owned login and logout.
func WithAuthMutationGeneration(generation *mcpauth.MutationGeneration) CallExecutorOption {
	return func(executor *CallExecutor) {
		executor.authGeneration = generation
	}
}

// WithSecretLookup resolves auth client secret environment-variable names.
func WithSecretLookup(lookup func(string) string) CallExecutorOption {
	return func(executor *CallExecutor) {
		executor.lookupSecret = lookup
	}
}

// WithSecretResolver resolves env: and vault: refs for MCP auth and stdio secret_env launch bindings.
func WithSecretResolver(resolver secretRefResolver) CallExecutorOption {
	return func(executor *CallExecutor) {
		executor.secretResolver = resolver
	}
}

// WithHTTPClient configures remote MCP and auth HTTP calls with an explicit client.
func WithHTTPClient(client *http.Client) CallExecutorOption {
	return func(executor *CallExecutor) {
		executor.httpClient = client
	}
}

// WithTimeout configures the default bounded call timeout when a caller has no deadline.
func WithTimeout(timeout time.Duration) CallExecutorOption {
	return func(executor *CallExecutor) {
		if timeout > 0 {
			executor.timeout = timeout
		}
	}
}

func withAuthService(service authService) CallExecutorOption {
	return func(executor *CallExecutor) {
		executor.auth = service
	}
}

// NewMCPCallExecutor constructs the daemon-owned MCP executor.
func NewMCPCallExecutor(
	servers ServerResolver,
	opts ...CallExecutorOption,
) (*CallExecutor, error) {
	if servers == nil {
		return nil, toolspkg.NewValidationError(
			"servers",
			toolspkg.ReasonDependencyMissing,
			"mcp server resolver is required",
		)
	}
	executor := &CallExecutor{
		servers: servers,
		httpClient: &http.Client{
			Timeout: defaultCallTimeout,
		},
		timeout: defaultCallTimeout,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(executor)
		}
	}
	if executor.lookupSecret == nil {
		executor.lookupSecret = func(string) string { return "" }
	}
	if executor.timeout <= 0 {
		executor.timeout = defaultCallTimeout
	}
	if executor.httpClient == nil {
		executor.httpClient = &http.Client{Timeout: executor.timeout}
	}
	if executor.httpClient.Timeout <= 0 {
		cloned := *executor.httpClient
		cloned.Timeout = executor.timeout
		executor.httpClient = &cloned
	}
	if executor.auth == nil && executor.tokenStore != nil {
		service, err := mcpauth.NewService(
			executor.tokenStore,
			mcpauth.WithHTTPClient(executor.httpClient),
			mcpauth.WithMutationGeneration(executor.authGeneration),
		)
		if err != nil {
			return nil, fmt.Errorf("mcp: create auth service: %w", err)
		}
		executor.auth = service
	}
	return executor, nil
}
