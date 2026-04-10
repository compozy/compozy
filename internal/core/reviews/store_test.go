package reviews

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/provider"
)

func TestReadRoundMetaAllowsOptionalPR(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		content string
	}{
		{
			name: "empty pr field",
			content: strings.Join([]string{
				"---",
				"provider: coderabbit",
				"pr:",
				"round: 1",
				"created_at: 2026-03-28T10:00:00Z",
				"---",
				"",
				"## Summary",
				"- Total: 1",
				"- Resolved: 0",
				"- Unresolved: 1",
				"",
			}, "\n"),
		},
		{
			name: "missing pr field",
			content: strings.Join([]string{
				"---",
				"provider: coderabbit",
				"round: 1",
				"created_at: 2026-03-28T10:00:00Z",
				"---",
				"",
				"## Summary",
				"- Total: 1",
				"- Resolved: 0",
				"- Unresolved: 1",
				"",
			}, "\n"),
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			reviewDir := t.TempDir()
			if err := os.WriteFile(MetaPath(reviewDir), []byte(tc.content), 0o600); err != nil {
				t.Fatalf("write meta: %v", err)
			}

			meta, err := ReadRoundMeta(reviewDir)
			if err != nil {
				t.Fatalf("read round meta: %v", err)
			}
			if meta.Provider != "coderabbit" {
				t.Fatalf("unexpected provider: %q", meta.Provider)
			}
			if meta.PR != "" {
				t.Fatalf("expected empty pr, got %q", meta.PR)
			}
			if meta.Round != 1 {
				t.Fatalf("unexpected round: %d", meta.Round)
			}
			if !meta.CreatedAt.Equal(createdAt) {
				t.Fatalf("unexpected created_at: %s", meta.CreatedAt.Format(time.RFC3339))
			}
			if meta.Total != 1 || meta.Resolved != 0 || meta.Unresolved != 1 {
				t.Fatalf("unexpected counts: %#v", meta)
			}
		})
	}
}

func TestWriteRoundAndReadBackEntries(t *testing.T) {
	t.Parallel()

	reviewDir := filepath.Join(t.TempDir(), ".compozy", "tasks", "demo", "reviews-001")
	meta := model.RoundMeta{
		Provider:  "coderabbit",
		PR:        "259",
		Round:     1,
		CreatedAt: time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC),
	}
	items := []provider.ReviewItem{
		{
			Title:                   "Add nil check",
			File:                    "internal/app/service.go",
			Line:                    42,
			Author:                  "coderabbitai[bot]",
			ProviderRef:             "thread:PRT_1,comment:RC_1",
			Body:                    "Please add a nil check before dereferencing the pointer.",
			ReviewHash:              "abc123def456",
			SourceReviewID:          "4089982130",
			SourceReviewSubmittedAt: "2026-04-10T13:33:25Z",
		},
	}

	if err := WriteRound(reviewDir, meta, items); err != nil {
		t.Fatalf("write round: %v", err)
	}

	readMeta, err := ReadRoundMeta(reviewDir)
	if err != nil {
		t.Fatalf("read round meta: %v", err)
	}
	if readMeta.Total != 1 || readMeta.Resolved != 0 || readMeta.Unresolved != 1 {
		t.Fatalf("unexpected counts after write: %#v", readMeta)
	}

	entries, err := ReadReviewEntries(reviewDir)
	if err != nil {
		t.Fatalf("read review entries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 review entry, got %d", len(entries))
	}
	if entries[0].CodeFile != "internal/app/service.go" {
		t.Fatalf("unexpected code file: %q", entries[0].CodeFile)
	}
	if !strings.Contains(entries[0].Content, "status: pending") {
		t.Fatalf("expected issue file to start pending, got:\n%s", entries[0].Content)
	}

	ctx, err := ParseReviewContext(entries[0].Content)
	if err != nil {
		t.Fatalf("parse review context: %v", err)
	}
	if ctx.ProviderRef != "thread:PRT_1,comment:RC_1" {
		t.Fatalf("unexpected provider ref: %q", ctx.ProviderRef)
	}
	if ctx.ReviewHash != "abc123def456" {
		t.Fatalf("unexpected review hash: %q", ctx.ReviewHash)
	}
	if ctx.SourceReviewID != "4089982130" {
		t.Fatalf("unexpected source review id: %q", ctx.SourceReviewID)
	}
	if ctx.SourceReviewSubmittedAt != "2026-04-10T13:33:25Z" {
		t.Fatalf("unexpected source review submitted_at: %q", ctx.SourceReviewSubmittedAt)
	}
}

