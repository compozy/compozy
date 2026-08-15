package worktree

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// Canonical suite: durable exit sequencing, cancellation, idempotency, and event reports.
func TestExitActions(t *testing.T) {
	t.Parallel()

	t.Run("Should run commit and push once with announced phases and one CTA", func(t *testing.T) {
		t.Parallel()
		fixture := newExitActionFixture(t, false)
		opID, err := fixture.service.RunExitAction(
			context.Background(), fixture.item.WorkspaceID, fixture.item.Name,
			ExitActionRequest{Action: ExitActionCommitPush},
		)
		if err != nil || opID != "op-exit" {
			t.Fatalf("RunExitAction() = %q, %v", opID, err)
		}
		operation := waitForExitOperation(t, fixture.store, opID, "completed")
		if operation.WorktreeID != fixture.item.ID || operation.FinishedAt == nil ||
			!fixture.runner.isCommitted() || !fixture.runner.isPushed() {
			t.Fatalf(
				"operation/runner = %#v committed=%t pushed=%t",
				operation,
				fixture.runner.isCommitted(),
				fixture.runner.isPushed(),
			)
		}
		commands := fixture.runner.commands()
		if countCommand(commands, "add -A") != 1 || countCommand(commands, "commit -m Update 1 files") != 1 ||
			countCommand(commands, "push -u origin feature/exit") != 1 {
			t.Fatalf("exit commands = %#v, want one stage, commit, and publish", commands)
		}
		started := exitEventPayload(t, fixture.events, EventExitActionStarted)
		if len(started.Phases) != 2 || started.Phases[0] != ExitPhaseCommit || started.Phases[1] != ExitPhasePush {
			t.Fatalf("started phases = %#v, want commit then push", started.Phases)
		}
		completed := exitEventPayload(t, fixture.events, EventExitActionCompleted)
		if completed.Result == nil || len(completed.Result.Steps) != 2 || completed.Result.CTA == nil ||
			completed.Result.CTA.Label != "Open in browser" {
			t.Fatalf("completed payload = %#v, want two steps and exactly one browser CTA", completed)
		}
	})

	t.Run("Should keep the completed commit when explicit cancel stops push", func(t *testing.T) {
		t.Parallel()
		fixture := newExitActionFixture(t, true)
		opID, err := fixture.service.RunExitAction(
			context.Background(), fixture.item.WorkspaceID, fixture.item.ID,
			ExitActionRequest{Action: ExitActionCommitPush, Message: "Keep this commit"},
		)
		if err != nil {
			t.Fatalf("RunExitAction() error = %v", err)
		}
		select {
		case <-fixture.runner.pushStarted:
		case <-time.After(time.Second):
			t.Fatal("push phase did not start")
		}
		if err := fixture.service.CancelExitAction(
			context.Background(), fixture.item.WorkspaceID, fixture.item.Name, opID,
		); err != nil {
			t.Fatalf("CancelExitAction() error = %v", err)
		}
		waitForExitEvent(t, fixture.events, EventExitActionCanceled)
		operation := exitOperation(fixture.store, opID)
		if operation.State != "canceled" || !fixture.runner.isCommitted() || fixture.runner.isPushed() {
			t.Fatalf(
				"canceled operation = %#v committed=%t pushed=%t",
				operation,
				fixture.runner.isCommitted(),
				fixture.runner.isPushed(),
			)
		}
		payload := exitEventPayload(t, fixture.events, EventExitActionCanceled)
		if payload.Phase != ExitPhasePush || payload.Result == nil || len(payload.Result.Steps) != 2 {
			t.Fatalf("cancel payload = %#v, want stopped push with committed step retained", payload)
		}
	})

	t.Run("Should refuse a second durable operation before starting another worker", func(t *testing.T) {
		t.Parallel()
		fixture := newExitActionFixture(t, false)
		if err := fixture.store.InsertExitOperation(context.Background(), ExitOperation{
			ID: "op-active", WorkspaceID: fixture.item.WorkspaceID, WorktreeID: fixture.item.ID,
			Action: string(ExitActionPush), State: "running", StartedAt: statusTestClock(),
		}); err != nil {
			t.Fatalf("seed active operation: %v", err)
		}
		_, err := fixture.service.RunExitAction(
			context.Background(), fixture.item.WorkspaceID, fixture.item.ID,
			ExitActionRequest{Action: ExitActionCommit},
		)
		if !errors.Is(err, ErrOperationInProgress) {
			t.Fatalf("RunExitAction(second) error = %v, want ErrOperationInProgress", err)
		}
		if fixture.events.count(EventExitActionStarted) != 0 {
			t.Fatal("second operation emitted a start event")
		}
	})

	t.Run("Should make a stale cancel a no-op that cannot hit a later operation", func(t *testing.T) {
		t.Parallel()
		store := newMemoryWorktreeStore()
		item := Worktree{ID: "wt-exit", WorkspaceID: "ws-exit", State: StateReady}
		if err := store.Insert(context.Background(), item); err != nil {
			t.Fatalf("seed worktree: %v", err)
		}
		finished := statusTestClock()
		store.exitOps["op-stale"] = ExitOperation{
			ID: "op-stale", WorkspaceID: item.WorkspaceID, WorktreeID: item.ID,
			Action: string(ExitActionCommit), State: "completed", StartedAt: finished, FinishedAt: &finished,
		}
		store.exitOps["op-later"] = ExitOperation{
			ID: "op-later", WorkspaceID: item.WorkspaceID, WorktreeID: item.ID,
			Action: string(ExitActionPush), State: "running", StartedAt: finished.Add(time.Second),
		}
		service := NewService(store, &recordingGitRunner{})
		if err := service.CancelExitAction(context.Background(), item.WorkspaceID, item.ID, "op-stale"); err != nil {
			t.Fatalf("CancelExitAction(stale) error = %v", err)
		}
		if got := exitOperation(store, "op-later"); got.State != "running" {
			t.Fatalf("later operation = %#v, want untouched", got)
		}
	})

	t.Run("Should refresh local and forge status once after a completed action", func(t *testing.T) {
		t.Parallel()
		store := newMemoryWorktreeStore()
		item := Worktree{
			ID: "wt-refresh", WorkspaceID: "ws-refresh", Branch: "feature/refresh",
			Path: "/repo/refresh", State: StateReady, CreatedAt: statusTestClock(), UpdatedAt: statusTestClock(),
		}
		if err := store.Insert(context.Background(), item); err != nil {
			t.Fatalf("seed refresh worktree: %v", err)
		}
		runner := &recordingGitRunner{responses: []gitResponse{
			{stdout: []byte("# branch.oid deadbeef\x00# branch.head feature/refresh\x00" +
				"# branch.upstream origin/feature/refresh\x00# branch.ab +0 -0\x00")},
			{}, {}, {stdout: []byte("git@github.com:acme/repo.git\n")},
		}}
		provider := &recordingExitForge{status: &ForgeStatus{Provider: "github"}}
		service := NewService(store, runner, WithForge(provider), WithClock(statusTestClock))
		service.refreshAfterExit(context.Background(), item)

		if provider.statusCalls != 1 || len(provider.lastStatus.RemoteURLs) != 1 ||
			provider.lastStatus.RemoteURLs[0] != "git@github.com:acme/repo.git" ||
			provider.lastStatus.Branch != item.Branch {
			t.Fatalf("forge refresh calls/request = %d/%#v", provider.statusCalls, provider.lastStatus)
		}
		cached, err := store.GetForgeStatus(context.Background(), item.WorkspaceID, item.ID)
		if err != nil || cached == nil || cached.Provider != "github" {
			t.Fatalf("cached forge refresh = %#v, %v", cached, err)
		}
	})

	t.Run("Should skip a push that is already up to date", func(t *testing.T) {
		t.Parallel()
		runner := &recordingGitRunner{responses: []gitResponse{{stdout: []byte(
			"# branch.oid deadbeef\x00# branch.head feature/exit\x00" +
				"# branch.upstream origin/feature/exit\x00# branch.ab +0 -0\x00",
		)}}}
		service := NewService(newMemoryWorktreeStore(), runner)
		step, err := service.runExitPush(context.Background(), Worktree{Path: "/repo", Branch: "feature/exit"}, false)
		if err != nil || step.State != "skipped" || step.Reason != "already up to date" ||
			containsCommandPrefix(invocationCommands(runner.invocations()), "push") {
			t.Fatalf("runExitPush() = %#v, %v; calls=%#v", step, err, runner.invocations())
		}
	})

	t.Run("Should report the configured upstream after pushing", func(t *testing.T) {
		t.Parallel()
		runner := &recordingGitRunner{responses: []gitResponse{
			{stdout: []byte("# branch.oid abc123\x00# branch.head feature/exit\x00" +
				"# branch.upstream fork/review\x00# branch.ab +1 -0\x00")},
			{},
			{stdout: []byte("abc123\n")},
		}}
		service := NewService(newMemoryWorktreeStore(), runner)
		step, err := service.runExitPush(
			context.Background(), Worktree{Path: "/repo", Branch: "feature/exit"}, false,
		)
		if err != nil || step.Upstream != "fork/review" {
			t.Fatalf("runExitPush() = %#v, %v, want configured upstream", step, err)
		}
		if got := invocationCommands(runner.invocations()); countCommand(got, "push") != 1 ||
			containsCommandPrefix(got, "push -u") {
			t.Fatalf("push commands = %#v, want existing upstream without publish flags", got)
		}
	})

	t.Run("Should surface a commit hook failure once and keep later phases stopped", func(t *testing.T) {
		t.Parallel()
		runner := &recordingGitRunner{responses: []gitResponse{
			{}, {stdout: []byte("new.txt\x00")},
			{stdout: []byte("hook stdout\n"), stderr: []byte("hook stderr\n"), err: errors.New("hook rejected")},
		}}
		service := NewService(newMemoryWorktreeStore(), runner)
		steps, err := service.runExitSequence(
			context.Background(), ExitOperation{ID: "op-hook"}, Worktree{Path: "/repo", Branch: "feature/exit"},
			&ExitPlan{CommitScope: ExitCommitScope{ChangedFiles: 1}},
			ExitActionRequest{Action: ExitActionCommitPush, Message: "Run hooks"},
		)
		commands := invocationCommands(runner.invocations())
		if err == nil || len(steps) != 1 || steps[0].State != "failed" ||
			!strings.Contains(steps[0].Output, "hook stdout") || !strings.Contains(steps[0].Output, "hook stderr") ||
			countCommand(commands, "commit -m Run hooks") != 1 || containsCommandPrefix(commands, "push") {
			t.Fatalf("runExitSequence() = %#v, %v; calls=%#v", steps, err, commands)
		}
	})

	t.Run("Should stream redacted commit hook output before the terminal step", func(t *testing.T) {
		t.Parallel()
		runner := &streamingRecordingGitRunner{
			recordingGitRunner: &recordingGitRunner{responses: []gitResponse{
				{}, {stdout: []byte("new.txt\x00")}, {stdout: []byte("committed\n")}, {stdout: []byte("deadbeef\n")},
			}},
			chunks: []GitOutput{
				{Stream: GitOutputStdout, Chunk: []byte("hook started\n")},
				{Stream: GitOutputStderr, Chunk: []byte("claim_token=compozy_claim_hook_secret\n")},
			},
		}
		events := &recordingEventSink{}
		service := NewService(newMemoryWorktreeStore(), runner, WithEvents(events))
		steps, err := service.runExitSequence(
			context.Background(),
			ExitOperation{ID: "op-hook-output", WorkspaceID: "ws-hook", WorktreeID: "wt-hook"},
			Worktree{Path: "/repo", Branch: "feature/exit"},
			&ExitPlan{CommitScope: ExitCommitScope{ChangedFiles: 1}},
			ExitActionRequest{Action: ExitActionCommit, Message: "Run hooks"},
		)
		if err != nil || len(steps) != 1 || steps[0].State != "completed" {
			t.Fatalf("runExitSequence() = %#v, %v", steps, err)
		}
		stdout := exitEventPayloadForStream(t, events, GitOutputStdout)
		if stdout.OperationID != "op-hook-output" || stdout.Action != ExitActionCommit ||
			stdout.Phase != ExitPhaseCommit || stdout.Stream != string(GitOutputStdout) ||
			stdout.Chunk != "hook started\n" {
			t.Fatalf("first hook output payload = %#v", stdout)
		}
		encoded, marshalErr := json.Marshal(events.events)
		if marshalErr != nil {
			t.Fatalf("marshal events: %v", marshalErr)
		}
		if strings.Contains(string(encoded), "compozy_claim_hook_secret") ||
			!strings.Contains(string(encoded), "compozy_claim_[REDACTED]") {
			t.Fatalf("hook output events leaked raw token: %s", encoded)
		}
		if outputIndex, terminalIndex := exitEventIndex(
			events,
			EventExitHookOutput,
		), exitLastEventIndex(
			events,
			EventExitActionStep,
		); outputIndex < 0 || terminalIndex < 0 ||
			outputIndex >= terminalIndex {
			t.Fatalf("event order output=%d terminal-step=%d, want output first", outputIndex, terminalIndex)
		}
	})

	t.Run("Should keep a completed commit when the following push fails", func(t *testing.T) {
		t.Parallel()
		runner := &recordingGitRunner{responses: []gitResponse{
			{}, {stdout: []byte("new.txt\x00")}, {stdout: []byte("committed\n")}, {stdout: []byte("deadbeef\n")},
			{stdout: []byte("# branch.oid deadbeef\x00# branch.head feature/exit\x00")},
			{stderr: []byte("remote rejected\n"), err: errors.New("push rejected")},
		}}
		service := NewService(newMemoryWorktreeStore(), runner)
		steps, err := service.runExitSequence(
			context.Background(), ExitOperation{ID: "op-push-failure"},
			Worktree{Path: "/repo", Branch: "feature/exit"},
			&ExitPlan{CommitScope: ExitCommitScope{ChangedFiles: 1}},
			ExitActionRequest{Action: ExitActionCommitPush},
		)
		if err == nil || len(steps) != 2 || steps[0].State != "completed" || steps[0].SHA != "deadbeef" ||
			steps[1].State != "failed" || !strings.Contains(steps[1].Output, "remote rejected") {
			t.Fatalf("runExitSequence() = %#v, %v", steps, err)
		}
	})

	t.Run("Should skip an empty staged commit without creating one", func(t *testing.T) {
		t.Parallel()
		runner := &recordingGitRunner{responses: []gitResponse{{}, {}}}
		service := NewService(newMemoryWorktreeStore(), runner)
		step, err := service.runExitCommit(
			context.Background(), ExitOperation{}, ExitActionCommit,
			Worktree{Path: "/repo"}, ExitCommitScope{}, "",
		)
		if err != nil || step.State != "skipped" || step.Reason != "nothing to commit" ||
			containsCommandPrefix(invocationCommands(runner.invocations()), "commit") {
			t.Fatalf("runExitCommit() = %#v, %v; calls=%#v", step, err, runner.invocations())
		}
	})

	t.Run("Should commit a staged whitespace-only filename", func(t *testing.T) {
		t.Parallel()
		runner := &recordingGitRunner{responses: []gitResponse{
			{}, {stdout: []byte(" \x00")}, {stdout: []byte("committed\n")}, {stdout: []byte("deadbeef\n")},
		}}
		service := NewService(newMemoryWorktreeStore(), runner)
		step, err := service.runExitCommit(
			context.Background(), ExitOperation{}, ExitActionCommit,
			Worktree{Path: "/repo"}, ExitCommitScope{}, "",
		)
		if err != nil || step.State != "completed" || step.SHA != "deadbeef" ||
			countCommand(invocationCommands(runner.invocations()), "commit -m Update 1 files") != 1 {
			t.Fatalf("runExitCommit(whitespace filename) = %#v, %v; calls=%#v", step, err, runner.invocations())
		}
	})

	t.Run("Should preserve an idempotent existing PR result and draft request", func(t *testing.T) {
		t.Parallel()
		store := newMemoryWorktreeStore()
		item := Worktree{
			ID: "wt-pr", WorkspaceID: "ws-pr", Path: "/repo", Branch: "feature/pr", State: StateReady,
			CreatedAt: statusTestClock(), UpdatedAt: statusTestClock(),
		}
		if err := store.Insert(context.Background(), item); err != nil {
			t.Fatalf("seed PR worktree: %v", err)
		}
		provider := &recordingExitForge{create: &ForgePRResult{
			Status: "opened_existing", Number: 51,
			URL: "https://secret@github.com/acme/repo/pull/51?token=bad#fragment",
		}}
		service := NewService(store, &recordingGitRunner{}, WithForge(provider), WithClock(statusTestClock))
		step, err := service.runExitPR(context.Background(), item, &ExitPlan{
			Base: "main", RemoteURLs: []string{"https://github.com/acme/repo.git"},
			Forge: &ForgeCapabilities{Provider: "github"},
		}, ExitActionRequest{Action: ExitActionOpenPR, Title: "Exit", Body: "Ready", Draft: true})
		cached, cacheErr := store.GetForgeStatus(context.Background(), item.WorkspaceID, item.ID)
		if err != nil || cacheErr != nil || step.PRStatus != "opened_existing" || step.PRNumber != 51 ||
			step.URL != "https://github.com/acme/repo/pull/51" || cached == nil || cached.PRURL != step.URL || provider.createCalls != 1 ||
			!provider.lastCreate.Draft || provider.lastCreate.Title != "Exit" || provider.lastCreate.Body != "Ready" ||
			provider.lastCreate.Base != "main" {
			t.Fatalf(
				"runExitPR() = %#v, %v; request=%#v calls=%d",
				step,
				err,
				provider.lastCreate,
				provider.createCalls,
			)
		}
	})

	t.Run("Should reuse the planned PR defaults when the request keeps the resolved base", func(t *testing.T) {
		t.Parallel()
		item := Worktree{
			ID: "wt-pr", WorkspaceID: "ws-pr", Path: "/repo", Branch: "feature/pr", State: StateReady,
			CreatedAt: statusTestClock(), UpdatedAt: statusTestClock(),
		}
		store := newMemoryWorktreeStore()
		if err := store.Insert(context.Background(), item); err != nil {
			t.Fatalf("seed worktree: %v", err)
		}
		provider := &recordingExitForge{create: &ForgePRResult{
			Status: "opened", Number: 52, URL: "https://github.com/acme/repo/pull/52",
		}}
		runner := &recordingGitRunner{}
		service := NewService(store, runner, WithForge(provider), WithClock(statusTestClock))
		step, err := service.runExitPR(
			context.Background(),
			item,
			&ExitPlan{
				Base: "main", RemoteURLs: []string{"https://github.com/acme/repo.git"},
				Forge:     &ForgeCapabilities{Provider: "github"},
				PRPrefill: &ExitPRPrefill{Title: "feature/pr", Body: "## Summary\nPlanned body."},
			},
			ExitActionRequest{Action: ExitActionOpenPR},
		)
		if err != nil || step.PRStatus != "opened" || provider.lastCreate.Title != "feature/pr" ||
			provider.lastCreate.Body != "## Summary\nPlanned body." || len(runner.invocations()) != 0 {
			t.Fatalf("runExitPR() = %#v, %v; request=%#v", step, err, provider.lastCreate)
		}
	})
}

