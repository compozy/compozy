package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	apiclient "github.com/compozy/compozy/internal/api/client"
	"github.com/compozy/compozy/internal/api/contract"
	apicore "github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/provider"
	"github.com/compozy/compozy/internal/core/reviews"
	uipkg "github.com/compozy/compozy/internal/core/run/ui"
	"github.com/compozy/compozy/internal/core/workpackages"
	workspacecfg "github.com/compozy/compozy/internal/core/workspace"
	"github.com/compozy/compozy/internal/daemon"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
	"github.com/spf13/cobra"
)

var (
	cliTestGlobalOverrideMu sync.Mutex
	cliTestGlobalOverrides  = struct {
		sync.Mutex
		refs map[string]int
	}{
		refs: make(map[string]int),
	}
)

type daemonCommandContextKey string

type stubDaemonCommandClient struct {
	target               apiclient.Target
	health               apicore.DaemonHealth
	healthErr            error
	healthCtx            context.Context
	healthCalls          int
	status               apicore.DaemonStatus
	statusErr            error
	statusCtx            context.Context
	statusCalls          int
	startCalls           int
	startSlug            string
	startRequest         apicore.TaskRunRequest
	startRun             apicore.Run
	startErr             error
	startMultipleCalls   int
	startMultipleRequest apicore.TaskRunMultipleRequest
	startMultipleRun     apicore.Run
	startMultipleErr     error
	multiSnapshot        apicore.TaskRunMultipleSnapshot
	multiSnapshotErr     error
	cancelCtx            context.Context
	cancelRunID          string
	cancelCalls          int
	cancelErr            error
	stopCtx              context.Context
	stopForce            bool
	stopErr              error
	workspaces           []apicore.Workspace
	workspace            apicore.Workspace
	workspaceErr         error
	register             apicore.WorkspaceRegisterResult
	registerErr          error
	listErr              error
	runs                 []apicore.Run
	runsErr              error
	runListRequests      []apiclient.RunListOptions
	getRun               apicore.Run
	getRunErr            error
	deleteRef            string
	deleteErr            error
	workflows            []apicore.WorkflowSummary
	workflowsErr         error
	archiveCalls         []string
	archive              apicore.ArchiveResult
	archiveBySlug        map[string]apicore.ArchiveResult
	archiveErr           error
	archiveErrors        map[string]error
	syncRequest          apicore.SyncRequest
	syncResult           apicore.SyncResult
	syncErr              error
	reviewFetch          apicore.ReviewFetchResult
	reviewFetchErr       error
	reviewLatest         apicore.ReviewSummary
	reviewLatestErr      error
	reviewRound          apicore.ReviewRound
	reviewRoundErr       error
	reviewIssues         []apicore.ReviewIssue
	reviewIssuesErr      error
	reviewRun            apicore.Run
	reviewRunErr         error
	reviewWatchRun       apicore.Run
	reviewWatchErr       error
	execRun              apicore.Run
	execRunErr           error
	runEventPage         apicore.RunEventPage
	runEventPageErr      error
	snapshot             apicore.RunSnapshot
	snapshotErr          error
	snapshotFunc         func(context.Context, string) (apicore.RunSnapshot, error)
	stream               apiclient.RunStream
	streamErr            error
}

func (c *stubDaemonCommandClient) Target() apiclient.Target {
	if c == nil {
		return apiclient.Target{}
	}
	return c.target
}

func (c *stubDaemonCommandClient) Health(ctx context.Context) (apicore.DaemonHealth, error) {
	if c == nil {
		return apicore.DaemonHealth{}, errors.New("stub daemon client is required")
	}
	c.healthCtx = ctx
	c.healthCalls++
	if c.healthErr != nil {
		return apicore.DaemonHealth{}, c.healthErr
	}
	return c.health, nil
}

func (c *stubDaemonCommandClient) DaemonStatus(ctx context.Context) (apicore.DaemonStatus, error) {
	if c == nil {
		return apicore.DaemonStatus{}, errors.New("stub daemon client is required")
	}
	c.statusCtx = ctx
	c.statusCalls++
	if c.statusErr != nil {
		return apicore.DaemonStatus{}, c.statusErr
	}
	return c.status, nil
}

func (c *stubDaemonCommandClient) StopDaemon(ctx context.Context, force bool) error {
	if c == nil {
		return errors.New("stub daemon client is required")
	}
	c.stopCtx = ctx
	c.stopForce = force
	if c.stopErr != nil {
		return c.stopErr
	}
	return nil
}

func (c *stubDaemonCommandClient) CancelRun(ctx context.Context, runID string) error {
	if c == nil {
		return errors.New("stub daemon client is required")
	}
	c.cancelCtx = ctx
	c.cancelRunID = runID
	c.cancelCalls++
	if c.cancelErr != nil {
		return c.cancelErr
	}
	return nil
}

func (c *stubDaemonCommandClient) PauseRunJob(
	context.Context,
	string,
	string,
) (apicore.RunJobControlResponse, error) {
	if c == nil {
		return apicore.RunJobControlResponse{}, errors.New("stub daemon client is required")
	}
	return apicore.RunJobControlResponse{}, nil
}

func (c *stubDaemonCommandClient) SendRunJobMessage(
	context.Context,
	string,
	string,
	apicore.RunJobMessageRequest,
) (apicore.RunJobControlResponse, error) {
	if c == nil {
		return apicore.RunJobControlResponse{}, errors.New("stub daemon client is required")
	}
	return apicore.RunJobControlResponse{}, nil
}

func (c *stubDaemonCommandClient) RegisterWorkspace(
	context.Context,
	string,
	string,
) (apicore.WorkspaceRegisterResult, error) {
	if c == nil {
		return apicore.WorkspaceRegisterResult{}, errors.New("stub daemon client is required")
	}
	if c.registerErr != nil {
		return apicore.WorkspaceRegisterResult{}, c.registerErr
	}
	return c.register, nil
}

func (c *stubDaemonCommandClient) ListWorkspaces(context.Context) ([]apicore.Workspace, error) {
	if c == nil {
		return nil, errors.New("stub daemon client is required")
	}
	if c.listErr != nil {
		return nil, c.listErr
	}
	return c.workspaces, nil
}

func (c *stubDaemonCommandClient) GetWorkspace(context.Context, string) (apicore.Workspace, error) {
	if c == nil {
		return apicore.Workspace{}, errors.New("stub daemon client is required")
	}
	if c.workspaceErr != nil {
		return apicore.Workspace{}, c.workspaceErr
	}
	return c.workspace, nil
}

func (c *stubDaemonCommandClient) DeleteWorkspace(_ context.Context, ref string) error {
	if c == nil {
		return errors.New("stub daemon client is required")
	}
	c.deleteRef = ref
	return c.deleteErr
}

func (c *stubDaemonCommandClient) ResolveWorkspace(context.Context, string) (apicore.Workspace, error) {
	if c == nil {
		return apicore.Workspace{}, errors.New("stub daemon client is required")
	}
	if c.workspaceErr != nil {
		return apicore.Workspace{}, c.workspaceErr
	}
	return c.workspace, nil
}

func (c *stubDaemonCommandClient) ListRuns(
	_ context.Context,
	opts apiclient.RunListOptions,
) ([]apicore.Run, error) {
	if c == nil {
		return nil, errors.New("stub daemon client is required")
	}
	c.runListRequests = append(c.runListRequests, opts)
	if c.runsErr != nil {
		return nil, c.runsErr
	}
	statuses := stubRunListStatuses(opts)
	if len(statuses) == 0 {
		return append([]apicore.Run(nil), c.runs...), nil
	}
	runs := make([]apicore.Run, 0, len(c.runs))
	for i := range c.runs {
		run := c.runs[i]
		if slices.ContainsFunc(statuses, func(status string) bool {
			return strings.EqualFold(strings.TrimSpace(run.Status), status)
		}) {
			runs = append(runs, run)
		}
	}
	return runs, nil
}

func (c *stubDaemonCommandClient) GetRun(_ context.Context, _ string) (apicore.Run, error) {
	if c == nil {
		return apicore.Run{}, errors.New("stub daemon client is required")
	}
	if c.getRunErr != nil {
		return apicore.Run{}, c.getRunErr
	}
	return c.getRun, nil
}

func stubRunListStatuses(opts apiclient.RunListOptions) []string {
	statuses := make([]string, 0, len(opts.Statuses)+1)
	appendStatus := func(raw string) {
		for _, candidate := range strings.Split(raw, ",") {
			trimmed := strings.TrimSpace(candidate)
			if trimmed == "" || slices.Contains(statuses, trimmed) {
				continue
			}
			statuses = append(statuses, trimmed)
		}
	}
	appendStatus(opts.Status)
	for _, status := range opts.Statuses {
		appendStatus(status)
	}
	return statuses
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func (c *stubDaemonCommandClient) ListTaskWorkflows(context.Context, string) ([]apicore.WorkflowSummary, error) {
	if c == nil {
		return nil, errors.New("stub daemon client is required")
	}
	if c.workflowsErr != nil {
		return nil, c.workflowsErr
	}
	return c.workflows, nil
}

func (c *stubDaemonCommandClient) ArchiveTaskWorkflow(
	_ context.Context,
	_ string,
	slug string,
) (apicore.ArchiveResult, error) {
	if c == nil {
		return apicore.ArchiveResult{}, errors.New("stub daemon client is required")
	}
	c.archiveCalls = append(c.archiveCalls, slug)
	if err, ok := c.archiveErrors[slug]; ok {
		return apicore.ArchiveResult{}, err
	}
	if result, ok := c.archiveBySlug[slug]; ok {
		return result, nil
	}
	if c.archiveErr != nil {
		return apicore.ArchiveResult{}, c.archiveErr
	}
	return c.archive, nil
}

func (c *stubDaemonCommandClient) SyncWorkflow(_ context.Context, req apicore.SyncRequest) (apicore.SyncResult, error) {
	if c == nil {
		return apicore.SyncResult{}, errors.New("stub daemon client is required")
	}
	c.syncRequest = req
	if c.syncErr != nil {
		return apicore.SyncResult{}, c.syncErr
	}
	return c.syncResult, nil
}

func (c *stubDaemonCommandClient) FetchReview(
	_ context.Context,
	_ string,
	_ string,
	_ apicore.ReviewFetchRequest,
) (apicore.ReviewFetchResult, error) {
	if c == nil {
		return apicore.ReviewFetchResult{}, errors.New("stub daemon client is required")
	}
	if c.reviewFetchErr != nil {
		return apicore.ReviewFetchResult{}, c.reviewFetchErr
	}
	return c.reviewFetch, nil
}

func (c *stubDaemonCommandClient) GetLatestReview(context.Context, string, string) (apicore.ReviewSummary, error) {
	if c == nil {
		return apicore.ReviewSummary{}, errors.New("stub daemon client is required")
	}
	if c.reviewLatestErr != nil {
		return apicore.ReviewSummary{}, c.reviewLatestErr
	}
	return c.reviewLatest, nil
}

func (c *stubDaemonCommandClient) GetReviewRound(context.Context, string, string, int) (apicore.ReviewRound, error) {
	if c == nil {
		return apicore.ReviewRound{}, errors.New("stub daemon client is required")
	}
	if c.reviewRoundErr != nil {
		return apicore.ReviewRound{}, c.reviewRoundErr
	}
	return c.reviewRound, nil
}

func (c *stubDaemonCommandClient) ListReviewIssues(
	context.Context,
	string,
	string,
	int,
) ([]apicore.ReviewIssue, error) {
	if c == nil {
		return nil, errors.New("stub daemon client is required")
	}
	if c.reviewIssuesErr != nil {
		return nil, c.reviewIssuesErr
	}
	return c.reviewIssues, nil
}

func (c *stubDaemonCommandClient) StartTaskRun(
	_ context.Context,
	slug string,
	req apicore.TaskRunRequest,
) (apicore.Run, error) {
	if c == nil {
		return apicore.Run{}, errors.New("stub daemon client is required")
	}
	c.startCalls++
	c.startSlug = slug
	c.startRequest = req
	if c.startErr != nil {
		return apicore.Run{}, c.startErr
	}
	return c.startRun, nil
}

func (c *stubDaemonCommandClient) StartTaskRunMultiple(
	_ context.Context,
	req apicore.TaskRunMultipleRequest,
) (apicore.Run, error) {
	if c == nil {
		return apicore.Run{}, errors.New("stub daemon client is required")
	}
	c.startMultipleCalls++
	c.startMultipleRequest = req
	if c.startMultipleErr != nil {
		return apicore.Run{}, c.startMultipleErr
	}
	return c.startMultipleRun, nil
}

func (c *stubDaemonCommandClient) GetTaskRunMultipleSnapshot(
	context.Context,
	string,
) (apicore.TaskRunMultipleSnapshot, error) {
	if c == nil {
		return apicore.TaskRunMultipleSnapshot{}, errors.New("stub daemon client is required")
	}
	if c.multiSnapshotErr != nil {
		return apicore.TaskRunMultipleSnapshot{}, c.multiSnapshotErr
	}
	return c.multiSnapshot, nil
}

func (c *stubDaemonCommandClient) StartReviewRun(
	_ context.Context,
	_ string,
	_ string,
	_ int,
	_ apicore.ReviewRunRequest,
) (apicore.Run, error) {
	if c == nil {
		return apicore.Run{}, errors.New("stub daemon client is required")
	}
	if c.reviewRunErr != nil {
		return apicore.Run{}, c.reviewRunErr
	}
	return c.reviewRun, nil
}

func (c *stubDaemonCommandClient) StartReviewWatch(
	_ context.Context,
	_ string,
	_ string,
	_ apicore.ReviewWatchRequest,
) (apicore.Run, error) {
	if c == nil {
		return apicore.Run{}, errors.New("stub daemon client is required")
	}
	if c.reviewWatchErr != nil {
		return apicore.Run{}, c.reviewWatchErr
	}
	return c.reviewWatchRun, nil
}

func (c *stubDaemonCommandClient) StartExecRun(_ context.Context, _ apicore.ExecRequest) (apicore.Run, error) {
	if c == nil {
		return apicore.Run{}, errors.New("stub daemon client is required")
	}
	if c.execRunErr != nil {
		return apicore.Run{}, c.execRunErr
	}
	return c.execRun, nil
}

func (c *stubDaemonCommandClient) GetRunSnapshot(ctx context.Context, runID string) (apicore.RunSnapshot, error) {
	if c == nil {
		return apicore.RunSnapshot{}, errors.New("stub daemon client is required")
	}
	if c.snapshotFunc != nil {
		return c.snapshotFunc(ctx, runID)
	}
	if c.snapshotErr != nil {
		return apicore.RunSnapshot{}, c.snapshotErr
	}
	return c.snapshot, nil
}

func (c *stubDaemonCommandClient) ListRunEvents(
	context.Context,
	string,
	apicore.StreamCursor,
	int,
) (apicore.RunEventPage, error) {
	if c == nil {
		return apicore.RunEventPage{}, errors.New("stub daemon client is required")
	}
	if c.runEventPageErr != nil {
		return apicore.RunEventPage{}, c.runEventPageErr
	}
	return c.runEventPage, nil
}

func (c *stubDaemonCommandClient) OpenRunStream(
	context.Context,
	string,
	apicore.StreamCursor,
) (apiclient.RunStream, error) {
	if c == nil {
		return nil, errors.New("stub daemon client is required")
	}
	if c.streamErr != nil {
		return nil, c.streamErr
	}
	return c.stream, nil
}

func installTestCLIDaemonBootstrap(t *testing.T, bootstrap cliDaemonBootstrap) {
	t.Helper()
	acquireCLITestGlobalOverride(t)

	original := newCLIDaemonBootstrap
	newCLIDaemonBootstrap = func() cliDaemonBootstrap { return bootstrap }
	t.Cleanup(func() {
		newCLIDaemonBootstrap = original
	})
}

func installTestCLIReadyDaemonBootstrap(t *testing.T, client daemonCommandClient) {
	t.Helper()

	installTestCLIDaemonBootstrap(t, cliDaemonBootstrap{
		resolveHomePaths: func() (compozyconfig.HomePaths, error) {
			return compozyconfig.HomePaths{InfoPath: "/tmp/compozy-home/daemon.json"}, nil
		},
		readInfo: func(string) (daemon.Info, error) {
			return daemon.Info{
				PID:        1234,
				SocketPath: "/tmp/compozy.sock",
				StartedAt:  time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC),
				State:      daemon.ReadyStateReady,
			}, nil
		},
		newClient: func(apiclient.Target) (daemonCommandClient, error) {
			return client, nil
		},
		launch:         func(compozyconfig.HomePaths) error { return nil },
		sleep:          func(time.Duration) {},
		now:            func() time.Time { return time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC) },
		startupTimeout: time.Second,
		pollInterval:   time.Millisecond,
	})
}

func installTestCLIRunObservers(
	t *testing.T,
	attachFn func(context.Context, daemonCommandClient, string) error,
	watchFn func(context.Context, io.Writer, daemonCommandClient, string) error,
) {
	t.Helper()
	acquireCLITestGlobalOverride(t)

	originalAttach := attachCLIRunUI
	originalAttachStarted := attachStartedCLIRunUI
	originalWatch := watchCLIRun
	if attachFn != nil {
		attachCLIRunUI = attachFn
		attachStartedCLIRunUI = attachFn
	}
	if watchFn != nil {
		watchCLIRun = watchFn
	}
	t.Cleanup(func() {
		attachCLIRunUI = originalAttach
		attachStartedCLIRunUI = originalAttachStarted
		watchCLIRun = originalWatch
	})
}

type fakeCLIUISession struct {
	quitHandler  func(uipkg.QuitRequest)
	shutdownCh   chan struct{}
	shutdownOnce sync.Once
	waitFn       func(*fakeCLIUISession) error
}

func newFakeCLIUISession() *fakeCLIUISession {
	return &fakeCLIUISession{
		shutdownCh: make(chan struct{}),
	}
}

func (*fakeCLIUISession) Enqueue(any) {}

func (s *fakeCLIUISession) SetQuitHandler(fn func(uipkg.QuitRequest)) {
	s.quitHandler = fn
}

func (*fakeCLIUISession) SetJobControlHandler(
	func(context.Context, uipkg.JobControlRequest) (uipkg.JobControlResponse, error),
) {
}

func (*fakeCLIUISession) CloseEvents() {}

func (s *fakeCLIUISession) Shutdown() {
	s.shutdownOnce.Do(func() {
		close(s.shutdownCh)
	})
}

func (s *fakeCLIUISession) Wait() error {
	if s.waitFn != nil {
		return s.waitFn(s)
	}
	if s.quitHandler != nil {
		s.quitHandler(uipkg.QuitRequestDrain)
	}
	<-s.shutdownCh
	return nil
}

