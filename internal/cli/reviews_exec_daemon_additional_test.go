package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	apiclient "github.com/compozy/compozy/internal/api/client"
	apicore "github.com/compozy/compozy/internal/api/core"
	core "github.com/compozy/compozy/internal/core"
	coreRun "github.com/compozy/compozy/internal/core/run"
	"github.com/compozy/compozy/internal/setup"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
	"github.com/spf13/cobra"
)

type reviewExecCaptureClient struct {
	*stubDaemonCommandClient

	fetchWorkspace string
	fetchSlug      string
	fetchReq       apicore.ReviewFetchRequest

	latestWorkspace string
	latestSlug      string

	roundWorkspace string
	roundSlug      string
	roundNumber    int

	issuesWorkspace string
	issuesSlug      string
	issuesRound     int

	startReviewWorkspace string
	startReviewSlug      string
	startReviewRound     int
	startReviewReq       apicore.ReviewRunRequest

	startExecReq apicore.ExecRequest
}

func (c *reviewExecCaptureClient) FetchReview(
	ctx context.Context,
	workspace string,
	slug string,
	req apicore.ReviewFetchRequest,
) (apicore.ReviewFetchResult, error) {
	c.fetchWorkspace = workspace
	c.fetchSlug = slug
	c.fetchReq = req
	return c.stubDaemonCommandClient.FetchReview(ctx, workspace, slug, req)
}

func (c *reviewExecCaptureClient) GetLatestReview(
	ctx context.Context,
	workspace string,
	slug string,
) (apicore.ReviewSummary, error) {
	c.latestWorkspace = workspace
	c.latestSlug = slug
	return c.stubDaemonCommandClient.GetLatestReview(ctx, workspace, slug)
}

func (c *reviewExecCaptureClient) GetReviewRound(
	ctx context.Context,
	workspace string,
	slug string,
	round int,
) (apicore.ReviewRound, error) {
	c.roundWorkspace = workspace
	c.roundSlug = slug
	c.roundNumber = round
	return c.stubDaemonCommandClient.GetReviewRound(ctx, workspace, slug, round)
}

func (c *reviewExecCaptureClient) ListReviewIssues(
	ctx context.Context,
	workspace string,
	slug string,
	round int,
) ([]apicore.ReviewIssue, error) {
	c.issuesWorkspace = workspace
	c.issuesSlug = slug
	c.issuesRound = round
	return c.stubDaemonCommandClient.ListReviewIssues(ctx, workspace, slug, round)
}

func (c *reviewExecCaptureClient) StartReviewRun(
	ctx context.Context,
	workspace string,
	slug string,
	round int,
	req apicore.ReviewRunRequest,
) (apicore.Run, error) {
	c.startReviewWorkspace = workspace
	c.startReviewSlug = slug
	c.startReviewRound = round
	c.startReviewReq = req
	return c.stubDaemonCommandClient.StartReviewRun(ctx, workspace, slug, round, req)
}

func (c *reviewExecCaptureClient) StartExecRun(ctx context.Context, req apicore.ExecRequest) (apicore.Run, error) {
	c.startExecReq = req
	return c.stubDaemonCommandClient.StartExecRun(ctx, req)
}

type staticClientRunStream struct {
	items  chan apiclient.RunStreamItem
	errors chan error
}

func newStaticClientRunStream() *staticClientRunStream {
	return &staticClientRunStream{
		items:  make(chan apiclient.RunStreamItem, 8),
		errors: make(chan error, 8),
	}
}

func (s *staticClientRunStream) Items() <-chan apiclient.RunStreamItem {
	return s.items
}

func (s *staticClientRunStream) Errors() <-chan error {
	return s.errors
}

func (*staticClientRunStream) Close() error {
	return nil
}

func testReviewExecCommandDefaults() commandStateDefaults {
	defaults := defaultCommandStateDefaults()
	defaults.listBundledSkills = func() ([]setup.Skill, error) { return nil, nil }
	defaults.verifyBundledSkills = func(setup.VerifyConfig) (setup.VerifyResult, error) {
		return setup.VerifyResult{
			Agent: setup.Agent{Name: "codex", DisplayName: "Codex"},
			Scope: setup.InstallScopeProject,
			Mode:  setup.InstallModeCopy,
		}, nil
	}
	defaults.installBundledSkills = func(setup.InstallConfig) (*setup.Result, error) {
		return &setup.Result{}, nil
	}
	defaults.confirmSkillRefresh = func(*cobra.Command, skillRefreshPrompt) (bool, error) {
		return true, nil
	}
	return defaults
}