type recordingExitForge struct {
	capabilities *ForgeCapabilities
	status       *ForgeStatus
	statusErr    error
	statusCalls  int
	lastStatus   ForgeStatusRequest
	create       *ForgePRResult
	createErr    error
	createCalls  int
	lastCreate   ForgePRRequest
}

func (f *recordingExitForge) Capabilities(context.Context, []string) (*ForgeCapabilities, error) {
	return f.capabilities, nil
}

type streamingRecordingGitRunner struct {
	*recordingGitRunner
	chunks []GitOutput
}

func (r *streamingRecordingGitRunner) RunStreaming(
	ctx context.Context,
	dir string,
	onOutput GitOutputHandler,
	args ...string,
) ([]byte, []byte, error) {
	stdout, stderr, err := r.Run(ctx, dir, args...)
	for _, chunk := range r.chunks {
		onOutput(chunk)
	}
	return stdout, stderr, err
}

func (f *recordingExitForge) Status(_ context.Context, request ForgeStatusRequest) (*ForgeStatus, error) {
	f.statusCalls++
	f.lastStatus = request
	return f.status, f.statusErr
}

func (f *recordingExitForge) CreatePR(_ context.Context, request ForgePRRequest) (*ForgePRResult, error) {
	f.createCalls++
	f.lastCreate = request
	return f.create, f.createErr
}