func TestRunSnapshotSettledBeforeUIAttach(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		snapshot apicore.RunSnapshot
		want     bool
	}{
		{
			name: "terminal run status",
			snapshot: apicore.RunSnapshot{
				Run: apicore.Run{Status: "completed"},
			},
			want: true,
		},
		{
			name: "all jobs terminal while row still running",
			snapshot: apicore.RunSnapshot{
				Run: apicore.Run{Status: "running"},
				Jobs: []apicore.RunJobState{
					{Index: 0, Status: "completed"},
					{Index: 1, Status: "failed"},
				},
			},
			want: true,
		},
		{
			name: "still active job",
			snapshot: apicore.RunSnapshot{
				Run: apicore.Run{Status: "running"},
				Jobs: []apicore.RunJobState{
					{Index: 0, Status: "completed"},
					{Index: 1, Status: "running"},
				},
			},
			want: false,
		},
		{
			name: "no jobs yet",
			snapshot: apicore.RunSnapshot{
				Run: apicore.Run{Status: "starting"},
			},
			want: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := runSnapshotSettledBeforeUIAttach(tc.snapshot); got != tc.want {
				t.Fatalf("runSnapshotSettledBeforeUIAttach() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDefaultAttachCLIRunUIReturnsReplaySentinelForSettledSnapshot(t *testing.T) {
	t.Parallel()

	client := &stubDaemonCommandClient{
		snapshot: apicore.RunSnapshot{
			Run: apicore.Run{RunID: "run-ui-settled", Status: "running"},
			Jobs: []apicore.RunJobState{
				{Index: 0, Status: "completed"},
			},
		},
	}

	err := defaultAttachCLIRunUI(context.Background(), client, "run-ui-settled")
	if !errors.Is(err, errRunSettledBeforeUIAttach) {
		t.Fatalf("defaultAttachCLIRunUI() error = %v, want errRunSettledBeforeUIAttach", err)
	}
}

func TestDefaultAttachStartedCLIRunUICancelsOwnedRunOnLocalExit(t *testing.T) {
	t.Parallel()

	acquireCLITestGlobalOverride(t)

	client := &stubDaemonCommandClient{
		snapshot: apicore.RunSnapshot{
			Run: apicore.Run{RunID: "run-ui-owned", Status: "running"},
			Jobs: []apicore.RunJobState{
				{Index: 0, Status: "running"},
			},
		},
	}
	session := newFakeCLIUISession()
	var ownerSession bool

	originalOpenRemoteUI := openCLIRemoteUISession
	t.Cleanup(func() {
		openCLIRemoteUISession = originalOpenRemoteUI
	})
	openCLIRemoteUISession = func(
		_ context.Context,
		opts uipkg.RemoteAttachOptions,
	) (uipkg.Session, error) {
		ownerSession = opts.OwnerSession
		return session, nil
	}

	if err := defaultAttachStartedCLIRunUI(context.Background(), client, "run-ui-owned"); err != nil {
		t.Fatalf("defaultAttachStartedCLIRunUI() error = %v", err)
	}
	if !ownerSession {
		t.Fatal("expected owner remote attach session for started run ui")
	}
	if client.cancelCalls != 1 {
		t.Fatalf("cancel calls = %d, want 1", client.cancelCalls)
	}
	if client.cancelRunID != "run-ui-owned" {
		t.Fatalf("cancel run id = %q, want run-ui-owned", client.cancelRunID)
	}
}

func TestDefaultAttachStartedCLIRunUIDoesNotCancelOwnedRunWhenUICloseDoesNotRequestStop(t *testing.T) {
	t.Parallel()

	acquireCLITestGlobalOverride(t)

	client := &stubDaemonCommandClient{
		snapshot: apicore.RunSnapshot{
			Run: apicore.Run{RunID: "run-ui-owned-close-only", Status: "running"},
			Jobs: []apicore.RunJobState{
				{Index: 0, Status: "running"},
			},
		},
	}
	session := newFakeCLIUISession()
	session.waitFn = func(*fakeCLIUISession) error {
		return nil
	}

	originalOpenRemoteUI := openCLIRemoteUISession
	t.Cleanup(func() {
		openCLIRemoteUISession = originalOpenRemoteUI
	})
	openCLIRemoteUISession = func(
		_ context.Context,
		opts uipkg.RemoteAttachOptions,
	) (uipkg.Session, error) {
		if !opts.OwnerSession {
			t.Fatal("expected owner remote attach session for started run ui")
		}
		return session, nil
	}

	if err := defaultAttachStartedCLIRunUI(context.Background(), client, "run-ui-owned-close-only"); err != nil {
		t.Fatalf("defaultAttachStartedCLIRunUI() close-only error = %v", err)
	}
	if client.cancelCalls != 0 {
		t.Fatalf("cancel calls = %d, want 0 when the UI closes without an explicit stop request", client.cancelCalls)
	}
}

func TestDefaultAttachCLIRunUIPassesWorkspaceRootToRemoteUI(t *testing.T) {
	t.Parallel()

	acquireCLITestGlobalOverride(t)

	client := &stubDaemonCommandClient{
		snapshot: apicore.RunSnapshot{
			Run: apicore.Run{RunID: "run-ui-workdir", WorkspaceID: "workspace-1", Status: "running"},
			Jobs: []apicore.RunJobState{
				{Index: 0, Status: "running"},
			},
		},
		workspace: apicore.Workspace{ID: "workspace-1", RootDir: "/tmp/compozy-workspace"},
	}
	session := newFakeCLIUISession()
	session.waitFn = func(*fakeCLIUISession) error {
		return nil
	}
	var workspaceRoot string

	originalOpenRemoteUI := openCLIRemoteUISession
	t.Cleanup(func() {
		openCLIRemoteUISession = originalOpenRemoteUI
	})
	openCLIRemoteUISession = func(
		_ context.Context,
		opts uipkg.RemoteAttachOptions,
	) (uipkg.Session, error) {
		workspaceRoot = opts.WorkspaceRoot
		return session, nil
	}

	if err := defaultAttachCLIRunUI(context.Background(), client, "run-ui-workdir"); err != nil {
		t.Fatalf("defaultAttachCLIRunUI() error = %v", err)
	}
	if workspaceRoot != "/tmp/compozy-workspace" {
		t.Fatalf("workspace root = %q, want daemon workspace root", workspaceRoot)
	}
}

func TestNewAttachStartedCLIRunUIUsesConfiguredOwnedRunCancelTimeout(t *testing.T) {
	t.Parallel()

	acquireCLITestGlobalOverride(t)

	client := &stubDaemonCommandClient{
		snapshot: apicore.RunSnapshot{
			Run: apicore.Run{RunID: "run-ui-owned-timeout", Status: "running"},
			Jobs: []apicore.RunJobState{
				{Index: 0, Status: "running"},
			},
		},
	}
	session := newFakeCLIUISession()

	originalOpenRemoteUI := openCLIRemoteUISession
	t.Cleanup(func() {
		openCLIRemoteUISession = originalOpenRemoteUI
	})
	openCLIRemoteUISession = func(
		_ context.Context,
		opts uipkg.RemoteAttachOptions,
	) (uipkg.Session, error) {
		if !opts.OwnerSession {
			t.Fatal("expected owner session when attaching started run")
		}
		return session, nil
	}

	timeout := 1500 * time.Millisecond
	start := time.Now()
	attachFn := newAttachStartedCLIRunUI(withOwnedRunCancelTimeout(timeout))
	if err := attachFn(context.Background(), client, "run-ui-owned-timeout"); err != nil {
		t.Fatalf("configured attach started ui error = %v", err)
	}
	if client.cancelCalls != 1 {
		t.Fatalf("cancel calls = %d, want 1", client.cancelCalls)
	}
	deadline, ok := client.cancelCtx.Deadline()
	if !ok {
		t.Fatal("expected configured cancel context deadline")
	}
	got := deadline.Sub(start)
	if got < time.Second || got > 2*time.Second {
		t.Fatalf("cancel deadline offset = %s, want ~%s", got, timeout)
	}
}

func TestLoadUIAttachSnapshotWaitsForJobsWhenInitialSnapshotIsEmpty(t *testing.T) {
	t.Parallel()

	var calls int
	client := &stubDaemonCommandClient{
		snapshotFunc: func(context.Context, string) (apicore.RunSnapshot, error) {
			calls++
			if calls == 1 {
				return apicore.RunSnapshot{
					Run: apicore.Run{RunID: "run-ui-warmup", Status: "running"},
				}, nil
			}
			return apicore.RunSnapshot{
				Run: apicore.Run{RunID: "run-ui-warmup", Status: "running"},
				Jobs: []apicore.RunJobState{
					{Index: 0, Status: "running"},
				},
			}, nil
		},
	}

	snapshot, err := loadUIAttachSnapshot(
		context.Background(),
		client,
		"run-ui-warmup",
		20*time.Millisecond,
		time.Millisecond,
	)
	if err != nil {
		t.Fatalf("loadUIAttachSnapshot() error = %v", err)
	}
	if calls < 2 {
		t.Fatalf("expected warmup polling, got %d snapshot calls", calls)
	}
	if got := len(snapshot.Jobs); got != 1 {
		t.Fatalf("snapshot jobs = %d, want 1", got)
	}
	if snapshot.Jobs[0].Status != "running" {
		t.Fatalf("snapshot job status = %q, want running", snapshot.Jobs[0].Status)
	}
}

func TestLoadUIAttachSnapshotReturnsPromptlyWhenContextCanceledDuringWarmup(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	var calls int
	client := &stubDaemonCommandClient{
		snapshotFunc: func(context.Context, string) (apicore.RunSnapshot, error) {
			calls++
			if calls == 1 {
				time.AfterFunc(20*time.Millisecond, cancel)
			}
			return apicore.RunSnapshot{
				Run: apicore.Run{RunID: "run-ui-canceled", Status: "running"},
			}, nil
		},
	}

	pollInterval := 500 * time.Millisecond
	start := time.Now()
	snapshot, err := loadUIAttachSnapshot(
		ctx,
		client,
		"run-ui-canceled",
		2*time.Second,
		pollInterval,
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("loadUIAttachSnapshot() error = %v, want context.Canceled", err)
	}
	if calls != 1 {
		t.Fatalf("snapshot calls = %d, want 1", calls)
	}
	if elapsed := time.Since(start); elapsed >= pollInterval/2 {
		t.Fatalf("loadUIAttachSnapshot() elapsed = %s, want less than %s", elapsed, pollInterval/2)
	}
	if got := len(snapshot.Jobs); got != 0 {
		t.Fatalf("snapshot jobs = %d, want 0", got)
	}
}

func TestNewAttachCLIRunUIDisablesWarmupWhenConfiguredTimeoutIsZero(t *testing.T) {
	t.Parallel()

	acquireCLITestGlobalOverride(t)

	var (
		calls            int
		capturedSnapshot apicore.RunSnapshot
	)
	client := &stubDaemonCommandClient{
		snapshotFunc: func(context.Context, string) (apicore.RunSnapshot, error) {
			calls++
			if calls == 1 {
				return apicore.RunSnapshot{
					Run: apicore.Run{RunID: "run-ui-no-warmup", Status: "running"},
				}, nil
			}
			return apicore.RunSnapshot{
				Run: apicore.Run{RunID: "run-ui-no-warmup", Status: "running"},
				Jobs: []apicore.RunJobState{
					{Index: 0, Status: "running"},
				},
			}, nil
		},
	}
	session := newFakeCLIUISession()
	session.Shutdown()

	originalOpenRemoteUI := openCLIRemoteUISession
	t.Cleanup(func() {
		openCLIRemoteUISession = originalOpenRemoteUI
	})
	openCLIRemoteUISession = func(
		_ context.Context,
		opts uipkg.RemoteAttachOptions,
	) (uipkg.Session, error) {
		capturedSnapshot = opts.Snapshot
		return session, nil
	}

	attachFn := newAttachCLIRunUI(
		withUIAttachSnapshotTimeout(0),
		withUIAttachSnapshotPollInterval(time.Millisecond),
	)
	if err := attachFn(context.Background(), client, "run-ui-no-warmup"); err != nil {
		t.Fatalf("configured attach ui error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("snapshot calls = %d, want 1", calls)
	}
	if got := len(capturedSnapshot.Jobs); got != 0 {
		t.Fatalf("captured snapshot jobs = %d, want 0", got)
	}
}

func TestHandleStartedTaskRunFallsBackToWatchWhenUIAttachIsAlreadySettled(t *testing.T) {
	t.Parallel()

	var (
		attachRunID string
		watchRunID  string
	)
	installTestCLIRunObservers(
		t,
		func(_ context.Context, _ daemonCommandClient, runID string) error {
			attachRunID = runID
			return errRunSettledBeforeUIAttach
		},
		func(_ context.Context, dst io.Writer, _ daemonCommandClient, runID string) error {
			watchRunID = runID
			_, err := io.WriteString(dst, "run completed | completed\n")
			return err
		},
	)

	var stdout bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)

	err := handleStartedTaskRun(
		context.Background(),
		cmd,
		&stubDaemonCommandClient{},
		apicore.Run{
			RunID:            "run-ui-settled",
			PresentationMode: attachModeUI,
		},
	)
	if err != nil {
		t.Fatalf("handleStartedTaskRun() error = %v", err)
	}
	if attachRunID != "run-ui-settled" {
		t.Fatalf("attach run id = %q, want run-ui-settled", attachRunID)
	}
	if watchRunID != "run-ui-settled" {
		t.Fatalf("watch run id = %q, want run-ui-settled", watchRunID)
	}
	if got := stdout.String(); got != "run completed | completed\n" {
		t.Fatalf("stdout = %q, want replay output", got)
	}
}

func TestDaemonStopCommandCancelsActiveRunsByDefault(t *testing.T) {
	t.Parallel()

	acquireCLITestGlobalOverride(t)

	readyClient := &stubDaemonCommandClient{}
	readyInfo := daemon.Info{
		PID:        4242,
		SocketPath: "/tmp/compozy-ready.sock",
		StartedAt:  time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC),
		State:      daemon.ReadyStateReady,
	}

	originalQueryStatus := queryDaemonCommandStatus
	originalNewClient := newDaemonCommandClientFromInfo
	queryDaemonCommandStatus = func(
		context.Context,
		compozyconfig.HomePaths,
		daemon.ProbeOptions,
	) (daemon.Status, error) {
		return daemon.Status{State: daemon.ReadyStateReady, Info: &readyInfo}, nil
	}
	newDaemonCommandClientFromInfo = func(daemon.Info) (daemonCommandClient, error) {
		return readyClient, nil
	}
	t.Cleanup(func() {
		queryDaemonCommandStatus = originalQueryStatus
		newDaemonCommandClientFromInfo = originalNewClient
	})

	cmd := newDaemonStopCommand()
	cmd.SetContext(context.Background())
	cmd.SetOut(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("daemon stop execute: %v", err)
	}
	if !readyClient.stopForce {
		t.Fatal("expected daemon stop to request forced cancellation by default")
	}
}

func acquireCLITestGlobalOverride(t *testing.T) {
	t.Helper()

	testName := t.Name()

	cliTestGlobalOverrides.Lock()
	if refs := cliTestGlobalOverrides.refs[testName]; refs > 0 {
		cliTestGlobalOverrides.refs[testName] = refs + 1
		cliTestGlobalOverrides.Unlock()
		t.Cleanup(func() {
			releaseCLITestGlobalOverride(testName)
		})
		return
	}
	cliTestGlobalOverrides.Unlock()

	cliTestGlobalOverrideMu.Lock()

	cliTestGlobalOverrides.Lock()
	cliTestGlobalOverrides.refs[testName] = 1
	cliTestGlobalOverrides.Unlock()

	t.Cleanup(func() {
		releaseCLITestGlobalOverride(testName)
	})
}

func releaseCLITestGlobalOverride(testName string) {
	cliTestGlobalOverrides.Lock()
	refs := cliTestGlobalOverrides.refs[testName]
	if refs <= 1 {
		delete(cliTestGlobalOverrides.refs, testName)
		cliTestGlobalOverrides.Unlock()
		cliTestGlobalOverrideMu.Unlock()
		return
	}
	cliTestGlobalOverrides.refs[testName] = refs - 1
	cliTestGlobalOverrides.Unlock()
}

func newTaskRunPresentationCommand(state *commandState) *cobra.Command {
	cmd := &cobra.Command{Use: "compozy tasks run"}
	cmd.Flags().StringVar(&state.attachMode, "attach", attachModeAuto, "attach mode")
	cmd.Flags().Bool("ui", false, "ui mode")
	cmd.Flags().Bool("stream", false, "stream mode")
	cmd.Flags().Bool("detach", false, "detach mode")
	return cmd
}

func decodeTaskRunOverrides(t *testing.T, raw json.RawMessage) daemonRuntimeOverrides {
	t.Helper()

	if len(raw) == 0 {
		return daemonRuntimeOverrides{}
	}
	var payload daemonRuntimeOverrides
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode task run overrides: %v", err)
	}
	return payload
}

func TestBuildTaskRunRuntimeOverridesParallelTasks(t *testing.T) {
	t.Parallel()

	t.Run("Should encode parallel task conflict resolver overrides", func(t *testing.T) {
		t.Parallel()

		state := newCommandState(commandKindTasksRun, core.ModePRDTasks)
		cmd := newTaskRunFlagCommandForTest(t, state)
		if err := cmd.Flags().Set(taskRunParallelTasksFlag, "true"); err != nil {
			t.Fatalf("set --parallel-tasks: %v", err)
		}
		for flag, value := range map[string]string{
			taskRunParallelConflictResolverIDEFlag:       "codex",
			taskRunParallelConflictResolverModelFlag:     "gpt-5.5",
			taskRunParallelConflictResolverReasoningFlag: "high",
		} {
			if err := cmd.Flags().Set(flag, value); err != nil {
				t.Fatalf("set --%s: %v", flag, err)
			}
		}

		raw, err := state.buildTaskRunRuntimeOverrides(cmd)
		if err != nil {
			t.Fatalf("buildTaskRunRuntimeOverrides() error = %v", err)
		}
		overrides := decodeTaskRunOverrides(t, raw)
		if overrides.ParallelTasks == nil {
			t.Fatal("expected parallel_tasks override")
		}
		if overrides.ParallelTasks.Enabled == nil || !*overrides.ParallelTasks.Enabled {
			t.Fatalf("parallel_tasks.enabled = %#v, want true", overrides.ParallelTasks.Enabled)
		}
		resolver := overrides.ParallelTasks.ConflictResolver
		if resolver == nil ||
			resolver.IDE == nil ||
			*resolver.IDE != "codex" ||
			resolver.Model == nil ||
			*resolver.Model != "gpt-5.5" ||
			resolver.ReasoningEffort == nil ||
			*resolver.ReasoningEffort != "high" {
			t.Fatalf("parallel conflict resolver override = %#v", resolver)
		}
	})
}

func newTaskRunFlagCommandForTest(t *testing.T, state *commandState) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "run"}
	addTaskRunFlags(cmd, state, taskRunFlagOptions{includeName: true})
	return cmd
}

