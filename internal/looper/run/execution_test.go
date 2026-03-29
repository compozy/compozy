package run

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/looper/internal/looper/model"
	"github.com/compozy/looper/internal/looper/provider"
	"github.com/compozy/looper/internal/looper/reviews"
)

type stubResolverProvider struct {
	name   string
	issues []provider.ResolvedIssue
}

func (s *stubResolverProvider) Name() string { return s.name }

func (s *stubResolverProvider) FetchReviews(context.Context, string) ([]provider.ReviewItem, error) {
	return nil, nil
}

func (s *stubResolverProvider) ResolveIssues(_ context.Context, _ string, issues []provider.ResolvedIssue) error {
	s.issues = append(s.issues, issues...)
	return nil
}

func TestAfterJobSuccessResolvesNewlyResolvedIssuesAndRefreshesMeta(t *testing.T) {
	tmpDir := t.TempDir()
	reviewDir := filepath.Join(tmpDir, "tasks", "prd-demo", "reviews-001")
	if err := reviews.WriteRound(reviewDir, model.RoundMeta{
		Provider:  "stub",
		PR:        "259",
		Round:     1,
		CreatedAt: time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC),
	}, []provider.ReviewItem{
		{
			Title:       "Add nil check",
			File:        "internal/app/service.go",
			Line:        42,
			Author:      "review-bot",
			ProviderRef: "thread:PRT_1,comment:RC_1",
			Body:        "Please add a nil check.",
		},
	}); err != nil {
		t.Fatalf("write round: %v", err)
	}

	entries, err := reviews.ReadReviewEntries(reviewDir)
	if err != nil {
		t.Fatalf("read review entries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	issuePath := filepath.Join(reviewDir, "issue_001.md")
	content, err := os.ReadFile(issuePath)
	if err != nil {
		t.Fatalf("read issue file: %v", err)
	}
	updated := strings.Replace(string(content), "## Status: pending", "## Status: resolved", 1)
	if err := os.WriteFile(issuePath, []byte(updated), 0o600); err != nil {
		t.Fatalf("write issue file: %v", err)
	}

	resolver := &stubResolverProvider{name: "stub"}
	restore := reviewProviderRegistry
	reviewProviderRegistry = func() *provider.Registry {
		registry := provider.NewRegistry()
		registry.Register(resolver)
		return registry
	}
	defer func() { reviewProviderRegistry = restore }()

	execCtx := &jobExecutionContext{
		cfg: &config{
			mode:       model.ExecutionModePRReview,
			provider:   "stub",
			pr:         "259",
			reviewsDir: reviewDir,
		},
	}
	if err := execCtx.afterJobSuccess(context.Background(), &job{
		groups: map[string][]model.IssueEntry{
			entries[0].CodeFile: {entries[0]},
		},
	}); err != nil {
		t.Fatalf("afterJobSuccess: %v", err)
	}

	if len(resolver.issues) != 1 {
		t.Fatalf("expected 1 resolved issue sent to provider, got %d", len(resolver.issues))
	}

	meta, err := reviews.ReadRoundMeta(reviewDir)
	if err != nil {
		t.Fatalf("read round meta: %v", err)
	}
	if meta.Resolved != 1 || meta.Unresolved != 0 {
		t.Fatalf("unexpected refreshed meta: %#v", meta)
	}
}