type exitActionFixture struct {
	service *Service
	store   *memoryWorktreeStore
	runner  *scriptedExitRunner
	events  *recordingEventSink
	item    Worktree
}

func newExitActionFixture(t *testing.T, blockPush bool) exitActionFixture {
	t.Helper()
	root := t.TempDir()
	commonDir := filepath.Join(root, ".git")
	if err := os.MkdirAll(commonDir, 0o755); err != nil {
		t.Fatalf("create Git directory: %v", err)
	}
	item := Worktree{
		ID: "wt-exit", WorkspaceID: "ws-exit", Name: "exit", Branch: "feature/exit",
		Path: root, State: StateReady, Origin: OriginManual, BaseRef: "main",
		CreatedAt: statusTestClock(), UpdatedAt: statusTestClock(),
	}
	store := newMemoryWorktreeStore()
	if err := store.Insert(context.Background(), item); err != nil {
		t.Fatalf("seed worktree: %v", err)
	}
	runner := &scriptedExitRunner{
		root: root, commonDir: commonDir, blockPush: blockPush, pushStarted: make(chan struct{}),
	}
	events := &recordingEventSink{}
	service := NewService(
		store, runner, WithCapabilityGate(readyCapabilityGate()),
		WithWorkspaceResolver(staticWorkspaceResolver{workspaces: map[string]Workspace{
			item.WorkspaceID: {ID: item.WorkspaceID, Root: root},
		}}),
		WithSessionGuard(fixedSessionGuard{}), WithEvents(events), WithClock(statusTestClock),
		WithIDGenerator(func(string) (string, error) { return "op-exit", nil }),
	)
	return exitActionFixture{service: service, store: store, runner: runner, events: events, item: item}
}