func TestResolveTaskRunMultipleMode(t *testing.T) {
	t.Parallel()

	t.Run("Should default to enqueued", func(t *testing.T) {
		t.Parallel()

		state := newCommandState(commandKindTasksRun, core.ModePRDTasks)
		cmd := &cobra.Command{}
		mode, err := state.resolveTaskRunMultipleMode(cmd)
		if err != nil {
			t.Fatalf("resolveTaskRunMultipleMode() error = %v", err)
		}
		if mode != workspacecfg.TaskRunMultipleModeEnqueued {
			t.Fatalf("mode = %q, want enqueued", mode)
		}
	})

	t.Run("Should honor configured parallel mode without fallback message", func(t *testing.T) {
		t.Parallel()

		state := newCommandState(commandKindTasksRun, core.ModePRDTasks)
		state.projectConfig.Tasks.Run.RunMultipleMode = stringPointer("parallel")
		cmd := &cobra.Command{}
		var stderr bytes.Buffer
		cmd.SetErr(&stderr)
		mode, err := state.resolveTaskRunMultipleMode(cmd)
		if err != nil {
			t.Fatalf("resolveTaskRunMultipleMode() error = %v", err)
		}
		if mode != workspacecfg.TaskRunMultipleModeParallel {
			t.Fatalf("mode = %q, want parallel", mode)
		}
		if stderr.Len() != 0 {
			t.Fatalf("expected no fallback message, got %q", stderr.String())
		}
	})

	t.Run("Should resolve --parallel to parallel when config is unset", func(t *testing.T) {
		t.Parallel()

		state := newCommandState(commandKindTasksRun, core.ModePRDTasks)
		cmd := newTaskRunFlagCommandForTest(t, state)
		if err := cmd.Flags().Set("parallel", "true"); err != nil {
			t.Fatalf("set --parallel: %v", err)
		}
		mode, err := state.resolveTaskRunMultipleMode(cmd)
		if err != nil {
			t.Fatalf("resolveTaskRunMultipleMode() error = %v", err)
		}
		if mode != workspacecfg.TaskRunMultipleModeParallel {
			t.Fatalf("mode = %q, want parallel", mode)
		}
	})

	t.Run("Should let --parallel override configured enqueued mode", func(t *testing.T) {
		t.Parallel()

		state := newCommandState(commandKindTasksRun, core.ModePRDTasks)
		state.projectConfig.Tasks.Run.RunMultipleMode = stringPointer("enqueued")
		cmd := newTaskRunFlagCommandForTest(t, state)
		if err := cmd.Flags().Set("parallel", "true"); err != nil {
			t.Fatalf("set --parallel: %v", err)
		}
		mode, err := state.resolveTaskRunMultipleMode(cmd)
		if err != nil {
			t.Fatalf("resolveTaskRunMultipleMode() error = %v", err)
		}
		if mode != workspacecfg.TaskRunMultipleModeParallel {
			t.Fatalf("mode = %q, want parallel", mode)
		}
	})

	t.Run("Should let explicit --parallel=false override configured parallel mode", func(t *testing.T) {
		t.Parallel()

		state := newCommandState(commandKindTasksRun, core.ModePRDTasks)
		state.projectConfig.Tasks.Run.RunMultipleMode = stringPointer("parallel")
		cmd := newTaskRunFlagCommandForTest(t, state)
		if err := cmd.Flags().Set("parallel", "false"); err != nil {
			t.Fatalf("set --parallel=false: %v", err)
		}
		mode, err := state.resolveTaskRunMultipleMode(cmd)
		if err != nil {
			t.Fatalf("resolveTaskRunMultipleMode() error = %v", err)
		}
		if mode != workspacecfg.TaskRunMultipleModeEnqueued {
			t.Fatalf("mode = %q, want enqueued", mode)
		}
	})

	t.Run("Should return error for invalid internal value", func(t *testing.T) {
		t.Parallel()

		state := newCommandState(commandKindTasksRun, core.ModePRDTasks)
		state.projectConfig.Tasks.Run.RunMultipleMode = stringPointer("bogus")
		_, err := state.resolveTaskRunMultipleMode(&cobra.Command{})
		if err == nil || !strings.Contains(err.Error(), "tasks.run.run_multiple_mode") {
			t.Fatalf("expected invalid mode error, got %v", err)
		}
	})

	t.Run("Should not treat --parallel-tasks as slug multi-run parallel mode", func(t *testing.T) {
		t.Parallel()

		state := newCommandState(commandKindTasksRun, core.ModePRDTasks)
		cmd := newTaskRunFlagCommandForTest(t, state)
		if err := cmd.Flags().Set(taskRunParallelTasksFlag, "true"); err != nil {
			t.Fatalf("set --parallel-tasks: %v", err)
		}
		mode, err := state.resolveTaskRunMultipleMode(cmd)
		if err != nil {
			t.Fatalf("resolveTaskRunMultipleMode() error = %v", err)
		}
		if mode != workspacecfg.TaskRunMultipleModeEnqueued {
			t.Fatalf("mode = %q, want enqueued", mode)
		}
	})
}

func TestResolveTaskRunMultipleParallelLimit(t *testing.T) {
	t.Parallel()

	t.Run("Should default to workspace default when unset", func(t *testing.T) {
		t.Parallel()

		state := newCommandState(commandKindTasksRun, core.ModePRDTasks)
		cmd := newTaskRunFlagCommandForTest(t, state)
		limit, err := state.resolveTaskRunMultipleParallelLimit(cmd)
		if err != nil {
			t.Fatalf("resolveTaskRunMultipleParallelLimit() error = %v", err)
		}
		if limit != workspacecfg.DefaultRunMultipleParallelLimit {
			t.Fatalf("limit = %d, want %d", limit, workspacecfg.DefaultRunMultipleParallelLimit)
		}
	})

	t.Run("Should use configured limit when flag is unset", func(t *testing.T) {
		t.Parallel()

		state := newCommandState(commandKindTasksRun, core.ModePRDTasks)
		state.projectConfig.Tasks.Run.RunMultipleParallelLimit = intPointer(5)
		cmd := newTaskRunFlagCommandForTest(t, state)
		limit, err := state.resolveTaskRunMultipleParallelLimit(cmd)
		if err != nil {
			t.Fatalf("resolveTaskRunMultipleParallelLimit() error = %v", err)
		}
		if limit != 5 {
			t.Fatalf("limit = %d, want 5", limit)
		}
	})

	t.Run("Should let --parallel-limit override config and default", func(t *testing.T) {
		t.Parallel()

		state := newCommandState(commandKindTasksRun, core.ModePRDTasks)
		state.projectConfig.Tasks.Run.RunMultipleParallelLimit = intPointer(5)
		cmd := newTaskRunFlagCommandForTest(t, state)
		if err := cmd.Flags().Set("parallel-limit", "3"); err != nil {
			t.Fatalf("set --parallel-limit: %v", err)
		}
		limit, err := state.resolveTaskRunMultipleParallelLimit(cmd)
		if err != nil {
			t.Fatalf("resolveTaskRunMultipleParallelLimit() error = %v", err)
		}
		if limit != 3 {
			t.Fatalf("limit = %d, want 3", limit)
		}
	})

	t.Run("Should reject zero limit", func(t *testing.T) {
		t.Parallel()

		state := newCommandState(commandKindTasksRun, core.ModePRDTasks)
		cmd := newTaskRunFlagCommandForTest(t, state)
		if err := cmd.Flags().Set("parallel-limit", "0"); err != nil {
			t.Fatalf("set --parallel-limit: %v", err)
		}
		_, err := state.resolveTaskRunMultipleParallelLimit(cmd)
		if err == nil || !strings.Contains(err.Error(), "must be greater than 0") {
			t.Fatalf("expected zero-limit error, got %v", err)
		}
	})

	t.Run("Should reject negative limit", func(t *testing.T) {
		t.Parallel()

		state := newCommandState(commandKindTasksRun, core.ModePRDTasks)
		cmd := newTaskRunFlagCommandForTest(t, state)
		if err := cmd.Flags().Set("parallel-limit", "-1"); err != nil {
			t.Fatalf("set --parallel-limit: %v", err)
		}
		_, err := state.resolveTaskRunMultipleParallelLimit(cmd)
		if err == nil || !strings.Contains(err.Error(), "must be greater than 0") {
			t.Fatalf("expected negative-limit error, got %v", err)
		}
	})
}

func TestRejectMultipleOnlyParallelFlags(t *testing.T) {
	t.Parallel()

	t.Run("Should allow run without parallel flags", func(t *testing.T) {
		t.Parallel()

		state := newCommandState(commandKindTasksRun, core.ModePRDTasks)
		cmd := newTaskRunFlagCommandForTest(t, state)
		if err := rejectMultipleOnlyParallelFlags(cmd); err != nil {
			t.Fatalf("rejectMultipleOnlyParallelFlags() error = %v", err)
		}
	})

	t.Run("Should reject --parallel without --multiple", func(t *testing.T) {
		t.Parallel()

		state := newCommandState(commandKindTasksRun, core.ModePRDTasks)
		cmd := newTaskRunFlagCommandForTest(t, state)
		if err := cmd.Flags().Set("parallel", "true"); err != nil {
			t.Fatalf("set --parallel: %v", err)
		}
		err := rejectMultipleOnlyParallelFlags(cmd)
		if err == nil || !strings.Contains(err.Error(), "--parallel is only valid with --multiple") {
			t.Fatalf("expected --parallel rejection, got %v", err)
		}
	})

	t.Run("Should reject --parallel-limit without --multiple", func(t *testing.T) {
		t.Parallel()

		state := newCommandState(commandKindTasksRun, core.ModePRDTasks)
		cmd := newTaskRunFlagCommandForTest(t, state)
		if err := cmd.Flags().Set("parallel-limit", "3"); err != nil {
			t.Fatalf("set --parallel-limit: %v", err)
		}
		err := rejectMultipleOnlyParallelFlags(cmd)
		if err == nil || !strings.Contains(err.Error(), "--parallel-limit is only valid with --multiple") {
			t.Fatalf("expected --parallel-limit rejection, got %v", err)
		}
	})
}

func TestRenderObservedTaskMultiLifecycle(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		kind    eventspkg.EventKind
		payload kinds.TaskRunMultiplePayload
		want    string
	}{
		{
			name:    "Should render queue started",
			kind:    eventspkg.EventKindTaskRunMultipleStarted,
			payload: kinds.TaskRunMultiplePayload{Mode: "enqueued", Slugs: []string{"alpha", "beta"}, Total: 2},
			want:    "task queue started | mode=enqueued total=2\n",
		},
		{
			name:    "Should render item queued",
			kind:    eventspkg.EventKindTaskRunMultipleItemQueued,
			payload: kinds.TaskRunMultiplePayload{Slug: "alpha", Index: 0, Total: 2, Status: "queued"},
			want:    "task[1/2] alpha queued\n",
		},
		{
			name: "Should render child started",
			kind: eventspkg.EventKindTaskRunMultipleChildStarted,
			payload: kinds.TaskRunMultiplePayload{
				Slug:       "alpha",
				Index:      0,
				Total:      2,
				Status:     "running",
				ChildRunID: "child-alpha",
			},
			want: "task[1/2] alpha running | run=child-alpha\n",
		},
		{
			name: "Should render child completed",
			kind: eventspkg.EventKindTaskRunMultipleChildCompleted,
			payload: kinds.TaskRunMultiplePayload{
				Slug:       "alpha",
				Index:      0,
				Total:      2,
				Status:     "completed",
				ChildRunID: "child-alpha",
			},
			want: "task[1/2] alpha completed | run=child-alpha\n",
		},
		{
			name: "Should render child failed",
			kind: eventspkg.EventKindTaskRunMultipleChildFailed,
			payload: kinds.TaskRunMultiplePayload{
				Slug:       "alpha",
				Index:      0,
				Total:      2,
				Status:     "failed",
				ChildRunID: "child-alpha",
				Error:      "forced failure",
			},
			want: "task[1/2] alpha failed | run=child-alpha | forced failure\n",
		},
		{
			name: "Should render item canceled",
			kind: eventspkg.EventKindTaskRunMultipleItemCanceled,
			payload: kinds.TaskRunMultiplePayload{
				Slug:   "beta",
				Index:  1,
				Total:  2,
				Status: "canceled",
				Error:  "parent failed",
			},
			want: "task[2/2] beta canceled | parent failed\n",
		},
		{
			name:    "Should render queue completed",
			kind:    eventspkg.EventKindTaskRunMultipleQueueCompleted,
			payload: kinds.TaskRunMultiplePayload{Total: 2},
			want:    "task queue completed | total=2\n",
		},
		{
			name:    "Should render queue canceled",
			kind:    eventspkg.EventKindTaskRunMultipleQueueCanceled,
			payload: kinds.TaskRunMultiplePayload{Error: "stop requested"},
			want:    "task queue canceled | stop requested\n",
		},
		{
			name:    "Should render queue failed",
			kind:    eventspkg.EventKindTaskRunMultipleQueueFailed,
			payload: kinds.TaskRunMultiplePayload{Error: "child failed"},
			want:    "task queue failed | child failed\n",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			raw, err := json.Marshal(tc.payload)
			if err != nil {
				t.Fatalf("marshal payload: %v", err)
			}
			got := renderObservedRunEvent(eventspkg.Event{
				Kind:    tc.kind,
				Payload: raw,
			})
			if got != tc.want {
				t.Fatalf("renderObservedRunEvent() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRenderObservedTaskMultiLifecycleFallbacks(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		kind eventspkg.EventKind
		want string
	}{
		{
			name: "Should render queue started fallback",
			kind: eventspkg.EventKindTaskRunMultipleStarted,
			want: "task queue started\n",
		},
		{
			name: "Should render item failed fallback",
			kind: eventspkg.EventKindTaskRunMultipleChildFailed,
			want: "task failed\n",
		},
		{
			name: "Should render queue completed fallback",
			kind: eventspkg.EventKindTaskRunMultipleQueueCompleted,
			want: "task queue completed\n",
		},
		{
			name: "Should render queue canceled fallback",
			kind: eventspkg.EventKindTaskRunMultipleQueueCanceled,
			want: "task queue canceled\n",
		},
		{
			name: "Should render queue failed fallback",
			kind: eventspkg.EventKindTaskRunMultipleQueueFailed,
			want: "task queue failed\n",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := renderObservedRunEvent(eventspkg.Event{
				Kind:    tc.kind,
				Payload: []byte("{"),
			})
			if got != tc.want {
				t.Fatalf("renderObservedRunEvent() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRenderObservedTaskParallelLifecycle(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		kind    eventspkg.EventKind
		payload json.RawMessage
		want    string
	}{
		{
			name: "Should render the task DAG plan",
			kind: eventspkg.EventKindTaskParallelPlanStarted,
			payload: mustMarshalCLIJSON(t, kinds.TaskParallelPlanPayload{
				Workflow: "demo",
				Tasks:    []kinds.TaskParallelPlanTask{{ID: "task_01"}, {ID: "task_02"}},
				Waves:    []kinds.TaskParallelPlanWave{{Index: 0}, {Index: 1}},
			}),
			want: "parallel task plan started | workflow=demo tasks=2 waves=2 worktrees=true\n",
		},
		{
			name: "Should render finalize activity",
			kind: eventspkg.EventKindTaskParallelPhaseChanged,
			payload: mustMarshalCLIJSON(t, kinds.TaskParallelPayload{
				WaveIndex: 1,
				WaveTotal: 2,
				Phase:     "syncing_artifacts",
			}),
			want: "parallel phase changed | wave=2/2 | phase=syncing_artifacts\n",
		},
		{
			name: "Should render per-task settlement",
			kind: eventspkg.EventKindTaskParallelTaskCompleted,
			payload: mustMarshalCLIJSON(t, kinds.TaskParallelPayload{
				TaskID:         "task_02",
				Status:         "failed",
				WorktreePath:   "/wt/task_02",
				WorktreeStatus: "preserved",
				WorktreeReason: "uncommitted changes",
				ResultBranch:   "compozy/result-task-02",
				Error:          "boom",
			}),
			want: "parallel task completed | task=task_02 | status=failed | worktree=/wt/task_02 | " +
				"worktree_status=preserved | result_branch=compozy/result-task-02 | " +
				"worktree_reason=uncommitted changes | error=boom\n",
		},
		{
			name: "Should render parallel settlement",
			kind: eventspkg.EventKindTaskParallelCompleted,
			payload: mustMarshalCLIJSON(
				t,
				kinds.TaskParallelPayload{Status: "completed", Phase: "completed"},
			),
			want: "parallel execution completed | phase=completed | status=completed\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := renderObservedRunEvent(eventspkg.Event{Kind: tc.kind, Payload: tc.payload})
			if got != tc.want {
				t.Fatalf("renderObservedRunEvent() = %q, want %q", got, tc.want)
			}
		})
	}
}

func mustMarshalCLIJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return raw
}

func TestRenderObservedTaskMultiItemIncludesWorktreePath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		kind    eventspkg.EventKind
		payload kinds.TaskRunMultiplePayload
		want    string
	}{
		{
			name: "Should include worktree path on child started",
			kind: eventspkg.EventKindTaskRunMultipleChildStarted,
			payload: kinds.TaskRunMultiplePayload{
				Slug: "alpha", Index: 0, Total: 2, Status: "running",
				ChildRunID: "child-alpha", WorktreePath: "/wt/01-alpha",
			},
			want: "task[1/2] alpha running | run=child-alpha | worktree=/wt/01-alpha\n",
		},
		{
			name: "Should include worktree path on child completed",
			kind: eventspkg.EventKindTaskRunMultipleChildCompleted,
			payload: kinds.TaskRunMultiplePayload{
				Slug: "alpha", Index: 0, Total: 2, Status: "completed",
				ChildRunID: "child-alpha", WorktreePath: "/wt/01-alpha",
			},
			want: "task[1/2] alpha completed | run=child-alpha | worktree=/wt/01-alpha\n",
		},
		{
			name: "Should include worktree path and error on child failed",
			kind: eventspkg.EventKindTaskRunMultipleChildFailed,
			payload: kinds.TaskRunMultiplePayload{
				Slug: "beta", Index: 1, Total: 2, Status: "failed",
				ChildRunID: "child-beta", WorktreePath: "/wt/02-beta", Error: "boom",
			},
			want: "task[2/2] beta failed | run=child-beta | worktree=/wt/02-beta | boom\n",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			raw, err := json.Marshal(tc.payload)
			if err != nil {
				t.Fatalf("marshal payload: %v", err)
			}
			got := renderObservedRunEvent(eventspkg.Event{Kind: tc.kind, Payload: raw})
			if got != tc.want {
				t.Fatalf("renderObservedRunEvent() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFormatTaskRunMultipleHandoff(t *testing.T) {
	t.Parallel()

	t.Run("Should format children in requested order", func(t *testing.T) {
		t.Parallel()
		snapshot := apicore.TaskRunMultipleSnapshot{
			Items: []apicore.TaskRunMultipleItem{
				{
					Slug: "alpha", Status: "completed", RunID: "child-alpha",
					WorktreePath: "/wt/01-alpha", BaseBranch: "main",
					ResultBranch: "compozy/multi-parent-01-alpha", WorktreeStatus: "removed",
				},
				{
					Slug: "beta", Status: "failed", RunID: "child-beta",
					WorktreePath: "/wt/02-beta", ErrorText: "boom",
					WorktreeStatus: "preserved", WorktreeReason: "uncommitted changes",
				},
			},
		}
		lines := formatTaskRunMultipleHandoff(snapshot)
		want := []string{
			"task multi-run handoff:\n",
			"  alpha completed | run=child-alpha | worktree=/wt/01-alpha | base_branch=main | " +
				"result_branch=compozy/multi-parent-01-alpha | worktree_status=removed\n",
			"  beta failed | run=child-beta | worktree=/wt/02-beta | worktree_status=preserved | " +
				"worktree_reason=uncommitted changes | boom\n",
		}
		if !slices.Equal(lines, want) {
			t.Fatalf("formatTaskRunMultipleHandoff() = %#v, want %#v", lines, want)
		}
	})

	t.Run("Should render dash for missing worktree metadata", func(t *testing.T) {
		t.Parallel()
		snapshot := apicore.TaskRunMultipleSnapshot{
			Items: []apicore.TaskRunMultipleItem{{Slug: "alpha", Status: "completed"}},
		}
		lines := formatTaskRunMultipleHandoff(snapshot)
		if len(lines) != 2 || lines[1] != "  alpha completed | run=- | worktree=-\n" {
			t.Fatalf("unexpected handoff lines: %#v", lines)
		}
	})

	t.Run("Should return nil when no items", func(t *testing.T) {
		t.Parallel()
		if lines := formatTaskRunMultipleHandoff(apicore.TaskRunMultipleSnapshot{}); lines != nil {
			t.Fatalf("expected nil handoff, got %#v", lines)
		}
	})
}

func TestDaemonStartCommandDetachedReturnsReadyStatus(t *testing.T) {
	acquireCLITestGlobalOverride(t)

	readyClient := &stubDaemonCommandClient{
		target: apiclient.Target{SocketPath: "/tmp/compozy-ready.sock"},
		status: apicore.DaemonStatus{
			PID:            4242,
			Version:        "test-version",
			StartedAt:      time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC),
			SocketPath:     "/tmp/compozy-ready.sock",
			HTTPPort:       2323,
			ActiveRunCount: 2,
			WorkspaceCount: 3,
		},
		health: apicore.DaemonHealth{Ready: true},
	}
	var launchCalls int
	installTestCLIDaemonBootstrap(t, cliDaemonBootstrap{
		resolveHomePaths: func() (compozyconfig.HomePaths, error) {
			return compozyconfig.HomePaths{InfoPath: "/tmp/compozy-home/daemon.json"}, nil
		},
		readInfo: func(string) (daemon.Info, error) {
			return daemon.Info{
				PID:        4242,
				Version:    "test-version",
				SocketPath: "/tmp/compozy-ready.sock",
				StartedAt:  time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC),
				State:      daemon.ReadyStateReady,
			}, nil
		},
		newClient: func(apiclient.Target) (daemonCommandClient, error) {
			return readyClient, nil
		},
		launch: func(compozyconfig.HomePaths) error {
			launchCalls++
			return nil
		},
		sleep:          func(time.Duration) {},
		now:            func() time.Time { return time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC) },
		cliVersion:     func() string { return "test-version" },
		startupTimeout: time.Second,
		pollInterval:   time.Millisecond,
	})

	output, err := executeCommandCombinedOutput(newDaemonStartCommand(), nil, "--format", "json")
	if err != nil {
		t.Fatalf("execute daemon start: %v\noutput:\n%s", err, output)
	}
	if launchCalls != 0 {
		t.Fatalf("expected healthy daemon reuse without launch, got %d launch attempts", launchCalls)
	}

	var payload daemonStatusOutput
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("decode daemon start payload: %v\noutput:\n%s", err, output)
	}
	if payload.State != string(daemon.ReadyStateReady) || !payload.Health.Ready {
		t.Fatalf("unexpected daemon start payload: %#v", payload)
	}
	if payload.Daemon == nil || payload.Daemon.PID != 4242 || payload.Daemon.WorkspaceCount != 3 {
		t.Fatalf("unexpected daemon start status payload: %#v", payload)
	}
}

func TestDaemonStartCommandForegroundUsesDaemonRunner(t *testing.T) {
	acquireCLITestGlobalOverride(t)

	originalRunner := runCLIDaemonForeground
	t.Cleanup(func() {
		runCLIDaemonForeground = originalRunner
	})

	ctxKey := daemonCommandContextKey("foreground")
	var (
		called bool
		gotCtx context.Context
		gotRun daemon.RunOptions
	)
	runCLIDaemonForeground = func(ctx context.Context, opts daemon.RunOptions) error {
		called = true
		gotCtx = ctx
		gotRun = opts
		return nil
	}
	t.Setenv(daemonHTTPPortEnv, "43123")
	t.Setenv(daemonWebDevProxyEnv, "http://127.0.0.1:3000")

	cmd := newDaemonStartCommand()
	cmd.SetContext(context.WithValue(context.Background(), ctxKey, "foreground-start"))
	output, err := executeCommandCombinedOutput(
		cmd,
		nil,
		"--foreground",
		"--web-dev-proxy",
		"http://127.0.0.1:3100",
	)
	if err != nil {
		t.Fatalf("execute daemon start --foreground: %v\noutput:\n%s", err, output)
	}
	if !called {
		t.Fatal("expected foreground daemon runner to be called")
	}
	if gotCtx == nil || gotCtx.Value(ctxKey) != "foreground-start" {
		t.Fatalf("foreground daemon context = %#v, want command context value", gotCtx)
	}
	if gotRun.HTTPPort != 43123 {
		t.Fatalf("foreground daemon http port = %d, want 43123", gotRun.HTTPPort)
	}
	if gotRun.Mode != daemon.RunModeForeground {
		t.Fatalf("foreground daemon mode = %q, want %q", gotRun.Mode, daemon.RunModeForeground)
	}
	if gotRun.WebDevProxyTarget != "http://127.0.0.1:3100" {
		t.Fatalf("foreground daemon web dev proxy = %q, want %q", gotRun.WebDevProxyTarget, "http://127.0.0.1:3100")
	}
	if strings.TrimSpace(gotRun.Version) == "" {
		t.Fatalf("expected foreground daemon version to be populated, got %#v", gotRun)
	}
	if output != "" {
		t.Fatalf("expected foreground daemon start to stay quiet, got %q", output)
	}
}

func TestDaemonStartCommandInternalChildUsesDetachedRunMode(t *testing.T) {
	acquireCLITestGlobalOverride(t)

	originalRunner := runCLIDaemonForeground
	t.Cleanup(func() {
		runCLIDaemonForeground = originalRunner
	})

	ctxKey := daemonCommandContextKey("internal-child")
	var (
		called bool
		gotCtx context.Context
		gotRun daemon.RunOptions
	)
	runCLIDaemonForeground = func(ctx context.Context, opts daemon.RunOptions) error {
		called = true
		gotCtx = ctx
		gotRun = opts
		return nil
	}
	t.Setenv(daemonHTTPPortEnv, "43124")

	cmd := newDaemonStartCommand()
	cmd.SetContext(context.WithValue(context.Background(), ctxKey, "detached-child"))
	output, err := executeCommandCombinedOutput(cmd, nil, "--"+daemonStartInternalChildFlag)
	if err != nil {
		t.Fatalf("execute daemon start --internal-child: %v\noutput:\n%s", err, output)
	}
	if !called {
		t.Fatal("expected detached daemon runner to be called")
	}
	if gotCtx == nil || gotCtx.Value(ctxKey) != "detached-child" {
		t.Fatalf("detached daemon context = %#v, want command context value", gotCtx)
	}
	if gotRun.HTTPPort != 43124 {
		t.Fatalf("detached daemon http port = %d, want 43124", gotRun.HTTPPort)
	}
	if gotRun.Mode != daemon.RunModeDetached {
		t.Fatalf("detached daemon mode = %q, want %q", gotRun.Mode, daemon.RunModeDetached)
	}
	if output != "" {
		t.Fatalf("expected internal child daemon start to stay quiet, got %q", output)
	}
}

func TestLaunchCLIDaemonProcessFailsWhenDaemonLogFileCannotBeOpened(t *testing.T) {
	t.Parallel()

	paths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	paths.LogFile = paths.LogsDir

	err = launchCLIDaemonProcessWithExecutable(paths, filepath.Join(t.TempDir(), "unused-compozy"))
	if err == nil {
		t.Fatal("expected launchCLIDaemonProcessWithExecutable to fail when the daemon log file path is a directory")
	}
	if !strings.Contains(err.Error(), "open daemon log file") {
		t.Fatalf("unexpected detached launch error: %v", err)
	}
}

func TestCLIDaemonRunOptionsFromEnvRejectsInvalidWebDevProxyTarget(t *testing.T) {
	t.Setenv(daemonWebDevProxyEnv, "ws://127.0.0.1:3000")

	_, err := cliDaemonRunOptionsFromEnv(daemon.RunModeDetached)
	if err == nil {
		t.Fatal("expected invalid web dev proxy target to fail")
	}
	if !strings.Contains(err.Error(), "must use http or https") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveDaemonWebDevProxyTargetRejectsInvalidFlagValueWithFlagContext(t *testing.T) {
	_, err := resolveDaemonWebDevProxyTarget("ws://127.0.0.1:3000")
	if err == nil {
		t.Fatal("expected invalid web dev proxy flag to fail")
	}
	if !strings.Contains(err.Error(), daemonWebDevProxyFlag) {
		t.Fatalf("resolveDaemonWebDevProxyTarget() error = %v, want %s context", err, daemonWebDevProxyFlag)
	}
	if strings.Contains(err.Error(), daemonWebDevProxyEnv) {
		t.Fatalf("resolveDaemonWebDevProxyTarget() error = %v, do not want %s context", err, daemonWebDevProxyEnv)
	}
}

func TestOverrideDaemonWebDevProxyEnv(t *testing.T) {
	t.Run("Should apply and restore a valid override", func(t *testing.T) {
		t.Setenv(daemonWebDevProxyEnv, "http://127.0.0.1:3000")

		restore, err := overrideDaemonWebDevProxyEnv("http://127.0.0.1:3100")
		if err != nil {
			t.Fatalf("overrideDaemonWebDevProxyEnv() error = %v", err)
		}
		currentValue, ok := os.LookupEnv(daemonWebDevProxyEnv)
		if !ok || currentValue != "http://127.0.0.1:3100" {
			t.Fatalf(
				"overrideDaemonWebDevProxyEnv() env = (%t, %q), want (%t, %q)",
				ok,
				currentValue,
				true,
				"http://127.0.0.1:3100",
			)
		}
		if err := restore(); err != nil {
			t.Fatalf("restore() error = %v", err)
		}
		restoredValue, ok := os.LookupEnv(daemonWebDevProxyEnv)
		if !ok || restoredValue != "http://127.0.0.1:3000" {
			t.Fatalf("restore() env = (%t, %q), want (%t, %q)", ok, restoredValue, true, "http://127.0.0.1:3000")
		}
	})

	t.Run("Should reject values os.Setenv cannot store", func(t *testing.T) {
		restore, err := overrideDaemonWebDevProxyEnv("http://127.0.0.1:3100\x00")
		if err == nil {
			t.Fatal("overrideDaemonWebDevProxyEnv() error = nil, want non-nil")
		}
		if restore != nil {
			t.Fatal("overrideDaemonWebDevProxyEnv() restore should be nil on failure")
		}
	})
}

func TestDaemonStartCommandFlagOverridesInvalidWebDevProxyEnv(t *testing.T) {
	acquireCLITestGlobalOverride(t)

	originalRunner := runCLIDaemonForeground
	t.Cleanup(func() {
		runCLIDaemonForeground = originalRunner
	})

	var (
		called bool
		gotRun daemon.RunOptions
	)
	runCLIDaemonForeground = func(_ context.Context, opts daemon.RunOptions) error {
		called = true
		gotRun = opts
		return nil
	}

	t.Setenv(daemonHTTPPortEnv, "43123")
	t.Setenv(daemonWebDevProxyEnv, "ws://127.0.0.1:3000")

	cmd := newDaemonStartCommand()
	output, err := executeCommandCombinedOutput(
		cmd,
		nil,
		"--foreground",
		"--web-dev-proxy",
		"http://127.0.0.1:3100",
	)
	if err != nil {
		t.Fatalf("execute daemon start --foreground with invalid env override: %v\noutput:\n%s", err, output)
	}
	if !called {
		t.Fatal("expected foreground daemon runner to be called")
	}
	if gotRun.HTTPPort != 43123 {
		t.Fatalf("foreground daemon http port = %d, want 43123", gotRun.HTTPPort)
	}
	if gotRun.WebDevProxyTarget != "http://127.0.0.1:3100" {
		t.Fatalf("foreground daemon web dev proxy = %q, want %q", gotRun.WebDevProxyTarget, "http://127.0.0.1:3100")
	}
	if gotRun.Mode != daemon.RunModeForeground {
		t.Fatalf("foreground daemon mode = %q, want %q", gotRun.Mode, daemon.RunModeForeground)
	}
	if strings.TrimSpace(gotRun.Version) == "" {
		t.Fatalf("expected foreground daemon version to be populated, got %#v", gotRun)
	}
	if strings.TrimSpace(output) != "" {
		t.Fatalf("expected foreground daemon start to stay quiet, got %q", output)
	}
}
func TestDaemonStartCommandRejectsInvalidFormatBeforeEarlyReturn(t *testing.T) {
	acquireCLITestGlobalOverride(t)

	originalRunner := runCLIDaemonForeground
	t.Cleanup(func() {
		runCLIDaemonForeground = originalRunner
	})

	var called bool
	runCLIDaemonForeground = func(context.Context, daemon.RunOptions) error {
		called = true
		return nil
	}

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "foreground",
			args: []string{"--foreground", "--format", "garbage"},
		},
		{
			name: "internal child",
			args: []string{"--" + daemonStartInternalChildFlag, "--format", "garbage"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called = false

			output, err := executeCommandCombinedOutput(newDaemonStartCommand(), nil, tt.args...)
			if err == nil {
				t.Fatalf("expected %s invalid format to fail", tt.name)
			}

			var exitErr *commandExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("expected commandExitError, got %T", err)
			}
			if exitErr.ExitCode() != 1 {
				t.Fatalf("unexpected exit code: got %d want 1", exitErr.ExitCode())
			}
			if !strings.Contains(output, "output format must be one of text or json") {
				t.Fatalf("unexpected %s output:\n%s", tt.name, output)
			}
			if called {
				t.Fatalf("expected %s invalid format to fail before launching foreground runner", tt.name)
			}
		})
	}
}

