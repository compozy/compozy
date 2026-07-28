package sandbox

import (
	"errors"
	"fmt"
	"maps"
	"reflect"
)

const (
	// DefaultBackend is the execution backend used when no profile selects one.
	DefaultBackend = BackendLocal
)

var (
	// ErrNilProvider reports an attempt to register a nil provider.
	ErrNilProvider = errors.New("sandbox: provider is nil")
	// ErrInvalidProviderBackend reports that a provider returned an unknown backend.
	ErrInvalidProviderBackend = errors.New("sandbox: provider backend is invalid")
	// ErrProviderNotRegistered reports that no provider is registered for a backend.
	ErrProviderNotRegistered = errors.New("sandbox: provider not registered")
)

// Registry resolves sandbox providers by backend.
type Registry struct {
	providers map[Backend]Provider
}

// NewRegistry constructs a provider registry populated with the supplied providers.
func NewRegistry(providers ...Provider) (*Registry, error) {
	registry := &Registry{
		providers: make(map[Backend]Provider, len(providers)),
	}
	for _, provider := range providers {
		if err := registry.Register(provider); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

// Register adds or replaces the provider for its backend.
func (r *Registry) Register(provider Provider) error {
	if providerIsNil(provider) {
		return ErrNilProvider
	}
	backend := provider.Backend()
	if !backend.Valid() {
		return fmt.Errorf("%w: %q", ErrInvalidProviderBackend, backend)
	}
	if r.providers == nil {
		r.providers = make(map[Backend]Provider)
	}
	r.providers[backend] = provider
	return nil
}

// Provider returns the provider registered for backend.
func (r *Registry) Provider(backend Backend) (Provider, error) {
	if r == nil || r.providers == nil {
		return nil, fmt.Errorf("%w: %q", ErrProviderNotRegistered, backend)
	}
	provider, ok := r.providers[backend]
	if !ok || providerIsNil(provider) {
		return nil, fmt.Errorf("%w: %q", ErrProviderNotRegistered, backend)
	}
	return provider, nil
}

// DefaultProvider returns the provider registered for the default backend.
func (r *Registry) DefaultProvider() (Provider, error) {
	return r.Provider(DefaultBackend)
}

// Providers returns a snapshot of registered providers keyed by backend.
func (r *Registry) Providers() map[Backend]Provider {
	if r == nil || len(r.providers) == 0 {
		return map[Backend]Provider{}
	}
	providers := make(map[Backend]Provider, len(r.providers))
	maps.Copy(providers, r.providers)
	return providers
}

func providerIsNil(provider Provider) bool {
	if provider == nil {
		return true
	}
	value := reflect.ValueOf(provider)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