func TestReviewsCommandFetchListShowUseDaemonRequests(t *testing.T) {
	workspaceRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspaceRoot, ".compozy", "tasks", "demo"), 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}
	withWorkingDir(t, workspaceRoot)

	client := &reviewExecCaptureClient{
		stubDaemonCommandClient: &stubDaemonCommandClient{
			health: apicore.DaemonHealth{Ready: true},
			reviewFetch: apicore.ReviewFetchResult{
				Summary: apicore.ReviewSummary{
					WorkflowSlug:    "demo",
					RoundNumber:     2,
					Provider:        "ext-review",
					PRRef:           "259",
					ResolvedCount:   1,
					UnresolvedCount: 2,
					UpdatedAt:       time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC),
				},
				Created: true,
			},
			reviewLatest: apicore.ReviewSummary{
				WorkflowSlug:    "demo",
				RoundNumber:     2,
				Provider:        "ext-review",
				PRRef:           "259",
				ResolvedCount:   1,
				UnresolvedCount: 2,
				UpdatedAt:       time.Date(2026, 4, 18, 12, 1, 0, 0, time.UTC),
			},
			reviewRound: apicore.ReviewRound{
				ID:              "round-demo-002",
				WorkflowSlug:    "demo",
				RoundNumber:     2,
				Provider:        "ext-review",
				PRRef:           "259",
				ResolvedCount:   1,
				UnresolvedCount: 2,
				UpdatedAt:       time.Date(2026, 4, 18, 12, 1, 0, 0, time.UTC),
			},
			reviewIssues: []apicore.ReviewIssue{{
				ID:          "issue-001",
				IssueNumber: 1,
				Status:      "pending",
				Severity:    "high",
				SourcePath:  "reviews-002/issue_001.md",
				UpdatedAt:   time.Date(2026, 4, 18, 12, 1, 0, 0, time.UTC),
			}},
		},
	}
	installTestCLIReadyDaemonBootstrap(t, client)

	output, err := executeCommandCombinedOutput(
		newReviewsCommandWithDefaults(testReviewExecCommandDefaults()),
		nil,
		"fetch",
		"demo",
		"--provider",
		"ext-review",
		"--pr",
		"259",
		"--round",
		"2",
	)
	if err != nil {
		t.Fatalf("execute reviews fetch: %v\noutput:\n%s", err, output)
	}
	if !containsAll(
		output,
		"Fetched review issues from ext-review for PR 259 into",
		filepath.Join(".compozy", "tasks", "demo", "reviews-002"),
	) {
		t.Fatalf("unexpected reviews fetch output:\n%s", output)
	}
	if mustEvalSymlinksCLITest(t, client.fetchWorkspace) != mustEvalSymlinksCLITest(t, workspaceRoot) {
		t.Fatalf("fetch workspace = %q, want %q", client.fetchWorkspace, workspaceRoot)
	}
	if client.fetchSlug != "demo" {
		t.Fatalf("fetch slug = %q, want demo", client.fetchSlug)
	}
	if client.fetchReq.Provider != "ext-review" || client.fetchReq.PRRef != "259" ||
		client.fetchReq.Round == nil || *client.fetchReq.Round != 2 {
		t.Fatalf("unexpected fetch request: %#v", client.fetchReq)
	}

	output, err = executeCommandCombinedOutput(
		newReviewsCommandWithDefaults(testReviewExecCommandDefaults()),
		nil,
		"list",
		"demo",
	)
	if err != nil {
		t.Fatalf("execute reviews list: %v\noutput:\n%s", err, output)
	}
	if !containsAll(output, "demo round 002", "provider=ext-review", "pr=259", "unresolved=2", "resolved=1") {
		t.Fatalf("unexpected reviews list output:\n%s", output)
	}
	if mustEvalSymlinksCLITest(t, client.latestWorkspace) != mustEvalSymlinksCLITest(t, workspaceRoot) {
		t.Fatalf("latest workspace = %q, want %q", client.latestWorkspace, workspaceRoot)
	}
	if client.latestSlug != "demo" {
		t.Fatalf("latest slug = %q, want demo", client.latestSlug)
	}

	output, err = executeCommandCombinedOutput(
		newReviewsCommandWithDefaults(testReviewExecCommandDefaults()),
		nil,
		"show",
		"demo",
		"2",
	)
	if err != nil {
		t.Fatalf("execute reviews show: %v\noutput:\n%s", err, output)
	}
	if !containsAll(
		output,
		"demo round 002",
		"provider=ext-review",
		"- issue_001 | status=pending severity=high path=reviews-002/issue_001.md",
	) {
		t.Fatalf("unexpected reviews show output:\n%s", output)
	}
	if mustEvalSymlinksCLITest(t, client.roundWorkspace) != mustEvalSymlinksCLITest(t, workspaceRoot) {
		t.Fatalf("round workspace = %q, want %q", client.roundWorkspace, workspaceRoot)
	}
	if client.roundSlug != "demo" || client.roundNumber != 2 {
		t.Fatalf("unexpected round lookup: slug=%q round=%d", client.roundSlug, client.roundNumber)
	}
	if mustEvalSymlinksCLITest(t, client.issuesWorkspace) != mustEvalSymlinksCLITest(t, workspaceRoot) {
		t.Fatalf("issues workspace = %q, want %q", client.issuesWorkspace, workspaceRoot)
	}
	if client.issuesSlug != "demo" || client.issuesRound != 2 {
		t.Fatalf("unexpected issue lookup: slug=%q round=%d", client.issuesSlug, client.issuesRound)
	}
}

func TestReviewsFixCommandResolvesLatestRoundAndBuildsDaemonRequest(t *testing.T) {
	workspaceRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspaceRoot, ".compozy", "tasks", "demo"), 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}
	withWorkingDir(t, workspaceRoot)

	client := &reviewExecCaptureClient{
		stubDaemonCommandClient: &stubDaemonCommandClient{
			health: apicore.DaemonHealth{Ready: true},
			reviewLatest: apicore.ReviewSummary{
				WorkflowSlug: "demo",
				RoundNumber:  7,
				Provider:     "ext-review",
				PRRef:        "259",
			},
			reviewRun: apicore.Run{
				RunID:            "review-run-007",
				Mode:             "review",
				Status:           "starting",
				PresentationMode: "detach",
				StartedAt:        time.Date(2026, 4, 18, 12, 2, 0, 0, time.UTC),
			},
		},
	}
	installTestCLIReadyDaemonBootstrap(t, client)

	output, err := executeCommandCombinedOutput(
		newReviewsCommandWithDefaults(testReviewExecCommandDefaults()),
		nil,
		"fix",
		"demo",
		"--detach",
		"--dry-run",
		"--auto-commit",
		"--ide",
		"codex",
		"--model",
		"gpt-5.4",
		"--reasoning-effort",
		"high",
		"--access-mode",
		"default",
		"--timeout",
		"2m",
		"--max-retries",
		"3",
		"--retry-backoff-multiplier",
		"2.5",
		"--concurrent",
		"4",
		"--batch-size",
		"2",
		"--include-resolved",
	)
	if err != nil {
		t.Fatalf("execute reviews fix: %v\noutput:\n%s", err, output)
	}
	if !containsAll(output, "task run started: review-run-007", "(mode=detach)") {
		t.Fatalf("unexpected reviews fix output:\n%s", output)
	}
	if mustEvalSymlinksCLITest(t, client.latestWorkspace) != mustEvalSymlinksCLITest(t, workspaceRoot) {
		t.Fatalf("latest workspace = %q, want %q", client.latestWorkspace, workspaceRoot)
	}
	if client.latestSlug != "demo" {
		t.Fatalf("latest slug = %q, want demo", client.latestSlug)
	}
	if mustEvalSymlinksCLITest(t, client.startReviewWorkspace) != mustEvalSymlinksCLITest(t, workspaceRoot) {
		t.Fatalf("review run workspace = %q, want %q", client.startReviewWorkspace, workspaceRoot)
	}
	if client.startReviewSlug != "demo" || client.startReviewRound != 7 {
		t.Fatalf(
			"unexpected review run target: slug=%q round=%d",
			client.startReviewSlug,
			client.startReviewRound,
		)
	}
	if client.startReviewReq.PresentationMode != attachModeDetach {
		t.Fatalf("presentation mode = %q, want %q", client.startReviewReq.PresentationMode, attachModeDetach)
	}

	var overrides daemonRuntimeOverrides
	if err := json.Unmarshal(client.startReviewReq.RuntimeOverrides, &overrides); err != nil {
		t.Fatalf("decode runtime overrides: %v", err)
	}
	if overrides.DryRun == nil || !*overrides.DryRun {
		t.Fatalf("expected dry-run override, got %#v", overrides)
	}
	if overrides.AutoCommit == nil || !*overrides.AutoCommit {
		t.Fatalf("expected auto-commit override, got %#v", overrides)
	}
	if overrides.IDE == nil || *overrides.IDE != "codex" {
		t.Fatalf("expected IDE override, got %#v", overrides)
	}
	if overrides.Model == nil || *overrides.Model != "gpt-5.4" {
		t.Fatalf("expected model override, got %#v", overrides)
	}
	if overrides.ReasoningEffort == nil || *overrides.ReasoningEffort != "high" {
		t.Fatalf("expected reasoning-effort override, got %#v", overrides)
	}
	if overrides.AccessMode == nil || *overrides.AccessMode != "default" {
		t.Fatalf("expected access-mode override, got %#v", overrides)
	}
	if overrides.Timeout == nil || *overrides.Timeout != "2m" {
		t.Fatalf("expected timeout override, got %#v", overrides)
	}
	if overrides.MaxRetries == nil || *overrides.MaxRetries != 3 {
		t.Fatalf("expected max-retries override, got %#v", overrides)
	}
	if overrides.RetryBackoffMultiplier == nil || *overrides.RetryBackoffMultiplier != 2.5 {
		t.Fatalf("expected retry-backoff-multiplier override, got %#v", overrides)
	}

	var batching struct {
		BatchSize       int  `json:"batch_size"`
		Concurrent      int  `json:"concurrent"`
		IncludeResolved bool `json:"include_resolved"`
	}
	if err := json.Unmarshal(client.startReviewReq.Batching, &batching); err != nil {
		t.Fatalf("decode batching overrides: %v", err)
	}
	if batching.BatchSize != 2 || batching.Concurrent != 4 || !batching.IncludeResolved {
		t.Fatalf("unexpected batching overrides: %#v", batching)
	}
}

