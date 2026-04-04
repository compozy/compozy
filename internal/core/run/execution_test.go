package run

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/provider"
	"github.com/compozy/compozy/internal/core/reviews"
	"github.com/compozy/compozy/internal/core/tasks"
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
	reviewDir := filepath.Join(tmpDir, ".compozy", "tasks", "demo", "reviews-001")
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
	updated := strings.Replace(string(content), "status: pending", "status: resolved", 1)
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

func TestAfterJobSuccessSkipsProviderResolutionWithoutProviderRefs(t *testing.T) {
	tmpDir := t.TempDir()
	reviewDir := filepath.Join(tmpDir, ".compozy", "tasks", "demo", "reviews-001")
	if err := reviews.WriteRound(reviewDir, model.RoundMeta{
		Provider:  "stub",
		PR:        "259",
		Round:     1,
		CreatedAt: time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC),
	}, []provider.ReviewItem{
		{
			Title:  "Resolved local-only issue",
			File:   "internal/app/service.go",
			Line:   42,
			Author: "review-bot",
			Body:   "This review has no provider thread reference.",
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
	resolvedContent := strings.Replace(string(content), "status: pending", "status: resolved", 1)
	if err := os.WriteFile(issuePath, []byte(resolvedContent), 0o600); err != nil {
		t.Fatalf("write resolved issue file: %v", err)
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

	if len(resolver.issues) != 0 {
		t.Fatalf("expected no provider-backed issues to be resolved, got %d", len(resolver.issues))
	}

	meta, err := reviews.ReadRoundMeta(reviewDir)
	if err != nil {
		t.Fatalf("read round meta: %v", err)
	}
	if meta.Resolved != 1 || meta.Unresolved != 0 {
		t.Fatalf("unexpected refreshed meta: %#v", meta)
	}
}

func TestAfterJobSuccessAllowsRoundMetaWithoutPR(t *testing.T) {
	tmpDir := t.TempDir()
	reviewDir := filepath.Join(tmpDir, ".compozy", "tasks", "demo", "reviews-001")
	if err := reviews.WriteRound(reviewDir, model.RoundMeta{
		Provider:  "stub",
		PR:        "",
		Round:     1,
		CreatedAt: time.Date(2026, 3, 31, 12, 0, 0, 0, time.UTC),
	}, []provider.ReviewItem{
		{
			Title:       "Keep metadata refresh working without PR",
			File:        "internal/app/service.go",
			Line:        42,
			Author:      "review-bot",
			ProviderRef: "thread:PRT_1,comment:RC_1",
			Body:        "This issue should still resolve when round metadata omits pr.",
		},
	}); err != nil {
		t.Fatalf("write round: %v", err)
	}

	metaPath := reviews.MetaPath(reviewDir)
	metaContent, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read meta: %v", err)
	}
	withoutPR := strings.Replace(string(metaContent), "pr: \n", "", 1)
	if err := os.WriteFile(metaPath, []byte(withoutPR), 0o600); err != nil {
		t.Fatalf("rewrite meta without pr: %v", err)
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
	resolvedContent := strings.Replace(string(content), "status: pending", "status: resolved", 1)
	if err := os.WriteFile(issuePath, []byte(resolvedContent), 0o600); err != nil {
		t.Fatalf("write resolved issue file: %v", err)
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
			pr:         "",
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
	if meta.PR != "" {
		t.Fatalf("expected empty pr after refresh, got %q", meta.PR)
	}
	if meta.Resolved != 1 || meta.Unresolved != 0 {
		t.Fatalf("unexpected refreshed meta: %#v", meta)
	}
}

func TestAfterJobSuccessRefreshesTaskMetaForPRDTasks(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, ".compozy", "tasks", "demo")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir tasks dir: %v", err)
	}
	writeRunTaskFile(t, tasksDir, "task_01.md", "pending")
	if _, err := tasks.RefreshTaskMeta(tasksDir); err != nil {
		t.Fatalf("refresh initial task meta: %v", err)
	}

	taskPath := filepath.Join(tasksDir, "task_01.md")
	content, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("read task file: %v", err)
	}

	execCtx := &jobExecutionContext{
		cfg: &config{
			mode:     model.ExecutionModePRDTasks,
			tasksDir: tasksDir,
		},
	}
	if err := execCtx.afterJobSuccess(context.Background(), &job{
		groups: map[string][]model.IssueEntry{
			"task_01": {{
				Name:     "task_01.md",
				AbsPath:  taskPath,
				Content:  string(content),
				CodeFile: "task_01",
			}},
		},
	}); err != nil {
		t.Fatalf("afterJobSuccess: %v", err)
	}

	updatedTask, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("read updated task file: %v", err)
	}
	if !strings.Contains(string(updatedTask), "status: completed") {
		t.Fatalf("expected updated task file to be completed, got:\n%s", string(updatedTask))
	}

	meta, err := tasks.ReadTaskMeta(tasksDir)
	if err != nil {
		t.Fatalf("read task meta: %v", err)
	}
	if meta.Total != 1 || meta.Completed != 1 || meta.Pending != 0 {
		t.Fatalf("unexpected refreshed task meta: %#v", meta)
	}
}

