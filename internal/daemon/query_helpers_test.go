package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/store/globaldb"
)

type erringDaemonStatusReader struct {
	status    apicore.DaemonStatus
	health    apicore.DaemonHealth
	statusErr error
	healthErr error
}

func (s erringDaemonStatusReader) Status(context.Context) (apicore.DaemonStatus, error) {
	if s.statusErr != nil {
		return apicore.DaemonStatus{}, s.statusErr
	}
	return s.status, nil
}

func (s erringDaemonStatusReader) Health(context.Context) (apicore.DaemonHealth, error) {
	if s.healthErr != nil {
		return apicore.DaemonHealth{}, s.healthErr
	}
	return s.health, nil
}

func TestQueryHelperErrorsAndDocumentTitles(t *testing.T) {
	t.Parallel()

	t.Run("Should match DocumentMissingError against ErrDocumentMissing", func(t *testing.T) {
		t.Parallel()

		docMissing := DocumentMissingError{
			Kind:         "task",
			WorkflowSlug: "demo",
			RelativePath: "task_01.md",
		}
		if !errors.Is(docMissing, ErrDocumentMissing) {
			t.Fatal("DocumentMissingError should match ErrDocumentMissing")
		}
		if got := docMissing.Error(); !strings.Contains(got, "task_01.md") || !strings.Contains(got, "demo") {
			t.Fatalf("DocumentMissingError.Error() = %q", got)
		}
	})

	t.Run("Should match StaleDocumentReferenceError against ErrStaleDocumentReference", func(t *testing.T) {
		t.Parallel()

		stale := StaleDocumentReferenceError{
			Kind:         "memory",
			WorkflowSlug: "demo",
			Reference:    "mem_123",
		}
		if !errors.Is(stale, ErrStaleDocumentReference) {
			t.Fatal("StaleDocumentReferenceError should match ErrStaleDocumentReference")
		}
		if got := stale.Error(); !strings.Contains(got, "mem_123") || !strings.Contains(got, "demo") {
			t.Fatalf("StaleDocumentReferenceError.Error() = %q", got)
		}
	})

	t.Run("Should match ReviewIssueNotFoundError against ErrReviewIssueNotFound", func(t *testing.T) {
		t.Parallel()

		issueErr := ReviewIssueNotFoundError{
			WorkflowSlug: "demo",
			Round:        4,
			IssueRef:     "issue_007.md",
		}
		if !errors.Is(issueErr, ErrReviewIssueNotFound) {
			t.Fatal("ReviewIssueNotFoundError should match ErrReviewIssueNotFound")
		}
		if got := issueErr.Error(); !strings.Contains(got, "issue_007.md") || !strings.Contains(got, "round 4") {
			t.Fatalf("ReviewIssueNotFoundError.Error() = %q", got)
		}
	})

	titleTests := []struct {
		name     string
		path     string
		kind     string
		metadata map[string]any
		body     string
		want     string
	}{
		{
			name: "Should extract task titles from markdown bodies",
			path: "task_07.md",
			kind: "task",
			body: daemonTaskBody("pending", "Helper Task"),
			want: "Helper Task",
		},
		{
			name: "Should map techspec filenames to TechSpec",
			path: "_techspec.md",
			kind: "techspec",
			body: "no heading",
			want: "TechSpec",
		},
		{
			name: "Should map MEMORY filenames to Memory",
			path: "MEMORY.md",
			kind: "memory",
			body: "no heading",
			want: "Memory",
		},
		{
			name: "Should fall back to the normalized filename",
			path: "design_notes.md",
			kind: "doc",
			body: "no heading",
			want: "design notes",
		},
		{
			name:     "Should prefer frontmatter titles when provided",
			path:     "custom.md",
			kind:     "doc",
			metadata: map[string]any{"title": "Frontmatter Title"},
			body:     "# Ignored",
			want:     "Frontmatter Title",
		},
	}
	for _, tt := range titleTests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := documentTitle(tt.path, tt.kind, tt.metadata, tt.body); got != tt.want {
				t.Fatalf("documentTitle(%s) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestQueryHelperDirectoryAndStatusBranches(t *testing.T) {
	t.Parallel()

	t.Run("Should read and sort markdown entries while ignoring non-markdown files", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "top.md"), []byte("# Top\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(top.md) error = %v", err)
		}
		if err := os.MkdirAll(filepath.Join(root, "nested"), 0o755); err != nil {
			t.Fatalf("MkdirAll(nested) error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, "nested", "child.md"), []byte("# Child\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(child.md) error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, "ignore.txt"), []byte("ignored"), 0o600); err != nil {
			t.Fatalf("WriteFile(ignore.txt) error = %v", err)
		}

		entries, err := readMarkdownDir(root)
		if err != nil {
			t.Fatalf("readMarkdownDir() error = %v", err)
		}
		if len(entries) != 2 {
			t.Fatalf("len(entries) = %d, want 2", len(entries))
		}
		if entries[0].displayPath != "nested/child.md" || entries[1].displayPath != "top.md" {
			t.Fatalf("unexpected directory ordering: %#v", entries)
		}
	})

	t.Run("Should report missing markdown directories", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		if _, err := readMarkdownDir(filepath.Join(root, "missing")); !errors.Is(err, ErrDocumentMissing) {
			t.Fatalf("readMarkdownDir(missing) error = %v, want ErrDocumentMissing", err)
		}
	})

	t.Run("Should reject empty markdown directory paths", func(t *testing.T) {
		t.Parallel()

		if _, err := readMarkdownDir(
			" ",
		); err == nil ||
			!strings.Contains(err.Error(), "markdown directory is required") {
			t.Fatalf("readMarkdownDir(empty) error = %v, want markdown directory required", err)
		}
	})

	t.Run("Should normalize lane titles and free-form title case", func(t *testing.T) {
		t.Parallel()

		if got := laneTitle("needs_review"); got != "Needs Review" {
			t.Fatalf("laneTitle(needs_review) = %q, want Needs Review", got)
		}
		if got := laneTitle("canceled"); got != "Canceled" {
			t.Fatalf("laneTitle(canceled) = %q, want Canceled", got)
		}
		if got := titleCase("needs-review NOW"); got != "Needs Review Now" {
			t.Fatalf("titleCase() = %q, want Needs Review Now", got)
		}
	})

	t.Run("Should summarize run job counts by status", func(t *testing.T) {
		t.Parallel()

		counts := summarizeRunJobCounts(apicore.RunSnapshot{
			Jobs: []apicore.RunJobState{
				{Status: snapshotJobStatusQueued},
				{Status: runStatusRunning},
				{Status: "retrying"},
				{Status: runStatusCompleted},
				{Status: runStatusFailed},
				{Status: "canceled"},
			},
		})
		if counts.Queued != 1 || counts.Running != 1 || counts.Retrying != 1 ||
			counts.Completed != 1 || counts.Failed != 1 || counts.Canceled != 1 {
			t.Fatalf("unexpected summarizeRunJobCounts() result: %#v", counts)
		}
	})
}

