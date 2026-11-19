package mcp

import (
	"bytes"
	"context"
	"errors"
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
	digestKey      []byte
	consent        func(context.Context, toolspkg.SourceRef, ResolvedServer) error
	authNetwork    func(context.Context, mcpauth.ServerConfig) error
	authRecovery   func(context.Context, *ResolvedServer) error
	servers        ServerResolver
	tokenStore     mcpauth.TokenStore
	auth           authService
	lookupSecret   func(string) string
	secretResolver secretRefResolver
	dataDirCreator func(string) error
	runtimeHealth  *RuntimeHealthRegistry
	httpClient     *http.Client
	secureClient   *securehttp.Client
	authGeneration *mcpauth.MutationGeneration
	redirectURI    string
	timeout        time.Duration
	toolListFloor  time.Duration
	toolCache      mcpToolListCache
	stdioPool      *StdioSessionPool
	catalogHTTP    *mcpCatalogHTTP
}

var _ toolspkg.MCPCallExecutor = (*CallExecutor)(nil)
var _ toolspkg.MCPAuthStatusProvider = (*CallExecutor)(nil)

// CallExecutorOption configures the daemon-owned MCP executor.
type CallExecutorOption func(*CallExecutor)

// WithToolCatalogNotifications enables lifecycle-owned HTTP discovery subscriptions.
func WithToolCatalogNotifications() CallExecutorOption {
	return func(e *CallExecutor) {
		e.catalogHTTP = &mcpCatalogHTTP{
			sessions: make(map[string]mcpCatalogHTTPEntry), changes: make(chan struct{}, 1),
		}
	}
}

func WithDefinitionDigestKey(key []byte) CallExecutorOption {
	return func(e *CallExecutor) { e.digestKey = bytes.Clone(key) }
}

func WithConsentCheck(check func(context.Context, toolspkg.SourceRef, ResolvedServer) error) CallExecutorOption {
	return func(e *CallExecutor) { e.consent = check }
}

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

// WithAuthRedirectURI supplies the configured OAuth callback URI for CIMD renewal.
func WithAuthRedirectURI(redirectURI string) CallExecutorOption {
	return func(executor *CallExecutor) { executor.redirectURI = redirectURI }
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

// WithDataDirCreator injects the first-launch package data-directory boundary.
func WithDataDirCreator(create func(string) error) CallExecutorOption {
	return func(executor *CallExecutor) {
		executor.dataDirCreator = create
	}
}

// WithRuntimeHealthRegistry shares restart-local extension MCP health with daemon read surfaces.
func WithRuntimeHealthRegistry(registry *RuntimeHealthRegistry) CallExecutorOption {
	return func(executor *CallExecutor) {
		executor.runtimeHealth = registry
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

// withToolListFloor bounds ordinary discovery frequency for test fixtures.
func withToolListFloor(interval time.Duration) CallExecutorOption {
	return func(executor *CallExecutor) {
		if interval > 0 {
			executor.toolListFloor = interval
		}
	}
}

// WithToolListNormalizationCapacity sets a positive limit in place of the default of 128 entries.
func WithToolListNormalizationCapacity(capacity int) CallExecutorOption {
	return func(executor *CallExecutor) {
		if capacity > 0 {
			executor.toolCache.normalizationCapacity = capacity
		}
	}
}

// WithStdioSessionPool reuses daemon-owned stdio MCP children instead of launching one per request.
func WithStdioSessionPool(pool *StdioSessionPool) CallExecutorOption {
	return func(executor *CallExecutor) {
		executor.stdioPool = pool
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
			normalizationCapacity: defaultToolNormalizationCacheCapacity,
			entries:               make(map[string]mcpToolListCacheEntry),
			projectionStates:      make(map[string]mcpToolProjectionState),
		},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(executor)
		}
	}
	if len(executor.digestKey) == 0 {
		return nil, errors.New("mcp: definition digest key is required")
	}
	if executor.lookupSecret == nil {
		executor.lookupSecret = func(string) string { return "" }
	}
	if executor.dataDirCreator == nil {
		executor.dataDirCreator = createWritableMCPDataDir
	}
	if executor.runtimeHealth == nil {
		executor.runtimeHealth = NewRuntimeHealthRegistry()
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
		authClient := executor.secureClient
		if authClient == nil {
			authClient = securehttp.NewClient(
				securehttp.WithAllowLoopback(true),
				securehttp.WithTimeout(executor.timeout),
			)
		}
		options := []mcpauth.ServiceOption{
			mcpauth.WithMutationGeneration(executor.authGeneration),
			mcpauth.WithNetworkAuthorization(executor.authNetwork),
			mcpauth.WithRedirectURI(executor.redirectURI),
			mcpauth.WithSecureHTTPClient(authClient),
		}
		if executor.secretResolver != nil {
			options = append(options, mcpauth.WithSecretRefResolver(executor.secretResolver.ResolveRef))
		}
		if registrations, ok := executor.tokenStore.(mcpauth.RegistrationStore); ok {
			options = append(options, mcpauth.WithRegistrationStore(registrations))
		}
		service, err := mcpauth.NewService(executor.tokenStore, options...)
		if err != nil {
			return nil, fmt.Errorf("mcp: create auth service: %w", err)
		}
		executor.auth = service
	}
	return executor, nil
}

// WithAuthNetworkAuthorization guards continued OAuth network activity.
func WithAuthNetworkAuthorization(check func(context.Context, mcpauth.ServerConfig) error) CallExecutorOption {
	return func(e *CallExecutor) { e.authNetwork = check }
}

// WithAuthRecovery clears authentication-related runtime failures after renewal.
func WithAuthRecovery(onRecovery func(context.Context, *ResolvedServer) error) CallExecutorOption {
	return func(e *CallExecutor) { e.authRecovery = onRecovery }
}
