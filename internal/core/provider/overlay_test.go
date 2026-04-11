package provider

import (
	"context"
	"strings"
	"testing"
)

type overlayTestProvider struct {
	name        string
	displayName string
}

func (p overlayTestProvider) Name() string { return p.name }

func (p overlayTestProvider) DisplayName() string {
	if strings.TrimSpace(p.displayName) == "" {
		return p.name
	}
	return p.displayName
}

func (overlayTestProvider) FetchReviews(context.Context, FetchRequest) ([]ReviewItem, error) {
	return nil, nil
}

func (overlayTestProvider) ResolveIssues(context.Context, string, []ResolvedIssue) error {
	return nil
}

type overlayTestBridge struct {
	fetchProvider string
	fetchRequest  FetchRequest
	resolvePR     string
	resolveIssues []ResolvedIssue
	closeCount    int
}

func (b *overlayTestBridge) FetchReviews(
	_ context.Context,
	providerName string,
	req FetchRequest,
) ([]ReviewItem, error) {
	b.fetchProvider = providerName
	b.fetchRequest = req
	return []ReviewItem{{
		Title: "bridge-item",
		Body:  "from extension",
	}}, nil
}

func (b *overlayTestBridge) ResolveIssues(
	_ context.Context,
	providerName string,
	pr string,
	issues []ResolvedIssue,
) error {
	b.fetchProvider = providerName
	b.resolvePR = pr
	b.resolveIssues = append([]ResolvedIssue(nil), issues...)
	return nil
}

func (b *overlayTestBridge) Close() error {
	b.closeCount++
	return nil
}

func TestOverlayRegistryReturnsOverlayProviderBeforeBaseProvider(t *testing.T) {
	t.Parallel()

	base := NewRegistry()
	base.Register(overlayTestProvider{name: "base"})

	overlay := NewOverlayRegistry(base)
	overlay.Register(overlayTestProvider{name: "ext"})

	provider, err := overlay.Get("ext")
	if err != nil {
		t.Fatalf("overlay get ext: %v", err)
	}
	if got := provider.Name(); got != "ext" {
		t.Fatalf("unexpected overlay provider name: %q", got)
	}

	baseProvider, err := overlay.Get("base")
	if err != nil {
		t.Fatalf("overlay get base: %v", err)
	}
	if got := baseProvider.Name(); got != "base" {
		t.Fatalf("unexpected base provider name: %q", got)
	}
}

func TestOverlayRegistryDoesNotMutateBaseRegistry(t *testing.T) {
	t.Parallel()

	base := NewRegistry()
	base.Register(overlayTestProvider{name: "base"})

	overlay := NewOverlayRegistry(base)
	overlay.Register(overlayTestProvider{name: "ext"})

	if _, err := base.Get("ext"); err == nil {
		t.Fatal("expected base registry to remain unchanged")
	}
}

func TestActivateOverlayBuildsAliasedReviewProvider(t *testing.T) {
	restore, err := ActivateOverlay([]OverlayEntry{{Name: "ext-review", Command: "base"}})
	if err != nil {
		t.Fatalf("activate review overlay: %v", err)
	}
	defer restore()

	base := NewRegistry()
	base.Register(overlayTestProvider{name: "base"})

	registry := ResolveRegistry(base)
	provider, err := registry.Get("ext-review")
	if err != nil {
		t.Fatalf("resolve overlay provider: %v", err)
	}
	if got := provider.Name(); got != "ext-review" {
		t.Fatalf("unexpected overlay provider name: %q", got)
	}

	if _, err := provider.FetchReviews(context.Background(), FetchRequest{}); err != nil {
		t.Fatalf("delegate overlay fetch: %v", err)
	}
}

func TestResolveRegistryReturnsBaseWhenNoOverlayIsActive(t *testing.T) {
	t.Parallel()

	base := NewRegistry()
	base.Register(overlayTestProvider{name: "base"})

	resolved := ResolveRegistry(base)
	if resolved != base {
		t.Fatal("expected resolve registry to return the base registry when no overlay is active")
	}
}

func TestAliasedProviderResolveIssuesDelegatesToTarget(t *testing.T) {
	restore, err := ActivateOverlay([]OverlayEntry{{Name: "ext-review", Command: "base"}})
	if err != nil {
		t.Fatalf("activate review overlay: %v", err)
	}
	defer restore()

	base := NewRegistry()
	base.Register(&overlayTestProvider{name: "base"})

	registry := ResolveRegistry(base)
	resolved, err := registry.Get("ext-review")
	if err != nil {
		t.Fatalf("resolve overlay provider: %v", err)
	}
	if err := resolved.ResolveIssues(context.Background(), "123", nil); err != nil {
		t.Fatalf("delegate overlay resolve issues: %v", err)
	}
}

