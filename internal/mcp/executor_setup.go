package mcp

import (
	"context"
	"fmt"
	"net/http"
	"time"

	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	"github.com/compozy/compozy/internal/mcp/securehttp"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

const defaultCallTimeout = 30 * time.Second

type authService interface {
	Status(ctx context.Context, cfg mcpauth.ServerConfig) (mcpauth.Status, error)
	Refresh(ctx context.Context, cfg mcpauth.ServerConfig) (mcpauth.Status, error)
	AuthorizationToken(ctx context.Context, cfg mcpauth.ServerConfig) (mcpauth.TokenRecord, error)
}

type secretRefResolver interface {
	ResolveRef(ctx context.Context, ref string) (string, error)
}

// CallExecutor lists and calls configured MCP servers through official MCP SDK clients.
type CallExecutor struct {
	servers        ServerResolver
	tokenStore     mcpauth.TokenStore
	auth           authService
	lookupSecret   func(string) string
	secretResolver secretRefResolver
	httpClient     *http.Client
	secureClient   *securehttp.Client
	authGeneration *mcpauth.MutationGeneration
	timeout        time.Duration
	toolCache      mcpToolListCache
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

// WithHTTPClient configures remote MCP HTTP calls with an explicit client.
func WithHTTPClient(client *http.Client) CallExecutorOption {
	return func(executor *CallExecutor) {
		executor.httpClient = client
		executor.secureClient = nil
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
		timeout: defaultCallTimeout,
		toolCache: mcpToolListCache{
			entries:          make(map[string]mcpToolListCacheEntry),
			projectionStates: make(map[string]mcpToolProjectionState),
		},
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
		executor.secureClient = securehttp.NewClient(
			securehttp.WithAllowLoopback(true),
			securehttp.WithTimeout(executor.timeout),
		)
		executor.httpClient = executor.secureClient.HTTPClient()
	}
	if executor.httpClient.Timeout <= 0 {
		cloned := *executor.httpClient
		cloned.Timeout = executor.timeout
		executor.httpClient = &cloned
	}
	if executor.auth == nil && executor.tokenStore != nil {
		options := []mcpauth.ServiceOption{mcpauth.WithMutationGeneration(executor.authGeneration)}
		authClient := executor.secureClient
		if authClient == nil {
			authClient = securehttp.NewClient(
				securehttp.WithAllowLoopback(true),
				securehttp.WithTimeout(executor.timeout),
			)
		}
		options = append(options, mcpauth.WithSecureHTTPClient(authClient))
		if executor.secretResolver != nil {
			options = append(options, mcpauth.WithSecretRefResolver(executor.secretResolver.ResolveRef))
		}
		if registrations, ok := executor.tokenStore.(mcpauth.RegistrationStore); ok {
			options = append(options, mcpauth.WithRegistrationStore(registrations))
		}
		service, err := mcpauth.NewService(
			executor.tokenStore,
			options...,
		)
		if err != nil {
			return nil, fmt.Errorf("mcp: create auth service: %w", err)
		}
		executor.auth = service
	}
	return executor, nil
}
