package daytona

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"strings"
	"time"

	"github.com/compozy/agh/internal/sandbox"
	"github.com/compozy/agh/internal/toolruntime"
)

const (
	providerAghSandboxIDKey = "agh_sandbox_id"
)

const (
	defaultSDKTimeout    = 60 * time.Second
	defaultCreateTimeout = 5 * time.Minute
)

var _ sandbox.Provider = (*daytonaProvider)(nil)
var _ sandbox.Finder = (*daytonaProvider)(nil)

// Option configures the Daytona provider.
type Option func(*daytonaProvider)

type daytonaProvider struct {
	logger            *slog.Logger
	newClient         sandboxClientFactory
	tokenManager      *sshTokenManager
	shellTransport    transport
	launcherTransport transport
	now               func() time.Time
	sdkTimeout        time.Duration
	createTimeout     time.Duration
	sshHost           string
	processRegistry   *toolruntime.Registry
}

// NewProvider returns the Daytona execution sandbox provider.
func NewProvider(opts ...Option) sandbox.Provider {
	now := time.Now
	tokenManager := newSSHTokenManager(newRESTSSHTokenSource(now), now)
	provider := &daytonaProvider{
		logger:         slog.Default(),
		newClient:      newSDKClient,
		tokenManager:   tokenManager,
		now:            now,
		sdkTimeout:     defaultSDKTimeout,
		createTimeout:  defaultCreateTimeout,
		sshHost:        defaultSSHHost,
		shellTransport: newSSHTransport(tokenManager),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(provider)
		}
	}
	if provider.logger == nil {
		provider.logger = slog.Default()
	}
	if provider.newClient == nil {
		provider.newClient = newSDKClient
	}
	if provider.now == nil {
		provider.now = time.Now
	}
	if provider.sdkTimeout <= 0 {
		provider.sdkTimeout = defaultSDKTimeout
	}
	if provider.createTimeout <= 0 {
		provider.createTimeout = defaultCreateTimeout
	}
	if provider.tokenManager == nil {
		provider.tokenManager = newSSHTokenManager(newRESTSSHTokenSource(provider.now), provider.now)
	}
	if provider.shellTransport == nil {
		provider.shellTransport = newSSHTransport(provider.tokenManager)
	}
	if provider.launcherTransport == nil {
		provider.launcherTransport = newSidecarTransport(
			provider.logger,
			provider.newClient,
			provider.shellTransport,
		)
	}
	if provider.sshHost == "" {
		provider.sshHost = defaultSSHHost
	}
	return provider
}

func WithLogger(logger *slog.Logger) Option {
	return func(provider *daytonaProvider) {
		provider.logger = logger
	}
}

// WithProcessRegistry injects the shared process registry for sandbox-owned tool processes.
func WithProcessRegistry(registry *toolruntime.Registry) Option {
	return func(provider *daytonaProvider) {
		provider.processRegistry = registry
	}
}

func withSandboxClientFactory(factory sandboxClientFactory) Option {
	return func(provider *daytonaProvider) {
		provider.newClient = factory
	}
}

func withTransport(transport transport) Option {
	return func(provider *daytonaProvider) {
		provider.shellTransport = transport
		provider.launcherTransport = transport
	}
}

func withTokenManager(manager *sshTokenManager) Option {
	return func(provider *daytonaProvider) {
		provider.tokenManager = manager
	}
}

func withNow(now func() time.Time) Option {
	return func(provider *daytonaProvider) {
		provider.now = now
	}
}

func (p *daytonaProvider) Backend() sandbox.Backend {
	return sandbox.BackendDaytona
}