func TestResolveLaunchCLIDaemonExecutableRejectsGoTestBinary(t *testing.T) {
	original := resolveCLIDaemonExecutable
	resolveCLIDaemonExecutable = func() (string, error) {
		return filepath.Join(t.TempDir(), "cli.test"), nil
	}
	t.Cleanup(func() {
		resolveCLIDaemonExecutable = original
	})

	_, err := resolveLaunchCLIDaemonExecutable()
	if err == nil {
		t.Fatal("expected go test binary rejection")
	}
	if !strings.Contains(err.Error(), "Go test binary") {
		t.Fatalf("unexpected rejection error: %v", err)
	}
}

func TestResolveTaskPresentationModeUsesInjectedInteractiveCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		interactive   bool
		wantMode      string
		wantErrSubstr string
		configure     func(*testing.T, *commandState, *cobra.Command)
	}{
		{
			name:        "auto resolves to ui on interactive terminals",
			interactive: true,
			wantMode:    attachModeUI,
		},
		{
			name:        "auto resolves to stream on non-interactive terminals",
			interactive: false,
			wantMode:    attachModeStream,
		},
		{
			name:          "explicit ui requires an interactive terminal",
			interactive:   false,
			wantErrSubstr: "requires an interactive terminal for ui mode",
			configure: func(t *testing.T, state *commandState, cmd *cobra.Command) {
				t.Helper()
				if err := cmd.Flags().Set("attach", attachModeUI); err != nil {
					t.Fatalf("set attach: %v", err)
				}
				state.attachMode = attachModeUI
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			state := newCommandState(commandKindTasksRun, "")
			state.isInteractive = func() bool { return tt.interactive }
			cmd := newTaskRunPresentationCommand(state)
			if tt.configure != nil {
				tt.configure(t, state, cmd)
			}

			got, err := state.resolveTaskPresentationMode(cmd)
			if tt.wantErrSubstr != "" {
				if err == nil {
					t.Fatalf("expected resolveTaskPresentationMode error, got mode %q", got)
				}
				if got != "" {
					t.Fatalf("expected no resolved mode on error, got %q", got)
				}
				if gotErr := err.Error(); !containsAll(gotErr, tt.wantErrSubstr) {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("resolveTaskPresentationMode: %v", err)
			}
			if got != tt.wantMode {
				t.Fatalf("unexpected presentation mode: got %q want %q", got, tt.wantMode)
			}
		})
	}
}

func TestNewDefaultCLIDaemonBootstrapProvidesRuntimeDependencies(t *testing.T) {
	t.Parallel()

	bootstrap := newDefaultCLIDaemonBootstrap()
	if bootstrap.resolveHomePaths == nil || bootstrap.readInfo == nil || bootstrap.newClient == nil ||
		bootstrap.launch == nil || bootstrap.cliVersion == nil || bootstrap.notify == nil {
		t.Fatalf("expected daemon bootstrap dependencies to be wired: %#v", bootstrap)
	}
	if bootstrap.sleep == nil || bootstrap.now == nil {
		t.Fatalf("expected daemon bootstrap timing hooks to be wired: %#v", bootstrap)
	}
	if bootstrap.startupTimeout != defaultDaemonStartupTimeout {
		t.Fatalf("unexpected startup timeout: %s", bootstrap.startupTimeout)
	}
	if bootstrap.pollInterval != defaultDaemonPollInterval {
		t.Fatalf("unexpected poll interval: %s", bootstrap.pollInterval)
	}

	client, err := bootstrap.newClient(apiclient.Target{HTTPPort: 43123})
	if err != nil {
		t.Fatalf("newClient: %v", err)
	}
	if client.Target().HTTPPort != 43123 {
		t.Fatalf("unexpected bootstrap client target: %#v", client.Target())
	}
}

func TestCLIDaemonBootstrapEnsureChecksDaemonVersion(t *testing.T) {
	t.Parallel()

	currentVersion := "v2.0.0 (commit=current date=2026-06-11)"
	currentVersionRebuilt := "v2.0.0 (commit=current date=2026-06-11T21:45:40Z)"
	currentVersionDifferentCommit := "v2.0.0 (commit=different date=2026-06-11)"
	oldVersion := "v1.9.0 (commit=old date=2026-06-10)"
	devVersion := "dev (commit=none date=unknown)"
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name              string
		cliVersion        string
		initialVersion    string
		initialStatus     apicore.DaemonStatus
		restartedVersion  string
		readInfoErrOnStop bool
		wantClient        string
		wantLaunchCalls   int
		wantStopCalls     int
		wantStopForce     bool
		wantNotice        []string
		wantErr           []string
	}{
		{
			name:           "Should reuse daemon when versions match",
			cliVersion:     currentVersion,
			initialVersion: currentVersion,
			initialStatus: apicore.DaemonStatus{
				Version: currentVersion,
			},
			wantClient: "initial",
		},
		{
			name:           "Should reuse busy daemon when release version and commit match but dates differ",
			cliVersion:     currentVersionRebuilt,
			initialVersion: currentVersion,
			initialStatus: apicore.DaemonStatus{
				Version:        currentVersion,
				ActiveRunCount: 2,
			},
			wantClient: "initial",
		},
		{
			name:           "Should reuse busy legacy daemon when commit differs",
			cliVersion:     currentVersionDifferentCommit,
			initialVersion: currentVersion,
			initialStatus: apicore.DaemonStatus{
				Version:        currentVersion,
				ActiveRunCount: 2,
			},
			wantClient: "initial",
		},
		{
			name:           "Should restart and reconnect idle daemon when release versions differ",
			cliVersion:     currentVersion,
			initialVersion: oldVersion,
			initialStatus: apicore.DaemonStatus{
				Version:        oldVersion,
				ActiveRunCount: 0,
			},
			restartedVersion:  currentVersion,
			readInfoErrOnStop: true,
			wantClient:        "restarted",
			wantLaunchCalls:   1,
			wantStopCalls:     1,
			wantNotice: []string{
				"Restarting stale compozy daemon",
				oldVersion,
				currentVersion,
			},
		},
		{
			name:           "Should reuse busy daemon when release versions differ and contract matches",
			cliVersion:     currentVersion,
			initialVersion: oldVersion,
			initialStatus: apicore.DaemonStatus{
				Version:         oldVersion,
				ContractVersion: contract.DaemonContractVersion,
				ActiveRunCount:  2,
			},
			wantClient: "initial",
		},
		{
			name:           "Should fail without stopping busy daemon when contract differs",
			cliVersion:     currentVersion,
			initialVersion: currentVersion,
			initialStatus: apicore.DaemonStatus{
				Version:         currentVersion,
				ContractVersion: "99",
				ActiveRunCount:  2,
			},
			wantErr: []string{
				"contract version",
				"99",
				contract.DaemonContractVersion,
				"2 active runs",
				"retry after",
				"compozy daemon stop --force",
			},
		},
		{
			name:           "Should restart and reconnect idle daemon when contract differs",
			cliVersion:     currentVersion,
			initialVersion: currentVersion,
			initialStatus: apicore.DaemonStatus{
				Version:         currentVersion,
				ContractVersion: "99",
				ActiveRunCount:  0,
			},
			restartedVersion:  currentVersion,
			readInfoErrOnStop: true,
			wantClient:        "restarted",
			wantLaunchCalls:   1,
			wantStopCalls:     1,
			wantNotice: []string{
				"Restarting incompatible compozy daemon",
				"99",
				contract.DaemonContractVersion,
			},
		},
		{
			name:           "Should reuse daemon when dev and empty versions are compatible",
			cliVersion:     devVersion,
			initialVersion: "",
			initialStatus:  apicore.DaemonStatus{},
			wantClient:     "initial",
		},
		{
			name:           "Should reuse daemon when daemon build is dev and CLI build is release",
			cliVersion:     currentVersion,
			initialVersion: devVersion,
			initialStatus: apicore.DaemonStatus{
				Version: devVersion,
			},
			wantClient: "initial",
		},
		{
			name:           "Should reuse daemon when CLI build is dev and daemon build is release",
			cliVersion:     devVersion,
			initialVersion: currentVersion,
			initialStatus: apicore.DaemonStatus{
				Version: currentVersion,
			},
			wantClient: "initial",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			initialInfo := daemon.Info{
				PID:        1234,
				Version:    tt.initialVersion,
				SocketPath: "/tmp/compozy-version-old.sock",
				StartedAt:  now,
				State:      daemon.ReadyStateReady,
			}
			restartedInfo := daemon.Info{
				PID:        4321,
				Version:    tt.restartedVersion,
				SocketPath: "/tmp/compozy-version-new.sock",
				StartedAt:  now.Add(time.Second),
				State:      daemon.ReadyStateReady,
			}
			initialClient := &stubDaemonCommandClient{
				target: apiclient.Target{SocketPath: initialInfo.SocketPath},
				health: apicore.DaemonHealth{
					Ready: true,
				},
				status: tt.initialStatus,
			}
			restartedClient := &stubDaemonCommandClient{
				target: apiclient.Target{SocketPath: restartedInfo.SocketPath},
				health: apicore.DaemonHealth{
					Ready: true,
				},
				status: apicore.DaemonStatus{
					Version:         tt.restartedVersion,
					ContractVersion: contract.DaemonContractVersion,
				},
			}

			var launchCalls int
			var notices []string
			var readInfoCalls int
			bootstrap := cliDaemonBootstrap{
				resolveHomePaths: func() (compozyconfig.HomePaths, error) {
					return compozyconfig.HomePaths{InfoPath: "/tmp/compozy-home/daemon.json"}, nil
				},
				readInfo: func(path string) (daemon.Info, error) {
					if path != "/tmp/compozy-home/daemon.json" {
						t.Fatalf("unexpected daemon info path: %q", path)
					}
					readInfoCalls++
					if readInfoCalls == 2 && tt.readInfoErrOnStop {
						return daemon.Info{}, fmt.Errorf("daemon info removed: %w", os.ErrNotExist)
					}
					if launchCalls > 0 {
						return restartedInfo, nil
					}
					return initialInfo, nil
				},
				newClient: func(target apiclient.Target) (daemonCommandClient, error) {
					switch target.SocketPath {
					case initialInfo.SocketPath:
						return initialClient, nil
					case restartedInfo.SocketPath:
						return restartedClient, nil
					default:
						return nil, fmt.Errorf("unexpected daemon target: %#v", target)
					}
				},
				launch: func(compozyconfig.HomePaths) error {
					launchCalls++
					return nil
				},
				sleep:          func(time.Duration) {},
				now:            func() time.Time { return now },
				startupTimeout: time.Second,
				pollInterval:   time.Millisecond,
				cliVersion:     func() string { return tt.cliVersion },
				notify: func(message string) error {
					notices = append(notices, message)
					return nil
				},
			}

			gotClient, err := bootstrap.ensure(context.Background())
			if len(tt.wantErr) > 0 {
				if err == nil {
					t.Fatal("ensure() error = nil, want version mismatch")
				}
				if !containsAll(err.Error(), tt.wantErr...) {
					t.Fatalf("ensure() error = %q, want fragments %#v", err.Error(), tt.wantErr)
				}
				if initialClient.stopCtx != nil {
					t.Fatal("expected busy version mismatch not to stop daemon")
				}
				if launchCalls != 0 {
					t.Fatalf("expected busy version mismatch not to launch, got %d", launchCalls)
				}
				return
			}

			if err != nil {
				t.Fatalf("ensure() error = %v", err)
			}
			switch tt.wantClient {
			case "initial":
				if gotClient != initialClient {
					t.Fatalf("ensure() client = %#v, want initial client", gotClient)
				}
			case "restarted":
				if gotClient != restartedClient {
					t.Fatalf("ensure() client = %#v, want restarted client", gotClient)
				}
			default:
				t.Fatalf("test has unsupported wantClient %q", tt.wantClient)
			}
			if launchCalls != tt.wantLaunchCalls {
				t.Fatalf("launch calls = %d, want %d", launchCalls, tt.wantLaunchCalls)
			}
			stopCalls := 0
			if initialClient.stopCtx != nil {
				stopCalls = 1
			}
			if stopCalls != tt.wantStopCalls {
				t.Fatalf("stop calls = %d, want %d", stopCalls, tt.wantStopCalls)
			}
			if initialClient.stopForce != tt.wantStopForce {
				t.Fatalf("stop force = %t, want %t", initialClient.stopForce, tt.wantStopForce)
			}
			if len(tt.wantNotice) == 0 {
				if len(notices) != 0 {
					t.Fatalf("notices = %#v, want none", notices)
				}
				return
			}
			if len(notices) != 1 || !containsAll(notices[0], tt.wantNotice...) {
				t.Fatalf("notices = %#v, want one notice with %#v", notices, tt.wantNotice)
			}
		})
	}
}