func TestRefreshRoundMetaCountsResolvedIssues(t *testing.T) {
	t.Parallel()

	reviewDir := filepath.Join(t.TempDir(), ".compozy", "tasks", "demo", "reviews-001")
	if err := WriteRound(reviewDir, model.RoundMeta{
		Provider:  "coderabbit",
		PR:        "259",
		Round:     1,
		CreatedAt: time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC),
	}, []provider.ReviewItem{
		{
			Title:       "Add nil check",
			File:        "internal/app/service.go",
			Line:        42,
			Author:      "coderabbitai[bot]",
			ProviderRef: "thread:PRT_1,comment:RC_1",
			Body:        "Please add a nil check before dereferencing the pointer.",
		},
	}); err != nil {
		t.Fatalf("write round: %v", err)
	}

	issuePath := filepath.Join(reviewDir, "issue_001.md")
	content, err := os.ReadFile(issuePath)
	if err != nil {
		t.Fatalf("read issue file: %v", err)
	}
	updated := strings.Replace(string(content), "status: pending", "status: resolved", 1)
	if err := os.WriteFile(issuePath, []byte(updated), 0o600); err != nil {
		t.Fatalf("write updated issue file: %v", err)
	}

	meta, err := RefreshRoundMeta(reviewDir)
	if err != nil {
		t.Fatalf("refresh round meta: %v", err)
	}
	if meta.Resolved != 1 || meta.Unresolved != 0 {
		t.Fatalf("unexpected refreshed counts: %#v", meta)
	}
}

func TestRefreshRoundMetaAllowsOptionalPR(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		content string
	}{
		{
			name: "empty pr field",
			content: strings.Join([]string{
				"---",
				"provider: coderabbit",
				"pr:",
				"round: 1",
				"created_at: 2026-03-28T10:00:00Z",
				"---",
				"",
				"## Summary",
				"- Total: 0",
				"- Resolved: 0",
				"- Unresolved: 0",
				"",
			}, "\n"),
		},
		{
			name: "missing pr field",
			content: strings.Join([]string{
				"---",
				"provider: coderabbit",
				"round: 1",
				"created_at: 2026-03-28T10:00:00Z",
				"---",
				"",
				"## Summary",
				"- Total: 0",
				"- Resolved: 0",
				"- Unresolved: 0",
				"",
			}, "\n"),
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			reviewDir := t.TempDir()
			if err := os.WriteFile(MetaPath(reviewDir), []byte(tc.content), 0o600); err != nil {
				t.Fatalf("write meta: %v", err)
			}
			if err := os.WriteFile(
				filepath.Join(reviewDir, "issue_001.md"),
				[]byte(strings.Join([]string{
					"---",
					"status: resolved",
					"file: internal/app/service.go",
					"line: 42",
					"author: review-bot",
					"---",
					"",
					"# Issue 001: Example",
					"",
				}, "\n")),
				0o600,
			); err != nil {
				t.Fatalf("write issue_001.md: %v", err)
			}
			if err := os.WriteFile(
				filepath.Join(reviewDir, "issue_002.md"),
				[]byte(strings.Join([]string{
					"---",
					"status: pending",
					"file: internal/app/service.go",
					"line: 43",
					"author: review-bot",
					"---",
					"",
					"# Issue 002: Example",
					"",
				}, "\n")),
				0o600,
			); err != nil {
				t.Fatalf("write issue_002.md: %v", err)
			}

			meta, err := RefreshRoundMeta(reviewDir)
			if err != nil {
				t.Fatalf("refresh round meta: %v", err)
			}
			if meta.PR != "" {
				t.Fatalf("expected empty pr, got %q", meta.PR)
			}
			if meta.Total != 2 || meta.Resolved != 1 || meta.Unresolved != 1 {
				t.Fatalf("unexpected refreshed counts: %#v", meta)
			}

			reloaded, err := ReadRoundMeta(reviewDir)
			if err != nil {
				t.Fatalf("read refreshed round meta: %v", err)
			}
			if reloaded.PR != "" {
				t.Fatalf("expected empty pr after refresh, got %q", reloaded.PR)
			}
			if reloaded.Total != 2 || reloaded.Resolved != 1 || reloaded.Unresolved != 1 {
				t.Fatalf("unexpected persisted counts: %#v", reloaded)
			}
		})
	}
}