type scriptedExitRunner struct {
	mu          sync.Mutex
	root        string
	commonDir   string
	committed   bool
	pushed      bool
	blockPush   bool
	pushStarted chan struct{}
	pushOnce    sync.Once
	calls       []string
}

func (r *scriptedExitRunner) Run(ctx context.Context, _ string, args ...string) ([]byte, []byte, error) {
	command := strings.Join(args, " ")
	r.mu.Lock()
	r.calls = append(r.calls, command)
	committed, pushed := r.committed, r.pushed
	r.mu.Unlock()
	switch {
	case command == "status --porcelain=v2 --branch -z":
		if !committed {
			return []byte("# branch.oid old\x00# branch.head feature/exit\x00? new.txt\x00"), nil, nil
		}
		if pushed {
			return []byte("# branch.oid new\x00# branch.head feature/exit\x00" +
				"# branch.upstream origin/feature/exit\x00# branch.ab +0 -0\x00"), nil, nil
		}
		return []byte("# branch.oid new\x00# branch.head feature/exit\x00"), nil, nil
	case command == "diff --numstat -z", command == "diff --cached --numstat -z":
		return nil, nil, nil
	case command == "rev-list --count main..HEAD":
		return []byte("1\n"), nil, nil
	case command == "remote get-url --all origin":
		return []byte("git@github.com:acme/repo.git\n"), nil, nil
	case strings.HasPrefix(command, "config --get branch."):
		return nil, nil, errors.New("not configured")
	case command == "status --porcelain=v2 -z --untracked-files=all":
		if committed {
			return nil, nil, nil
		}
		return []byte("? new.txt\x00! ignored.env\x00"), nil, nil
	case strings.HasPrefix(command, "rev-list --count HEAD --not"):
		return []byte("1\n"), nil, nil
	case command == "branch -r --list */feature/exit":
		if pushed {
			return []byte("origin/feature/exit\n"), nil, nil
		}
		return nil, nil, nil
	case command == "rev-parse --path-format=absolute --git-common-dir":
		return []byte(r.commonDir + "\n"), nil, nil
	case command == "add -A":
		return nil, nil, nil
	case command == "diff --cached --name-only -z":
		return []byte("new.txt\x00"), nil, nil
	case strings.HasPrefix(command, "commit -m "):
		r.mu.Lock()
		r.committed = true
		r.mu.Unlock()
		return []byte("committed\n"), nil, nil
	case command == "push -u origin feature/exit":
		if r.blockPush {
			r.pushOnce.Do(func() { close(r.pushStarted) })
			<-ctx.Done()
			return nil, nil, ctx.Err()
		}
		r.mu.Lock()
		r.pushed = true
		r.mu.Unlock()
		return []byte("pushed\n"), nil, nil
	case command == "rev-parse HEAD":
		return []byte("deadbeef\n"), nil, nil
	default:
		return nil, nil, errors.New("unexpected exit Git command: " + command)
	}
}