func TestDaemonBuildVersionsCompatible(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		daemon string
		cli    string
		want   bool
	}{
		{
			name:   "Should match exact build strings",
			daemon: "v2.0.0 (commit=abc123 date=2026-06-11T17:15:44Z)",
			cli:    "v2.0.0 (commit=abc123 date=2026-06-11T17:15:44Z)",
			want:   true,
		},
		{
			name:   "Should ignore build date when release version and commit match",
			daemon: "v2.0.0 (commit=abc123 date=2026-06-11T17:15:44Z)",
			cli:    "v2.0.0 (commit=abc123 date=2026-06-11T21:45:40Z)",
			want:   true,
		},
		{
			name:   "Should reject same version with different commits",
			daemon: "v2.0.0 (commit=abc123 date=2026-06-11T17:15:44Z)",
			cli:    "v2.0.0 (commit=def456 date=2026-06-11T21:45:40Z)",
			want:   false,
		},
		{
			name:   "Should reject different versions with same commit",
			daemon: "v2.0.0 (commit=abc123 date=2026-06-11T17:15:44Z)",
			cli:    "v2.1.0 (commit=abc123 date=2026-06-11T21:45:40Z)",
			want:   false,
		},
		{
			name:   "Should preserve dev compatibility",
			daemon: "dev (commit=none date=unknown)",
			cli:    "v2.0.0 (commit=abc123 date=2026-06-11T21:45:40Z)",
			want:   true,
		},
		{
			name:   "Should reject malformed release strings unless exact",
			daemon: "v2.0.0 commit=abc123 date=2026-06-11T17:15:44Z",
			cli:    "v2.0.0 commit=abc123 date=2026-06-11T21:45:40Z",
			want:   false,
		},
		{
			name:   "Should keep exact-match fallback for release strings missing date metadata",
			daemon: "v2.0.0 (commit=abc123)",
			cli:    "v2.0.0 (commit=abc123)",
			want:   true,
		},
		{
			name:   "Should not treat different missing-date release strings as compatible",
			daemon: "v2.0.0 (commit=abc123)",
			cli:    "v2.0.0 (commit=abc123 extra=true)",
			want:   false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := daemonBuildVersionsCompatible(tt.daemon, tt.cli); got != tt.want {
				t.Fatalf("daemonBuildVersionsCompatible(%q, %q) = %t, want %t", tt.daemon, tt.cli, got, tt.want)
			}
		})
	}
}

func TestDaemonContractVersionsCompatible(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		daemon string
		cli    string
		want   bool
	}{
		{
			name:   "Should accept legacy daemon without contract version",
			daemon: "",
			cli:    contract.DaemonContractVersion,
			want:   true,
		},
		{
			name:   "Should accept matching contract versions",
			daemon: contract.DaemonContractVersion,
			cli:    contract.DaemonContractVersion,
			want:   true,
		},
		{
			name:   "Should reject different contract versions",
			daemon: "99",
			cli:    contract.DaemonContractVersion,
			want:   false,
		},
		{
			name:   "Should reject declared daemon contract when CLI contract is unknown",
			daemon: contract.DaemonContractVersion,
			cli:    "",
			want:   false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := daemonContractVersionsCompatible(tt.daemon, tt.cli); got != tt.want {
				t.Fatalf("daemonContractVersionsCompatible(%q, %q) = %t, want %t", tt.daemon, tt.cli, got, tt.want)
			}
		})
	}
}

func TestCLIDaemonBootstrapWaitForDaemonInfoRelease(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 11, 12, 30, 0, 0, time.UTC)
	previous := daemon.Info{
		PID:        1234,
		SocketPath: "/tmp/compozy-version-old.sock",
		StartedAt:  now,
		State:      daemon.ReadyStateReady,
	}
	permissionErr := errors.New("permission denied")

	tests := []struct {
		name        string
		readErrors  []error
		wantCalls   int
		wantErrText []string
	}{
		{
			name:       "Should proceed when daemon info is missing",
			readErrors: []error{fmt.Errorf("daemon info removed: %w", os.ErrNotExist)},
			wantCalls:  1,
		},
		{
			name: "Should proceed after transient read error when daemon info becomes missing",
			readErrors: []error{
				permissionErr,
				fmt.Errorf("daemon info removed: %w", os.ErrNotExist),
			},
			wantCalls: 2,
		},
		{
			name:       "Should time out when daemon info read keeps failing without missing sentinel",
			readErrors: []error{permissionErr},
			wantCalls:  4,
			wantErrText: []string{
				"wait for stale daemon shutdown",
				"read daemon info while waiting for stale daemon release",
				"permission denied",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			currentTime := now
			readInfoCalls := 0
			bootstrap := cliDaemonBootstrap{
				readInfo: func(path string) (daemon.Info, error) {
					if path != "/tmp/compozy-home/daemon.json" {
						t.Fatalf("unexpected daemon info path: %q", path)
					}
					readInfoCalls++
					errIndex := readInfoCalls - 1
					if errIndex >= len(tt.readErrors) {
						errIndex = len(tt.readErrors) - 1
					}
					if err := tt.readErrors[errIndex]; err != nil {
						return daemon.Info{}, err
					}
					return previous, nil
				},
				sleep: func(duration time.Duration) {
					currentTime = currentTime.Add(duration)
				},
				now:            func() time.Time { return currentTime },
				startupTimeout: 3 * time.Millisecond,
				pollInterval:   time.Millisecond,
			}

			err := bootstrap.waitForDaemonInfoRelease(
				context.Background(),
				" /tmp/compozy-home/daemon.json ",
				previous,
			)
			if len(tt.wantErrText) == 0 {
				if err != nil {
					t.Fatalf("waitForDaemonInfoRelease() error = %v, want nil", err)
				}
			} else {
				if err == nil {
					t.Fatal("waitForDaemonInfoRelease() error = nil, want timeout")
				}
				if !containsAll(err.Error(), tt.wantErrText...) {
					t.Fatalf(
						"waitForDaemonInfoRelease() error = %q, want fragments %#v",
						err.Error(),
						tt.wantErrText,
					)
				}
			}
			if readInfoCalls != tt.wantCalls {
				t.Fatalf("readInfo calls = %d, want %d", readInfoCalls, tt.wantCalls)
			}
		})
	}
}

func TestCLIDaemonBootstrapEnsureReusesHealthyDaemon(t *testing.T) {
	t.Parallel()

	readyClient := &stubDaemonCommandClient{
		target: apiclient.Target{SocketPath: "/tmp/compozy-ready.sock"},
		health: apicore.DaemonHealth{Ready: true},
	}
	now := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	var launchCalls int
	var newClientTargets []apiclient.Target

	bootstrap := cliDaemonBootstrap{
		resolveHomePaths: func() (compozyconfig.HomePaths, error) {
			return compozyconfig.HomePaths{InfoPath: "/tmp/compozy-home/daemon.json"}, nil
		},
		readInfo: func(path string) (daemon.Info, error) {
			if path != "/tmp/compozy-home/daemon.json" {
				t.Fatalf("unexpected daemon info path: %q", path)
			}
			return daemon.Info{
				PID:        1234,
				SocketPath: "/tmp/compozy-ready.sock",
				StartedAt:  now,
				State:      daemon.ReadyStateReady,
			}, nil
		},
		newClient: func(target apiclient.Target) (daemonCommandClient, error) {
			newClientTargets = append(newClientTargets, target)
			return readyClient, nil
		},
		launch: func(compozyconfig.HomePaths) error {
			launchCalls++
			return nil
		},
		sleep:          func(time.Duration) {},
		now:            func() time.Time { return now },
		startupTimeout: time.Second,
		pollInterval:   time.Millisecond,
	}

	gotClient, err := bootstrap.ensure(context.Background())
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if gotClient != readyClient {
		t.Fatalf("ensure returned unexpected client: %#v", gotClient)
	}
	if launchCalls != 0 {
		t.Fatalf("expected healthy daemon reuse without launch, got %d launch attempts", launchCalls)
	}
	if readyClient.healthCalls != 1 {
		t.Fatalf("expected one health probe, got %d", readyClient.healthCalls)
	}
	if len(newClientTargets) != 1 || newClientTargets[0].SocketPath != "/tmp/compozy-ready.sock" {
		t.Fatalf("unexpected bootstrap client target sequence: %#v", newClientTargets)
	}
}

func TestCLIDaemonBootstrapProbeReportsNotReadyHealth(t *testing.T) {
	t.Parallel()

	bootstrap := cliDaemonBootstrap{
		readInfo: func(string) (daemon.Info, error) {
			return daemon.Info{
				PID:        1234,
				SocketPath: "/tmp/compozy-health.sock",
				StartedAt:  time.Date(2026, 4, 17, 12, 30, 0, 0, time.UTC),
				State:      daemon.ReadyStateReady,
			}, nil
		},
		newClient: func(target apiclient.Target) (daemonCommandClient, error) {
			return &stubDaemonCommandClient{
				target: target,
				health: apicore.DaemonHealth{
					Ready: false,
					Details: []apicore.HealthDetail{{
						Code:    "db_unavailable",
						Message: "global.db is not ready",
					}},
				},
			}, nil
		},
	}

	_, err := bootstrap.probe(context.Background(), "/tmp/compozy-home/daemon.json")
	if err == nil {
		t.Fatal("expected probe failure for not-ready daemon health")
	}
	if got := err.Error(); !containsAll(
		got,
		"probe daemon health via unix:///tmp/compozy-health.sock",
		"global.db is not ready",
	) {
		t.Fatalf("unexpected probe error: %v", err)
	}
}

func TestCLIDaemonBootstrapProbeWrapsReadInfoAndClientErrors(t *testing.T) {
	t.Parallel()

	readInfoErrBootstrap := cliDaemonBootstrap{
		readInfo: func(string) (daemon.Info, error) {
			return daemon.Info{}, errors.New("daemon info missing")
		},
	}
	_, err := readInfoErrBootstrap.probe(context.Background(), "/tmp/compozy-home/daemon.json")
	if err == nil || !containsAll(err.Error(), "read daemon info", "daemon info missing") {
		t.Fatalf("expected wrapped readInfo error, got %v", err)
	}

	newClientErrBootstrap := cliDaemonBootstrap{
		readInfo: func(string) (daemon.Info, error) {
			return daemon.Info{
				PID:        1234,
				SocketPath: "/tmp/compozy-health.sock",
				StartedAt:  time.Date(2026, 4, 17, 12, 45, 0, 0, time.UTC),
				State:      daemon.ReadyStateReady,
			}, nil
		},
		newClient: func(apiclient.Target) (daemonCommandClient, error) {
			return nil, errors.New("target rejected")
		},
	}
	_, err = newClientErrBootstrap.probe(context.Background(), "/tmp/compozy-home/daemon.json")
	if err == nil || !containsAll(err.Error(), "build daemon client", "target rejected") {
		t.Fatalf("expected wrapped newClient error, got %v", err)
	}
}

func TestCLIDaemonBootstrapEnsureRepairsStaleTransportAfterLaunch(t *testing.T) {
	t.Parallel()

	nowSequence := []time.Time{
		time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 0, 0, 500000000, time.UTC),
	}
	nowIndex := 0
	nextNow := func() time.Time {
		if nowIndex >= len(nowSequence) {
			return nowSequence[len(nowSequence)-1]
		}
		value := nowSequence[nowIndex]
		nowIndex++
		return value
	}

	staleClient := &stubDaemonCommandClient{
		target:    apiclient.Target{SocketPath: "/tmp/compozy-stale.sock"},
		healthErr: errors.New("dial unix /tmp/compozy-stale.sock: connect: no such file or directory"),
	}
	readyClient := &stubDaemonCommandClient{
		target: apiclient.Target{SocketPath: "/tmp/compozy-stale.sock"},
		health: apicore.DaemonHealth{Ready: true},
	}

	var launchCalls int
	var clientCalls int

	bootstrap := cliDaemonBootstrap{
		resolveHomePaths: func() (compozyconfig.HomePaths, error) {
			return compozyconfig.HomePaths{InfoPath: "/tmp/compozy-home/daemon.json"}, nil
		},
		readInfo: func(string) (daemon.Info, error) {
			return daemon.Info{
				PID:        1234,
				SocketPath: "/tmp/compozy-stale.sock",
				StartedAt:  nowSequence[0],
				State:      daemon.ReadyStateReady,
			}, nil
		},
		newClient: func(apiclient.Target) (daemonCommandClient, error) {
			clientCalls++
			if clientCalls == 1 {
				return staleClient, nil
			}
			return readyClient, nil
		},
		launch: func(compozyconfig.HomePaths) error {
			launchCalls++
			return nil
		},
		sleep:          func(time.Duration) {},
		now:            nextNow,
		startupTimeout: 2 * time.Second,
		pollInterval:   time.Millisecond,
	}

	gotClient, err := bootstrap.ensure(context.Background())
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if gotClient != readyClient {
		t.Fatalf("ensure returned unexpected repaired client: %#v", gotClient)
	}
	if launchCalls != 1 {
		t.Fatalf("expected one daemon launch to repair stale transport, got %d", launchCalls)
	}
	if staleClient.healthCalls != 1 {
		t.Fatalf("expected one stale health probe, got %d", staleClient.healthCalls)
	}
	if readyClient.healthCalls != 1 {
		t.Fatalf("expected one repaired health probe, got %d", readyClient.healthCalls)
	}
}

func TestDaemonStatusRunUsesCommandContextForProbeAndRPCs(t *testing.T) {
	acquireCLITestGlobalOverride(t)

	ctxKey := daemonCommandContextKey("status")
	cmdCtx := context.WithValue(context.Background(), ctxKey, "status-command")
	startedAt := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	client := &stubDaemonCommandClient{
		status: apicore.DaemonStatus{
			PID:        1234,
			Version:    "test-version",
			StartedAt:  startedAt,
			SocketPath: "/tmp/compozy.sock",
		},
		health: apicore.DaemonHealth{Ready: true},
	}

	originalQueryStatus := queryDaemonCommandStatus
	originalNewClient := newDaemonCommandClientFromInfo
	var probeCtx context.Context
	queryDaemonCommandStatus = func(
		ctx context.Context,
		_ compozyconfig.HomePaths,
		_ daemon.ProbeOptions,
	) (daemon.Status, error) {
		probeCtx = ctx
		return daemon.Status{
			State: daemon.ReadyStateReady,
			Info: &daemon.Info{
				PID:        1234,
				SocketPath: "/tmp/compozy.sock",
				StartedAt:  startedAt,
				State:      daemon.ReadyStateReady,
			},
		}, nil
	}
	newDaemonCommandClientFromInfo = func(daemon.Info) (daemonCommandClient, error) {
		return client, nil
	}
	t.Cleanup(func() {
		queryDaemonCommandStatus = originalQueryStatus
		newDaemonCommandClientFromInfo = originalNewClient
	})

	cmd := &cobra.Command{}
	cmd.SetContext(cmdCtx)
	cmd.SetOut(io.Discard)
	state := daemonStatusState{outputFormat: operatorOutputFormatJSON}

	if err := state.run(cmd, nil); err != nil {
		t.Fatalf("daemonStatusState.run() error = %v", err)
	}
	if probeCtx == nil || probeCtx.Value(ctxKey) != "status-command" {
		t.Fatalf("probe context = %#v, want command context value", probeCtx)
	}
	if client.statusCtx == nil || client.statusCtx.Value(ctxKey) != "status-command" {
		t.Fatalf("daemon status context = %#v, want command context value", client.statusCtx)
	}
	if client.healthCtx == nil || client.healthCtx.Value(ctxKey) != "status-command" {
		t.Fatalf("health context = %#v, want command context value", client.healthCtx)
	}
}

