package llmadapter

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/compozy/compozy/engine/core"
)

var (
	// ErrProviderNil indicates a nil provider registration attempt.
	ErrProviderNil = errors.New("provider must not be nil")
	// ErrProviderNameEmpty indicates a provider registration with empty name.
	ErrProviderNameEmpty = errors.New("provider name must not be empty")
	// ErrProviderAlreadyRegistered indicates duplicate provider registration.
	ErrProviderAlreadyRegistered = errors.New("provider already registered")
)

// ProviderCapabilities describes the features supported by an LLM provider.
type ProviderCapabilities struct {
	StructuredOutput bool
	Streaming        bool
	Vision           bool
	// ContextWindowTokens conveys the maximum combined prompt+completion tokens
	// that the provider can process in a single request. When zero, the limit is
	// unknown and callers should apply conservative fallbacks.
	ContextWindowTokens int
}

// Provider exposes capability metadata and client construction for an LLM provider.
type Provider interface {
	Name() core.ProviderName
	Capabilities() ProviderCapabilities
	NewClient(ctx context.Context, cfg *core.ProviderConfig) (LLMClient, error)
}

// Registry stores provider registrations used by the default factory.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

// NewProviderRegistry creates an empty provider registry.
func NewProviderRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

// Register adds a provider to the registry, guarding against duplicates.
func (r *Registry) Register(provider Provider) error {
	if provider == nil {
		return ErrProviderNil
	}
	name := provider.Name()
	if name == "" {
		return ErrProviderNameEmpty
	}
	key := canonicalProviderName(name)
	if key == "" {
		return ErrProviderNameEmpty
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.providers[key]; exists {
		return fmt.Errorf("%w: %s", ErrProviderAlreadyRegistered, name)
	}
	r.providers[key] = provider
	return nil
}

// Resolve retrieves a provider by name.
func (r *Registry) Resolve(name core.ProviderName) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	provider, ok := r.providers[canonicalProviderName(name)]
	if !ok {
		return nil, fmt.Errorf("provider %s is not registered", name)
	}
	return provider, nil
}

// Capabilities returns capability metadata for the requested provider.
func (r *Registry) Capabilities(name core.ProviderName) (ProviderCapabilities, error) {
	provider, err := r.Resolve(name)
	if err != nil {
		return ProviderCapabilities{}, err
	}
	return provider.Capabilities(), nil
}

// NewClient creates a client for a single provider configuration.
func (r *Registry) NewClient(ctx context.Context, config *core.ProviderConfig) (LLMClient, error) {
	if config == nil {
		return nil, fmt.Errorf("provider config must not be nil")
	}
	provider, err := r.Resolve(config.Provider)
	if err != nil {
		return nil, err
	}
	return provider.NewClient(ctx, config)
}

// ProviderRoute captures an ordered list of providers for fallback sequencing.
type ProviderRoute struct {
	entries   []routeEntry
	index     int
	lastError error
	mu        sync.Mutex
}

type routeEntry struct {
	provider Provider
	config   *core.ProviderConfig
}

// BuildRoute constructs a route for the primary config and optional fallbacks.
func (r *Registry) BuildRoute(
	config *core.ProviderConfig,
	fallbacks ...*core.ProviderConfig,
) (*ProviderRoute, error) {
	if config == nil {
		return nil, fmt.Errorf("provider config must not be nil")
	}
	configs := append([]*core.ProviderConfig{config}, fallbacks...)
	entries := make([]routeEntry, 0, len(configs))
	for _, cfg := range configs {
		if cfg == nil {
			return nil, fmt.Errorf("provider config in route must not be nil")
		}
		provider, err := r.Resolve(cfg.Provider)
		if err != nil {
			return nil, err
		}
		clone := cloneProviderConfig(cfg)
		entries = append(entries, routeEntry{
			provider: provider,
			config:   clone,
		})
	}
	return &ProviderRoute{entries: entries}, nil
}

// Next attempts to create a client using the next provider in the route.
func (r *ProviderRoute) Next(ctx context.Context) (LLMClient, error) {
	if r == nil {
		return nil, fmt.Errorf("provider route is nil")
	}
	for {
		entry, ok := r.nextEntry()
		if !ok {
			if err := r.readLastError(); err != nil {
				return nil, fmt.Errorf("no providers left in route, last error: %w", err)
			}
			return nil, fmt.Errorf("no providers available in route")
		}
		client, err := entry.provider.NewClient(ctx, entry.config)
		if err != nil {
			r.setLastError(err)
			continue
		}
		return client, nil
	}
}

func (r *ProviderRoute) nextEntry() (routeEntry, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.index >= len(r.entries) {
		return routeEntry{}, false
	}
	entry := r.entries[r.index]
	r.index++
	return entry, true
}

func (r *ProviderRoute) setLastError(err error) {
	if err == nil {
		return
	}
	r.mu.Lock()
	r.lastError = err
	r.mu.Unlock()
}

func (r *ProviderRoute) readLastError() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastError
}

func cloneProviderConfig(cfg *core.ProviderConfig) *core.ProviderConfig {
	if cfg == nil {
		return nil
	}
	clone := *cfg
	clone.Params = clonePromptParams(&cfg.Params)
	return &clone
}

func clonePromptParams(params *core.PromptParams) core.PromptParams {
	if params == nil {
		return core.PromptParams{}
	}
	clone := *params
	if len(params.StopWords) > 0 {
		clone.StopWords = append([]string(nil), params.StopWords...)
	}
	// Avoid map aliasing
	if clone.Metadata != nil {
		clone.Metadata = core.CloneMap(clone.Metadata)
	}
	return clone
}

func canonicalProviderName(name core.ProviderName) string {
	return strings.ToLower(strings.TrimSpace(string(name)))
}
