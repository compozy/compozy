package provider

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
)

// RegistryReader captures the provider lookup surface used by runtime code and overlays.
type RegistryReader interface {
	Get(name string) (Provider, error)
}

type registryLister interface {
	List() []Provider
}

// OverlayKind declares the runtime behavior of one review-provider overlay entry.
type OverlayKind string

const (
	// OverlayKindAlias delegates to another provider by name.
	OverlayKindAlias OverlayKind = "alias"
	// OverlayKindExtension is implemented by the declaring extension subprocess.
	OverlayKindExtension OverlayKind = "extension"
)

// CatalogEntry captures the user-facing review-provider catalog shape.
type CatalogEntry struct {
	Name        string
	DisplayName string
}

type displayNamer interface {
	DisplayName() string
}

// ExtensionBridge lazily bridges one extension-backed review provider to the
// extension subprocess protocol.
type ExtensionBridge interface {
	FetchReviews(ctx context.Context, providerName string, req FetchRequest) ([]ReviewItem, error)
	ResolveIssues(ctx context.Context, providerName string, pr string, issues []ResolvedIssue) error
	Close() error
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
	Name        string
	DisplayName string
	Kind        OverlayKind
	Target      string
	Command     string
	Bridge      ExtensionBridge
	Metadata    map[string]string
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

	for _, entry := range entries {
		if overlayKind(entry) == OverlayKindExtension && entry.Bridge == nil {
			return nil, fmt.Errorf("activate review provider overlay %q: missing extension bridge", entry.Name)
		}
	}

	factory := func(base RegistryReader) RegistryReader {
		return buildDeclaredReviewOverlay(base, entries)
	}

	activeOverlayMu.Lock()
	previous := activeOverlayFactory
	activeOverlayFactory = factory
	activeOverlayMu.Unlock()

	return func() {
		closeOverlayBridges(entries)
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

// Catalog returns the active provider catalog layered on top of the supplied base registry.
func Catalog(base RegistryReader) []CatalogEntry {
	registry := ResolveRegistry(base)
	lister, ok := registry.(registryLister)
	if !ok {
		return nil
	}

	providers := lister.List()
	entries := make([]CatalogEntry, 0, len(providers))
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		entry := CatalogEntry{Name: strings.TrimSpace(provider.Name())}
		if named, ok := provider.(displayNamer); ok {
			entry.DisplayName = strings.TrimSpace(named.DisplayName())
		}
		if entry.DisplayName == "" {
			entry.DisplayName = entry.Name
		}
		if entry.Name != "" {
			entries = append(entries, entry)
		}
	}
	return entries
}

func buildDeclaredReviewOverlay(base RegistryReader, entries []OverlayEntry) RegistryReader {
	overlay := NewOverlayRegistry(base)
	for _, entry := range entries {
		switch overlayKind(entry) {
		case OverlayKindExtension:
			overlay.Register(&extensionBackedProvider{
				name:        strings.TrimSpace(entry.Name),
				displayName: overlayDisplayName(entry),
				bridge:      entry.Bridge,
			})
		default:
			overlay.Register(&aliasedProvider{
				name:        strings.TrimSpace(entry.Name),
				displayName: overlayDisplayName(entry),
				targetName:  overlayTarget(entry),
				registry:    overlay,
			})
		}
	}
	return overlay
}

type aliasedProvider struct {
	name        string
	displayName string
	targetName  string
	registry    RegistryReader
}

func (p *aliasedProvider) Name() string {
	if p == nil {
		return ""
	}
	return p.name
}

func (p *aliasedProvider) DisplayName() string {
	if p == nil {
		return ""
	}
	if strings.TrimSpace(p.displayName) != "" {
		return p.displayName
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

func (r *OverlayRegistry) List() []Provider {
	if r == nil {
		return nil
	}

	providers := make(map[string]Provider)
	if base, ok := r.base.(registryLister); ok {
		for _, provider := range base.List() {
			if provider == nil {
				continue
			}
			name := strings.TrimSpace(strings.ToLower(provider.Name()))
			if name != "" {
				providers[name] = provider
			}
		}
	}
	for name, provider := range r.providers {
		providers[name] = provider
	}

	names := make([]string, 0, len(providers))
	for name := range providers {
		names = append(names, name)
	}
	slices.Sort(names)

	items := make([]Provider, 0, len(names))
	for _, name := range names {
		items = append(items, providers[name])
	}
	return items
}

func overlayTarget(entry OverlayEntry) string {
	if target := strings.TrimSpace(entry.Target); target != "" {
		return target
	}
	return strings.TrimSpace(entry.Command)
}

func overlayKind(entry OverlayEntry) OverlayKind {
	switch entry.Kind {
	case OverlayKindExtension:
		return OverlayKindExtension
	default:
		return OverlayKindAlias
	}
}

func overlayDisplayName(entry OverlayEntry) string {
	if displayName := strings.TrimSpace(entry.DisplayName); displayName != "" {
		return displayName
	}
	return strings.TrimSpace(entry.Name)
}

func closeOverlayBridges(entries []OverlayEntry) {
	seen := make(map[ExtensionBridge]struct{})
	for _, entry := range entries {
		if entry.Bridge == nil {
			continue
		}
		if _, ok := seen[entry.Bridge]; ok {
			continue
		}
		seen[entry.Bridge] = struct{}{}
		if err := entry.Bridge.Close(); err != nil {
			slog.Warn(
				"close review provider bridge",
				"component", "provider.overlay",
				"provider", strings.TrimSpace(entry.Name),
				"err", err,
			)
		}
	}
}

type extensionBackedProvider struct {
	name        string
	displayName string
	bridge      ExtensionBridge
}

func (p *extensionBackedProvider) Name() string {
	if p == nil {
		return ""
	}
	return p.name
}

func (p *extensionBackedProvider) DisplayName() string {
	if p == nil {
		return ""
	}
	if strings.TrimSpace(p.displayName) != "" {
		return p.displayName
	}
	return p.name
}

func (p *extensionBackedProvider) FetchReviews(
	ctx context.Context,
	req FetchRequest,
) ([]ReviewItem, error) {
	if p == nil || p.bridge == nil {
		return nil, fmt.Errorf("declared review provider %q is missing an extension bridge", p.name)
	}
	return p.bridge.FetchReviews(ctx, p.name, req)
}

func (p *extensionBackedProvider) ResolveIssues(
	ctx context.Context,
	pr string,
	issues []ResolvedIssue,
) error {
	if p == nil || p.bridge == nil {
		return fmt.Errorf("declared review provider %q is missing an extension bridge", p.name)
	}
	return p.bridge.ResolveIssues(ctx, p.name, pr, issues)
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