func TestAfterJobSuccessFinalizesTriagedIssuesAndRefreshesMeta(t *testing.T) {
	tmpDir := t.TempDir()
	reviewDir := filepath.Join(tmpDir, ".compozy", "tasks", "demo", "reviews-001")
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

	issuePath := entries[0].AbsPath
	content, err := os.ReadFile(issuePath)
	if err != nil {
		t.Fatalf("read issue file: %v", err)
	}
	triagedContent := strings.Replace(string(content), "status: pending", "status: valid", 1)
	if err := os.WriteFile(issuePath, []byte(triagedContent), 0o600); err != nil {
		t.Fatalf("write triaged issue file: %v", err)
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

	updatedIssue, err := os.ReadFile(issuePath)
	if err != nil {
		t.Fatalf("read updated issue: %v", err)
	}
	if !strings.Contains(string(updatedIssue), "status: resolved") {
		t.Fatalf("expected triaged issue to be finalized as resolved, got:\n%s", string(updatedIssue))
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

func TestAfterJobSuccessFailsWhenReviewIssueRemainsPending(t *testing.T) {
	tmpDir := t.TempDir()
	reviewDir := filepath.Join(tmpDir, ".compozy", "tasks", "demo", "reviews-001")
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

	execCtx := &jobExecutionContext{
		cfg: &config{
			mode:       model.ExecutionModePRReview,
			provider:   "stub",
			pr:         "259",
			reviewsDir: reviewDir,
		},
	}
	err = execCtx.afterJobSuccess(context.Background(), &job{
		groups: map[string][]model.IssueEntry{
			entries[0].CodeFile: {entries[0]},
		},
	})
	if err == nil {
		t.Fatal("expected pending review issue to fail post-success hook")
	}
	if !strings.Contains(err.Error(), "remained pending") {
		t.Fatalf("expected pending issue error, got %v", err)
	}

	meta, err := reviews.ReadRoundMeta(reviewDir)
	if err != nil {
		t.Fatalf("read round meta: %v", err)
	}
	if meta.Resolved != 0 || meta.Unresolved != 1 {
		t.Fatalf("expected round meta to remain unresolved after failure, got %#v", meta)
	}
}

func TestRefreshTaskMetaOnExitUpdatesAggregateCounts(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, ".compozy", "tasks", "demo")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir tasks dir: %v", err)
	}
	writeRunTaskFile(t, tasksDir, "task_01.md", "pending")
	if err := tasks.WriteTaskMeta(tasksDir, model.TaskMeta{
		CreatedAt: time.Date(2026, 3, 31, 10, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 3, 31, 10, 5, 0, 0, time.UTC),
		Total:     1,
		Completed: 1,
		Pending:   0,
	}); err != nil {
		t.Fatalf("write stale task meta: %v", err)
	}

	refreshTaskMetaOnExit(&config{
		mode:     model.ExecutionModePRDTasks,
		tasksDir: tasksDir,
	})

	meta, err := tasks.ReadTaskMeta(tasksDir)
	if err != nil {
		t.Fatalf("read task meta: %v", err)
	}
	if meta.Total != 1 || meta.Completed != 0 || meta.Pending != 1 {
		t.Fatalf("unexpected exit-refreshed task meta: %#v", meta)
	}
}

func writeRunTaskFile(t *testing.T, tasksDir, name, status string) {
	t.Helper()

	content := strings.Join([]string{
		"---",
		"status: " + status,
		"title: " + name,
		"type: backend",
		"complexity: low",
		"---",
		"",
		"# " + name,
		"",
	}, "\n")

	if err := os.WriteFile(filepath.Join(tasksDir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