func (p *daytonaProvider) Prepare(
	ctx context.Context,
	req sandbox.PrepareRequest,
) (sandbox.Prepared, error) {
	if ctx == nil {
		return sandbox.Prepared{}, errors.New("sandbox/daytona: prepare context is required")
	}
	if req.Sandbox.Backend != sandbox.BackendDaytona {
		return sandbox.Prepared{}, fmt.Errorf(
			"sandbox/daytona: prepare backend = %q, want %q",
			req.Sandbox.Backend,
			sandbox.BackendDaytona,
		)
	}
	if req.Sandbox.Daytona == nil {
		return sandbox.Prepared{}, errors.New("sandbox/daytona: Daytona profile is required")
	}
	if err := p.validateNetworkPolicy(req.Sandbox.Network); err != nil {
		return sandbox.Prepared{}, err
	}
	daytona := req.Sandbox.Daytona
	if daytona.StartupSource == "" {
		return sandbox.Prepared{}, errors.New("sandbox/daytona: daytona snapshot or image is required")
	}

	existingState, err := decodeProviderState(req.ProviderState)
	if err != nil {
		return sandbox.Prepared{}, err
	}
	apiURL := normalizeAPIURL(firstNonEmpty(daytona.APIURL, existingState.APIURL))
	client, err := p.newClient(clientConfig{APIURL: apiURL, Target: daytona.Target})
	if err != nil {
		return sandbox.Prepared{}, err
	}

	instance, err := p.prepareSandbox(ctx, client, req, existingState)
	if err != nil {
		return sandbox.Prepared{}, err
	}

	runtimeRoot := p.runtimeRoot(ctx, instance, req.Sandbox.RuntimeRootDir)
	runtimeAdditional := remoteAdditionalDirs(runtimeRoot, req.LocalAdditionalDirs)
	remoteEnv := remoteEnvMap(req.AgentEnv, req.Sandbox.Env)
	info := sandboxInfo{
		ID:      instance.ID(),
		APIURL:  apiURL,
		SSHHost: p.sshHost,
	}
	access, err := p.tokenManager.Ensure(ctx, apiURL, instance.ID(), false)
	if err != nil {
		return sandbox.Prepared{}, err
	}
	info.SSHAccessExpiresAt = &access.ExpiresAt

	return p.buildPrepared(req, instance, info, access, runtimeRoot, runtimeAdditional, remoteEnv)
}

func (p *daytonaProvider) FindSandbox(
	ctx context.Context,
	req sandbox.FindSandboxRequest,
) (sandbox.SessionState, error) {
	sandboxID, err := validateFindSandboxRequest(ctx, req)
	if err != nil {
		return sandbox.SessionState{}, err
	}

	findConfig, err := newFindSandboxConfig(req)
	if err != nil {
		return sandbox.SessionState{}, err
	}
	client, err := p.newClient(clientConfig{APIURL: findConfig.apiURL, Target: findConfig.target})
	if err != nil {
		return sandbox.SessionState{}, err
	}

	found, err := p.findByLabels(ctx, client, findSandboxLabels(req, sandboxID))
	if err != nil {
		if errors.Is(err, errSandboxNotFound) {
			return sandbox.SessionState{}, fmt.Errorf("%w: %s", sandbox.ErrSandboxNotFound, sandboxID)
		}
		return sandbox.SessionState{}, err
	}
	return p.foundSandboxState(ctx, req, findConfig, found, sandboxID)
}

type findSandboxConfig struct {
	existing      providerState
	apiURL        string
	target        string
	startupSource sandbox.DaytonaStartupSource
	startupRef    string
}

func validateFindSandboxRequest(ctx context.Context, req sandbox.FindSandboxRequest) (string, error) {
	if ctx == nil {
		return "", errors.New("sandbox/daytona: find context is required")
	}
	if req.Sandbox.Backend != sandbox.BackendDaytona {
		return "", fmt.Errorf(
			"sandbox/daytona: find backend = %q, want %q",
			req.Sandbox.Backend,
			sandbox.BackendDaytona,
		)
	}
	sandboxID := strings.TrimSpace(req.SandboxID)
	if sandboxID == "" {
		return "", errors.New("sandbox/daytona: find sandbox id is required")
	}
	return sandboxID, nil
}

func newFindSandboxConfig(req sandbox.FindSandboxRequest) (findSandboxConfig, error) {
	existingState, err := decodeProviderState(req.ProviderState)
	if err != nil {
		return findSandboxConfig{}, err
	}
	config := findSandboxConfig{
		existing:      existingState,
		apiURL:        existingState.APIURL,
		startupSource: existingState.StartupSource,
		startupRef:    existingState.StartupRef,
	}
	daytona := req.Sandbox.Daytona
	if daytona != nil {
		config.apiURL = firstNonEmpty(daytona.APIURL, config.apiURL)
		config.target = daytona.Target
		if daytona.StartupSource != "" {
			config.startupSource = daytona.StartupSource
		}
		if strings.TrimSpace(daytona.StartupRef) != "" {
			config.startupRef = daytona.StartupRef
		}
	}
	config.apiURL = normalizeAPIURL(config.apiURL)
	return config, nil
}

func findSandboxLabels(req sandbox.FindSandboxRequest, sandboxID string) map[string]string {
	labels := map[string]string{providerAghSandboxIDKey: sandboxID}
	if len(req.Labels) > 0 {
		labels = cloneStringMap(req.Labels)
	}
	return labels
}
