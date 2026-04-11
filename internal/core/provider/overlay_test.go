package provider

import (
	"context"
	"strings"
	"testing"
)

type overlayTestProvider struct {
	name string
}

func (p overlayTestProvider) Name() string { return p.name }

func (overlayTestProvider) FetchReviews(context.Context, FetchRequest) ([]ReviewItem, error) {
	return nil, nil
}

func (overlayTestProvider) ResolveIssues(context.Context, string, []ResolvedIssue) error {
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
