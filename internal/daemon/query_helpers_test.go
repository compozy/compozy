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

	if got := documentTitle("task_07.md", "task", nil, daemonTaskBody("pending", "Helper Task")); got != "Helper Task" {
		t.Fatalf("documentTitle(task) = %q, want Helper Task", got)
	}
	if got := documentTitle("_techspec.md", "techspec", nil, "no heading"); got != "TechSpec" {
		t.Fatalf("documentTitle(_techspec) = %q, want TechSpec", got)
	}
	if got := documentTitle("MEMORY.md", "memory", nil, "no heading"); got != "Memory" {
		t.Fatalf("documentTitle(MEMORY) = %q, want Memory", got)
	}
	if got := documentTitle("design_notes.md", "doc", nil, "no heading"); got != "design notes" {
		t.Fatalf("documentTitle(default) = %q, want design notes", got)
	}
	if got := documentTitle(
		"custom.md",
		"doc",
		map[string]any{"title": "Frontmatter Title"},
		"# Ignored",
	); got != "Frontmatter Title" {
		t.Fatalf("documentTitle(frontmatter) = %q, want Frontmatter Title", got)
	}
}

func TestQueryHelperDirectoryAndStatusBranches(t *testing.T) {
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

	if _, err := readMarkdownDir(filepath.Join(root, "missing")); !errors.Is(err, ErrDocumentMissing) {
		t.Fatalf("readMarkdownDir(missing) error = %v, want ErrDocumentMissing", err)
	}
	if _, err := readMarkdownDir(" "); err == nil {
		t.Fatal("readMarkdownDir(empty) error = nil, want non-nil")
	}

	if got := laneTitle("needs_review"); got != "Needs Review" {
		t.Fatalf("laneTitle(needs_review) = %q, want Needs Review", got)
	}
	if got := laneTitle("canceled"); got != "Canceled" {
		t.Fatalf("laneTitle(canceled) = %q, want Canceled", got)
	}
	if got := titleCase("needs-review NOW"); got != "Needs Review Now" {
		t.Fatalf("titleCase() = %q, want Needs Review Now", got)
	}

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
}

func TestQueryServiceReadHelpersHandleOptionalAndErrorBranches(t *testing.T) {
	env := newRunManagerTestEnv(t, runManagerTestDeps{})

	service := &queryService{
		globalDB:   env.globalDB,
		runManager: env.manager,
		documents:  newDocumentReader(),
	}

	if doc, ok, err := service.readWorkflowDocument(
		context.Background(),
		env.workflowDir(env.workflowSlug),
		"missing.md",
		"task",
		"task_missing",
	); err != nil || ok || doc.ID != "" || doc.Kind != "" || doc.Markdown != "" {
		t.Fatalf("readWorkflowDocument(missing) = %#v, %v, %v; want zero-like doc, false, nil", doc, ok, err)
	}

	status, health, err := service.readDaemonState(context.Background())
	if err != nil {
		t.Fatalf("readDaemonState(nil daemon) error = %v", err)
	}
	if status != (apicore.DaemonStatus{}) || health.Ready || health.Degraded || len(health.Details) != 0 {
		t.Fatalf("readDaemonState(nil daemon) = %#v %#v, want zero values", status, health)
	}

	statusErr := errors.New("status failed")
	service.daemon = erringDaemonStatusReader{statusErr: statusErr}
	if _, _, err := service.readDaemonState(context.Background()); !errors.Is(err, statusErr) {
		t.Fatalf("readDaemonState(status error) = %v, want %v", err, statusErr)
	}

	healthErr := errors.New("health failed")
	service.daemon = erringDaemonStatusReader{
		status:    apicore.DaemonStatus{PID: 7},
		healthErr: healthErr,
	}
	if _, _, err := service.readDaemonState(context.Background()); !errors.Is(err, healthErr) {
		t.Fatalf("readDaemonState(health error) = %v, want %v", err, healthErr)
	}

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
}

func mustWorkspaceID(t *testing.T, db *globaldb.GlobalDB, workspaceRoot string) string {
	t.Helper()

	workspace, err := db.ResolveOrRegister(context.Background(), workspaceRoot)
	if err != nil {
		t.Fatalf("ResolveOrRegister(%q) error = %v", workspaceRoot, err)
	}
	return workspace.ID
}