func TestQueryHelpersProtectMarkdownDocumentsFromSymlinkAndMetadataAliasing(t *testing.T) {
	t.Parallel()

	t.Run("Should reject symlinked markdown entries during directory scans", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		target := filepath.Join(t.TempDir(), "secret.md")
		if err := os.WriteFile(target, []byte("# Secret\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(secret.md) error = %v", err)
		}

		linkPath := filepath.Join(root, "linked.md")
		if err := os.Symlink(target, linkPath); err != nil {
			t.Skipf("os.Symlink() unavailable: %v", err)
		}

		if _, err := readMarkdownDir(root); err == nil || !strings.Contains(err.Error(), "symlinked markdown file") {
			t.Fatalf("readMarkdownDir(symlink) error = %v, want symlink rejection", err)
		}
	})

	t.Run("Should reject direct reads of symlinked markdown files", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		target := filepath.Join(t.TempDir(), "secret.md")
		if err := os.WriteFile(target, []byte("# Secret\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(secret.md) error = %v", err)
		}

		linkPath := filepath.Join(root, "linked.md")
		if err := os.Symlink(target, linkPath); err != nil {
			t.Skipf("os.Symlink() unavailable: %v", err)
		}

		reader := newDocumentReader()
		if _, err := reader.Read(context.Background(), linkPath, "memory", "mem_1"); err == nil ||
			!strings.Contains(err.Error(), "symlinked markdown file") {
			t.Fatalf("Read(symlink) error = %v, want symlink rejection", err)
		}
	})

	t.Run("Should deep clone nested metadata collections", func(t *testing.T) {
		t.Parallel()

		source := map[string]any{
			"owner": "daemon",
			"nested": map[string]any{
				"state": "draft",
				"items": []any{
					map[string]any{"label": "first"},
				},
			},
		}

		cloned := cloneMetadataMap(source)
		cloned["owner"] = "browser"
		clonedNested := cloned["nested"].(map[string]any)
		clonedNested["state"] = "accepted"
		clonedItems := clonedNested["items"].([]any)
		clonedItems[0].(map[string]any)["label"] = "changed"

		if got := source["owner"]; got != "daemon" {
			t.Fatalf("source owner = %#v, want daemon", got)
		}
		sourceNested := source["nested"].(map[string]any)
		if got := sourceNested["state"]; got != "draft" {
			t.Fatalf("source nested state = %#v, want draft", got)
		}
		if got := sourceNested["items"].([]any)[0].(map[string]any)["label"]; got != "first" {
			t.Fatalf("source nested label = %#v, want first", got)
		}
	})
}

