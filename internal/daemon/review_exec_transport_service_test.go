package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	apicore "github.com/compozy/compozy/internal/api/core"
	extensions "github.com/compozy/compozy/internal/core/extension"
	workspacecfg "github.com/compozy/compozy/internal/core/workspace"
	"github.com/compozy/compozy/internal/store/globaldb"
)

var (
	sdkReviewExtensionBinaryOnce sync.Once
	sdkReviewExtensionBinaryPath string
	sdkReviewExtensionBinaryErr  error
)

func TestTransportReviewServiceFetchQueriesAndStartRunUseDaemonState(t *testing.T) {
	env := newRunManagerTestEnv(t, runManagerTestDeps{})
	recordPath := filepath.Join(t.TempDir(), "sdk-review-records.jsonl")
	installSDKReviewProviderExtension(t, env.homeDir, env.workspaceRoot, recordPath)

	service := newTransportReviewService(env.globalDB, env.manager)
	result, err := service.Fetch(context.Background(), env.workspaceRoot, env.workflowSlug, apicore.ReviewFetchRequest{
		Provider: "sdk-review",
		PRRef:    "123",
	})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if !result.Created {
		t.Fatal("expected fetch to create a new review round")
	}
	if result.Summary.WorkflowSlug != env.workflowSlug || result.Summary.RoundNumber != 1 {
		t.Fatalf("unexpected fetch summary: %#v", result.Summary)
	}
	if result.Summary.Provider != "sdk-review" || result.Summary.PRRef != "123" {
		t.Fatalf("unexpected fetch provider/pr summary: %#v", result.Summary)
	}

	reviewDir := filepath.Join(env.workflowDir(env.workflowSlug), "reviews-001")
	if _, err := os.Stat(filepath.Join(reviewDir, "_meta.md")); err != nil {
		t.Fatalf("expected review meta after fetch: %v", err)
	}
	if _, err := os.Stat(filepath.Join(reviewDir, "issue_001.md")); err != nil {
		t.Fatalf("expected review issue after fetch: %v", err)
	}
	if !strings.Contains(readFile(t, recordPath), `"type":"fetch_reviews"`) {
		t.Fatalf("expected extension fetch record in %s", recordPath)
	}

	latest, err := service.GetLatest(context.Background(), env.workspaceRoot, env.workflowSlug)
	if err != nil {
		t.Fatalf("GetLatest() error = %v", err)
	}
	if latest.RoundNumber != 1 || latest.Provider != "sdk-review" || latest.UnresolvedCount != 1 {
		t.Fatalf("unexpected latest review summary: %#v", latest)
	}

	round, err := service.GetRound(context.Background(), env.workspaceRoot, env.workflowSlug, 1)
	if err != nil {
		t.Fatalf("GetRound() error = %v", err)
	}
	if round.RoundNumber != 1 || round.Provider != "sdk-review" {
		t.Fatalf("unexpected review round: %#v", round)
	}

	issues, err := service.ListIssues(context.Background(), env.workspaceRoot, env.workflowSlug, 1)
	if err != nil {
		t.Fatalf("ListIssues() error = %v", err)
	}
	if len(issues) != 1 || issues[0].IssueNumber != 1 || issues[0].Status != "pending" {
		t.Fatalf("unexpected review issues: %#v", issues)
	}

	env.writeWorkflowFile(
		t,
		env.workflowSlug,
		filepath.Join("reviews-001", "issue_001.md"),
		daemonReviewIssueBody("resolved", "high"),
	)

	run, err := service.StartRun(context.Background(), env.workspaceRoot, env.workflowSlug, 1, apicore.ReviewRunRequest{
		Workspace:        env.workspaceRoot,
		PresentationMode: defaultPresentationMode,
		RuntimeOverrides: rawJSON(t, `{"run_id":"review-transport-001","dry_run":true}`),
		Batching:         rawJSON(t, `{"batch_size":2,"concurrent":3,"include_resolved":true}`),
	})
	if err != nil {
		t.Fatalf("StartRun() error = %v", err)
	}
	row := waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return isTerminalRunStatus(row.Status)
	})
	if row.Mode != runModeReview || row.Status != runStatusCompleted {
		t.Fatalf("unexpected review run row: %#v", row)
	}
	if reviewStatus, ok := queryReviewIssueStatus(t, env.paths.GlobalDBPath, env.workflowSlug, 1, 1); !ok ||
		reviewStatus != "resolved" {
		t.Fatalf("review issue status after start sync = %q, ok=%v, want resolved", reviewStatus, ok)
	}
}

