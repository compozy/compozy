package executor

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/provider"
	"github.com/compozy/compozy/internal/core/reviews"
	"github.com/compozy/compozy/internal/core/tasks"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

type stubResolverProvider struct {
	name       string
	issues     []provider.ResolvedIssue
	resolveErr error
}

func (s *stubResolverProvider) Name() string { return s.name }

func (s *stubResolverProvider) FetchReviews(context.Context, provider.FetchRequest) ([]provider.ReviewItem, error) {
	return nil, nil
}

func (s *stubResolverProvider) ResolveIssues(_ context.Context, _ string, issues []provider.ResolvedIssue) error {
	s.issues = append(s.issues, issues...)
	return s.resolveErr
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
		ctx: context.Background(),
		cfg: &config{
			Mode:       model.ExecutionModePRReview,
			Provider:   "stub",
			PR:         "259",
			ReviewsDir: reviewDir,
		},
	}
	if err := execCtx.afterJobSuccess(context.Background(), &job{
		Groups: map[string][]model.IssueEntry{
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
		ctx: context.Background(),
		cfg: &config{
			Mode:       model.ExecutionModePRReview,
			Provider:   "stub",
			PR:         "259",
			ReviewsDir: reviewDir,
		},
	}
	if err := execCtx.afterJobSuccess(context.Background(), &job{
		Groups: map[string][]model.IssueEntry{
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
	withoutPR := strings.Replace(string(metaContent), "PR: \n", "", 1)
	if err := os.WriteFile(metaPath, []byte(withoutPR), 0o600); err != nil {
		t.Fatalf("rewrite meta without PR: %v", err)
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
		ctx: context.Background(),
		cfg: &config{
			Mode:       model.ExecutionModePRReview,
			Provider:   "stub",
			PR:         "",
			ReviewsDir: reviewDir,
		},
	}
	if err := execCtx.afterJobSuccess(context.Background(), &job{
		Groups: map[string][]model.IssueEntry{
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

func TestAfterJobSuccessReviewPreResolveCanSkipProviderResolution(t *testing.T) {
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

	manager := &executionHookManager{
		mutators: map[string]func(any) (any, error){
			"review.pre_resolve": func(input any) (any, error) {
				payload := input.(reviewPreResolvePayload)
				resolve := false
				payload.Resolve = &resolve
				payload.Message = "keep thread open"
				return payload, nil
			},
		},
	}

	execCtx := &jobExecutionContext{
		ctx: context.Background(),
		cfg: &config{
			Mode:           model.ExecutionModePRReview,
			Provider:       "stub",
			PR:             "259",
			ReviewsDir:     reviewDir,
			RunArtifacts:   model.NewRunArtifacts(tmpDir, "review-pre-resolve"),
			RuntimeManager: manager,
		},
	}
	if err := execCtx.afterJobSuccess(context.Background(), &job{
		Groups: map[string][]model.IssueEntry{
			entries[0].CodeFile: {entries[0]},
		},
	}); err != nil {
		t.Fatalf("afterJobSuccess: %v", err)
	}

	if len(resolver.issues) != 0 {
		t.Fatalf("expected provider resolution to be skipped, got %d issues", len(resolver.issues))
	}
	if got := manager.mutableHooks; !reflect.DeepEqual(got, []string{"review.pre_resolve"}) {
		t.Fatalf("unexpected mutable hooks: %#v", got)
	}
	if len(manager.observerPayloads["review.post_fix"]) != 1 {
		t.Fatalf("expected review.post_fix once, got %d", len(manager.observerPayloads["review.post_fix"]))
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
	runID, runJournal, eventsCh, cleanup := openRuntimeEventCapture(t)
	defer cleanup()

	execCtx := &jobExecutionContext{
		ctx: context.Background(),
		cfg: &config{
			Mode:     model.ExecutionModePRDTasks,
			TasksDir: tasksDir,
			RunArtifacts: model.RunArtifacts{
				RunID: runID,
			},
		},
		journal: runJournal,
	}
	if err := execCtx.afterJobSuccess(context.Background(), &job{
		Groups: map[string][]model.IssueEntry{
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

	events := collectRuntimeEvents(t, eventsCh, 2)
	if got := events[0].Kind; got != eventspkg.EventKindTaskFileUpdated {
		t.Fatalf("expected task.file_updated event, got %s", got)
	}
	if got := events[1].Kind; got != eventspkg.EventKindTaskMetadataRefreshed {
		t.Fatalf("expected task.metadata_refreshed event, got %s", got)
	}

	var filePayload kinds.TaskFileUpdatedPayload
	decodeRuntimeEventPayload(t, events[0], &filePayload)
	if filePayload.TaskName != "task_01.md" || filePayload.OldStatus != "pending" ||
		filePayload.NewStatus != "completed" {
		t.Fatalf("unexpected task file payload: %#v", filePayload)
	}

	var metaPayload kinds.TaskMetadataRefreshedPayload
	decodeRuntimeEventPayload(t, events[1], &metaPayload)
	if metaPayload.Completed != 1 || metaPayload.Pending != 0 || metaPayload.Total != 1 {
		t.Fatalf("unexpected task metadata payload: %#v", metaPayload)
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
	runID, runJournal, eventsCh, cleanup := openRuntimeEventCapture(t)
	defer cleanup()

	execCtx := &jobExecutionContext{
		ctx: context.Background(),
		cfg: &config{
			Mode:       model.ExecutionModePRReview,
			Provider:   "stub",
			PR:         "259",
			ReviewsDir: reviewDir,
			RunArtifacts: model.RunArtifacts{
				RunID: runID,
			},
		},
		journal: runJournal,
	}
	if err := execCtx.afterJobSuccess(context.Background(), &job{
		Groups: map[string][]model.IssueEntry{
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

	events := collectRuntimeEvents(t, eventsCh, 5)
	gotKinds := []eventspkg.EventKind{
		events[0].Kind,
		events[1].Kind,
		events[2].Kind,
		events[3].Kind,
		events[4].Kind,
	}
	wantKinds := []eventspkg.EventKind{
		eventspkg.EventKindReviewStatusFinalized,
		eventspkg.EventKindProviderCallStarted,
		eventspkg.EventKindProviderCallCompleted,
		eventspkg.EventKindReviewIssueResolved,
		eventspkg.EventKindReviewRoundRefreshed,
	}
	for i := range wantKinds {
		if gotKinds[i] != wantKinds[i] {
			t.Fatalf("unexpected review event order: got %v want %v", gotKinds, wantKinds)
		}
	}

	var resolvedPayload kinds.ReviewIssueResolvedPayload
	decodeRuntimeEventPayload(t, events[3], &resolvedPayload)
	if !resolvedPayload.ProviderPosted || resolvedPayload.ProviderRef == "" {
		t.Fatalf("unexpected review issue resolved payload: %#v", resolvedPayload)
	}
}

func TestAfterTaskJobSuccessDoesNotEmitTaskFileUpdatedWhenMarkTaskCompletedFails(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, ".compozy", "tasks", "demo")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir tasks dir: %v", err)
	}

	runID, runJournal, eventsCh, cleanup := openRuntimeEventCapture(t)
	defer cleanup()

	execCtx := &jobExecutionContext{
		ctx: context.Background(),
		cfg: &config{
			Mode:     model.ExecutionModePRDTasks,
			TasksDir: tasksDir,
			RunArtifacts: model.RunArtifacts{
				RunID: runID,
			},
		},
		journal: runJournal,
	}

	err := execCtx.afterTaskJobSuccess(&job{
		Groups: map[string][]model.IssueEntry{
			"task_missing": {{
				Name:     "task_missing.md",
				AbsPath:  filepath.Join(tasksDir, "task_missing.md"),
				Content:  "---\nstatus: pending\ntitle: Missing\ntype: backend\ncomplexity: low\n---\n",
				CodeFile: "task_missing",
			}},
		},
	})
	if err == nil {
		t.Fatal("expected missing task file to fail completion")
	}

	assertNoRuntimeEvents(t, eventsCh, 200*time.Millisecond)
}

func TestResolveProviderBackedIssuesWarnsAndContinuesOnProviderFailure(t *testing.T) {
	runID, runJournal, eventsCh, cleanup := openRuntimeEventCapture(t)
	defer cleanup()

	resolver := &stubResolverProvider{
		name:       "stub",
		resolveErr: &statusCodeErr{code: 502, err: errors.New("provider unavailable")},
	}
	restore := reviewProviderRegistry
	reviewProviderRegistry = func() *provider.Registry {
		registry := provider.NewRegistry()
		registry.Register(resolver)
		return registry
	}
	defer func() { reviewProviderRegistry = restore }()

	execCtx := &jobExecutionContext{
		ctx: context.Background(),
		cfg: &config{
			Mode:       model.ExecutionModePRReview,
			Provider:   "stub",
			PR:         "259",
			ReviewsDir: "/tmp/reviews",
			RunArtifacts: model.RunArtifacts{
				RunID: runID,
			},
		},
		journal: runJournal,
	}

	err := execCtx.resolveProviderBackedIssues(context.Background(), []resolvedReviewIssue{{
		Entry: model.IssueEntry{
			Name:     "issue_001.md",
			AbsPath:  "/tmp/reviews/issue_001.md",
			CodeFile: "internal/app/service.go",
		},
		Provider: providerResolvedIssue{
			FilePath:    "/tmp/reviews/issue_001.md",
			ProviderRef: "thread:PRT_1,comment:RC_1",
		},
	}})
	if err != nil {
		t.Fatalf("resolveProviderBackedIssues: %v", err)
	}

	events := collectRuntimeEvents(t, eventsCh, 3)
	if got := events[0].Kind; got != eventspkg.EventKindProviderCallStarted {
		t.Fatalf("expected provider.call_started, got %s", got)
	}
	if got := events[1].Kind; got != eventspkg.EventKindProviderCallFailed {
		t.Fatalf("expected provider.call_failed, got %s", got)
	}
	if got := events[2].Kind; got != eventspkg.EventKindReviewIssueResolved {
		t.Fatalf("expected review.issue_resolved, got %s", got)
	}

	var failedPayload kinds.ProviderCallFailedPayload
	decodeRuntimeEventPayload(t, events[1], &failedPayload)
	if failedPayload.StatusCode != 502 || !strings.Contains(failedPayload.Error, "provider unavailable") {
		t.Fatalf("unexpected provider failure payload: %#v", failedPayload)
	}

	var resolvedPayload kinds.ReviewIssueResolvedPayload
	decodeRuntimeEventPayload(t, events[2], &resolvedPayload)
	if resolvedPayload.ProviderPosted {
		t.Fatalf("expected provider_posted=false after provider failure, got %#v", resolvedPayload)
	}
}

func TestEmitRunTerminalEventPublishesCancelledAndFailedKinds(t *testing.T) {
	cases := []struct {
		name     string
		result   executionResult
		jobs     []job
		wantKind eventspkg.EventKind
	}{
		{
			name: "successful run",
			result: executionResult{
				Status:       runStatusSucceeded,
				ArtifactsDir: "/tmp/run",
				ResultPath:   "/tmp/run/result.json",
			},
			jobs:     []job{{Status: runStatusSucceeded}},
			wantKind: eventspkg.EventKindRunCompleted,
		},
		{
			name: "canceled run",
			result: executionResult{
				Status:        runStatusCanceled,
				Error:         "canceled by user",
				ArtifactsDir:  "/tmp/run",
				ResultPath:    "/tmp/run/result.json",
				TeardownError: "",
			},
			jobs:     []job{{Status: runStatusCanceled}},
			wantKind: eventspkg.EventKindRunCancelled,
		},
		{
			name: "failed run",
			result: executionResult{
				Status:        runStatusFailed,
				Error:         "boom",
				ArtifactsDir:  "/tmp/run",
				ResultPath:    "/tmp/run/result.json",
				TeardownError: "",
			},
			jobs:     []job{{Status: runStatusFailed}},
			wantKind: eventspkg.EventKindRunFailed,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			runID, runJournal, eventsCh, cleanup := openRuntimeEventCapture(t)
			defer cleanup()

			tc.result.RunID = runID
			if err := emitRunTerminalEvent(
				context.Background(),
				runJournal,
				tc.result,
				tc.jobs,
				time.Now().Add(-2*time.Second),
			); err != nil {
				t.Fatalf("emitRunTerminalEvent: %v", err)
			}

			events := collectRuntimeEvents(t, eventsCh, 1)
			if got := events[0].Kind; got != tc.wantKind {
				t.Fatalf("expected terminal event %s, got %s", tc.wantKind, got)
			}
		})
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
		ctx: context.Background(),
		cfg: &config{
			Mode:       model.ExecutionModePRReview,
			Provider:   "stub",
			PR:         "259",
			ReviewsDir: reviewDir,
		},
	}
	err = execCtx.afterJobSuccess(context.Background(), &job{
		Groups: map[string][]model.IssueEntry{
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

func TestJobRunnerPreRetryHookCanAbortRetry(t *testing.T) {
	manager := &executionHookManager{
		mutators: map[string]func(any) (any, error){
			"job.pre_retry": func(input any) (any, error) {
				payload := input.(jobPreRetryPayload)
				proceed := false
				payload.Proceed = &proceed
				return payload, nil
			},
		},
	}

	tmpDir := t.TempDir()
	runArtifacts := model.NewRunArtifacts(tmpDir, "retry-abort")
	jb := job{
		SafeName:  "task_01",
		CodeFiles: []string{"task_01.md"},
		OutLog:    filepath.Join(tmpDir, "task_01.out.log"),
		ErrLog:    filepath.Join(tmpDir, "task_01.err.log"),
	}
	execCtx := &jobExecutionContext{
		ctx: context.Background(),
		cfg: &config{
			MaxRetries:             1,
			RunArtifacts:           runArtifacts,
			RuntimeManager:         manager,
			RetryBackoffMultiplier: 1.5,
		},
	}
	runner := newJobRunner(0, &jb, execCtx)
	_, _, continueLoop := runner.handleResult(
		context.Background(),
		1,
		2,
		time.Second,
		jobAttemptResult{
			ExitCode:  42,
			Retryable: true,
			Failure: &failInfo{
				CodeFile: jb.CodeFileLabel(),
				ExitCode: 42,
				OutLog:   jb.OutLog,
				ErrLog:   jb.ErrLog,
				Err:      errors.New("boom"),
			},
		},
	)
	if continueLoop {
		t.Fatal("expected retry loop to stop when extension vetoes retry")
	}
	if runner.lifecycle.state != jobPhaseFailed {
		t.Fatalf("unexpected lifecycle state: %s", runner.lifecycle.state)
	}
	if got := jb.Failure; !strings.Contains(got, "retry canceled by extension") {
		t.Fatalf("expected veto reason in failure, got %q", got)
	}
	if got := atomic.LoadInt32(&execCtx.failed); got != 1 {
		t.Fatalf("failed counter = %d, want 1", got)
	}
}

func TestJobRunnerPostExecuteHookFiresExactlyOncePerJob(t *testing.T) {
	manager := &executionHookManager{}
	tmpDir := t.TempDir()
	execCtx := &jobExecutionContext{
		ctx: context.Background(),
		cfg: &config{
			DryRun:         true,
			RunArtifacts:   model.NewRunArtifacts(tmpDir, "job-post-execute"),
			RuntimeManager: manager,
		},
	}
	jb := job{
		SafeName:      "task_01",
		CodeFiles:     []string{"task_01.md"},
		OutPromptPath: filepath.Join(tmpDir, "task_01.prompt.md"),
		OutLog:        filepath.Join(tmpDir, "task_01.out.log"),
		ErrLog:        filepath.Join(tmpDir, "task_01.err.log"),
	}

	newJobRunner(0, &jb, execCtx).run(context.Background())

	if got := len(manager.observerPayloads["job.post_execute"]); got != 1 {
		t.Fatalf("expected one job.post_execute payload, got %d", got)
	}
	payload, ok := manager.observerPayloads["job.post_execute"][0].(jobPostExecutePayload)
	if !ok {
		t.Fatalf("payload type = %T, want jobPostExecutePayload", manager.observerPayloads["job.post_execute"][0])
	}
	if payload.Result.Status != runStatusSucceeded {
		t.Fatalf("unexpected job result status: %q", payload.Result.Status)
	}
	if payload.Job.SafeName != "task_01" {
		t.Fatalf("unexpected job safe name: %q", payload.Job.SafeName)
	}
}

func TestExecuteRunPostShutdownHookFiresExactlyOnceWithFinalSummary(t *testing.T) {
	manager := &executionHookManager{}
	tmpDir := t.TempDir()
	runArtifacts := model.NewRunArtifacts(tmpDir, "run-post-shutdown")
	jobPayload := model.Job{
		CodeFiles: []string{"task_01.md"},
		Groups: map[string][]model.IssueEntry{
			"task_01.md": {{Name: "task_01.md", CodeFile: "task_01.md"}},
		},
		SafeName:      "task_01",
		Prompt:        []byte("finish the task"),
		OutPromptPath: filepath.Join(tmpDir, "task_01.prompt.md"),
		OutLog:        filepath.Join(tmpDir, "task_01.out.log"),
		ErrLog:        filepath.Join(tmpDir, "task_01.err.log"),
	}

	_, _, err := captureExecuteStreams(t, func() error {
		return Execute(
			context.Background(),
			[]model.Job{jobPayload},
			runArtifacts,
			nil,
			nil,
			&model.RuntimeConfig{
				WorkspaceRoot:          tmpDir,
				Mode:                   model.ExecutionModePRDTasks,
				DryRun:                 true,
				OutputFormat:           model.OutputFormatJSON,
				RetryBackoffMultiplier: 1.5,
			},
			manager,
		)
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if got := len(manager.observerPayloads["run.post_shutdown"]); got != 1 {
		t.Fatalf("expected one run.post_shutdown payload, got %d", got)
	}
	payload, ok := manager.observerPayloads["run.post_shutdown"][0].(runPostShutdownPayload)
	if !ok {
		t.Fatalf("payload type = %T, want runPostShutdownPayload", manager.observerPayloads["run.post_shutdown"][0])
	}
	if payload.Summary.Status != runStatusSucceeded {
		t.Fatalf("unexpected run summary status: %q", payload.Summary.Status)
	}
	if payload.Summary.JobsTotal != 1 || payload.Summary.JobsSucceeded != 1 {
		t.Fatalf("unexpected run summary: %#v", payload.Summary)
	}
}

func TestExecuteNilManagerMatchesNoopManagerResult(t *testing.T) {
	tmpDir := t.TempDir()
	runArtifacts := model.NewRunArtifacts(tmpDir, "nil-vs-noop")
	jobs := []model.Job{{
		CodeFiles: []string{"task_01.md"},
		Groups: map[string][]model.IssueEntry{
			"task_01.md": {{Name: "task_01.md", CodeFile: "task_01.md"}},
		},
		SafeName:      "task_01",
		Prompt:        []byte("finish the task"),
		OutPromptPath: filepath.Join(tmpDir, "task_01.prompt.md"),
		OutLog:        filepath.Join(tmpDir, "task_01.out.log"),
		ErrLog:        filepath.Join(tmpDir, "task_01.err.log"),
	}}
	cfg := &model.RuntimeConfig{
		WorkspaceRoot:          tmpDir,
		Mode:                   model.ExecutionModePRDTasks,
		DryRun:                 true,
		OutputFormat:           model.OutputFormatJSON,
		RetryBackoffMultiplier: 1.5,
	}

	stdoutNil, _, err := captureExecuteStreams(t, func() error {
		return Execute(context.Background(), jobs, runArtifacts, nil, nil, cfg, nil)
	})
	if err != nil {
		t.Fatalf("Execute(nil manager) error = %v", err)
	}
	stdoutNoop, _, err := captureExecuteStreams(t, func() error {
		return Execute(context.Background(), jobs, runArtifacts, nil, nil, cfg, &executionHookManager{})
	})
	if err != nil {
		t.Fatalf("Execute(noop manager) error = %v", err)
	}

	gotNil := decodeWorkflowJSONLTestEvents(t, stdoutNil)
	gotNoop := decodeWorkflowJSONLTestEvents(t, stdoutNoop)
	if !reflect.DeepEqual(gotNil, gotNoop) {
		t.Fatalf("nil manager stream mismatch\nnil:  %#v\nnoop: %#v", gotNil, gotNoop)
	}
}

func decodeWorkflowJSONLTestEvents(t *testing.T, stdout string) []map[string]any {
	t.Helper()

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	events := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			t.Fatalf("decode workflow jsonl line: %v\nline:\n%s", err, line)
		}
		events = append(events, payload)
	}
	return events
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
		Mode:     model.ExecutionModePRDTasks,
		TasksDir: tasksDir,
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

type executionHookManager struct {
	mutators         map[string]func(any) (any, error)
	mutableHooks     []string
	observerHooks    []string
	observerPayloads map[string][]any
}

func (*executionHookManager) Start(context.Context) error { return nil }

func (*executionHookManager) Shutdown(context.Context) error { return nil }

func (m *executionHookManager) DispatchMutableHook(
	_ context.Context,
	hook string,
	input any,
) (any, error) {
	if m == nil {
		return input, nil
	}
	m.mutableHooks = append(m.mutableHooks, hook)
	if m.mutators == nil {
		return input, nil
	}
	if mutate := m.mutators[hook]; mutate != nil {
		return mutate(input)
	}
	return input, nil
}

func (m *executionHookManager) DispatchObserverHook(_ context.Context, hook string, payload any) {
	if m == nil {
		return
	}
	m.observerHooks = append(m.observerHooks, hook)
	if m.observerPayloads == nil {
		m.observerPayloads = make(map[string][]any)
	}
	m.observerPayloads[hook] = append(m.observerPayloads[hook], payload)
}

func assertNoRuntimeEvents(t *testing.T, ch <-chan eventspkg.Event, wait time.Duration) {
	t.Helper()

	timer := time.NewTimer(wait)
	defer timer.Stop()

	select {
	case ev := <-ch:
		t.Fatalf("expected no runtime events, got %s", ev.Kind)
	case <-timer.C:
	}
}

type statusCodeErr struct {
	code int
	err  error
}

func (e *statusCodeErr) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *statusCodeErr) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func (e *statusCodeErr) StatusCode() int {
	if e == nil {
		return 0
	}
	return e.code
}
