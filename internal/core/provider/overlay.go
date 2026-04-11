package provider

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// RegistryReader captures the provider lookup surface used by runtime code and overlays.
type RegistryReader interface {
	Get(name string) (Provider, error)
}

// OverlayRegistry layers overlay providers on top of a base registry without mutating the base catalog.
type OverlayRegistry struct {
	base      RegistryReader
	providers map[string]Provider
}

type overlayFactory func(base RegistryReader) RegistryReader

var (
	activeOverlayMu      sync.RWMutex
	activeOverlayFactory overlayFactory
)

// OverlayEntry captures one declarative review-provider overlay entry assembled during command bootstrap.
type OverlayEntry struct {
	Name     string
	Command  string
	Metadata map[string]string
}

// NewOverlayRegistry constructs a provider overlay on top of a base registry.
func NewOverlayRegistry(base RegistryReader) *OverlayRegistry {
	return &OverlayRegistry{
		base:      base,
		providers: make(map[string]Provider),
	}
}

// Register adds or replaces one overlay provider without mutating the base registry.
func (r *OverlayRegistry) Register(p Provider) {
	if r == nil || p == nil {
		return
	}
	name := strings.TrimSpace(strings.ToLower(p.Name()))
	if name == "" {
		return
	}
	r.providers[name] = p
}

// Get resolves an overlay provider first, then falls back to the base registry.
func (r *OverlayRegistry) Get(name string) (Provider, error) {
	if r == nil {
		return nil, fmt.Errorf("provider overlay registry is nil")
	}
	key := strings.TrimSpace(strings.ToLower(name))
	if provider, ok := r.providers[key]; ok {
		return provider, nil
	}
	if r.base == nil {
		return nil, fmt.Errorf("unknown review provider %q", name)
	}
	return r.base.Get(name)
}

// ActivateOverlay installs a command-scoped review-provider overlay and returns a restore function.
func ActivateOverlay(entries []OverlayEntry) (func(), error) {
	if len(entries) == 0 {
		return func() {}, nil
	}

	factory := func(base RegistryReader) RegistryReader {
		return buildDeclaredReviewOverlay(base, entries)
	}

	activeOverlayMu.Lock()
	previous := activeOverlayFactory
	activeOverlayFactory = factory
	activeOverlayMu.Unlock()

	return func() {
		activeOverlayMu.Lock()
		activeOverlayFactory = previous
		activeOverlayMu.Unlock()
	}, nil
}

// ResolveRegistry applies the active command-scoped overlay to the provided base registry.
func ResolveRegistry(base RegistryReader) RegistryReader {
	activeOverlayMu.RLock()
	factory := activeOverlayFactory
	activeOverlayMu.RUnlock()
	if factory == nil {
		return base
	}
	return factory(base)
}

func buildDeclaredReviewOverlay(base RegistryReader, entries []OverlayEntry) RegistryReader {
	overlay := NewOverlayRegistry(base)
	for _, entry := range entries {
		overlay.Register(&aliasedProvider{
			name:       strings.TrimSpace(entry.Name),
			targetName: strings.TrimSpace(entry.Command),
			registry:   overlay,
		})
	}
	return overlay
}

type aliasedProvider struct {
	name       string
	targetName string
	registry   RegistryReader
}

func (p *aliasedProvider) Name() string {
	if p == nil {
		return ""
	}
	return p.name
}

func (p *aliasedProvider) FetchReviews(ctx context.Context, req FetchRequest) ([]ReviewItem, error) {
	target, err := p.resolveTarget(nil)
	if err != nil {
		return nil, err
	}
	return target.FetchReviews(ctx, req)
}

func (p *aliasedProvider) ResolveIssues(ctx context.Context, pr string, issues []ResolvedIssue) error {
	target, err := p.resolveTarget(nil)
	if err != nil {
		return err
	}
	return target.ResolveIssues(ctx, pr, issues)
}

func (p *aliasedProvider) resolveTarget(seen map[string]struct{}) (Provider, error) {
	if p == nil {
		return nil, fmt.Errorf("declared review provider is nil")
	}

	name := strings.TrimSpace(strings.ToLower(p.name))
	if seen == nil {
		seen = make(map[string]struct{})
	}
	if _, ok := seen[name]; ok {
		return nil, fmt.Errorf("review provider alias cycle detected for %q", p.name)
	}
	seen[name] = struct{}{}

	targetName := strings.TrimSpace(p.targetName)
	if targetName == "" {
		return nil, fmt.Errorf("declared review provider %q is missing a target provider name", p.name)
	}
	if strings.EqualFold(targetName, p.name) {
		return nil, fmt.Errorf("declared review provider %q cannot target itself", p.name)
	}

	target, err := p.registry.Get(targetName)
	if err != nil {
		return nil, fmt.Errorf("resolve declared review provider %q target %q: %w", p.name, targetName, err)
	}
	alias, ok := target.(*aliasedProvider)
	if !ok {
		return target, nil
	}
	return alias.resolveTarget(seen)
}