func (r *scriptedExitRunner) isCommitted() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.committed
}

func (r *scriptedExitRunner) isPushed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.pushed
}

func (r *scriptedExitRunner) commands() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.calls...)
}

func waitForExitOperation(t *testing.T, store *memoryWorktreeStore, opID, state string) ExitOperation {
	t.Helper()
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		if operation := exitOperation(store, opID); operation.State == state {
			return operation
		}
		select {
		case <-deadline.C:
			t.Fatalf("operation %q did not reach %q; got %#v", opID, state, exitOperation(store, opID))
		case <-ticker.C:
		}
	}
}

func exitOperation(store *memoryWorktreeStore, opID string) ExitOperation {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.exitOps[opID]
}

func waitForExitEvent(t *testing.T, events *recordingEventSink, name string) {
	t.Helper()
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for events.count(name) == 0 {
		select {
		case <-deadline.C:
			t.Fatalf("event %q was not emitted", name)
		case <-ticker.C:
		}
	}
}

func exitEventPayload(t *testing.T, events *recordingEventSink, name string) ExitEventPayload {
	t.Helper()
	events.mu.Lock()
	defer events.mu.Unlock()
	for _, v := range slices.Backward(events.events) {
		if v.Name != name {
			continue
		}
		var payload ExitEventPayload
		if err := json.Unmarshal(v.Payload, &payload); err != nil {
			t.Fatalf("decode %s payload: %v", name, err)
		}
		return payload
	}
	t.Fatalf("event %q not found", name)
	return ExitEventPayload{}
}

func exitEventIndex(events *recordingEventSink, name string) int {
	events.mu.Lock()
	defer events.mu.Unlock()
	for index, event := range events.events {
		if event.Name == name {
			return index
		}
	}
	return -1
}

func exitLastEventIndex(events *recordingEventSink, name string) int {
	events.mu.Lock()
	defer events.mu.Unlock()
	for index, v := range slices.Backward(events.events) {
		if v.Name == name {
			return index
		}
	}
	return -1
}

func exitEventPayloadForStream(
	t *testing.T,
	events *recordingEventSink,
	stream GitOutputStream,
) ExitEventPayload {
	t.Helper()
	events.mu.Lock()
	defer events.mu.Unlock()
	for _, event := range events.events {
		if event.Name != EventExitHookOutput {
			continue
		}
		var payload ExitEventPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatalf("decode %s payload: %v", EventExitHookOutput, err)
		}
		if payload.Stream == string(stream) {
			return payload
		}
	}
	t.Fatalf("hook output stream %q not found", stream)
	return ExitEventPayload{}
}

func countCommand(commands []string, expected string) int {
	count := 0
	for _, command := range commands {
		if command == expected {
			count++
		}
	}
	return count
}