func TestDaemonStopRunUsesCommandContextForProbeAndRPCs(t *testing.T) {
	acquireCLITestGlobalOverride(t)

	ctxKey := daemonCommandContextKey("stop")
	cmdCtx := context.WithValue(context.Background(), ctxKey, "stop-command")
	startedAt := time.Date(2026, 4, 18, 12, 5, 0, 0, time.UTC)
	client := &stubDaemonCommandClient{}

	originalQueryStatus := queryDaemonCommandStatus
	originalNewClient := newDaemonCommandClientFromInfo
	var probeCtx context.Context
	queryDaemonCommandStatus = func(
		ctx context.Context,
		_ compozyconfig.HomePaths,
		_ daemon.ProbeOptions,
	) (daemon.Status, error) {
		probeCtx = ctx
		return daemon.Status{
			State: daemon.ReadyStateReady,
			Info: &daemon.Info{
				PID:        1234,
				SocketPath: "/tmp/compozy.sock",
				StartedAt:  startedAt,
				State:      daemon.ReadyStateReady,
			},
		}, nil
	}
	newDaemonCommandClientFromInfo = func(daemon.Info) (daemonCommandClient, error) {
		return client, nil
	}
	t.Cleanup(func() {
		queryDaemonCommandStatus = originalQueryStatus
		newDaemonCommandClientFromInfo = originalNewClient
	})

	cmd := &cobra.Command{}
	cmd.SetContext(cmdCtx)
	cmd.SetOut(io.Discard)
	state := daemonStopState{
		outputFormat: operatorOutputFormatJSON,
		force:        true,
	}

	if err := state.run(cmd, nil); err != nil {
		t.Fatalf("daemonStopState.run() error = %v", err)
	}
	if probeCtx == nil || probeCtx.Value(ctxKey) != "stop-command" {
		t.Fatalf("probe context = %#v, want command context value", probeCtx)
	}
	if client.stopCtx == nil || client.stopCtx.Value(ctxKey) != "stop-command" {
		t.Fatalf("stop context = %#v, want command context value", client.stopCtx)
	}
	if !client.stopForce {
		t.Fatal("expected stop command to propagate force flag")
	}
}

func TestDaemonStatusAndStopWrapSetupErrors(t *testing.T) {
	acquireCLITestGlobalOverride(t)

	assertExitCode := func(t *testing.T, err error, want int) {
		t.Helper()

		var exitErr *commandExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("expected commandExitError, got %T", err)
		}
		if exitErr.ExitCode() != want {
			t.Fatalf("unexpected exit code: got %d want %d", exitErr.ExitCode(), want)
		}
	}

	readyInfo := daemon.Info{
		PID:        1234,
		SocketPath: "/tmp/compozy.sock",
		StartedAt:  time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC),
		State:      daemon.ReadyStateReady,
	}

	tests := []struct {
		name          string
		run           func(*cobra.Command) error
		configure     func()
		wantErrSubstr []string
	}{
		{
			name: "status query error includes probe context",
			run: func(cmd *cobra.Command) error {
				state := daemonStatusState{outputFormat: operatorOutputFormatText}
				return state.run(cmd, nil)
			},
			configure: func() {
				queryDaemonCommandStatus = func(
					context.Context,
					compozyconfig.HomePaths,
					daemon.ProbeOptions,
				) (daemon.Status, error) {
					return daemon.Status{}, errors.New("status backend down")
				}
			},
			wantErrSubstr: []string{"query daemon status", "status backend down"},
		},
		{
			name: "status client error includes build context",
			run: func(cmd *cobra.Command) error {
				state := daemonStatusState{outputFormat: operatorOutputFormatText}
				return state.run(cmd, nil)
			},
			configure: func() {
				queryDaemonCommandStatus = func(
					context.Context,
					compozyconfig.HomePaths,
					daemon.ProbeOptions,
				) (daemon.Status, error) {
					return daemon.Status{State: daemon.ReadyStateReady, Info: &readyInfo}, nil
				}
				newDaemonCommandClientFromInfo = func(daemon.Info) (daemonCommandClient, error) {
					return nil, errors.New("target rejected")
				}
			},
			wantErrSubstr: []string{"build daemon status client", "target rejected"},
		},
		{
			name: "stop query error includes stop context",
			run: func(cmd *cobra.Command) error {
				state := daemonStopState{outputFormat: operatorOutputFormatText}
				return state.run(cmd, nil)
			},
			configure: func() {
				queryDaemonCommandStatus = func(
					context.Context,
					compozyconfig.HomePaths,
					daemon.ProbeOptions,
				) (daemon.Status, error) {
					return daemon.Status{}, errors.New("status backend down")
				}
			},
			wantErrSubstr: []string{"query daemon status before stop", "status backend down"},
		},
		{
			name: "stop client error includes build context",
			run: func(cmd *cobra.Command) error {
				state := daemonStopState{outputFormat: operatorOutputFormatText}
				return state.run(cmd, nil)
			},
			configure: func() {
				queryDaemonCommandStatus = func(
					context.Context,
					compozyconfig.HomePaths,
					daemon.ProbeOptions,
				) (daemon.Status, error) {
					return daemon.Status{State: daemon.ReadyStateReady, Info: &readyInfo}, nil
				}
				newDaemonCommandClientFromInfo = func(daemon.Info) (daemonCommandClient, error) {
					return nil, errors.New("target rejected")
				}
			},
			wantErrSubstr: []string{"build daemon stop client", "target rejected"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalQueryStatus := queryDaemonCommandStatus
			originalNewClient := newDaemonCommandClientFromInfo
			t.Cleanup(func() {
				queryDaemonCommandStatus = originalQueryStatus
				newDaemonCommandClientFromInfo = originalNewClient
			})

			queryDaemonCommandStatus = originalQueryStatus
			newDaemonCommandClientFromInfo = originalNewClient
			tt.configure()

			cmd := &cobra.Command{}
			cmd.SetContext(context.Background())
			cmd.SetOut(io.Discard)

			err := tt.run(cmd)
			if err == nil {
				t.Fatal("expected setup error")
			}
			assertExitCode(t, err, 2)
			if !containsAll(err.Error(), tt.wantErrSubstr...) {
				t.Fatalf("unexpected wrapped error: %v", err)
			}
		})
	}
}

func TestWriteDaemonOutputsUseStableJSONSchema(t *testing.T) {
	t.Parallel()

	t.Run("status omits daemon when nil", func(t *testing.T) {
		t.Parallel()

		cmd := &cobra.Command{}
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)

		if err := writeDaemonStatusOutput(
			cmd,
			operatorOutputFormatJSON,
			nil,
			apicore.DaemonHealth{Ready: false},
			string(daemon.ReadyStateStopped),
		); err != nil {
			t.Fatalf("writeDaemonStatusOutput(nil daemon) error = %v", err)
		}

		var payload map[string]json.RawMessage
		if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
			t.Fatalf("decode daemon status json: %v", err)
		}
		if _, ok := payload["daemon"]; ok {
			t.Fatalf("daemon field should be omitted when status is nil: %s", stdout.String())
		}
		if _, ok := payload["state"]; !ok {
			t.Fatalf("status json missing state: %s", stdout.String())
		}
		if _, ok := payload["health"]; !ok {
			t.Fatalf("status json missing health: %s", stdout.String())
		}
	})

	t.Run("status includes daemon payload when present", func(t *testing.T) {
		t.Parallel()

		cmd := &cobra.Command{}
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)

		status := &apicore.DaemonStatus{
			PID:        1234,
			Version:    "test-version",
			StartedAt:  time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC),
			SocketPath: "/tmp/compozy.sock",
		}
		health := apicore.DaemonHealth{Ready: true}
		if err := writeDaemonStatusOutput(
			cmd,
			operatorOutputFormatJSON,
			status,
			health,
			string(daemon.ReadyStateReady),
		); err != nil {
			t.Fatalf("writeDaemonStatusOutput(status daemon) error = %v", err)
		}

		var payload daemonStatusOutput
		if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
			t.Fatalf("decode daemon status output: %v", err)
		}
		if payload.State != string(daemon.ReadyStateReady) || !payload.Health.Ready {
			t.Fatalf("unexpected daemon status payload: %#v", payload)
		}
		if payload.Daemon == nil || payload.Daemon.PID != 1234 || payload.Daemon.Version != "test-version" {
			t.Fatalf("unexpected daemon payload: %#v", payload)
		}
	})

	t.Run("stop emits accepted force and state fields", func(t *testing.T) {
		t.Parallel()

		cmd := &cobra.Command{}
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)

		if err := writeDaemonStopOutput(
			cmd,
			operatorOutputFormatJSON,
			true,
			true,
			string(daemon.ReadyStateReady),
		); err != nil {
			t.Fatalf("writeDaemonStopOutput() error = %v", err)
		}

		var payload daemonStopOutput
		if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
			t.Fatalf("decode daemon stop output: %v", err)
		}
		if !payload.Accepted || !payload.Force || payload.State != string(daemon.ReadyStateReady) {
			t.Fatalf("unexpected daemon stop payload: %#v", payload)
		}
	})
}

func TestResolveTaskWorkflowName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		args          []string
		flagName      string
		wantName      string
		wantErrSubstr string
	}{
		{
			name:     "positional slug wins when flag is empty",
			args:     []string{"demo"},
			wantName: "demo",
		},
		{
			name:     "name flag works without positional slug",
			flagName: "demo",
			wantName: "demo",
		},
		{
			name:          "positional mismatch is rejected",
			args:          []string{"demo"},
			flagName:      "other",
			wantErrSubstr: "workflow slug mismatch",
		},
		{
			name:          "missing slug is rejected",
			wantErrSubstr: "workflow slug is required",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			state := newCommandState(commandKindTasksRun, "")
			state.name = tt.flagName

			err := state.resolveTaskWorkflowName(tt.args)
			if tt.wantErrSubstr != "" {
				if err == nil {
					t.Fatal("expected resolveTaskWorkflowName error")
				}
				if !containsAll(err.Error(), tt.wantErrSubstr) {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("resolveTaskWorkflowName: %v", err)
			}
			if state.name != tt.wantName {
				t.Fatalf("unexpected resolved workflow name: got %q want %q", state.name, tt.wantName)
			}
		})
	}
}

func TestResolveTaskPresentationModeRejectsConflictsAndInvalidModes(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindTasksRun, "")
	state.isInteractive = func() bool { return true }
	cmd := newTaskRunPresentationCommand(state)
	if err := cmd.Flags().Set("attach", attachModeStream); err != nil {
		t.Fatalf("set attach: %v", err)
	}
	state.attachMode = attachModeStream
	if err := cmd.Flags().Set("ui", "true"); err != nil {
		t.Fatalf("set ui: %v", err)
	}
	if _, err := state.resolveTaskPresentationMode(cmd); err == nil || !containsAll(err.Error(), "choose only one") {
		t.Fatalf("expected conflicting attach mode error, got %v", err)
	}

	state = newCommandState(commandKindTasksRun, "")
	state.isInteractive = func() bool { return true }
	cmd = newTaskRunPresentationCommand(state)
	state.attachMode = "bogus"
	if _, err := state.resolveTaskPresentationMode(cmd); err == nil ||
		!containsAll(err.Error(), "attach mode must be one of auto, ui, stream, or detach") {
		t.Fatalf("expected invalid attach mode error, got %v", err)
	}
}

func TestWorkPackagePickerOptions(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	initiative := "auth"
	writeCLIWorkPackagePlan(t, workspaceRoot, initiative, true)
	packageRoot := filepath.Join(workspaceRoot, ".compozy", "tasks", initiative, "_packages")
	writeFormTaskFile(t, filepath.Join(packageRoot, "WP-001"), "task_001.md", "completed")
	writeFormTaskFile(t, filepath.Join(packageRoot, "WP-002"), "task_001.md", "completed")
	writeFormTaskFile(t, filepath.Join(packageRoot, "WP-002"), "task_002.md", "pending")

	target, err := (workpackages.TargetResolver{}).Resolve(context.Background(), workspaceRoot, initiative)
	if err != nil {
		t.Fatalf("resolve initiative: %v", err)
	}
	options, err := buildWorkPackagePickerOptions(workPackagePickerInput{
		Target:  target,
		RunMode: daemonRunModeTask,
	}, map[string]string{initiative + "/WP-002": execStatusFailed})
	if err != nil {
		t.Fatalf("build picker options: %v", err)
	}
	if len(options) != 2 {
		t.Fatalf("picker options = %#v, want two Work Packages", options)
	}
	for _, want := range []string{
		"[✓] WP-001 — Foundation — Completed — 1/1 tasks completed",
		"[ ] WP-002 — Delivery — Ready to retry — 1/2 tasks completed",
	} {
		if !slices.ContainsFunc(options, func(option workPackagePickerOption) bool {
			return option.Label == want
		}) {
			t.Fatalf("picker options = %#v, missing %q", options, want)
		}
	}

	if err := validateWorkPackagePickerSelection(options, "WP-001", false); err == nil ||
		!strings.Contains(err.Error(), "completed Work Package is locked") {
		t.Fatalf("completed selection error = %v, want locked explanation", err)
	}
	if err := validateWorkPackagePickerSelection(options, "WP-001", true); err != nil {
		t.Fatalf("include-completed selection error = %v", err)
	}
	if err := validateWorkPackagePickerSelection(options, "WP-002", false); err != nil {
		t.Fatalf("ready selection error = %v", err)
	}
}

func TestWorkPackagePickerShowsDependencyBlockedMarker(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	initiative := "auth"
	writeCLIWorkPackagePlan(t, workspaceRoot, initiative, false)
	packageRoot := filepath.Join(workspaceRoot, ".compozy", "tasks", initiative, "_packages")
	writeFormTaskFile(t, filepath.Join(packageRoot, "WP-001"), "task_001.md", "pending")
	writeFormTaskFile(t, filepath.Join(packageRoot, "WP-002"), "task_001.md", "pending")

	target, err := (workpackages.TargetResolver{}).Resolve(context.Background(), workspaceRoot, initiative)
	if err != nil {
		t.Fatalf("resolve initiative: %v", err)
	}
	options, err := buildWorkPackagePickerOptions(workPackagePickerInput{
		Target:  target,
		RunMode: daemonRunModeTask,
	}, nil)
	if err != nil {
		t.Fatalf("build picker options: %v", err)
	}
	blockedIndex := slices.IndexFunc(options, func(option workPackagePickerOption) bool {
		return option.Value == "WP-002"
	})
	if blockedIndex < 0 {
		t.Fatalf("picker options = %#v, missing blocked Work Package", options)
	}
	want := "[⊘] WP-002 — Delivery — Blocked — 0/1 tasks completed — waits for WP-001"
	if got := options[blockedIndex].Label; got != want {
		t.Fatalf("blocked Work Package label = %q, want %q", got, want)
	}
	if got := workPackagePickerSelectedLabel(options[blockedIndex].Label); !strings.HasPrefix(got, "[x] WP-002") {
		t.Fatalf("selected blocked Work Package label = %q, want [x] marker", got)
	}
}

// Invariant: review target markers reflect implementation and pending-review state,
// and only review-clean completed targets are struck through.
func TestBuildReviewFixTargetPickerOptions(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	initiative := "auth"
	writeCLIWorkPackagePlan(t, workspaceRoot, initiative, true)
	packageRoot := filepath.Join(workspaceRoot, ".compozy", "tasks", initiative, "_packages")
	writeFormTaskFile(t, filepath.Join(packageRoot, "WP-001"), "task_001.md", "completed")
	writeFormTaskFile(t, filepath.Join(packageRoot, "WP-002"), "task_001.md", "pending")
	cleanRoot := filepath.Join(workspaceRoot, ".compozy", "tasks", "clean")
	if err := os.MkdirAll(cleanRoot, 0o755); err != nil {
		t.Fatalf("create clean workflow: %v", err)
	}
	writeFormTaskFile(t, cleanRoot, "task_001.md", "completed")
	reviewDir := filepath.Join(packageRoot, "WP-001", "reviews-003")
	if err := reviews.WriteRound(reviewDir, model.RoundMeta{
		Provider:  "manual",
		Round:     3,
		CreatedAt: time.Date(2026, 7, 20, 16, 41, 35, 0, time.UTC),
	}, []provider.ReviewItem{
		{Title: "First issue", File: "first.go", Line: 10, Body: "Fix the first issue."},
		{Title: "Second issue", File: "second.go", Line: 20, Body: "Fix the second issue."},
	}); err != nil {
		t.Fatalf("write review round: %v", err)
	}
	resolvedIssuePath := filepath.Join(reviewDir, "issue_002.md")
	resolvedIssue, err := os.ReadFile(resolvedIssuePath)
	if err != nil {
		t.Fatalf("read review issue: %v", err)
	}
	if err := os.WriteFile(
		resolvedIssuePath,
		[]byte(strings.Replace(string(resolvedIssue), "status: pending", "status: resolved", 1)),
		0o600,
	); err != nil {
		t.Fatalf("resolve review issue: %v", err)
	}
	options, err := buildReviewFixTargetPickerOptions(context.Background(), workspaceRoot, map[string]string{
		initiative + "/WP-001": execStatusFailed,
		initiative + "/WP-002": taskRunWizardRunStatusRunning,
	})
	if err != nil {
		t.Fatalf("build review target picker options: %v", err)
	}
	wants := map[string]string{
		initiative + "/WP-001": "[ ] auth/WP-001 — Foundation — Review round 3 — 1 issue pending",
		initiative + "/WP-002": "[⊘] auth/WP-002 — Delivery — No review round — No issues pending",
		"clean":                "[✓] clean — No review round — No issues pending",
	}
	if len(options) != len(wants) {
		t.Fatalf("review target options = %#v, want %d", options, len(wants))
	}
	for value, label := range wants {
		if !slices.ContainsFunc(options, func(option workPackagePickerOption) bool {
			return option.Value == value && option.Label == label
		}) {
			t.Fatalf("review target options = %#v, missing %q => %q", options, value, label)
		}
	}
	pendingIndex := slices.IndexFunc(options, func(option workPackagePickerOption) bool {
		return option.Value == initiative+"/WP-001"
	})
	if pendingIndex < 0 {
		t.Fatal("pending review target is missing")
	}
	pendingLabel := workPackagePickerOptionLabel(options[pendingIndex])
	if strings.Contains(pendingLabel, "\x1b[9m") {
		t.Fatalf("pending review target label = %q, want no strikethrough", pendingLabel)
	}
	completedIndex := slices.IndexFunc(options, func(option workPackagePickerOption) bool {
		return option.Value == "clean"
	})
	if completedIndex < 0 {
		t.Fatal("clean completed review target is missing")
	}
	completedLabel := workPackagePickerOptionLabel(options[completedIndex])
	if !strings.Contains(completedLabel, "\x1b[9m") {
		t.Fatalf("clean completed review target label = %q, want strikethrough", completedLabel)
	}
	blockedIndex := slices.IndexFunc(options, func(option workPackagePickerOption) bool {
		return option.Value == initiative+"/WP-002"
	})
	if blockedIndex < 0 {
		t.Fatal("review-blocked target is missing")
	}
	if got := workPackagePickerOptionLabel(options[blockedIndex]); got != wants[initiative+"/WP-002"] {
		t.Fatalf("review-blocked target label = %q, want blocked marker", got)
	}
	if err := validateWorkPackagePickerSelection(options, initiative+"/WP-002", true); err == nil ||
		!strings.Contains(err.Error(), "review is blocked until at least one implementation task is complete") {
		t.Fatalf("review-blocked selection error = %v, want implementation guidance", err)
	}
}