func TestTransportExecServiceStartAndReviewHelpers(t *testing.T) {
	env := newRunManagerTestEnv(t, runManagerTestDeps{})

	execService := newTransportExecService(env.manager)

	run, err := execService.Start(context.Background(), apicore.ExecRequest{
		WorkspacePath:    env.workspaceRoot,
		Prompt:           "hello daemon exec",
		PresentationMode: defaultPresentationMode,
		RuntimeOverrides: rawJSON(t, `{"run_id":"exec-transport-001","dry_run":true}`),
	})
	if err != nil {
		t.Fatalf("Start(exec) error = %v", err)
	}
	row := waitForRun(t, env.globalDB, run.RunID, func(row globaldb.Run) bool {
		return isTerminalRunStatus(row.Status)
	})
	if row.Mode != runModeExec || row.Status != runStatusCompleted {
		t.Fatalf("unexpected exec run row: %#v", row)
	}

	registry, cleanup, err := buildWorkspaceReviewRegistry(
		context.Background(),
		env.workspaceRoot,
		"compozy reviews fetch",
	)
	if err != nil {
		t.Fatalf("buildWorkspaceReviewRegistry() error = %v", err)
	}
	t.Cleanup(cleanup)
	provider, err := registry.Get("coderabbit")
	if err != nil {
		t.Fatalf("registry.Get(coderabbit) error = %v", err)
	}
	if provider.Name() != "coderabbit" {
		t.Fatalf("provider.Name() = %q, want coderabbit", provider.Name())
	}

	projectCfg := workspacecfg.ProjectConfig{}
	if got := resolveFetchProvider(projectCfg, "sdk-review"); got != "sdk-review" {
		t.Fatalf("resolveFetchProvider(request) = %q, want sdk-review", got)
	}
	if got := resolveFetchNitpicks(projectCfg); !got {
		t.Fatalf("resolveFetchNitpicks(default) = %v, want true", got)
	}

	if mapped := mapReviewLookupError(globaldb.ErrReviewRoundNotFound); mapped == nil ||
		!strings.Contains(mapped.Error(), "review round was not found") {
		t.Fatalf("mapReviewLookupError() = %v, want review round not found problem", mapped)
	}
	if err := reviewTransportUnavailable("review lookup"); err == nil ||
		!strings.Contains(err.Error(), "review lookup is not available") {
		t.Fatalf("reviewTransportUnavailable() = %v", err)
	}
	if err := execTransportUnavailable(); err == nil || !strings.Contains(err.Error(), "exec runs are not available") {
		t.Fatalf("execTransportUnavailable() = %v", err)
	}

	summary := transportReviewSummary("demo", globaldb.ReviewRound{
		RoundNumber:     3,
		Provider:        "sdk-review",
		PRRef:           "321",
		ResolvedCount:   2,
		UnresolvedCount: 1,
	})
	if summary.WorkflowSlug != "demo" || summary.RoundNumber != 3 || summary.Provider != "sdk-review" {
		t.Fatalf("unexpected review summary helper payload: %#v", summary)
	}

	issue := transportReviewIssue(globaldb.ReviewIssue{
		ID:          "issue-123",
		IssueNumber: 7,
		Status:      "resolved",
		Severity:    "medium",
		SourcePath:  "reviews-003/issue_007.md",
	})
	if issue.IssueNumber != 7 || issue.Status != "resolved" || issue.SourcePath == "" {
		t.Fatalf("unexpected review issue helper payload: %#v", issue)
	}

	if got := cloneStringMap(map[string]string{"tier": "gold"}); got["tier"] != "gold" {
		t.Fatalf("cloneStringMap() = %#v", got)
	}
}

func TestTransportReviewExecServicesReportUnavailableAndLookupErrors(t *testing.T) {
	env := newRunManagerTestEnv(t, runManagerTestDeps{})

	reviewService := newTransportReviewService(env.globalDB, env.manager)
	if _, err := reviewService.GetLatest(context.Background(), env.workspaceRoot, env.workflowSlug); err == nil ||
		!strings.Contains(err.Error(), "review round was not found") {
		t.Fatalf("GetLatest() error = %v, want review round not found", err)
	}
	if _, err := reviewService.GetRound(context.Background(), env.workspaceRoot, env.workflowSlug, 1); err == nil ||
		!strings.Contains(err.Error(), "review round was not found") {
		t.Fatalf("GetRound() error = %v, want review round not found", err)
	}
	if _, err := reviewService.ListIssues(context.Background(), env.workspaceRoot, env.workflowSlug, 1); err == nil ||
		!strings.Contains(err.Error(), "review round was not found") {
		t.Fatalf("ListIssues() error = %v, want review round not found", err)
	}

	nilReviewService := newTransportReviewService(nil, env.manager)
	if _, err := nilReviewService.Fetch(
		context.Background(),
		env.workspaceRoot,
		env.workflowSlug,
		apicore.ReviewFetchRequest{},
	); err == nil || !strings.Contains(err.Error(), "review fetch is not available") {
		t.Fatalf("Fetch() unavailable error = %v", err)
	}
	if _, err := nilReviewService.StartRun(
		context.Background(),
		env.workspaceRoot,
		env.workflowSlug,
		1,
		apicore.ReviewRunRequest{},
	); err == nil || strings.Contains(err.Error(), "is not available") {
		t.Fatalf("StartRun() error = %v, want concrete run error instead of transport unavailable", err)
	}

	nilRunManagerReviewService := newTransportReviewService(env.globalDB, nil)
	if _, err := nilRunManagerReviewService.GetLatest(
		context.Background(),
		env.workspaceRoot,
		env.workflowSlug,
	); err == nil ||
		!strings.Contains(err.Error(), "review lookup is not available") {
		t.Fatalf("GetLatest() unavailable error = %v", err)
	}
	if _, err := nilRunManagerReviewService.StartRun(
		context.Background(),
		env.workspaceRoot,
		env.workflowSlug,
		1,
		apicore.ReviewRunRequest{},
	); err == nil || !strings.Contains(err.Error(), "review runs is not available") {
		t.Fatalf("StartRun() unavailable error = %v", err)
	}

	nilExecService := newTransportExecService(nil)
	if _, err := nilExecService.Start(context.Background(), apicore.ExecRequest{}); err == nil ||
		!strings.Contains(err.Error(), "exec runs are not available") {
		t.Fatalf("Start(exec) unavailable error = %v", err)
	}

	projectCfg := workspacecfg.ProjectConfig{}
	providerName := "project-review"
	projectCfg.FetchReviews.Provider = &providerName
	nitpicks := false
	projectCfg.FetchReviews.Nitpicks = &nitpicks
	if got := resolveFetchProvider(projectCfg, ""); got != "project-review" {
		t.Fatalf("resolveFetchProvider(project default) = %q, want project-review", got)
	}
	if got := resolveFetchNitpicks(projectCfg); got {
		t.Fatalf("resolveFetchNitpicks(project false) = %v, want false", got)
	}
	if mapped := mapReviewLookupError(errors.New("plain boom")); mapped == nil || mapped.Error() != "plain boom" {
		t.Fatalf("mapReviewLookupError(passthrough) = %v, want plain boom", mapped)
	}
}

