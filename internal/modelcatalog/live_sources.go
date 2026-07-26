package modelcatalog

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"maps"
	"net/http"

	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/compozy/agh/internal/vault"
)

const (
	liveSourcesOpenAIEnvName        = "OPENAI_API_KEY"
	liveSourcesClaudeKey            = "claude"
	liveSourcesCodexKey             = "codex"
	liveSourcesHermesKey            = "hermes"
	liveSourcesOllamaKey            = "ollama"
	liveSourcesOpenaiKey            = "openai"
	liveSourcesOpencodeKey          = "opencode"
	liveSourcesOpenrouterKey        = "openrouter"
	liveSourcesVercelAIGatewayValue = "vercel-ai-gateway"
)

const (
	defaultLiveDiscoveryTimeout = 10 * time.Second
	maxLiveDiscoveryPayloadSize = 8 << 20
)

// ProviderSecretResolver resolves provider credential refs for live discovery.
type ProviderSecretResolver interface {
	ResolveRef(ctx context.Context, ref string) (string, error)
}

// EnvSecretResolver resolves env: secret refs from an environment lookup.
type EnvSecretResolver struct {
	LookupEnv func(string) (string, bool)
}

var _ ProviderSecretResolver = EnvSecretResolver{}

// ResolveRef resolves one env-backed provider credential ref.
func (r EnvSecretResolver) ResolveRef(ctx context.Context, ref string) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("model catalog: provider secret context is required")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	normalized := vault.NormalizeRef(ref)
	if !vault.IsEnvRef(normalized) {
		return "", fmt.Errorf("%w: %s", vault.ErrUnsupportedSecretRef, normalized)
	}
	envName, err := vault.EnvNameFromRef(normalized)
	if err != nil {
		return "", err
	}
	lookup := r.LookupEnv
	if lookup == nil {
		lookup = os.LookupEnv
	}
	value, ok := lookup(envName)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%w: env:%s", vault.ErrMissingSecret, envName)
	}
	return value, nil
}

// DiscoveryCommandRequest describes one timeout-bound discovery subprocess.
type DiscoveryCommandRequest struct {
	ProviderID string
	Command    string
	Args       []string
	Env        []string
	Timeout    time.Duration
}

// DiscoveryCommandResult captures safe subprocess output for parsing.
type DiscoveryCommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// DiscoveryCommandExecutor runs a provider discovery command.
type DiscoveryCommandExecutor interface {
	RunDiscoveryCommand(ctx context.Context, req DiscoveryCommandRequest) (DiscoveryCommandResult, error)
}

// ExecDiscoveryCommandExecutor runs discovery commands as subprocesses.
type ExecDiscoveryCommandExecutor struct{}

var _ DiscoveryCommandExecutor = ExecDiscoveryCommandExecutor{}

// RunDiscoveryCommand runs one subprocess with the caller-supplied deadline.
func (ExecDiscoveryCommandExecutor) RunDiscoveryCommand(
	ctx context.Context,
	req DiscoveryCommandRequest,
) (DiscoveryCommandResult, error) {
	if ctx == nil {
		return DiscoveryCommandResult{}, fmt.Errorf("model catalog: discovery command context is required")
	}
	if strings.TrimSpace(req.Command) == "" {
		return DiscoveryCommandResult{}, fmt.Errorf("model catalog: discovery command is required")
	}
	// #nosec G204 -- discovery commands come from validated provider model discovery config.
	cmd := exec.CommandContext(ctx, req.Command, req.Args...)
	cmd.Env = append([]string(nil), req.Env...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := DiscoveryCommandResult{
		Stdout: strings.TrimSpace(stdout.String()),
		Stderr: strings.TrimSpace(stderr.String()),
	}
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return result, fmt.Errorf("model catalog: discovery command timed out after %s: %w", req.Timeout, ctx.Err())
		}
		return result, fmt.Errorf("model catalog: discovery command failed: %w", err)
	}
	return result, nil
}

// LiveProviderSourcesConfig configures built-in provider live discovery sources.
type LiveProviderSourcesConfig struct {
	Providers       map[string]aghconfig.ProviderConfig
	HomePaths       aghconfig.HomePaths
	BaseEnv         []string
	SecretResolver  ProviderSecretResolver
	HTTPClient      *http.Client
	CommandExecutor DiscoveryCommandExecutor
	DefaultTimeout  time.Duration
}