func TestLoadWorkPackagePickerLatestRunStatusesUsesReviewMode(t *testing.T) {
	t.Parallel()

	client := &stubDaemonCommandClient{runs: []apicore.Run{
		{RunID: "latest", WorkflowSlug: "auth/WP-001", Mode: daemonRunModeReview, Status: execStatusFailed},
		{RunID: "older", WorkflowSlug: "auth/WP-001", Mode: daemonRunModeReview, Status: execStatusCompleted},
	}}
	got, err := loadWorkPackagePickerLatestRunStatuses(
		context.Background(),
		client,
		"/workspace",
		daemonRunModeReview,
	)
	if err != nil {
		t.Fatalf("load review picker statuses: %v", err)
	}
	if got["auth/WP-001"] != execStatusFailed {
		t.Fatalf("review picker statuses = %#v, want latest failed status", got)
	}
	if len(client.runListRequests) != 1 || client.runListRequests[0].Mode != daemonRunModeReview ||
		client.runListRequests[0].Limit != apicore.MaxPageLimit {
		t.Fatalf("run list requests = %#v, want review mode", client.runListRequests)
	}
}

func TestLoadWorkPackagePickerLatestRunStatusesTreatsUnregisteredWorkspaceAsEmpty(t *testing.T) {
	t.Parallel()

	client := &stubDaemonCommandClient{runsErr: &apiclient.RemoteError{
		StatusCode: http.StatusPreconditionFailed,
		Envelope: contract.TransportError{
			Code:    "workspace_context_stale",
			Message: "active workspace context is stale",
		},
	}}
	got, err := loadWorkPackagePickerLatestRunStatuses(
		context.Background(),
		client,
		"/new-workspace",
		daemonRunModeReview,
	)
	if err != nil {
		t.Fatalf("load unregistered workspace history: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("unregistered workspace statuses = %#v, want empty history", got)
	}
}

// E2E-005, E2E-007, E2E-008, E2E-009 and E2E-013: package selection is
// explicit, cancellable, and advisory; it never mutates plan completion.
func TestResolveTaskRunTargetRequiresExplicitPackageSelection(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	initiative := "customer-management"
	planPath := writeCLIWorkPackagePlan(t, workspaceRoot, initiative, false)
	originalPlan, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("read plan: %v", err)
	}

	t.Run("non tty initiative requires an exact package reference", func(t *testing.T) {
		state := newCommandState(commandKindTasksRun, "")
		state.workspaceRoot = workspaceRoot
		state.name = initiative
		state.isInteractive = func() bool { return false }
		state.pickWorkPackage = func(*cobra.Command, workPackagePickerInput) (string, error) {
			t.Fatal("picker must not run outside a TTY")
			return "", nil
		}

		_, err := state.resolveTaskRunTarget(context.Background(), newTaskRunPresentationCommand(state))
		var packageErr *workpackages.Error
		if !errors.As(err, &packageErr) || !errors.Is(err, workpackages.ErrSelectionRequired) {
			t.Fatalf("selection error = %#v (%v), want selection-required work package error", packageErr, err)
		}
		if state.packageID != "" {
			t.Fatalf("package id = %q, want empty after rejected selection", state.packageID)
		}
	})

	t.Run("picker cancellation starts no target", func(t *testing.T) {
		state := newCommandState(commandKindTasksRun, "")
		state.workspaceRoot = workspaceRoot
		state.name = initiative
		state.isInteractive = func() bool { return true }
		state.pickWorkPackage = func(*cobra.Command, workPackagePickerInput) (string, error) {
			return "", errWorkPackageSelectionCanceled
		}

		_, err := state.resolveTaskRunTarget(context.Background(), newTaskRunPresentationCommand(state))
		if !errors.Is(err, errWorkPackageSelectionCanceled) {
			t.Fatalf("picker error = %v, want cancellation", err)
		}
		if state.name != initiative || state.packageID != "" {
			t.Fatalf("state after cancellation = name:%q package:%q", state.name, state.packageID)
		}
	})

	t.Run("interactive picker and confirmation authorize only this run", func(t *testing.T) {
		state := newCommandState(commandKindTasksRun, "")
		state.workspaceRoot = workspaceRoot
		state.name = initiative
		state.isInteractive = func() bool { return true }
		state.pickWorkPackage = func(_ *cobra.Command, input workPackagePickerInput) (string, error) {
			target := input.Target
			if target.Mode != workpackages.TargetModeInitiative {
				t.Fatalf("picker target mode = %q, want initiative", target.Mode)
			}
			if input.RunMode != daemonRunModeTask {
				t.Fatalf("picker run mode = %q, want %q", input.RunMode, daemonRunModeTask)
			}
			if !input.LockCompleted {
				t.Fatal("task picker must lock completed Work Packages")
			}
			return "WP-002", nil
		}
		confirmed := false
		state.confirmPackageRun = func(
			_ *cobra.Command,
			target workpackages.Target,
			readiness workpackages.Readiness,
		) (bool, error) {
			confirmed = target.Ref.String() == initiative+"/WP-002" && !readiness.Eligible
			return true, nil
		}

		target, err := state.resolveTaskRunTarget(context.Background(), newTaskRunPresentationCommand(state))
		if err != nil {
			t.Fatalf("resolve task target: %v", err)
		}
		if !confirmed || target.Ref.String() != initiative+"/WP-002" ||
			state.name != initiative || state.packageID != "WP-002" || !state.allowOutOfOrder {
			t.Fatalf(
				"resolved target/state = %#v name:%q package:%q allow:%t confirmed:%t",
				target,
				state.name,
				state.packageID,
				state.allowOutOfOrder,
				confirmed,
			)
		}
		currentPlan, readErr := os.ReadFile(planPath)
		if readErr != nil {
			t.Fatalf("read plan after confirmation: %v", readErr)
		}
		if !bytes.Equal(currentPlan, originalPlan) {
			t.Fatal("CLI dependency confirmation changed package completion state")
		}
	})
}

// E2E-013 and IT-036: non-interactive dependency bypass is opt-in and its
// local check does not substitute for the daemon's authoritative preflight.
func TestResolveTaskRunTargetRequiresNonInteractiveDependencyOverride(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	initiative := "customer-management"
	writeCLIWorkPackagePlan(t, workspaceRoot, initiative, false)

	state := newCommandState(commandKindTasksRun, "")
	state.workspaceRoot = workspaceRoot
	state.name = initiative + "/WP-002"
	state.isInteractive = func() bool { return false }
	_, err := state.resolveTaskRunTarget(context.Background(), newTaskRunPresentationCommand(state))
	var problem *apicore.Problem
	if !errors.As(err, &problem) || problem.Code != "work_package_dependencies_unmet" {
		t.Fatalf("non-interactive error = %#v (%v), want dependency problem", problem, err)
	}

	state = newCommandState(commandKindTasksRun, "")
	state.workspaceRoot = workspaceRoot
	state.name = initiative + "/WP-002"
	state.allowOutOfOrder = true
	state.isInteractive = func() bool { return false }
	target, err := state.resolveTaskRunTarget(context.Background(), newTaskRunPresentationCommand(state))
	if err != nil {
		t.Fatalf("resolve with --allow-out-of-order: %v", err)
	}
	if target.Ref.String() != initiative+"/WP-002" || state.name != initiative || state.packageID != "WP-002" {
		t.Fatalf("resolved target/state = %#v name:%q package:%q", target, state.name, state.packageID)
	}
}

func writeCLIWorkPackagePlan(t *testing.T, workspaceRoot string, initiative string, firstCompleted bool) string {
	t.Helper()
	plan, err := workpackages.RenderPlan(workpackages.Plan{
		SchemaVersion: workpackages.SchemaVersion,
		Initiative:    initiative,
		Packages: []workpackages.Package{
			{
				ID:         "WP-001",
				Title:      "Foundation",
				Outcome:    "Provide the prerequisite",
				Directory:  "_packages/WP-001",
				Completed:  firstCompleted,
				OwnedScope: []string{"foundation"},
			},
			{
				ID:         "WP-002",
				Title:      "Delivery",
				Outcome:    "Use the prerequisite",
				Directory:  "_packages/WP-002",
				OwnedScope: []string{"delivery"},
			},
		},
		Edges: []workpackages.Dependency{{
			From:      "WP-001",
			To:        "WP-002",
			Rationale: "Foundation must be complete first",
		}},
	})
	if err != nil {
		t.Fatalf("RenderPlan() error = %v", err)
	}
	initiativeDir := filepath.Join(workspaceRoot, ".compozy", "tasks", initiative)
	if err := os.MkdirAll(initiativeDir, 0o755); err != nil {
		t.Fatalf("mkdir initiative: %v", err)
	}
	for _, packageID := range []string{"WP-001", "WP-002"} {
		if err := os.MkdirAll(filepath.Join(initiativeDir, "_packages", packageID), 0o755); err != nil {
			t.Fatalf("mkdir package %s: %v", packageID, err)
		}
	}
	planPath := filepath.Join(initiativeDir, "_work_packages.md")
	if err := os.WriteFile(planPath, plan, 0o600); err != nil {
		t.Fatalf("write plan: %v", err)
	}
	return planPath
}

func TestWarnIfOtherWorkspaceTaskRunsActive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		configure           func(t *testing.T) (*commandState, *stubDaemonCommandClient)
		wantWarningContains []string
		wantNoWarning       bool
		wantStatusCalls     int
		wantRunListCalls    int
		wantRunStatuses     []string
	}{
		{
			name: "Should warn for active run in different workspace",
			configure: func(t *testing.T) (*commandState, *stubDaemonCommandClient) {
				t.Helper()
				currentRoot := t.TempDir()
				busyRoot := t.TempDir()
				state := newCommandState(commandKindTasksRun, "")
				state.workspaceRoot = currentRoot
				client := &stubDaemonCommandClient{
					status: apicore.DaemonStatus{ActiveRunCount: 1},
					workspaces: []apicore.Workspace{
						{ID: "ws-current", RootDir: currentRoot, Name: "current"},
						{ID: "ws-busy", RootDir: busyRoot, Name: "busy-project"},
					},
					runs: []apicore.Run{{
						RunID:       "run-busy-001",
						WorkspaceID: "ws-busy",
						Status:      "running",
					}},
				}
				return state, client
			},
			wantWarningContains: []string{
				"Warning: daemon already has active run(s) in another workspace",
				"busy-project",
				"ws-busy",
				"run-busy-001",
			},
			wantStatusCalls:  1,
			wantRunListCalls: 1,
			wantRunStatuses:  taskRunGuardActiveStatuses,
		},
		{
			name: "Should not warn for active run in same workspace",
			configure: func(t *testing.T) (*commandState, *stubDaemonCommandClient) {
				t.Helper()
				currentRoot := t.TempDir()
				state := newCommandState(commandKindTasksRun, "")
				state.workspaceRoot = currentRoot
				client := &stubDaemonCommandClient{
					status: apicore.DaemonStatus{ActiveRunCount: 1},
					workspaces: []apicore.Workspace{{
						ID:      "ws-current",
						RootDir: currentRoot,
						Name:    "current",
					}},
					runs: []apicore.Run{{
						RunID:       "run-current-001",
						WorkspaceID: "ws-current",
						Status:      "running",
					}},
				}
				return state, client
			},
			wantNoWarning:    true,
			wantStatusCalls:  1,
			wantRunListCalls: 1,
			wantRunStatuses:  taskRunGuardActiveStatuses,
		},
		{
			name: "Should ignore daemon status errors because guard is best effort",
			configure: func(t *testing.T) (*commandState, *stubDaemonCommandClient) {
				t.Helper()
				state := newCommandState(commandKindTasksRun, "")
				state.workspaceRoot = t.TempDir()
				client := &stubDaemonCommandClient{
					statusErr: errors.New("status unavailable"),
				}
				return state, client
			},
			wantNoWarning:    true,
			wantStatusCalls:  1,
			wantRunListCalls: 0,
		},
		{
			name: "Should ignore run list errors because guard is best effort",
			configure: func(t *testing.T) (*commandState, *stubDaemonCommandClient) {
				t.Helper()
				state := newCommandState(commandKindTasksRun, "")
				state.workspaceRoot = t.TempDir()
				client := &stubDaemonCommandClient{
					status:  apicore.DaemonStatus{ActiveRunCount: 1},
					runsErr: errors.New("runs unavailable"),
				}
				return state, client
			},
			wantNoWarning:    true,
			wantStatusCalls:  1,
			wantRunListCalls: 1,
			wantRunStatuses:  taskRunGuardActiveStatuses,
		},
		{
			name: "Should ignore workspace list errors because guard is best effort",
			configure: func(t *testing.T) (*commandState, *stubDaemonCommandClient) {
				t.Helper()
				state := newCommandState(commandKindTasksRun, "")
				state.workspaceRoot = t.TempDir()
				client := &stubDaemonCommandClient{
					status:  apicore.DaemonStatus{ActiveRunCount: 1},
					listErr: errors.New("workspaces unavailable"),
					runs: []apicore.Run{{
						RunID:       "run-busy-001",
						WorkspaceID: "ws-busy",
						Status:      "running",
					}},
				}
				return state, client
			},
			wantNoWarning:    true,
			wantStatusCalls:  1,
			wantRunListCalls: 1,
			wantRunStatuses:  taskRunGuardActiveStatuses,
		},
		{
			name: "Should not inspect runs when daemon is idle",
			configure: func(t *testing.T) (*commandState, *stubDaemonCommandClient) {
				t.Helper()
				state := newCommandState(commandKindTasksRun, "")
				state.workspaceRoot = t.TempDir()
				client := &stubDaemonCommandClient{
					status: apicore.DaemonStatus{ActiveRunCount: 0},
				}
				return state, client
			},
			wantNoWarning:    true,
			wantStatusCalls:  1,
			wantRunListCalls: 0,
		},
		{
			name: "Should skip guard for dry run",
			configure: func(t *testing.T) (*commandState, *stubDaemonCommandClient) {
				t.Helper()
				state := newCommandState(commandKindTasksRun, "")
				state.workspaceRoot = t.TempDir()
				state.dryRun = true
				client := &stubDaemonCommandClient{
					status: apicore.DaemonStatus{ActiveRunCount: 1},
					runs: []apicore.Run{{
						RunID:       "run-busy-001",
						WorkspaceID: "ws-busy",
						Status:      "running",
					}},
				}
				return state, client
			},
			wantNoWarning:    true,
			wantStatusCalls:  0,
			wantRunListCalls: 0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			state, client := tt.configure(t)
			cmd := &cobra.Command{Use: "compozy tasks run"}
			var stderr bytes.Buffer
			cmd.SetErr(&stderr)

			state.warnIfOtherWorkspaceTaskRunsActive(context.Background(), cmd, client)

			output := stderr.String()
			if tt.wantNoWarning && output != "" {
				t.Fatalf("expected no warning, got %q", output)
			}
			for _, want := range tt.wantWarningContains {
				if !strings.Contains(output, want) {
					t.Fatalf("expected warning to contain %q, got %q", want, output)
				}
			}
			if client.statusCalls != tt.wantStatusCalls {
				t.Fatalf("status calls = %d, want %d", client.statusCalls, tt.wantStatusCalls)
			}
			if len(client.runListRequests) != tt.wantRunListCalls {
				t.Fatalf("run list calls = %d, want %d", len(client.runListRequests), tt.wantRunListCalls)
			}
			if len(tt.wantRunStatuses) > 0 {
				if len(client.runListRequests) == 0 {
					t.Fatal("expected one run list request")
				}
				if !slices.Equal(client.runListRequests[0].Statuses, tt.wantRunStatuses) {
					t.Fatalf(
						"run list statuses = %#v, want %#v",
						client.runListRequests[0].Statuses,
						tt.wantRunStatuses,
					)
				}
			}
		})
	}
}

func TestWarnIfOtherWorkspaceTaskRunsActiveIgnoresWarningWriteFailure(t *testing.T) {
	t.Parallel()

	currentRoot := t.TempDir()
	busyRoot := t.TempDir()
	state := newCommandState(commandKindTasksRun, "")
	state.workspaceRoot = currentRoot
	client := &stubDaemonCommandClient{
		status: apicore.DaemonStatus{ActiveRunCount: 1},
		workspaces: []apicore.Workspace{
			{ID: "ws-current", RootDir: currentRoot, Name: "current"},
			{ID: "ws-busy", RootDir: busyRoot, Name: "busy-project"},
		},
		runs: []apicore.Run{{
			RunID:       "run-busy-001",
			WorkspaceID: "ws-busy",
			Status:      "running",
		}},
	}
	cmd := &cobra.Command{Use: "compozy tasks run"}
	cmd.SetErr(failingWriter{})

	state.warnIfOtherWorkspaceTaskRunsActive(context.Background(), cmd, client)

	if len(client.runListRequests) != 1 {
		t.Fatalf("run list calls = %d, want 1", len(client.runListRequests))
	}
}

func TestBuildTaskRunRuntimeOverridesIncludesOnlyExplicitFlags(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindTasksRun, "")
	cmd := newTaskRunPresentationCommand(state)
	addCommonFlags(cmd, state, commonFlagOptions{})
	cmd.Flags().BoolVar(&state.includeCompleted, "include-completed", false, "include completed")

	if err := cmd.Flags().Set("dry-run", "true"); err != nil {
		t.Fatalf("set dry-run: %v", err)
	}
	state.dryRun = true
	if err := cmd.Flags().Set("include-completed", "true"); err != nil {
		t.Fatalf("set include-completed: %v", err)
	}
	state.includeCompleted = true

	raw, err := state.buildTaskRunRuntimeOverrides(cmd)
	if err != nil {
		t.Fatalf("buildTaskRunRuntimeOverrides: %v", err)
	}
	overrides := decodeTaskRunOverrides(t, raw)
	if overrides.DryRun == nil || !*overrides.DryRun {
		t.Fatalf("expected explicit dry-run override, got %#v", overrides)
	}
	if overrides.IncludeCompleted == nil || !*overrides.IncludeCompleted {
		t.Fatalf("expected explicit include-completed override, got %#v", overrides)
	}
	if overrides.AutoCommit != nil || overrides.Model != nil || overrides.Timeout != nil {
		t.Fatalf("expected unset flags to remain absent, got %#v", overrides)
	}
}

func TestBuildTaskRunRuntimeOverridesIncludesRecursiveWhenExplicit(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindTasksRun, "")
	cmd := newTaskRunPresentationCommand(state)
	addCommonFlags(cmd, state, commonFlagOptions{})
	cmd.Flags().BoolVarP(&state.recursive, "recursive", "r", false, "recursive discovery")

	if err := cmd.Flags().Set("recursive", "true"); err != nil {
		t.Fatalf("set recursive: %v", err)
	}
	state.recursive = true

	raw, err := state.buildTaskRunRuntimeOverrides(cmd)
	if err != nil {
		t.Fatalf("buildTaskRunRuntimeOverrides: %v", err)
	}
	overrides := decodeTaskRunOverrides(t, raw)
	if overrides.Recursive == nil || !*overrides.Recursive {
		t.Fatalf("expected explicit recursive override, got %#v", overrides)
	}
	if overrides.IncludeCompleted != nil {
		t.Fatalf("expected unset include-completed to remain absent, got %#v", overrides)
	}
}

func TestBuildTaskRunRuntimeOverridesOmitsRecursiveWhenUnset(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindTasksRun, "")
	cmd := newTaskRunPresentationCommand(state)
	addCommonFlags(cmd, state, commonFlagOptions{})
	cmd.Flags().BoolVarP(&state.recursive, "recursive", "r", false, "recursive discovery")

	if err := cmd.Flags().Set("dry-run", "true"); err != nil {
		t.Fatalf("set dry-run: %v", err)
	}
	state.dryRun = true

	raw, err := state.buildTaskRunRuntimeOverrides(cmd)
	if err != nil {
		t.Fatalf("buildTaskRunRuntimeOverrides: %v", err)
	}
	overrides := decodeTaskRunOverrides(t, raw)
	if overrides.Recursive != nil {
		t.Fatalf("expected recursive override to remain absent, got %#v", overrides)
	}
	rawJSON := string(raw)
	if containsAll(rawJSON, "\"recursive\"") {
		t.Fatalf("expected JSON to omit recursive key, got %s", rawJSON)
	}
}