func TestReviewsFixCommandAutoAttachStreamsWhenNonInteractive(t *testing.T) {
	workspaceRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspaceRoot, ".compozy", "tasks", "demo"), 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}
	withWorkingDir(t, workspaceRoot)

	client := &reviewExecCaptureClient{
		stubDaemonCommandClient: &stubDaemonCommandClient{
			health: apicore.DaemonHealth{Ready: true},
			reviewLatest: apicore.ReviewSummary{
				WorkflowSlug: "demo",
				RoundNumber:  7,
				Provider:     "ext-review",
				PRRef:        "259",
			},
			reviewRun: apicore.Run{
				RunID:            "review-run-stream-007",
				Mode:             "review",
				Status:           "starting",
				PresentationMode: attachModeStream,
				StartedAt:        time.Date(2026, 4, 18, 12, 3, 0, 0, time.UTC),
			},
		},
	}
	installTestCLIReadyDaemonBootstrap(t, client)

	var watchedRunID string
	installTestCLIRunObservers(
		t,
		nil,
		func(_ context.Context, dst io.Writer, _ daemonCommandClient, runID string) error {
			watchedRunID = runID
			_, _ = io.WriteString(dst, "watching "+runID+"\n")
			return nil
		},
	)

	output, err := executeCommandCombinedOutput(
		newReviewsCommandWithDefaults(testReviewExecCommandDefaults()),
		nil,
		"fix",
		"--name",
		"demo",
	)
	if err != nil {
		t.Fatalf("execute reviews fix auto stream: %v\noutput:\n%s", err, output)
	}
	if client.startReviewReq.PresentationMode != attachModeStream {
		t.Fatalf("presentation mode = %q, want %q", client.startReviewReq.PresentationMode, attachModeStream)
	}
	if watchedRunID != "review-run-stream-007" {
		t.Fatalf("expected watch for review-run-stream-007, got %q", watchedRunID)
	}
	if !containsAll(output, "task run started: review-run-stream-007 (mode=stream)", "watching review-run-stream-007") {
		t.Fatalf("unexpected reviews fix auto stream output:\n%s", output)
	}
}