func TestAliasedProviderRejectsInvalidTargets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		prov    *aliasedProvider
		wantErr string
	}{
		{
			name:    "missing target",
			prov:    &aliasedProvider{name: "ext-review", registry: NewRegistry()},
			wantErr: `missing a target provider name`,
		},
		{
			name:    "self target",
			prov:    &aliasedProvider{name: "ext-review", targetName: "ext-review", registry: NewRegistry()},
			wantErr: `cannot target itself`,
		},
		{
			name:    "nil provider",
			prov:    nil,
			wantErr: `declared review provider is nil`,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := tc.prov.resolveTarget(nil)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestAliasedProviderRejectsAliasCycle(t *testing.T) {
	base := NewRegistry()
	overlay := NewOverlayRegistry(base)
	first := &aliasedProvider{name: "first", targetName: "second", registry: overlay}
	second := &aliasedProvider{name: "second", targetName: "first", registry: overlay}
	overlay.Register(first)
	overlay.Register(second)

	_, err := first.resolveTarget(nil)
	if err == nil || !strings.Contains(err.Error(), `alias cycle`) {
		t.Fatalf("expected alias cycle error, got %v", err)
	}
}

func TestActivateOverlayBuildsExtensionBackedReviewProvider(t *testing.T) {
	base := NewRegistry()
	base.Register(overlayTestProvider{name: "base", displayName: "Base Provider"})

	bridge := &overlayTestBridge{}
	restore, err := ActivateOverlay([]OverlayEntry{{
		Name:        "ext-review",
		Kind:        OverlayKindExtension,
		DisplayName: "Extension Review",
		Bridge:      bridge,
	}})
	if err != nil {
		t.Fatalf("activate extension review overlay: %v", err)
	}
	defer restore()

	registry := ResolveRegistry(base)
	resolved, err := registry.Get("ext-review")
	if err != nil {
		t.Fatalf("resolve extension review provider: %v", err)
	}
	if got := resolved.Name(); got != "ext-review" {
		t.Fatalf("resolved.Name() = %q, want %q", got, "ext-review")
	}

	items, err := resolved.FetchReviews(context.Background(), FetchRequest{
		PR:              "123",
		IncludeNitpicks: true,
	})
	if err != nil {
		t.Fatalf("FetchReviews() error = %v", err)
	}
	if len(items) != 1 || items[0].Title != "bridge-item" {
		t.Fatalf("FetchReviews() = %#v, want bridged review item", items)
	}
	if bridge.fetchProvider != "ext-review" {
		t.Fatalf("bridge fetch provider = %q, want %q", bridge.fetchProvider, "ext-review")
	}
	if bridge.fetchRequest.PR != "123" || !bridge.fetchRequest.IncludeNitpicks {
		t.Fatalf("bridge fetch request = %#v, want propagated request", bridge.fetchRequest)
	}

	err = resolved.ResolveIssues(context.Background(), "123", []ResolvedIssue{{
		FilePath:    "issue_001.md",
		ProviderRef: "thread-1",
	}})
	if err != nil {
		t.Fatalf("ResolveIssues() error = %v", err)
	}
	if bridge.resolvePR != "123" {
		t.Fatalf("bridge resolve PR = %q, want %q", bridge.resolvePR, "123")
	}
	if len(bridge.resolveIssues) != 1 || bridge.resolveIssues[0].FilePath != "issue_001.md" {
		t.Fatalf("bridge resolve issues = %#v, want propagated issues", bridge.resolveIssues)
	}

	catalog := Catalog(base)
	if len(catalog) != 2 {
		t.Fatalf("Catalog() len = %d, want 2", len(catalog))
	}
	found := false
	for _, entry := range catalog {
		if entry.Name == "ext-review" {
			found = true
			if entry.DisplayName != "Extension Review" {
				t.Fatalf("catalog display name = %q, want %q", entry.DisplayName, "Extension Review")
			}
		}
	}
	if !found {
		t.Fatalf("Catalog() missing extension-backed provider: %#v", catalog)
	}
}

func TestActivateOverlayRejectsExtensionBackedProviderWithoutBridge(t *testing.T) {
	_, err := ActivateOverlay([]OverlayEntry{{
		Name: "ext-review",
		Kind: OverlayKindExtension,
	}})
	if err == nil || !strings.Contains(err.Error(), "missing extension bridge") {
		t.Fatalf("expected missing bridge error, got %v", err)
	}
}