func TestBuildTaskRunRuntimeOverridesIncludesAllExplicitRuntimeFlags(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindTasksRun, "")
	cmd := newTaskRunPresentationCommand(state)
	addCommonFlags(cmd, state, commonFlagOptions{})
	cmd.Flags().BoolVar(&state.includeCompleted, "include-completed", false, "include completed")
	cmd.Flags().Var(
		newTaskRuntimeFlagValue(&state.executionTaskRuntimeRules),
		"task-runtime",
		"task runtime",
	)

	mustSetFlag := func(name string, value string) {
		t.Helper()
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatalf("set %s: %v", name, err)
		}
	}

	mustSetFlag("auto-commit", "true")
	state.autoCommit = true
	mustSetFlag("ide", "claude")
	state.ide = "claude"
	mustSetFlag("model", "gpt-5.5")
	state.model = "gpt-5.5"
	mustSetFlag("add-dir", "../shared")
	state.addDirs = []string{"../shared"}
	mustSetFlag("tail-lines", "42")
	state.tailLines = 42
	mustSetFlag("reasoning-effort", "high")
	state.reasoningEffort = "high"
	mustSetFlag("access-mode", "default")
	state.accessMode = "default"
	mustSetFlag("timeout", "2m")
	state.timeout = "2m"
	mustSetFlag("max-retries", "3")
	state.maxRetries = 3
	mustSetFlag("retry-backoff-multiplier", "2.5")
	state.retryBackoffMultiplier = 2.5
	mustSetFlag("task-runtime", "id=task_01,model=codex-fast")
	state.explicitRuntime = captureExplicitRuntimeFlags(cmd)

	raw, err := state.buildTaskRunRuntimeOverrides(cmd)
	if err != nil {
		t.Fatalf("buildTaskRunRuntimeOverrides: %v", err)
	}
	overrides := decodeTaskRunOverrides(t, raw)

	if overrides.AutoCommit == nil || !*overrides.AutoCommit {
		t.Fatalf("expected auto-commit override, got %#v", overrides)
	}
	if overrides.IDE == nil || *overrides.IDE != "claude" {
		t.Fatalf("expected ide override, got %#v", overrides)
	}
	if overrides.Model == nil || *overrides.Model != "gpt-5.5" {
		t.Fatalf("expected model override, got %#v", overrides)
	}
	if overrides.AddDirs == nil || len(*overrides.AddDirs) != 1 || (*overrides.AddDirs)[0] != "../shared" {
		t.Fatalf("expected add-dir override, got %#v", overrides)
	}
	if overrides.TailLines == nil || *overrides.TailLines != 42 {
		t.Fatalf("expected tail-lines override, got %#v", overrides)
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
	if overrides.ExplicitRuntime == nil || !overrides.ExplicitRuntime.IDE ||
		!overrides.ExplicitRuntime.Model || !overrides.ExplicitRuntime.ReasoningEffort ||
		!overrides.ExplicitRuntime.AccessMode {
		t.Fatalf("expected explicit runtime markers, got %#v", overrides.ExplicitRuntime)
	}
	if overrides.TaskRuntimeRules == nil || len(*overrides.TaskRuntimeRules) != 1 {
		t.Fatalf("expected task-runtime override, got %#v", overrides)
	}
	taskRuntimeRule := (*overrides.TaskRuntimeRules)[0]
	if taskRuntimeRule.ID == nil || *taskRuntimeRule.ID != "task_01" {
		t.Fatalf("expected task-runtime id override, got %#v", taskRuntimeRule)
	}
	if taskRuntimeRule.Model == nil || *taskRuntimeRule.Model != "codex-fast" {
		t.Fatalf("expected task-runtime model override, got %#v", taskRuntimeRule)
	}
}

func TestBuildTaskRunRuntimeOverridesIncludesRecoveryFlags(t *testing.T) {
	t.Parallel()

	t.Run("Should encode recovery flag overrides", func(t *testing.T) {
		t.Parallel()

		state := newCommandState(commandKindTasksRun, "")
		cmd := newTaskRunPresentationCommand(state)
		addCommonFlags(cmd, state, commonFlagOptions{})

		mustSetFlag := func(name string, value string) {
			t.Helper()
			if err := cmd.Flags().Set(name, value); err != nil {
				t.Fatalf("set %s: %v", name, err)
			}
		}
		mustSetFlag("recovery", "true")
		mustSetFlag("recovery-ide", "codex")
		mustSetFlag("recovery-model", "gpt-5.5")
		mustSetFlag("recovery-reasoning", "high")
		mustSetFlag("recovery-max-attempts", "2")

		raw, err := state.buildTaskRunRuntimeOverrides(cmd)
		if err != nil {
			t.Fatalf("buildTaskRunRuntimeOverrides: %v", err)
		}
		overrides := decodeTaskRunOverrides(t, raw)
		if overrides.Recovery == nil {
			t.Fatalf("expected recovery override, got %#v", overrides)
		}
		if overrides.Recovery.Enabled == nil || !*overrides.Recovery.Enabled {
			t.Fatalf("expected recovery enabled override, got %#v", overrides.Recovery)
		}
		if overrides.Recovery.IDE == nil || *overrides.Recovery.IDE != "codex" {
			t.Fatalf("expected recovery ide override, got %#v", overrides.Recovery)
		}
		if overrides.Recovery.Model == nil || *overrides.Recovery.Model != "gpt-5.5" {
			t.Fatalf("expected recovery model override, got %#v", overrides.Recovery)
		}
		if overrides.Recovery.ReasoningEffort == nil || *overrides.Recovery.ReasoningEffort != "high" {
			t.Fatalf("expected recovery reasoning override, got %#v", overrides.Recovery)
		}
		if overrides.Recovery.MaxAttempts == nil || *overrides.Recovery.MaxAttempts != 2 {
			t.Fatalf("expected recovery max attempts override, got %#v", overrides.Recovery)
		}
	})
}

func TestHelpOnlyDaemonCommandRootsReturnHelp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cmd  *cobra.Command
		want string
	}{
		{
			name: "workspaces",
			cmd:  newWorkspacesCommand(),
			want: "Manage daemon workspace registrations",
		},
		{
			name: "reviews",
			cmd:  newReviewsCommand(),
			want: "Fetch, inspect, and remediate review workflows",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			output, err := executeCommandCombinedOutput(tt.cmd, nil)
			if err != nil {
				t.Fatalf("execute %s help root: %v", tt.name, err)
			}
			if !containsAll(output, tt.want) {
				t.Fatalf("unexpected %s help output:\n%s", tt.name, output)
			}
		})
	}
}

func TestMapDaemonCommandErrorUsesStableExitCodes(t *testing.T) {
	t.Parallel()

	assertExitCode := func(t *testing.T, err error, want int) {
		t.Helper()

		var exitErr *commandExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("expected commandExitError, got %T", err)
		}
		if exitErr.ExitCode() != want {
			t.Fatalf("unexpected exit code: got %d want %d", exitErr.ExitCode(), want)
		}
	}

	conflictErr := mapDaemonCommandError(&apiclient.RemoteError{
		StatusCode: 409,
		Envelope: apicore.TransportError{
			RequestID: "req-conflict",
			Code:      "workflow_conflict",
			Message:   "workflow already active",
		},
	})
	assertExitCode(t, conflictErr, 1)

	validationErr := mapDaemonCommandError(&apiclient.RemoteError{
		StatusCode: 422,
		Envelope: apicore.TransportError{
			RequestID: "req-invalid",
			Code:      "invalid_request",
			Message:   "invalid workflow request",
		},
	})
	assertExitCode(t, validationErr, 1)

	packageErr := mapDaemonCommandError(&apiclient.RemoteError{
		StatusCode: http.StatusConflict,
		Envelope: apicore.TransportError{
			RequestID: "req-package",
			Code:      "work_package_dependencies_unmet",
			Message:   "work package dependencies are not complete",
		},
	})
	assertExitCode(t, packageErr, 1)
	if !strings.Contains(packageErr.Error(), "work_package_dependencies_unmet") {
		t.Fatalf("package command error = %v, want stable package code", packageErr)
	}

	localSelectionErr := mapWorkPackageSelectionError(&workpackages.Error{
		Cause:      workpackages.ErrSelectionRequired,
		Initiative: "customer-management",
	})
	assertExitCode(t, localSelectionErr, 1)
	if !strings.Contains(localSelectionErr.Error(), "work_package_selection_required") {
		t.Fatalf("local selection error = %v, want stable package code", localSelectionErr)
	}

	transportErr := mapDaemonCommandError(fmt.Errorf("dial daemon: %w", errors.New("connection refused")))
	assertExitCode(t, transportErr, 2)

	remoteErr := mapDaemonCommandError(&apiclient.RemoteError{
		StatusCode: 503,
		Envelope: apicore.TransportError{
			RequestID: "req-unavailable",
			Code:      "daemon_unavailable",
			Message:   "daemon unavailable",
		},
	})
	assertExitCode(t, remoteErr, 2)
}

func TestWorkspacesCommandUsesDaemonBootstrapAndStableJSON(t *testing.T) {
	t.Parallel()

	workspace := apicore.Workspace{
		ID:        "ws-123",
		Name:      "demo",
		RootDir:   "/tmp/demo",
		CreatedAt: time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 4, 18, 12, 5, 0, 0, time.UTC),
	}
	client := &stubDaemonCommandClient{
		health: apicore.DaemonHealth{Ready: true},
		register: apicore.WorkspaceRegisterResult{
			Workspace: workspace,
			Created:   true,
		},
		workspace:  workspace,
		workspaces: []apicore.Workspace{workspace},
	}
	installTestCLIReadyDaemonBootstrap(t, client)

	output, err := executeCommandCombinedOutput(
		newWorkspacesCommand(),
		nil,
		"register",
		workspace.RootDir,
		"--name",
		workspace.Name,
		"--format",
		"json",
	)
	if err != nil {
		t.Fatalf("execute workspaces register: %v\noutput:\n%s", err, output)
	}
	var registerPayload struct {
		Action    string            `json:"action"`
		Created   bool              `json:"created"`
		Workspace apicore.Workspace `json:"workspace"`
	}
	if err := json.Unmarshal([]byte(output), &registerPayload); err != nil {
		t.Fatalf("decode register payload: %v\noutput:\n%s", err, output)
	}
	if registerPayload.Action != "registered" || !registerPayload.Created ||
		registerPayload.Workspace.RootDir != workspace.RootDir {
		t.Fatalf("unexpected register payload: %#v", registerPayload)
	}

	output, err = executeCommandCombinedOutput(newWorkspacesCommand(), nil, "list", "--format", "json")
	if err != nil {
		t.Fatalf("execute workspaces list: %v\noutput:\n%s", err, output)
	}
	var listPayload struct {
		Workspaces []apicore.Workspace `json:"workspaces"`
	}
	if err := json.Unmarshal([]byte(output), &listPayload); err != nil {
		t.Fatalf("decode list payload: %v\noutput:\n%s", err, output)
	}
	if len(listPayload.Workspaces) != 1 || listPayload.Workspaces[0].ID != workspace.ID {
		t.Fatalf("unexpected workspace list payload: %#v", listPayload)
	}

	output, err = executeCommandCombinedOutput(newWorkspacesCommand(), nil, "show", workspace.ID, "--format", "json")
	if err != nil {
		t.Fatalf("execute workspaces show: %v\noutput:\n%s", err, output)
	}
	var showPayload struct {
		Workspace apicore.Workspace `json:"workspace"`
	}
	if err := json.Unmarshal([]byte(output), &showPayload); err != nil {
		t.Fatalf("decode show payload: %v\noutput:\n%s", err, output)
	}
	if showPayload.Workspace.ID != workspace.ID {
		t.Fatalf("unexpected show payload: %#v", showPayload)
	}

	output, err = executeCommandCombinedOutput(
		newWorkspacesCommand(),
		nil,
		"resolve",
		workspace.RootDir,
		"--format",
		"json",
	)
	if err != nil {
		t.Fatalf("execute workspaces resolve: %v\noutput:\n%s", err, output)
	}
	var resolvePayload struct {
		Action    string            `json:"action"`
		Workspace apicore.Workspace `json:"workspace"`
	}
	if err := json.Unmarshal([]byte(output), &resolvePayload); err != nil {
		t.Fatalf("decode resolve payload: %v\noutput:\n%s", err, output)
	}
	if resolvePayload.Action != "resolved" || resolvePayload.Workspace.RootDir != workspace.RootDir {
		t.Fatalf("unexpected resolve payload: %#v", resolvePayload)
	}

	output, err = executeCommandCombinedOutput(
		newWorkspacesCommand(),
		nil,
		"unregister",
		workspace.ID,
		"--format",
		"json",
	)
	if err != nil {
		t.Fatalf("execute workspaces unregister: %v\noutput:\n%s", err, output)
	}
	var deletePayload struct {
		Action       string `json:"action"`
		WorkspaceRef string `json:"workspace_ref"`
	}
	if err := json.Unmarshal([]byte(output), &deletePayload); err != nil {
		t.Fatalf("decode unregister payload: %v\noutput:\n%s", err, output)
	}
	if deletePayload.Action != "unregistered" || deletePayload.WorkspaceRef != workspace.ID {
		t.Fatalf("unexpected unregister payload: %#v", deletePayload)
	}
	if client.deleteRef != workspace.ID {
		t.Fatalf("deleteRef = %q, want %q", client.deleteRef, workspace.ID)
	}
}

func TestSyncCommandUsesDaemonBackedRequestAndJSONOutput(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	workflowDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "demo")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}
	nestedDir := filepath.Join(workspaceRoot, "pkg", "feature")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("mkdir nested dir: %v", err)
	}
	withWorkingDir(t, nestedDir)

	client := &stubDaemonCommandClient{
		health: apicore.DaemonHealth{Ready: true},
		syncResult: apicore.SyncResult{
			WorkspaceID:       "ws-123",
			WorkflowSlug:      "demo",
			Target:            filepath.Join(workspaceRoot, ".compozy", "tasks", "demo"),
			WorkflowsScanned:  1,
			TaskItemsUpserted: 3,
		},
	}
	installTestCLIReadyDaemonBootstrap(t, client)

	output, err := executeCommandCombinedOutput(
		newSyncCommand(newLazyRootDispatcher()),
		nil,
		"--name",
		"demo",
		"--format",
		"json",
	)
	if err != nil {
		t.Fatalf("execute sync: %v\noutput:\n%s", err, output)
	}
	if mustEvalSymlinksCLITest(t, client.syncRequest.Workspace) != mustEvalSymlinksCLITest(t, workspaceRoot) ||
		client.syncRequest.WorkflowSlug != "demo" ||
		client.syncRequest.Path != "" {
		t.Fatalf("unexpected sync request: %#v", client.syncRequest)
	}

	var payload apicore.SyncResult
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("decode sync payload: %v\noutput:\n%s", err, output)
	}
	if payload.WorkflowSlug != "demo" || payload.WorkflowsScanned != 1 || payload.TaskItemsUpserted != 3 {
		t.Fatalf("unexpected sync payload: %#v", payload)
	}
}

func TestArchiveCommandWorkspaceWideSkipsConflictsDeterministically(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	for _, slug := range []string{"alpha", "beta"} {
		if err := os.MkdirAll(filepath.Join(workspaceRoot, ".compozy", "tasks", slug), 0o755); err != nil {
			t.Fatalf("mkdir workflow dir %q: %v", slug, err)
		}
	}
	withWorkingDir(t, workspaceRoot)

	client := &stubDaemonCommandClient{
		health: apicore.DaemonHealth{Ready: true},
		workflows: []apicore.WorkflowSummary{
			{Slug: "beta"},
			{Slug: "alpha"},
		},
		archiveBySlug: map[string]apicore.ArchiveResult{
			"alpha": {Archived: true},
		},
		archiveErrors: map[string]error{
			"beta": &apiclient.RemoteError{
				StatusCode: 409,
				Envelope: apicore.TransportError{
					RequestID: "req-beta",
					Code:      "workflow_conflict",
					Message:   "workflow \"beta\" is not archivable",
				},
			},
		},
	}
	installTestCLIReadyDaemonBootstrap(t, client)

	output, err := executeCommandCombinedOutput(newArchiveCommand(newLazyRootDispatcher()), nil, "--format", "json")
	if err != nil {
		t.Fatalf("execute archive: %v\noutput:\n%s", err, output)
	}
	if got, want := client.archiveCalls, []string{
		"alpha",
		"beta",
	}; len(got) != len(want) || got[0] != want[0] ||
		got[1] != want[1] {
		t.Fatalf("unexpected archive calls: got %#v want %#v", got, want)
	}

	var payload core.ArchiveResult
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("decode archive payload: %v\noutput:\n%s", err, output)
	}
	if payload.Archived != 1 || payload.Skipped != 1 || len(payload.ArchivedPaths) != 1 ||
		payload.ArchivedPaths[0] != "alpha" {
		t.Fatalf("unexpected archive payload: %#v", payload)
	}
	if len(payload.SkippedPaths) != 1 || payload.SkippedPaths[0] != "beta" {
		t.Fatalf("unexpected skipped payload: %#v", payload)
	}
	if payload.SkippedReasons["beta"] == "" {
		t.Fatalf("expected skip reason for beta, got %#v", payload.SkippedReasons)
	}
}

func TestArchiveCommandWorkspaceWideUsesFilesystemWhenDaemonCatalogIsEmpty(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	for _, slug := range []string{"alpha", "beta"} {
		if err := os.MkdirAll(filepath.Join(workspaceRoot, ".compozy", "tasks", slug), 0o755); err != nil {
			t.Fatalf("mkdir workflow dir %q: %v", slug, err)
		}
	}
	withWorkingDir(t, workspaceRoot)

	client := &stubDaemonCommandClient{
		health: apicore.DaemonHealth{Ready: true},
		archiveBySlug: map[string]apicore.ArchiveResult{
			"alpha": {Archived: true},
		},
		archiveErrors: map[string]error{
			"beta": &apiclient.RemoteError{
				StatusCode: 409,
				Envelope: apicore.TransportError{
					RequestID: "req-beta",
					Code:      "workflow_conflict",
					Message:   "workflow \"beta\" is not archivable: workflow state not synced",
				},
			},
		},
	}
	installTestCLIReadyDaemonBootstrap(t, client)

	output, err := executeCommandCombinedOutput(newArchiveCommand(newLazyRootDispatcher()), nil, "--format", "json")
	if err != nil {
		t.Fatalf("execute archive: %v\noutput:\n%s", err, output)
	}
	if got, want := client.archiveCalls, []string{"alpha", "beta"}; len(got) != len(want) ||
		got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("unexpected archive calls: got %#v want %#v", got, want)
	}

	var payload core.ArchiveResult
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("decode archive payload: %v\noutput:\n%s", err, output)
	}
	if payload.WorkflowsScanned != 2 || payload.Archived != 1 || payload.Skipped != 1 {
		t.Fatalf("unexpected archive counts: %#v", payload)
	}
	if len(payload.ArchivedPaths) != 1 || payload.ArchivedPaths[0] != "alpha" {
		t.Fatalf("unexpected archived payload: %#v", payload)
	}
	if len(payload.SkippedPaths) != 1 || payload.SkippedPaths[0] != "beta" {
		t.Fatalf("unexpected skipped payload: %#v", payload)
	}
	if payload.SkippedReasons["beta"] == "" {
		t.Fatalf("expected skip reason for beta, got %#v", payload.SkippedReasons)
	}
}

func TestStreamTaskRunMultipleToTerminalSkipsHandoffOnCanceledContext(t *testing.T) {
	t.Run("Should exit cleanly without fetching the handoff when the context is canceled", func(t *testing.T) {
		client := &stubDaemonCommandClient{
			stream:           newStaticClientRunStream(),
			multiSnapshotErr: errors.New("handoff snapshot must not be fetched after cancel"),
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // simulate Ctrl+C before the queue settles

		cmd := &cobra.Command{}
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)

		if err := streamTaskRunMultipleToTerminal(ctx, cmd, client, "run-task-multi-cancel"); err != nil {
			t.Fatalf("streamTaskRunMultipleToTerminal() = %v, want nil (clean Ctrl+C exit, handoff skipped)", err)
		}
	})
}