func TestReviewsExecDaemonHelperFunctions(t *testing.T) {
	t.Run("resolve review round prefers reviews dir", func(t *testing.T) {
		state := newCommandState(commandKindFixReviews, core.ModePRReview)
		state.reviewsDir = filepath.Join(t.TempDir(), "reviews-009")
		if err := state.resolveReviewRound(context.Background()); err != nil {
			t.Fatalf("resolveReviewRound() error = %v", err)
		}
		if state.round != 9 {
			t.Fatalf("state.round = %d, want 9", state.round)
		}
	})

	t.Run("resolve workflow name and round args validates positionals", func(t *testing.T) {
		state := newCommandState(commandKindFetchReviews, core.ModePRReview)
		cmd := &cobra.Command{Use: "reviews fetch"}
		if err := state.resolveWorkflowNameAndRoundArgs(cmd, []string{"demo", "3"}); err != nil {
			t.Fatalf("resolveWorkflowNameAndRoundArgs() error = %v", err)
		}
		if state.name != "demo" || state.round != 3 {
			t.Fatalf("state = %#v, want name demo round 3", state.workflowIdentity)
		}
	})

	t.Run("exec translation and lean helpers preserve compatibility payloads", func(t *testing.T) {
		workspaceRoot := t.TempDir()
		homeDir := t.TempDir()
		t.Setenv("HOME", homeDir)

		runArtifactsDir := persistedRunDirForCLI(t, workspaceRoot, "exec-translate")
		if err := os.MkdirAll(filepath.Join(runArtifactsDir, "turns", "0001"), 0o755); err != nil {
			t.Fatalf("mkdir turns dir: %v", err)
		}
		writePersistedExecRunForCLI(t, workspaceRoot, coreRun.PersistedExecRun{
			Version:       1,
			Mode:          "exec",
			RunID:         "exec-translate",
			Status:        "completed",
			WorkspaceRoot: workspaceRoot,
			TurnCount:     1,
			TurnsDir:      filepath.Join(runArtifactsDir, "turns"),
		})
		if err := os.WriteFile(
			filepath.Join(runArtifactsDir, "turns", "0001", "response.txt"),
			[]byte("assistant output"),
			0o600,
		); err != nil {
			t.Fatalf("write response.txt: %v", err)
		}

		started, err := translateDaemonExecEvent(
			workspaceRoot,
			"exec-translate",
			eventspkg.Event{
				Kind:      eventspkg.EventKindRunStarted,
				Timestamp: time.Date(2026, 4, 18, 12, 3, 0, 0, time.UTC),
			},
			false,
			true,
		)
		if err != nil {
			t.Fatalf("translateDaemonExecEvent(run.started) error = %v", err)
		}
		if got := started[0]["type"]; got != "run.started" {
			t.Fatalf("run.started type = %v, want run.started", got)
		}
		if got := started[0]["dry_run"]; got != true {
			t.Fatalf("run.started dry_run = %v, want true", got)
		}

		sessionStartedPayload, err := json.Marshal(kinds.SessionStartedPayload{
			ACPSessionID:   "acp-123",
			AgentSessionID: "agent-123",
			Resumed:        true,
		})
		if err != nil {
			t.Fatalf("marshal SessionStartedPayload: %v", err)
		}
		sessionStarted, err := translateDaemonExecEvent(
			workspaceRoot,
			"exec-translate",
			eventspkg.Event{
				Kind:      eventspkg.EventKindSessionStarted,
				Timestamp: time.Date(2026, 4, 18, 12, 3, 1, 0, time.UTC),
				Payload:   sessionStartedPayload,
			},
			false,
			false,
		)
		if err != nil {
			t.Fatalf("translateDaemonExecEvent(session.started) error = %v", err)
		}
		session, ok := sessionStarted[0]["session"].(map[string]any)
		if !ok || session["acp_session_id"] != "acp-123" || session["agent_session_id"] != "agent-123" {
			t.Fatalf("unexpected session payload: %#v", sessionStarted)
		}

		textBlock, err := kinds.NewContentBlock(kinds.TextBlock{Text: "chunk"})
		if err != nil {
			t.Fatalf("NewContentBlock() error = %v", err)
		}
		sessionUpdatePayload, err := json.Marshal(kinds.SessionUpdatePayload{
			Update: kinds.SessionUpdate{
				Kind:   kinds.UpdateKindAgentMessageChunk,
				Status: kinds.StatusRunning,
				Blocks: []kinds.ContentBlock{textBlock},
			},
		})
		if err != nil {
			t.Fatalf("marshal SessionUpdatePayload: %v", err)
		}
		sessionUpdate, err := translateDaemonExecEvent(
			workspaceRoot,
			"exec-translate",
			eventspkg.Event{
				Kind:      eventspkg.EventKindSessionUpdate,
				Timestamp: time.Date(2026, 4, 18, 12, 3, 2, 0, time.UTC),
				Payload:   sessionUpdatePayload,
			},
			false,
			false,
		)
		if err != nil {
			t.Fatalf("translateDaemonExecEvent(session.update) error = %v", err)
		}
		if got := sessionUpdate[0]["type"]; got != "session.update" {
			t.Fatalf("session.update type = %v, want session.update", got)
		}

		completedPayload, err := json.Marshal(kinds.RunCompletedPayload{})
		if err != nil {
			t.Fatalf("marshal RunCompletedPayload: %v", err)
		}
		completed, err := translateDaemonExecEvent(
			workspaceRoot,
			"exec-translate",
			eventspkg.Event{
				Kind:      eventspkg.EventKindRunCompleted,
				Timestamp: time.Date(2026, 4, 18, 12, 3, 3, 0, time.UTC),
				Payload:   completedPayload,
			},
			false,
			false,
		)
		if err != nil {
			t.Fatalf("translateDaemonExecEvent(run.completed) error = %v", err)
		}
		if got := completed[0]["type"]; got != "run.succeeded" {
			t.Fatalf("run.completed type = %v, want run.succeeded", got)
		}
		if got := completed[0]["output"]; got != "assistant output" {
			t.Fatalf("run.completed output = %v, want assistant output", got)
		}

		failedPayload, err := json.Marshal(kinds.RunFailedPayload{Error: "boom"})
		if err != nil {
			t.Fatalf("marshal RunFailedPayload: %v", err)
		}
		failed, err := translateDaemonExecEvent(
			workspaceRoot,
			"exec-translate",
			eventspkg.Event{
				Kind:      eventspkg.EventKindRunFailed,
				Timestamp: time.Date(2026, 4, 18, 12, 3, 4, 0, time.UTC),
				Payload:   failedPayload,
			},
			false,
			false,
		)
		if err != nil {
			t.Fatalf("translateDaemonExecEvent(run.failed) error = %v", err)
		}
		if got := failed[0]["status"]; got != execStatusFailed {
			t.Fatalf("run.failed status = %v, want %q", got, execStatusFailed)
		}

		cancelledPayload, err := json.Marshal(kinds.RunCancelledPayload{Reason: "canceled"})
		if err != nil {
			t.Fatalf("marshal RunCancelledPayload: %v", err)
		}
		cancelledErr := workflowTerminalError(eventspkg.Event{
			Kind:    eventspkg.EventKindRunCancelled,
			Payload: cancelledPayload,
		})
		if cancelledErr == nil || cancelledErr.Error() != "canceled" {
			t.Fatalf("workflowTerminalError(canceled) = %v, want canceled", cancelledErr)
		}
		crashedPayload, err := json.Marshal(kinds.RunCrashedPayload{Error: "crashed"})
		if err != nil {
			t.Fatalf("marshal RunCrashedPayload: %v", err)
		}
		crashedErr := execTerminalError(eventspkg.Event{
			Kind:    eventspkg.EventKindRunCrashed,
			Payload: crashedPayload,
		})
		if crashedErr == nil || crashedErr.Error() != "crashed" {
			t.Fatalf("execTerminalError(crashed) = %v, want crashed", crashedErr)
		}

		if !shouldEmitLeanExecUpdate(kinds.SessionUpdate{Kind: kinds.UpdateKindToolCallStarted}) {
			t.Fatal("expected tool-call updates to emit in lean mode")
		}
		if !shouldEmitLeanExecUpdate(
			kinds.SessionUpdate{Kind: kinds.UpdateKindUnknown, Status: kinds.StatusCompleted},
		) {
			t.Fatal("expected completed unknown updates to emit in lean mode")
		}
		if shouldEmitLeanExecUpdate(kinds.SessionUpdate{Kind: kinds.UpdateKindUnknown, Status: kinds.StatusRunning}) {
			t.Fatal("expected running unknown updates to stay hidden in lean mode")
		}
		if !shouldEmitLeanWorkflowEvent(eventspkg.Event{Kind: eventspkg.EventKindRunStarted}) {
			t.Fatal("expected run.started to emit in lean workflow mode")
		}
		if !shouldEmitLeanWorkflowEvent(eventspkg.Event{
			Kind:    eventspkg.EventKindSessionUpdate,
			Payload: sessionUpdatePayload,
		}) {
			t.Fatal("expected session.update with lean exec update to emit in lean workflow mode")
		}
		if shouldEmitLeanWorkflowEvent(eventspkg.Event{
			Kind: eventspkg.EventKindSessionUpdate,
			Payload: json.RawMessage(
				`{"update":{"kind":"unknown","status":"running"}}`,
			),
		}) {
			t.Fatal("expected non-terminal unknown session updates to stay hidden in lean workflow mode")
		}

		if got := reviewRoundDirForWorkflow(workspaceRoot, "demo", 5); got != filepath.Join(
			workspaceRoot,
			".compozy",
			"tasks",
			"demo",
			"reviews-005",
		) {
			t.Fatalf("reviewRoundDirForWorkflow() = %q", got)
		}
		if ptr := intPointerOrNil(0); ptr != nil {
			t.Fatalf("intPointerOrNil(0) = %#v, want nil", ptr)
		}
		if ptr := intPointerOrNil(5); ptr == nil || *ptr != 5 {
			t.Fatalf("intPointerOrNil(5) = %#v, want pointer to 5", ptr)
		}
		if got, err := loadLatestExecTurnResponse(
			filepath.Join(runArtifactsDir, "turns"),
		); err != nil ||
			got != "assistant output" {
			t.Fatalf("loadLatestExecTurnResponse() = (%q, %v), want assistant output", got, err)
		}
		if _, err := decodeDaemonPayload[kinds.RunFailedPayload](json.RawMessage(`{"error":"boom"}`)); err != nil {
			t.Fatalf("decodeDaemonPayload() error = %v", err)
		}
	})

	t.Run("exec translation helpers cover raw, hidden, and terminal variants", func(t *testing.T) {
		workspaceRoot := t.TempDir()
		homeDir := t.TempDir()
		t.Setenv("HOME", homeDir)

		runArtifactsDir := persistedRunDirForCLI(t, workspaceRoot, "exec-helper-variants")
		if err := os.MkdirAll(filepath.Join(runArtifactsDir, "turns", "0002"), 0o755); err != nil {
			t.Fatalf("mkdir turns dir: %v", err)
		}
		writePersistedExecRunForCLI(t, workspaceRoot, coreRun.PersistedExecRun{
			Version:       1,
			Mode:          "exec",
			RunID:         "exec-helper-variants",
			Status:        "completed",
			WorkspaceRoot: workspaceRoot,
			TurnCount:     2,
			TurnsDir:      filepath.Join(runArtifactsDir, "turns"),
		})
		if err := os.WriteFile(
			filepath.Join(runArtifactsDir, "turns", "0002", "response.txt"),
			[]byte("latest turn output"),
			0o600,
		); err != nil {
			t.Fatalf("write response.txt: %v", err)
		}

		hiddenUpdatePayload, err := json.Marshal(kinds.SessionUpdatePayload{
			Update: kinds.SessionUpdate{Kind: kinds.UpdateKindUnknown, Status: kinds.StatusRunning},
		})
		if err != nil {
			t.Fatalf("marshal hidden SessionUpdatePayload: %v", err)
		}
		hiddenEvents, err := translateDaemonExecEvent(
			workspaceRoot,
			"exec-helper-variants",
			eventspkg.Event{
				Kind:      eventspkg.EventKindSessionUpdate,
				Timestamp: time.Date(2026, 4, 18, 12, 20, 0, 0, time.UTC),
				Payload:   hiddenUpdatePayload,
			},
			false,
			false,
		)
		if err != nil {
			t.Fatalf("translateDaemonExecEvent(hidden session.update) error = %v", err)
		}
		if hiddenEvents != nil {
			t.Fatalf("expected hidden session update to produce no output, got %#v", hiddenEvents)
		}

		rawEvents, err := translateDaemonExecEvent(
			workspaceRoot,
			"exec-helper-variants",
			eventspkg.Event{
				Kind:      eventspkg.EventKindSessionUpdate,
				Timestamp: time.Date(2026, 4, 18, 12, 20, 1, 0, time.UTC),
				Payload:   hiddenUpdatePayload,
			},
			true,
			false,
		)
		if err != nil {
			t.Fatalf("translateDaemonExecEvent(raw session.update) error = %v", err)
		}
		if len(rawEvents) != 1 || rawEvents[0]["type"] != "session.update" {
			t.Fatalf("unexpected raw session update output: %#v", rawEvents)
		}

		cancelledPayload, err := json.Marshal(kinds.RunCancelledPayload{Reason: "stopped"})
		if err != nil {
			t.Fatalf("marshal RunCancelledPayload: %v", err)
		}
		cancelledEvents, err := translateDaemonExecTerminalEvent(
			workspaceRoot,
			"exec-helper-variants",
			eventspkg.Event{
				Kind:      eventspkg.EventKindRunCancelled,
				Timestamp: time.Date(2026, 4, 18, 12, 20, 2, 0, time.UTC),
				Payload:   cancelledPayload,
			},
		)
		if err != nil {
			t.Fatalf("translateDaemonExecTerminalEvent(canceled) error = %v", err)
		}
		if len(cancelledEvents) != 1 || cancelledEvents[0]["status"] != execStatusCanceled {
			t.Fatalf("unexpected canceled output: %#v", cancelledEvents)
		}

		crashedPayload, err := json.Marshal(kinds.RunCrashedPayload{Error: "panic"})
		if err != nil {
			t.Fatalf("marshal RunCrashedPayload: %v", err)
		}
		crashedEvents, err := translateDaemonExecTerminalEvent(
			workspaceRoot,
			"exec-helper-variants",
			eventspkg.Event{
				Kind:      eventspkg.EventKindRunCrashed,
				Timestamp: time.Date(2026, 4, 18, 12, 20, 3, 0, time.UTC),
				Payload:   crashedPayload,
			},
		)
		if err != nil {
			t.Fatalf("translateDaemonExecTerminalEvent(crashed) error = %v", err)
		}
		if len(crashedEvents) != 1 || crashedEvents[0]["status"] != execStatusCrashed {
			t.Fatalf("unexpected crashed output: %#v", crashedEvents)
		}

		unsupportedEvents, err := translateDaemonExecEvent(
			workspaceRoot,
			"exec-helper-variants",
			eventspkg.Event{
				Kind:      eventspkg.EventKindJobStarted,
				Timestamp: time.Date(2026, 4, 18, 12, 20, 4, 0, time.UTC),
			},
			true,
			false,
		)
		if err != nil {
			t.Fatalf("translateDaemonExecEvent(unsupported) error = %v", err)
		}
		if unsupportedEvents != nil {
			t.Fatalf("expected unsupported event to stay hidden, got %#v", unsupportedEvents)
		}

		if got, err := loadExecResponseText(
			workspaceRoot,
			"exec-helper-variants",
		); err != nil ||
			got != "latest turn output" {
			t.Fatalf("loadExecResponseText() = (%q, %v), want latest turn output", got, err)
		}
		if got, err := loadLatestExecTurnResponse(
			filepath.Join(runArtifactsDir, "missing-turns"),
		); err != nil ||
			got != "" {
			t.Fatalf("loadLatestExecTurnResponse(missing) = (%q, %v), want empty nil", got, err)
		}
		if joined := filepathJoin("", "one", "", "two"); joined != filepath.Join("one", "two") {
			t.Fatalf("filepathJoin() = %q, want %q", joined, filepath.Join("one", "two"))
		}
		if base := filepathBase(filepath.Join("one", "two", "")); base != "two" {
			t.Fatalf("filepathBase() = %q, want two", base)
		}
	})

	t.Run("workflow argument parsing reports invalid combinations", func(t *testing.T) {
		state := newCommandState(commandKindFetchReviews, core.ModePRReview)
		cmd := &cobra.Command{Use: "reviews fetch"}
		if err := state.resolveWorkflowNameArg(
			cmd,
			nil,
		); err == nil ||
			!strings.Contains(err.Error(), "requires --name") {
			t.Fatalf("resolveWorkflowNameArg(fetch) error = %v, want requires --name", err)
		}

		state = newCommandState(commandKindExec, core.ModeExec)
		cmd = &cobra.Command{Use: "reviews"}
		if err := state.resolveWorkflowNameArg(cmd, nil); err == nil ||
			!strings.Contains(err.Error(), "requires a workflow slug") {
			t.Fatalf("resolveWorkflowNameArg(default) error = %v, want workflow slug error", err)
		}

		state = newCommandState(commandKindFetchReviews, core.ModePRReview)
		cmd = &cobra.Command{Use: "reviews fetch"}
		if err := state.resolveWorkflowNameAndRoundArgs(cmd, []string{"demo", "zero"}); err == nil ||
			!strings.Contains(err.Error(), "positive integer") {
			t.Fatalf("resolveWorkflowNameAndRoundArgs(invalid) error = %v, want positive integer error", err)
		}

		state = newCommandState(commandKindFetchReviews, core.ModePRReview)
		if err := state.resolveWorkflowNameAndRoundArgs(cmd, []string{"demo"}); err == nil ||
			!strings.Contains(err.Error(), "review round is required") {
			t.Fatalf("resolveWorkflowNameAndRoundArgs(missing round) error = %v, want review round required", err)
		}
	})
}

