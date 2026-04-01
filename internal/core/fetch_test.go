package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/provider"
)

type stubReviewProvider struct {
	name  string
	items []provider.ReviewItem
}

func (s stubReviewProvider) Name() string { return s.name }

func (s stubReviewProvider) FetchReviews(context.Context, string) ([]provider.ReviewItem, error) {
	return append([]provider.ReviewItem(nil), s.items...), nil
}

func (s stubReviewProvider) ResolveIssues(context.Context, string, []provider.ResolvedIssue) error {
	return nil
}

func TestFetchReviewsWritesRoundFiles(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	prdDir := filepath.Join(tmpDir, ".compozy", "tasks", "demo")
	if err := os.MkdirAll(prdDir, 0o755); err != nil {
		t.Fatalf("mkdir prd dir: %v", err)
	}

	restore := defaultProviderRegistry
	defaultProviderRegistry = func() *provider.Registry {
		registry := provider.NewRegistry()
		registry.Register(stubReviewProvider{
			name: "stub",
			items: []provider.ReviewItem{
				{
					Title:       "Add nil check",
					File:        "internal/app/service.go",
					Line:        42,
					Author:      "review-bot",
					ProviderRef: "thread:PRT_1,comment:RC_1",
					Body:        "Please add a nil check here.",
				},
			},
		})
		return registry
	}
	t.Cleanup(func() { defaultProviderRegistry = restore })

	result, err := fetchReviews(context.Background(), &model.RuntimeConfig{
		Name:     "demo",
		Provider: "stub",
		PR:       "259",
	})
	if err != nil {
		t.Fatalf("fetch reviews: %v", err)
	}
	if result.Round != 1 {
		t.Fatalf("expected round 1, got %d", result.Round)
	}
	if !strings.HasSuffix(result.ReviewsDir, filepath.Join(".compozy", "tasks", "demo", "reviews-001")) {
		t.Fatalf("unexpected reviews dir: %q", result.ReviewsDir)
	}
	if _, err := os.Stat(filepath.Join(result.ReviewsDir, "_meta.md")); err != nil {
		t.Fatalf("expected meta file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(result.ReviewsDir, "issue_001.md")); err != nil {
		t.Fatalf("expected issue file: %v", err)
	}
}

func TestFetchReviewsAutoIncrementsRound(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	prdDir := filepath.Join(tmpDir, ".compozy", "tasks", "demo")
	if err := os.MkdirAll(filepath.Join(prdDir, "reviews-001"), 0o755); err != nil {
		t.Fatalf("mkdir round dir: %v", err)
	}

	restore := defaultProviderRegistry
	defaultProviderRegistry = func() *provider.Registry {
		registry := provider.NewRegistry()
		registry.Register(stubReviewProvider{name: "stub"})
		return registry
	}
	t.Cleanup(func() { defaultProviderRegistry = restore })

	result, err := fetchReviews(context.Background(), &model.RuntimeConfig{
		Name:     "demo",
		Provider: "stub",
		PR:       "259",
	})
	if err != nil {
		t.Fatalf("fetch reviews: %v", err)
	}
	if result.Round != 2 {
		t.Fatalf("expected auto-incremented round 2, got %d", result.Round)
	}
}