func installSDKReviewProviderExtension(
	t *testing.T,
	homeDir string,
	workspaceRoot string,
	recordPath string,
) {
	t.Helper()

	binary := sdkReviewExtensionBinary(t)
	extensionDir := filepath.Join(workspaceRoot, ".compozy", "extensions", "sdk-review-ext")
	manifest := &extensions.Manifest{
		Extension: extensions.ExtensionInfo{
			Name:              "sdk-review-ext",
			Version:           "1.0.0",
			Description:       "SDK review provider fixture",
			MinCompozyVersion: "0.0.1",
		},
		Subprocess: &extensions.SubprocessConfig{
			Command: binary,
			Env: map[string]string{
				"COMPOZY_SDK_RECORD_PATH":     recordPath,
				"COMPOZY_SDK_REVIEW_PROVIDER": "sdk-review",
				"COMPOZY_SDK_REVIEW_MODE":     "",
			},
		},
		Security: extensions.SecurityConfig{
			Capabilities: []extensions.Capability{extensions.CapabilityProvidersRegister},
		},
		Providers: extensions.ProvidersConfig{
			Review: []extensions.ProviderEntry{{
				Name: "sdk-review",
				Kind: extensions.ProviderKindExtension,
			}},
		},
	}

	writeJSONFile(t, filepath.Join(extensionDir, extensions.ManifestFileNameJSON), manifest)

	store, err := extensions.NewEnablementStore(context.Background(), homeDir)
	if err != nil {
		t.Fatalf("NewEnablementStore() error = %v", err)
	}
	if err := store.Enable(context.Background(), extensions.Ref{
		Name:          "sdk-review-ext",
		Source:        extensions.SourceWorkspace,
		WorkspaceRoot: workspaceRoot,
	}); err != nil {
		t.Fatalf("Enable(workspace review extension) error = %v", err)
	}
}

func sdkReviewExtensionBinary(t *testing.T) string {
	t.Helper()

	sdkReviewExtensionBinaryOnce.Do(func() {
		currentFile := currentTestFile()
		extensionDir := filepath.Join(filepath.Dir(currentFile), "..", "core", "extension")
		binaryDir, err := os.MkdirTemp("", "compozy-sdk-review-extension-*")
		if err != nil {
			sdkReviewExtensionBinaryErr = err
			return
		}

		binary := filepath.Join(binaryDir, "sdk-review-extension")
		cmd := exec.CommandContext(
			context.Background(),
			"go",
			"build",
			"-o",
			binary,
			"./testdata/sdk_review_extension",
		)
		cmd.Dir = extensionDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			sdkReviewExtensionBinaryErr = err
			sdkReviewExtensionBinaryErr = execBuildError(err, output)
			return
		}
		sdkReviewExtensionBinaryPath = binary
	})

	if sdkReviewExtensionBinaryErr != nil {
		t.Fatalf("build sdk review extension: %v", sdkReviewExtensionBinaryErr)
	}
	return sdkReviewExtensionBinaryPath
}

func execBuildError(err error, output []byte) error {
	if len(output) == 0 {
		return err
	}
	return &buildError{err: err, output: strings.TrimSpace(string(output))}
}

type buildError struct {
	err    error
	output string
}

func (e *buildError) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.output) == "" {
		return e.err.Error()
	}
	return e.err.Error() + ": " + e.output
}

func currentTestFile() string {
	_, file, _, _ := runtime.Caller(0)
	return file
}

func writeJSONFile(t *testing.T, path string, value any) {
	t.Helper()

	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent(%s) error = %v", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}
