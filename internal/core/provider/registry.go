package provider

import (
	"fmt"
	"slices"
	"strings"
)

// Registry maps provider names to implementations.
type Registry struct {
	providers map[string]Provider
}

func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]Provider)}
}

func (r *Registry) Register(p Provider) {
	if r == nil || p == nil {
		return
	}
	name := strings.TrimSpace(strings.ToLower(p.Name()))
	if name == "" {
		return
	}
	r.providers[name] = p
}

func (r *Registry) Get(name string) (Provider, error) {
	if r == nil {
		return nil, fmt.Errorf("provider registry is nil")
	}
	key := strings.TrimSpace(strings.ToLower(name))
	p, ok := r.providers[key]
	if !ok {
		return nil, fmt.Errorf("unknown review provider %q", name)
	}
	return p, nil
}

// List returns the registered providers in deterministic name order.
func (r *Registry) List() []Provider {
	if r == nil {
		return nil
	}

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	slices.Sort(names)

	providers := make([]Provider, 0, len(names))
	for _, name := range names {
		if provider, ok := r.providers[name]; ok {
			providers = append(providers, provider)
		}
	}
	return providers
}