func TestExecCommandUsesDaemonLifecycleAcrossFormats(t *testing.T) {
	t.Run("text mode waits for terminal snapshot and prints persisted output", func(t *testing.T) {
		homeDir := t.TempDir()
		workspaceRoot := t.TempDir()
		t.Setenv("HOME", homeDir)
		writeCLIWorkspaceConfig(t, workspaceRoot, "")
		withWorkingDir(t, workspaceRoot)

		runID := "exec-text-001"
		runArtifactsDir := persistedRunDirForCLI(t, workspaceRoot, runID)
		if err := os.MkdirAll(filepath.Join(runArtifactsDir, "turns", "0001"), 0o755); err != nil {
			t.Fatalf("mkdir turns dir: %v", err)
		}
		writePersistedExecRunForCLI(t, workspaceRoot, coreRun.PersistedExecRun{
			Version:       1,
			Mode:          "exec",
			RunID:         runID,
			Status:        "completed",
			WorkspaceRoot: workspaceRoot,
			TurnCount:     1,
			TurnsDir:      filepath.Join(runArtifactsDir, "turns"),
		})
		if err := os.WriteFile(
			filepath.Join(runArtifactsDir, "turns", "0001", "response.txt"),
			[]byte("final assistant output"),
			0o600,
		); err != nil {
			t.Fatalf("write response.txt: %v", err)
		}

		stream := newStaticClientRunStream()
		stream.items <- apiclient.RunStreamItem{
			Event: &eventspkg.Event{
				Kind:      eventspkg.EventKindRunCompleted,
				Timestamp: time.Date(2026, 4, 18, 12, 10, 0, 0, time.UTC),
			},
		}
		close(stream.items)

		client := &reviewExecCaptureClient{
			stubDaemonCommandClient: &stubDaemonCommandClient{
				health: apicore.DaemonHealth{Ready: true},
				execRun: apicore.Run{
					RunID:            runID,
					Mode:             "exec",
					Status:           "starting",
					PresentationMode: attachModeDetach,
				},
				snapshot: apicore.RunSnapshot{
					Run: apicore.Run{
						RunID:  runID,
						Status: "completed",
					},
				},
				stream: stream,
			},
		}
		installTestCLIReadyDaemonBootstrap(t, client)

		output, err := executeCommandCombinedOutput(
			newExecCommandWithDefaults(testReviewExecCommandDefaults()),
			nil,
			"Summarize the repository state",
		)
		if err != nil {
			t.Fatalf("execute exec text mode: %v\noutput:\n%s", err, output)
		}
		if strings.TrimSpace(output) != "final assistant output" {
			t.Fatalf("unexpected exec text output: %q", output)
		}
		if mustEvalSymlinksCLITest(t, client.startExecReq.WorkspacePath) != mustEvalSymlinksCLITest(t, workspaceRoot) {
			t.Fatalf("workspace = %q, want %q", client.startExecReq.WorkspacePath, workspaceRoot)
		}
		if client.startExecReq.Prompt != "Summarize the repository state" {
			t.Fatalf("prompt = %q, want positional prompt", client.startExecReq.Prompt)
		}
		if client.startExecReq.PresentationMode != attachModeDetach {
			t.Fatalf("presentation mode = %q, want %q", client.startExecReq.PresentationMode, attachModeDetach)
		}
		if client.startExecReq.RuntimeOverrides != nil {
			t.Fatalf(
				"expected no runtime overrides for default text mode, got %s",
				client.startExecReq.RuntimeOverrides,
			)
		}
	})

	t.Run("json mode preserves stdin prompt and runtime overrides", func(t *testing.T) {
		homeDir := t.TempDir()
		workspaceRoot := t.TempDir()
		t.Setenv("HOME", homeDir)
		writeCLIWorkspaceConfig(t, workspaceRoot, "")
		withWorkingDir(t, workspaceRoot)

		runID := "exec-json-001"
		runArtifactsDir := persistedRunDirForCLI(t, workspaceRoot, runID)
		if err := os.MkdirAll(filepath.Join(runArtifactsDir, "turns", "0001"), 0o755); err != nil {
			t.Fatalf("mkdir turns dir: %v", err)
		}
		writePersistedExecRunForCLI(t, workspaceRoot, coreRun.PersistedExecRun{
			Version:       1,
			Mode:          "exec",
			RunID:         runID,
			Status:        "completed",
			WorkspaceRoot: workspaceRoot,
			TurnCount:     1,
			TurnsDir:      filepath.Join(runArtifactsDir, "turns"),
		})
		if err := os.WriteFile(
			filepath.Join(runArtifactsDir, "turns", "0001", "response.txt"),
			[]byte("json assistant output"),
			0o600,
		); err != nil {
			t.Fatalf("write response.txt: %v", err)
		}

		textBlock, err := kinds.NewContentBlock(kinds.TextBlock{Text: "chunk"})
		if err != nil {
			t.Fatalf("NewContentBlock() error = %v", err)
		}
		sessionUpdatePayload, err := json.Marshal(kinds.SessionUpdatePayload{
			Update: kinds.SessionUpdate{
				Kind:   kinds.UpdateKindAgentMessageChunk,
				Status: kinds.StatusRunning,
				Blocks: []kinds.ContentBlock{textBlock},
			},
		})
		if err != nil {
			t.Fatalf("marshal SessionUpdatePayload: %v", err)
		}

		stream := newStaticClientRunStream()
		stream.items <- apiclient.RunStreamItem{
			Event: &eventspkg.Event{
				Kind:      eventspkg.EventKindRunStarted,
				Timestamp: time.Date(2026, 4, 18, 12, 11, 0, 0, time.UTC),
			},
		}
		stream.items <- apiclient.RunStreamItem{
			Event: &eventspkg.Event{
				Kind:      eventspkg.EventKindSessionUpdate,
				Timestamp: time.Date(2026, 4, 18, 12, 11, 1, 0, time.UTC),
				Payload:   sessionUpdatePayload,
			},
		}
		stream.items <- apiclient.RunStreamItem{
			Event: &eventspkg.Event{
				Kind:      eventspkg.EventKindRunCompleted,
				Timestamp: time.Date(2026, 4, 18, 12, 11, 2, 0, time.UTC),
			},
		}
		close(stream.items)

		client := &reviewExecCaptureClient{
			stubDaemonCommandClient: &stubDaemonCommandClient{
				health: apicore.DaemonHealth{Ready: true},
				execRun: apicore.Run{
					RunID:            runID,
					Mode:             "exec",
					Status:           "starting",
					PresentationMode: attachModeStream,
				},
				stream: stream,
			},
		}
		installTestCLIReadyDaemonBootstrap(t, client)

		output, err := executeCommandCombinedOutput(
			newExecCommandWithDefaults(testReviewExecCommandDefaults()),
			strings.NewReader("Prompt from stdin\n"),
			"--format",
			"json",
			"--verbose",
			"--persist",
			"--extensions",
			"--dry-run",
		)
		if err != nil {
			t.Fatalf("execute exec json mode: %v\noutput:\n%s", err, output)
		}
		events := decodeExecJSONLEvents(t, output)
		if len(events) != 3 {
			t.Fatalf("expected 3 json events, got %d\noutput:\n%s", len(events), output)
		}
		if got := events[0]["type"]; got != "run.started" {
			t.Fatalf("unexpected first json event: %#v", events[0])
		}
		if got := events[1]["type"]; got != "session.update" {
			t.Fatalf("unexpected session update event: %#v", events[1])
		}
		if got := events[2]["type"]; got != "run.succeeded" || events[2]["output"] != "json assistant output" {
			t.Fatalf("unexpected terminal event: %#v", events[2])
		}
		if client.startExecReq.Prompt != "Prompt from stdin\n" {
			t.Fatalf("prompt = %q, want stdin prompt", client.startExecReq.Prompt)
		}
		if client.startExecReq.PresentationMode != attachModeStream {
			t.Fatalf("presentation mode = %q, want %q", client.startExecReq.PresentationMode, attachModeStream)
		}

		var overrides daemonRuntimeOverrides
		if err := json.Unmarshal(client.startExecReq.RuntimeOverrides, &overrides); err != nil {
			t.Fatalf("decode runtime overrides: %v", err)
		}
		if overrides.OutputFormat == nil || *overrides.OutputFormat != "json" {
			t.Fatalf("expected json format override, got %#v", overrides)
		}
		if overrides.Verbose == nil || !*overrides.Verbose {
			t.Fatalf("expected verbose override, got %#v", overrides)
		}
		if overrides.Persist == nil || !*overrides.Persist {
			t.Fatalf("expected persist override, got %#v", overrides)
		}
		if overrides.EnableExecutableExtensions == nil || !*overrides.EnableExecutableExtensions {
			t.Fatalf("expected extensions override, got %#v", overrides)
		}
		if overrides.DryRun == nil || !*overrides.DryRun {
			t.Fatalf("expected dry-run override, got %#v", overrides)
		}
	})

	t.Run("raw-json mode preserves prompt-file input", func(t *testing.T) {
		homeDir := t.TempDir()
		workspaceRoot := t.TempDir()
		t.Setenv("HOME", homeDir)
		writeCLIWorkspaceConfig(t, workspaceRoot, "")
		withWorkingDir(t, workspaceRoot)

		runID := "exec-raw-001"
		runArtifactsDir := persistedRunDirForCLI(t, workspaceRoot, runID)
		if err := os.MkdirAll(filepath.Join(runArtifactsDir, "turns", "0001"), 0o755); err != nil {
			t.Fatalf("mkdir turns dir: %v", err)
		}
		writePersistedExecRunForCLI(t, workspaceRoot, coreRun.PersistedExecRun{
			Version:       1,
			Mode:          "exec",
			RunID:         runID,
			Status:        "completed",
			WorkspaceRoot: workspaceRoot,
			TurnCount:     1,
			TurnsDir:      filepath.Join(runArtifactsDir, "turns"),
		})
		if err := os.WriteFile(
			filepath.Join(runArtifactsDir, "turns", "0001", "response.txt"),
			[]byte("raw assistant output"),
			0o600,
		); err != nil {
			t.Fatalf("write response.txt: %v", err)
		}

		promptPath := filepath.Join(workspaceRoot, "prompt.md")
		if err := os.WriteFile(promptPath, []byte("Prompt from file\n"), 0o600); err != nil {
			t.Fatalf("write prompt file: %v", err)
		}

		stream := newStaticClientRunStream()
		stream.items <- apiclient.RunStreamItem{
			Event: &eventspkg.Event{
				Kind:      eventspkg.EventKindRunStarted,
				Timestamp: time.Date(2026, 4, 18, 12, 12, 0, 0, time.UTC),
			},
		}
		stream.items <- apiclient.RunStreamItem{
			Event: &eventspkg.Event{
				Kind:      eventspkg.EventKindRunCompleted,
				Timestamp: time.Date(2026, 4, 18, 12, 12, 1, 0, time.UTC),
			},
		}
		close(stream.items)

		client := &reviewExecCaptureClient{
			stubDaemonCommandClient: &stubDaemonCommandClient{
				health: apicore.DaemonHealth{Ready: true},
				execRun: apicore.Run{
					RunID:            runID,
					Mode:             "exec",
					Status:           "starting",
					PresentationMode: attachModeStream,
				},
				stream: stream,
			},
		}
		installTestCLIReadyDaemonBootstrap(t, client)

		output, err := executeCommandCombinedOutput(
			newExecCommandWithDefaults(testReviewExecCommandDefaults()),
			nil,
			"--format",
			"raw-json",
			"--prompt-file",
			promptPath,
		)
		if err != nil {
			t.Fatalf("execute exec raw-json mode: %v\noutput:\n%s", err, output)
		}
		events := decodeExecJSONLEvents(t, output)
		if len(events) != 2 {
			t.Fatalf("expected 2 raw events, got %d\noutput:\n%s", len(events), output)
		}
		if got := events[0]["type"]; got != "run.started" {
			t.Fatalf("unexpected first raw event: %#v", events[0])
		}
		if got := events[1]["type"]; got != "run.succeeded" || events[1]["output"] != "raw assistant output" {
			t.Fatalf("unexpected terminal raw event: %#v", events[1])
		}
		if client.startExecReq.Prompt != "Prompt from file\n" {
			t.Fatalf("prompt = %q, want prompt-file contents", client.startExecReq.Prompt)
		}
		if client.startExecReq.PresentationMode != attachModeStream {
			t.Fatalf("presentation mode = %q, want %q", client.startExecReq.PresentationMode, attachModeStream)
		}
	})
}

