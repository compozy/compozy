package provider

import (
	"context"
	"testing"
)

type stubProvider struct {
	name string
}

func (s stubProvider) Name() string { return s.name }

func (s stubProvider) FetchReviews(context.Context, string) ([]ReviewItem, error) {
	return nil, nil
}

func (s stubProvider) ResolveIssues(context.Context, string, []ResolvedIssue) error {
	return nil
}

func TestRegistryRegisterAndGet(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	registry.Register(stubProvider{name: "example"})

	got, err := registry.Get("example")
	if err != nil {
		t.Fatalf("get provider: %v", err)
	}
	if got.Name() != "example" {
		t.Fatalf("unexpected provider name: %q", got.Name())
	}
}

func TestRegistryGetMissingProvider(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	if _, err := registry.Get("missing"); err == nil {
		t.Fatal("expected missing provider error")
	}
}
