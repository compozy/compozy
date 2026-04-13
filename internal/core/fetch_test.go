package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/provider"
	"github.com/compozy/compozy/internal/core/reviews"
)

type stubReviewProvider struct {
	name  string
	items []provider.ReviewItem
}

var fetchReviewProviderRegistryMu sync.Mutex

func (s stubReviewProvider) Name() string { return s.name }

func (s stubReviewProvider) FetchReviews(context.Context, provider.FetchRequest) ([]provider.ReviewItem, error) {
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

func TestFetchReviewsReviewBodyCommentHistoryFiltering(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		historicalItem  provider.ReviewItem
		historicalState string
		fetchedItems    []provider.ReviewItem
		wantTotal       int
	}{
		{
			name: "filter resolved review body comments when the fetched review is older",
			historicalItem: provider.ReviewItem{
				Title:                   "Keep helper reuse consistent",
				File:                    "internal/app/service.go",
				Line:                    42,
				Severity:                "nitpick",
				Author:                  "coderabbitai[bot]",
				Body:                    "Use the existing helper instead of duplicating logic.",
				ReviewHash:              "hash-stale",
				SourceReviewID:          "4002",
				SourceReviewSubmittedAt: "2026-04-10T10:30:00Z",
			},
			historicalState: "resolved",
			fetchedItems: []provider.ReviewItem{
				{
					Title:                   "Keep helper reuse consistent",
					File:                    "internal/app/service.go",
					Line:                    42,
					Severity:                "nitpick",
					Author:                  "coderabbitai[bot]",
					Body:                    "Use the existing helper instead of duplicating logic.",
					ReviewHash:              "hash-stale",
					SourceReviewID:          "4003",
					SourceReviewSubmittedAt: "2026-04-10T10:00:00Z",
				},
				{
					Title:       "Add nil check",
					File:        "internal/app/service.go",
					Line:        18,
					Author:      "coderabbitai[bot]",
					Body:        "Please add a nil check before dereferencing the pointer.",
					ProviderRef: "thread:PRT_1,comment:RC_1",
				},
			},
			wantTotal: 1,
		},
		{
			name: "re-import unresolved review body comment hashes",
			historicalItem: provider.ReviewItem{
				Title:                   "Keep helper reuse consistent",
				File:                    "internal/app/service.go",
				Line:                    42,
				Severity:                "nitpick",
				Author:                  "coderabbitai[bot]",
				Body:                    "Use the existing helper instead of duplicating logic.",
				ReviewHash:              "hash-open",
				SourceReviewID:          "4001",
				SourceReviewSubmittedAt: "2026-04-10T10:00:00Z",
			},
			historicalState: "pending",
			fetchedItems: []provider.ReviewItem{
				{
					Title:                   "Keep helper reuse consistent",
					File:                    "internal/app/service.go",
					Line:                    42,
					Severity:                "nitpick",
					Author:                  "coderabbitai[bot]",
					Body:                    "Use the existing helper instead of duplicating logic.",
					ReviewHash:              "hash-open",
					SourceReviewID:          "4002",
					SourceReviewSubmittedAt: "2026-04-10T10:05:00Z",
				},
			},
			wantTotal: 1,
		},
		{
			name: "re-import resolved review body comments when the fetched review has a newer timestamp",
			historicalItem: provider.ReviewItem{
				Title:                   "Keep helper reuse consistent",
				File:                    "internal/app/service.go",
				Line:                    42,
				Severity:                "nitpick",
				Author:                  "coderabbitai[bot]",
				Body:                    "Use the existing helper instead of duplicating logic.",
				ReviewHash:              "hash-returned",
				SourceReviewID:          "4001",
				SourceReviewSubmittedAt: "2026-04-10T10:00:00Z",
			},
			historicalState: "resolved",
			fetchedItems: []provider.ReviewItem{
				{
					Title:                   "Keep helper reuse consistent",
					File:                    "internal/app/service.go",
					Line:                    42,
					Severity:                "nitpick",
					Author:                  "coderabbitai[bot]",
					Body:                    "Use the existing helper instead of duplicating logic.",
					ReviewHash:              "hash-returned",
					SourceReviewID:          "4002",
					SourceReviewSubmittedAt: "2026-04-10T10:30:00Z",
				},
			},
			wantTotal: 1,
		},
		{
			name: "re-import resolved review body comments when the fetched review has the same timestamp but a newer review id",
			historicalItem: provider.ReviewItem{
				Title:                   "Keep helper reuse consistent",
				File:                    "internal/app/service.go",
				Line:                    42,
				Severity:                "nitpick",
				Author:                  "coderabbitai[bot]",
				Body:                    "Use the existing helper instead of duplicating logic.",
				ReviewHash:              "hash-same-second",
				SourceReviewID:          "4001",
				SourceReviewSubmittedAt: "2026-04-10T10:00:00Z",
			},
			historicalState: "resolved",
			fetchedItems: []provider.ReviewItem{
				{
					Title:                   "Keep helper reuse consistent",
					File:                    "internal/app/service.go",
					Line:                    42,
					Severity:                "nitpick",
					Author:                  "coderabbitai[bot]",
					Body:                    "Use the existing helper instead of duplicating logic.",
					ReviewHash:              "hash-same-second",
					SourceReviewID:          "4002",
					SourceReviewSubmittedAt: "2026-04-10T10:00:00Z",
				},
			},
			wantTotal: 1,
		},
		{
			name: "re-import same hash when severity changes but the fetched review is newer",
			historicalItem: provider.ReviewItem{
				Title:                   "Keep helper reuse consistent",
				File:                    "internal/app/service.go",
				Line:                    42,
				Severity:                "major",
				Author:                  "coderabbitai[bot]",
				Body:                    "Use the existing helper instead of duplicating logic.",
				ReviewHash:              "hash-severity-changed",
				SourceReviewID:          "4001",
				SourceReviewSubmittedAt: "2026-04-10T10:00:00Z",
			},
			historicalState: "resolved",
			fetchedItems: []provider.ReviewItem{
				{
					Title:                   "Keep helper reuse consistent",
					File:                    "internal/app/service.go",
					Line:                    42,
					Severity:                "minor",
					Author:                  "coderabbitai[bot]",
					Body:                    "Use the existing helper instead of duplicating logic.",
					ReviewHash:              "hash-severity-changed",
					SourceReviewID:          "4002",
					SourceReviewSubmittedAt: "2026-04-10T10:30:00Z",
				},
			},
			wantTotal: 1,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("Should "+tc.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			prdDir := filepath.Join(tmpDir, ".compozy", "tasks", "demo")
			if err := os.MkdirAll(prdDir, 0o755); err != nil {
				t.Fatalf("mkdir prd dir: %v", err)
			}
			writeHistoricalReviewBodyCommentRound(t, prdDir, 1, tc.historicalItem, tc.historicalState == "resolved")
			installStubReviewProviderRegistry(t, tc.fetchedItems)

			result, err := fetchReviews(context.Background(), &model.RuntimeConfig{
				Name:          "demo",
				Provider:      "stub",
				PR:            "259",
				WorkspaceRoot: tmpDir,
			})
			if err != nil {
				t.Fatalf("fetch reviews: %v", err)
			}
			if result.Total != tc.wantTotal {
				t.Fatalf("unexpected fetched total: got %d, want %d", result.Total, tc.wantTotal)
			}
		})
	}
}