func TestReviewsExecDaemonStreamHelpers(t *testing.T) {
	t.Run("workflow stream emits lean events and terminal failures", func(t *testing.T) {
		updatePayload, err := json.Marshal(kinds.SessionUpdatePayload{
			Update: kinds.SessionUpdate{Kind: kinds.UpdateKindToolCallStarted},
		})
		if err != nil {
			t.Fatalf("marshal SessionUpdatePayload: %v", err)
		}
		failedPayload, err := json.Marshal(kinds.RunFailedPayload{Error: "workflow boom"})
		if err != nil {
			t.Fatalf("marshal RunFailedPayload: %v", err)
		}

		stream := newStaticClientRunStream()
		stream.items <- apiclient.RunStreamItem{Overflow: &apiclient.RunStreamOverflow{Reason: "lagging"}}
		stream.items <- apiclient.RunStreamItem{
			Event: &eventspkg.Event{
				Kind:      eventspkg.EventKindRunStarted,
				Timestamp: time.Date(2026, 4, 18, 12, 13, 0, 0, time.UTC),
			},
		}
		stream.items <- apiclient.RunStreamItem{
			Event: &eventspkg.Event{
				Kind:      eventspkg.EventKindSessionUpdate,
				Timestamp: time.Date(2026, 4, 18, 12, 13, 1, 0, time.UTC),
				Payload:   updatePayload,
			},
		}
		stream.items <- apiclient.RunStreamItem{
			Event: &eventspkg.Event{
				Kind:      eventspkg.EventKindRunFailed,
				Timestamp: time.Date(2026, 4, 18, 12, 13, 2, 0, time.UTC),
				Payload:   failedPayload,
			},
		}
		close(stream.items)

		client := &stubDaemonCommandClient{stream: stream}
		state := newCommandState(commandKindFixReviews, core.ModePRReview)
		var dst bytes.Buffer
		err = state.streamDaemonWorkflowEvents(context.Background(), &dst, client, "workflow-run-1", false)
		if err == nil || !strings.Contains(err.Error(), "workflow boom") {
			t.Fatalf("streamDaemonWorkflowEvents() error = %v, want workflow boom", err)
		}
		events := decodeExecJSONLEvents(t, dst.String())
		if len(events) != 3 {
			t.Fatalf("expected 3 workflow events, got %d\noutput:\n%s", len(events), dst.String())
		}
		if events[0]["type"] != "run.started" || events[1]["type"] != "session.update" ||
			events[2]["type"] != string(eventspkg.EventKindRunFailed) {
			t.Fatalf("unexpected workflow events: %#v", events)
		}
	})

	t.Run("exec stream emits translated terminal failures", func(t *testing.T) {
		stream := newStaticClientRunStream()
		failedPayload, err := json.Marshal(kinds.RunFailedPayload{Error: "exec boom"})
		if err != nil {
			t.Fatalf("marshal RunFailedPayload: %v", err)
		}
		stream.items <- apiclient.RunStreamItem{
			Event: &eventspkg.Event{
				Kind:      eventspkg.EventKindRunFailed,
				Timestamp: time.Date(2026, 4, 18, 12, 16, 0, 0, time.UTC),
				Payload:   failedPayload,
			},
		}
		close(stream.items)

		state := newCommandState(commandKindExec, core.ModeExec)
		client := &stubDaemonCommandClient{stream: stream}
		var dst bytes.Buffer
		err = state.streamDaemonExecEvents(context.Background(), &dst, client, "exec-stream-1", false)
		if err == nil || !strings.Contains(err.Error(), "exec boom") {
			t.Fatalf("streamDaemonExecEvents() error = %v, want exec boom", err)
		}
		events := decodeExecJSONLEvents(t, dst.String())
		if len(events) != 1 || events[0]["status"] != execStatusFailed {
			t.Fatalf("unexpected exec stream output: %#v", events)
		}
	})

	t.Run("consume stream and wait helpers handle errors and fallback snapshots", func(t *testing.T) {
		errStream := newStaticClientRunStream()
		errStream.errors <- errors.New("stream failed")
		close(errStream.errors)

		client := &stubDaemonCommandClient{stream: errStream}
		err := consumeDaemonRunStream(context.Background(), client, "run-err", func(apiclient.RunStreamItem) error {
			return nil
		})
		if err == nil || !strings.Contains(err.Error(), "stream failed") {
			t.Fatalf("consumeDaemonRunStream() error = %v, want stream failed", err)
		}

		fallbackStream := newStaticClientRunStream()
		fallbackStream.items <- apiclient.RunStreamItem{
			Event: &eventspkg.Event{
				Kind:      eventspkg.EventKindRunStarted,
				Timestamp: time.Date(2026, 4, 18, 12, 14, 0, 0, time.UTC),
			},
		}
		close(fallbackStream.items)

		client = &stubDaemonCommandClient{
			stream: fallbackStream,
			snapshot: apicore.RunSnapshot{
				Run: apicore.Run{
					RunID:  "run-fallback",
					Status: "completed",
				},
			},
		}
		run, err := waitForDaemonRunTerminal(context.Background(), client, "run-fallback")
		if err != nil {
			t.Fatalf("waitForDaemonRunTerminal() error = %v", err)
		}
		if run.Status != "completed" {
			t.Fatalf("terminal status = %q, want completed", run.Status)
		}
	})

	t.Run("waitForDaemonRunTerminal waits for durable terminal snapshot after terminal event", func(t *testing.T) {
		stream := newStaticClientRunStream()
		stream.items <- apiclient.RunStreamItem{
			Event: &eventspkg.Event{
				Kind:      eventspkg.EventKindRunCompleted,
				Timestamp: time.Date(2026, 4, 18, 12, 14, 30, 0, time.UTC),
			},
		}
		close(stream.items)

		snapshotCalls := 0
		client := &stubDaemonCommandClient{
			stream: stream,
			snapshotFunc: func(context.Context, string) (apicore.RunSnapshot, error) {
				snapshotCalls++
				status := "running"
				if snapshotCalls >= 3 {
					status = "completed"
				}
				return apicore.RunSnapshot{
					Run: apicore.Run{
						RunID:  "run-terminal-late",
						Status: status,
					},
				}, nil
			},
		}

		run, err := waitForDaemonRunTerminal(context.Background(), client, "run-terminal-late")
		if err != nil {
			t.Fatalf("waitForDaemonRunTerminal() error = %v", err)
		}
		if run.Status != "completed" {
			t.Fatalf("terminal status = %q, want completed", run.Status)
		}
		if snapshotCalls < 3 {
			t.Fatalf("snapshot calls = %d, want at least 3 polls", snapshotCalls)
		}
	})

	t.Run("waitAndPrintExecResult surfaces terminal failures", func(t *testing.T) {
		stream := newStaticClientRunStream()
		failedPayload, err := json.Marshal(kinds.RunFailedPayload{Error: "exec failed"})
		if err != nil {
			t.Fatalf("marshal RunFailedPayload: %v", err)
		}
		stream.items <- apiclient.RunStreamItem{
			Event: &eventspkg.Event{
				Kind:      eventspkg.EventKindRunFailed,
				Timestamp: time.Date(2026, 4, 18, 12, 15, 0, 0, time.UTC),
				Payload:   failedPayload,
			},
		}
		close(stream.items)

		state := newCommandState(commandKindExec, core.ModeExec)
		client := &stubDaemonCommandClient{
			stream: stream,
			snapshot: apicore.RunSnapshot{
				Run: apicore.Run{
					RunID:     "run-failed",
					Status:    execStatusFailed,
					ErrorText: "exec failed",
				},
			},
		}
		err = state.waitAndPrintExecResult(context.Background(), &bytes.Buffer{}, client, "run-failed")
		if err == nil || !strings.Contains(err.Error(), "exec failed") {
			t.Fatalf("waitAndPrintExecResult() error = %v, want exec failed", err)
		}
	})

	t.Run("resolveExecPresentationMode enforces tui interactivity", func(t *testing.T) {
		state := newCommandState(commandKindExec, core.ModeExec)
		state.tui = true
		state.isInteractive = func() bool { return false }
		cmd := &cobra.Command{Use: "exec"}
		if _, err := state.resolveExecPresentationMode(cmd); err == nil ||
			!strings.Contains(err.Error(), "requires an interactive terminal") {
			t.Fatalf("resolveExecPresentationMode() error = %v, want interactive terminal error", err)
		}

		state.tui = false
		state.outputFormat = string(core.OutputFormatJSON)
		mode, err := state.resolveExecPresentationMode(cmd)
		if err != nil {
			t.Fatalf("resolveExecPresentationMode(json) error = %v", err)
		}
		if mode != attachModeStream {
			t.Fatalf("json presentation mode = %q, want %q", mode, attachModeStream)
		}

		state.outputFormat = string(core.OutputFormatText)
		mode, err = state.resolveExecPresentationMode(cmd)
		if err != nil {
			t.Fatalf("resolveExecPresentationMode(text) error = %v", err)
		}
		if mode != attachModeDetach {
			t.Fatalf("text presentation mode = %q, want %q", mode, attachModeDetach)
		}
	})
}