func TestQueryServiceReadHelpersHandleOptionalAndErrorBranches(t *testing.T) {
	t.Parallel()

	newService := func(t *testing.T) (*runManagerTestEnv, *queryService) {
		t.Helper()

		env := newRunManagerTestEnv(t, runManagerTestDeps{})
		service := &queryService{
			globalDB:   env.globalDB,
			runManager: env.manager,
			documents:  newDocumentReader(),
		}
		return env, service
	}

	t.Run("Should return a zero document and false when workflow documents are missing", func(t *testing.T) {
		t.Parallel()

		env, service := newService(t)
		if doc, ok, err := service.readWorkflowDocument(
			context.Background(),
			env.workflowDir(env.workflowSlug),
			"missing.md",
			"task",
			"task_missing",
		); err != nil || ok || doc.ID != "" || doc.Kind != "" || doc.Markdown != "" {
			t.Fatalf("readWorkflowDocument(missing) = %#v, %v, %v; want zero-like doc, false, nil", doc, ok, err)
		}
	})

	t.Run("Should return zero daemon state when no daemon reader is configured", func(t *testing.T) {
		t.Parallel()

		_, service := newService(t)
		status, health, err := service.readDaemonState(context.Background())
		if err != nil {
			t.Fatalf("readDaemonState(nil daemon) error = %v", err)
		}
		if status != (apicore.DaemonStatus{}) || health.Ready || health.Degraded || len(health.Details) != 0 {
			t.Fatalf("readDaemonState(nil daemon) = %#v %#v, want zero values", status, health)
		}
	})

	t.Run("Should propagate daemon status errors", func(t *testing.T) {
		t.Parallel()

		_, service := newService(t)
		statusErr := errors.New("status failed")
		service.daemon = erringDaemonStatusReader{statusErr: statusErr}
		if _, _, err := service.readDaemonState(context.Background()); !errors.Is(err, statusErr) {
			t.Fatalf("readDaemonState(status error) = %v, want %v", err, statusErr)
		}
	})

	t.Run("Should propagate daemon health errors after reading status", func(t *testing.T) {
		t.Parallel()

		_, service := newService(t)
		healthErr := errors.New("health failed")
		service.daemon = erringDaemonStatusReader{
			status:    apicore.DaemonStatus{PID: 7},
			healthErr: healthErr,
		}
		if _, _, err := service.readDaemonState(context.Background()); !errors.Is(err, healthErr) {
			t.Fatalf("readDaemonState(health error) = %v, want %v", err, healthErr)
		}
	})

	t.Run("Should return no latest review summary when a workflow has no reviews", func(t *testing.T) {
		t.Parallel()

		env, service := newService(t)
		workflow, err := env.globalDB.PutWorkflow(context.Background(), globaldb.Workflow{
			WorkspaceID: mustWorkspaceID(t, env.globalDB, env.workspaceRoot),
			Slug:        "no-reviews",
		})
		if err != nil {
			t.Fatalf("PutWorkflow(no-reviews) error = %v", err)
		}
		review, ok, err := service.latestReviewSummary(context.Background(), workflow)
		if err != nil {
			t.Fatalf("latestReviewSummary(no reviews) error = %v", err)
		}
		if ok || review != (apicore.ReviewSummary{}) {
			t.Fatalf("latestReviewSummary(no reviews) = %#v, %v; want zero summary and false", review, ok)
		}
	})
}

func mustWorkspaceID(t *testing.T, db *globaldb.GlobalDB, workspaceRoot string) string {
	t.Helper()

	workspace, err := db.ResolveOrRegister(context.Background(), workspaceRoot)
	if err != nil {
		t.Fatalf("ResolveOrRegister(%q) error = %v", workspaceRoot, err)
	}
	return workspace.ID
}