func TestDiscoverRoundsAndNextRound(t *testing.T) {
	t.Parallel()

	prdDir := filepath.Join(t.TempDir(), ".compozy", "tasks", "demo")
	for _, round := range []int{1, 3, 2} {
		if err := os.MkdirAll(filepath.Join(prdDir, RoundDirName(round)), 0o755); err != nil {
			t.Fatalf("mkdir round %d: %v", round, err)
		}
	}

	rounds, err := DiscoverRounds(prdDir)
	if err != nil {
		t.Fatalf("discover rounds: %v", err)
	}
	if got := []int{rounds[0], rounds[1], rounds[2]}; !equalInts(got, []int{1, 2, 3}) {
		t.Fatalf("unexpected rounds: %#v", rounds)
	}

	latest, err := LatestRound(prdDir)
	if err != nil {
		t.Fatalf("latest round: %v", err)
	}
	if latest != 3 {
		t.Fatalf("expected latest round 3, got %d", latest)
	}

	next, err := NextRound(prdDir)
	if err != nil {
		t.Fatalf("next round: %v", err)
	}
	if next != 4 {
		t.Fatalf("expected next round 4, got %d", next)
	}
}

func TestFinalizeIssueStatusesResolvesTriagedIssues(t *testing.T) {
	t.Parallel()

	reviewDir := filepath.Join(t.TempDir(), ".compozy", "tasks", "demo", "reviews-001")
	if err := WriteRound(reviewDir, model.RoundMeta{
		Provider:  "coderabbit",
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

	entries, err := ReadReviewEntries(reviewDir)
	if err != nil {
		t.Fatalf("read entries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	issuePath := entries[0].AbsPath
	content, err := os.ReadFile(issuePath)
	if err != nil {
		t.Fatalf("read issue: %v", err)
	}
	triaged := strings.Replace(string(content), "status: pending", "status: valid", 1)
	if err := os.WriteFile(issuePath, []byte(triaged), 0o600); err != nil {
		t.Fatalf("write triaged issue: %v", err)
	}

	if err := FinalizeIssueStatuses(reviewDir, entries); err != nil {
		t.Fatalf("finalize issue statuses: %v", err)
	}

	rewritten, err := os.ReadFile(issuePath)
	if err != nil {
		t.Fatalf("read finalized issue: %v", err)
	}
	if !strings.Contains(string(rewritten), "status: resolved") {
		t.Fatalf("expected finalized issue to be resolved, got:\n%s", string(rewritten))
	}
}

func TestFinalizeIssueStatusesRejectsPendingIssues(t *testing.T) {
	t.Parallel()

	reviewDir := filepath.Join(t.TempDir(), ".compozy", "tasks", "demo", "reviews-001")
	if err := WriteRound(reviewDir, model.RoundMeta{
		Provider:  "coderabbit",
		PR:        "259",
		Round:     1,
		CreatedAt: time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC),
	}, []provider.ReviewItem{
		{
			Title:  "Add nil check",
			File:   "internal/app/service.go",
			Line:   42,
			Author: "review-bot",
			Body:   "Please add a nil check.",
		},
	}); err != nil {
		t.Fatalf("write round: %v", err)
	}

	entries, err := ReadReviewEntries(reviewDir)
	if err != nil {
		t.Fatalf("read entries: %v", err)
	}

	err = FinalizeIssueStatuses(reviewDir, entries)
	if err == nil {
		t.Fatal("expected pending issue finalization to fail")
	}
	if !strings.Contains(err.Error(), "remained pending") {
		t.Fatalf("expected pending issue error, got %v", err)
	}
}

func equalInts(left []int, right []int) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