// NewLiveProviderSources creates provider_live sources for known provider adapters.
func NewLiveProviderSources(cfg LiveProviderSourcesConfig) ([]Source, error) {
	providers := aghconfig.BuiltinProviders()
	maps.Copy(providers, cfg.Providers)
	providerIDs := make([]string, 0, len(providers))
	for providerID := range providers {
		if _, ok := liveProviderAdapters[providerID]; ok {
			providerIDs = append(providerIDs, providerID)
		}
	}
	sort.Strings(providerIDs)
	sources := make([]Source, 0, len(providerIDs))
	for _, providerID := range providerIDs {
		source, err := NewLiveProviderSource(providerID, providers[providerID], cfg)
		if err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}
	return sources, nil
}

// NewLiveProviderSource creates one provider_live source.
func NewLiveProviderSource(
	providerID string,
	provider aghconfig.ProviderConfig,
	cfg LiveProviderSourcesConfig,
) (*LiveProviderSource, error) {
	trimmedProviderID := strings.TrimSpace(providerID)
	adapter, ok := liveProviderAdapters[trimmedProviderID]
	if !ok {
		return nil, fmt.Errorf(
			"model catalog: live discovery adapter for provider %q is not registered",
			trimmedProviderID,
		)
	}
	sourceID := SourceKindProviderLiveID(trimmedProviderID)
	if err := ValidateSourceIdentity(sourceID, SourceKindProviderLive); err != nil {
		return nil, err
	}
	timeout := cfg.DefaultTimeout
	if timeout <= 0 {
		timeout = defaultLiveDiscoveryTimeout
	}
	executor := cfg.CommandExecutor
	if executor == nil {
		executor = ExecDiscoveryCommandExecutor{}
	}
	secretResolver := cfg.SecretResolver
	if secretResolver == nil {
		secretResolver = EnvSecretResolver{}
	}
	return &LiveProviderSource{
		providerID:      trimmedProviderID,
		provider:        aghconfig.CloneProviderConfig(provider),
		adapter:         adapter,
		sourceID:        sourceID,
		homePaths:       cfg.HomePaths,
		baseEnv:         append([]string(nil), cfg.BaseEnv...),
		secretResolver:  secretResolver,
		httpClient:      cfg.HTTPClient,
		commandExecutor: executor,
		defaultTimeout:  timeout,
	}, nil
}

// SourceKindProviderLiveID returns the stable source id for a live provider source.
func SourceKindProviderLiveID(providerID string) string {
	return string(SourceKindProviderLive) + ":" + strings.TrimSpace(providerID)
}

// LiveProviderSource performs side-effect-free model discovery for one provider.
type LiveProviderSource struct {
	providerID      string
	provider        aghconfig.ProviderConfig
	adapter         liveProviderAdapter
	sourceID        string
	homePaths       aghconfig.HomePaths
	baseEnv         []string
	secretResolver  ProviderSecretResolver
	httpClient      *http.Client
	commandExecutor DiscoveryCommandExecutor
	defaultTimeout  time.Duration
}

var _ Source = (*LiveProviderSource)(nil)

// ID returns the provider_live source id.
func (s *LiveProviderSource) ID() string {
	return s.sourceID
}

// Kind returns provider_live.
func (s *LiveProviderSource) Kind() SourceKind {
	return SourceKindProviderLive
}

// Priority returns the provider_live merge priority.
func (s *LiveProviderSource) Priority() int {
	return PriorityProviderLive
}

// ProviderIDs returns the single AGH provider id this source owns.
func (s *LiveProviderSource) ProviderIDs() []string {
	return []string{s.providerID}
}

// ListModels discovers live provider models without touching ACP sessions.
func (s *LiveProviderSource) ListModels(ctx context.Context, opts ListOptions) ([]ModelRow, error) {
	if ctx == nil {
		return nil, fmt.Errorf("model catalog: live provider context is required")
	}
	if requested := strings.TrimSpace(opts.ProviderID); requested != "" && requested != s.providerID {
		return nil, nil
	}
	target, err := s.discoveryTarget()
	if err != nil {
		return nil, err
	}
	env, err := s.discoveryEnv(ctx)
	if err != nil {
		return nil, err
	}
	timeout := target.timeout
	if timeout <= 0 {
		timeout = s.defaultTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	now := defaultNow(opts.Now)
	switch target.kind {
	case liveDiscoveryHTTP:
		rows, err := s.listHTTP(runCtx, target.endpoint, env, timeout, now)
		if err != nil {
			return nil, err
		}
		return rows, nil
	case liveDiscoveryCommand:
		rows, err := s.listCommand(runCtx, target.command, env, timeout, now)
		if err != nil {
			return nil, err
		}
		return rows, nil
	default:
		return nil, fmt.Errorf("model catalog: provider %q has no side-effect-free model discovery path", s.providerID)
	}
}