func installStubReviewProviderRegistry(t *testing.T, items []provider.ReviewItem) {
	t.Helper()

	fetchReviewProviderRegistryMu.Lock()

	restore := defaultProviderRegistry
	defaultProviderRegistry = func() *provider.Registry {
		registry := provider.NewRegistry()
		registry.Register(stubReviewProvider{
			name:  "stub",
			items: items,
		})
		return registry
	}

	t.Cleanup(func() {
		defaultProviderRegistry = restore
		fetchReviewProviderRegistryMu.Unlock()
	})
}

func writeHistoricalReviewBodyCommentRound(
	t *testing.T,
	prdDir string,
	round int,
	item provider.ReviewItem,
	resolved bool,
) {
	t.Helper()

	reviewDir := reviews.ReviewDirectory(prdDir, round)
	if err := reviews.WriteRound(reviewDir, model.RoundMeta{
		Provider:  "stub",
		PR:        "259",
		Round:     round,
		CreatedAt: time.Date(2026, 4, 10, 10, 0, 0, 0, time.UTC),
	}, []provider.ReviewItem{item}); err != nil {
		t.Fatalf("write historical round: %v", err)
	}

	if !resolved {
		return
	}

	issuePath := filepath.Join(reviewDir, "issue_001.md")
	content, err := os.ReadFile(issuePath)
	if err != nil {
		t.Fatalf("read issue file: %v", err)
	}
	updated := strings.Replace(string(content), "status: pending", "status: resolved", 1)
	if err := os.WriteFile(issuePath, []byte(updated), 0o600); err != nil {
		t.Fatalf("write resolved issue file: %v", err)
	}
}
