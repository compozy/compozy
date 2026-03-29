package reviews

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/looper/internal/looper/model"
	"github.com/compozy/looper/internal/looper/prompt"
	"github.com/compozy/looper/internal/looper/provider"
)

func TestWriteRoundAndReadBackEntries(t *testing.T) {
	t.Parallel()

	reviewDir := filepath.Join(t.TempDir(), "tasks", "prd-demo", "reviews-001")
	meta := model.RoundMeta{
		Provider:  "coderabbit",
		PR:        "259",
		Round:     1,
		CreatedAt: time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC),
	}
	items := []provider.ReviewItem{
		{
			Title:       "Add nil check",
			File:        "internal/app/service.go",
			Line:        42,
			Author:      "coderabbitai[bot]",
			ProviderRef: "thread:PRT_1,comment:RC_1",
			Body:        "Please add a nil check before dereferencing the pointer.",
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
	if !strings.Contains(entries[0].Content, "## Status: pending") {
		t.Fatalf("expected issue file to start pending, got:\n%s", entries[0].Content)
	}

	ctx, err := prompt.ParseReviewContext(entries[0].Content)
	if err != nil {
		t.Fatalf("parse review context: %v", err)
	}
	if ctx.ProviderRef != "thread:PRT_1,comment:RC_1" {
		t.Fatalf("unexpected provider ref: %q", ctx.ProviderRef)
	}
}

func TestRefreshRoundMetaCountsResolvedIssues(t *testing.T) {
	t.Parallel()

	reviewDir := filepath.Join(t.TempDir(), "tasks", "prd-demo", "reviews-001")
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
	updated := strings.Replace(string(content), "## Status: pending", "## Status: resolved", 1)
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

func TestDiscoverRoundsAndNextRound(t *testing.T) {
	t.Parallel()

	prdDir := filepath.Join(t.TempDir(), "tasks", "prd-demo")
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
