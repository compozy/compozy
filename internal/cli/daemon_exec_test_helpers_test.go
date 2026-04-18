package cli

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	apiclient "github.com/compozy/compozy/internal/api/client"
	apicore "github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/daemon"
	"github.com/compozy/compozy/internal/store/globaldb"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/spf13/cobra"
)

type inProcessDaemonCommandClient struct {
	manager *daemon.RunManager
	target  apiclient.Target
}

const (
	testCLIDaemonHomeEnv = "COMPOZY_TEST_CLI_DAEMON_HOME"
	testCLIXDGHomeEnv    = "COMPOZY_TEST_CLI_XDG_CONFIG_HOME"
)

var _ daemonCommandClient = (*inProcessDaemonCommandClient)(nil)

func installInProcessCLIDaemonBootstrap(t *testing.T) {
	t.Helper()

	prepareInProcessCLIDaemonHome(t)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	paths, err := compozyconfig.ResolveHomePaths()
	if err != nil {
		t.Fatalf("ResolveHomePaths() error = %v", err)
	}
	if err := compozyconfig.EnsureHomeLayout(paths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	db, err := globaldb.Open(ctx, paths.GlobalDBPath)
	if err != nil {
		t.Fatalf("globaldb.Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	manager, err := daemon.NewRunManager(daemon.RunManagerConfig{
		GlobalDB:             db,
		LifecycleContext:     ctx,
		ShutdownDrainTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("daemon.NewRunManager() error = %v", err)
	}

	installTestCLIReadyDaemonBootstrap(t, &inProcessDaemonCommandClient{
		manager: manager,
		target:  apiclient.Target{SocketPath: "in-process://daemon"},
	})
}

func prepareInProcessCLIDaemonHome(t *testing.T) {
	t.Helper()

	homeDir := strings.TrimSpace(os.Getenv(testCLIDaemonHomeEnv))
	xdgConfigHome := strings.TrimSpace(os.Getenv(testCLIXDGHomeEnv))
	if homeDir == "" {
		homeDir = t.TempDir()
		t.Setenv(testCLIDaemonHomeEnv, homeDir)
	}
	if xdgConfigHome == "" {
		xdgConfigHome = t.TempDir()
		t.Setenv(testCLIXDGHomeEnv, xdgConfigHome)
	}
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", xdgConfigHome)
}

func executeDaemonBackedRootCommandCapturingProcessIO(
	t *testing.T,
	in io.Reader,
	args ...string,
) (string, string, error) {
	t.Helper()

	installInProcessCLIDaemonBootstrap(t)
	return executeRootCommandCapturingProcessIO(t, in, args...)
}

func executeDaemonBackedCommandCapturingProcessIO(
	t *testing.T,
	cmd *cobra.Command,
	in io.Reader,
	args ...string,
) (string, string, error) {
	t.Helper()

	installInProcessCLIDaemonBootstrap(t)
	return executeCommandCapturingProcessIO(t, cmd, in, args...)
}

func (c *inProcessDaemonCommandClient) Target() apiclient.Target {
	if c == nil {
		return apiclient.Target{}
	}
	return c.target
}

func (*inProcessDaemonCommandClient) Health(context.Context) (apicore.DaemonHealth, error) {
	return apicore.DaemonHealth{Ready: true}, nil
}

func (*inProcessDaemonCommandClient) DaemonStatus(context.Context) (apicore.DaemonStatus, error) {
	return apicore.DaemonStatus{}, nil
}

func (*inProcessDaemonCommandClient) StopDaemon(context.Context, bool) error { return nil }

func (*inProcessDaemonCommandClient) RegisterWorkspace(
	context.Context,
	string,
	string,
) (apicore.WorkspaceRegisterResult, error) {
	return apicore.WorkspaceRegisterResult{}, errors.New("RegisterWorkspace not implemented for in-process exec tests")
}

func (*inProcessDaemonCommandClient) ListWorkspaces(context.Context) ([]apicore.Workspace, error) {
	return nil, errors.New("ListWorkspaces not implemented for in-process exec tests")
}

func (*inProcessDaemonCommandClient) GetWorkspace(context.Context, string) (apicore.Workspace, error) {
	return apicore.Workspace{}, errors.New("GetWorkspace not implemented for in-process exec tests")
}

func (*inProcessDaemonCommandClient) DeleteWorkspace(context.Context, string) error {
	return errors.New("DeleteWorkspace not implemented for in-process exec tests")
}

func (*inProcessDaemonCommandClient) ResolveWorkspace(context.Context, string) (apicore.Workspace, error) {
	return apicore.Workspace{}, errors.New("ResolveWorkspace not implemented for in-process exec tests")
}

func (*inProcessDaemonCommandClient) ListTaskWorkflows(context.Context, string) ([]apicore.WorkflowSummary, error) {
	return nil, errors.New("ListTaskWorkflows not implemented for in-process exec tests")
}

func (*inProcessDaemonCommandClient) ArchiveTaskWorkflow(
	context.Context,
	string,
	string,
) (apicore.ArchiveResult, error) {
	return apicore.ArchiveResult{}, errors.New("ArchiveTaskWorkflow not implemented for in-process exec tests")
}

func (*inProcessDaemonCommandClient) SyncWorkflow(context.Context, apicore.SyncRequest) (apicore.SyncResult, error) {
	return apicore.SyncResult{}, errors.New("SyncWorkflow not implemented for in-process exec tests")
}

func (*inProcessDaemonCommandClient) FetchReview(
	context.Context,
	string,
	string,
	apicore.ReviewFetchRequest,
) (apicore.ReviewFetchResult, error) {
	return apicore.ReviewFetchResult{}, errors.New("FetchReview not implemented for in-process exec tests")
}

func (*inProcessDaemonCommandClient) GetLatestReview(context.Context, string, string) (apicore.ReviewSummary, error) {
	return apicore.ReviewSummary{}, errors.New("GetLatestReview not implemented for in-process exec tests")
}

func (*inProcessDaemonCommandClient) GetReviewRound(context.Context, string, string, int) (apicore.ReviewRound, error) {
	return apicore.ReviewRound{}, errors.New("GetReviewRound not implemented for in-process exec tests")
}

func (*inProcessDaemonCommandClient) ListReviewIssues(
	context.Context,
	string,
	string,
	int,
) ([]apicore.ReviewIssue, error) {
	return nil, errors.New("ListReviewIssues not implemented for in-process exec tests")
}

func (c *inProcessDaemonCommandClient) StartTaskRun(
	context.Context,
	string,
	apicore.TaskRunRequest,
) (apicore.Run, error) {
	return apicore.Run{}, errors.New("StartTaskRun not implemented for in-process exec tests")
}

func (c *inProcessDaemonCommandClient) StartReviewRun(
	ctx context.Context,
	workspace string,
	slug string,
	round int,
	req apicore.ReviewRunRequest,
) (apicore.Run, error) {
	return c.manager.StartReviewRun(ctx, workspace, slug, round, req)
}

func (c *inProcessDaemonCommandClient) StartExecRun(
	ctx context.Context,
	req apicore.ExecRequest,
) (apicore.Run, error) {
	return c.manager.StartExecRun(ctx, req)
}

func (c *inProcessDaemonCommandClient) GetRunSnapshot(ctx context.Context, runID string) (apicore.RunSnapshot, error) {
	return c.manager.Snapshot(ctx, runID)
}

func (c *inProcessDaemonCommandClient) ListRunEvents(
	ctx context.Context,
	runID string,
	after apicore.StreamCursor,
	limit int,
) (apicore.RunEventPage, error) {
	return c.manager.Events(ctx, runID, apicore.RunEventPageQuery{
		After: after,
		Limit: limit,
	})
}

func (c *inProcessDaemonCommandClient) OpenRunStream(
	ctx context.Context,
	runID string,
	after apicore.StreamCursor,
) (apiclient.RunStream, error) {
	stream, err := c.manager.OpenStream(ctx, runID, after)
	if err != nil {
		return nil, err
	}
	return newInProcessClientRunStream(stream), nil
}

type inProcessClientRunStream struct {
	items         chan apiclient.RunStreamItem
	errors        chan error
	done          chan struct{}
	closeUpstream func() error
	closeOnce     sync.Once
}

func newInProcessClientRunStream(stream apicore.RunStream) apiclient.RunStream {
	items := make(chan apiclient.RunStreamItem)
	errs := make(chan error, 1)
	done := make(chan struct{})

	go func() {
		defer close(items)
		defer close(errs)
		eventsCh := stream.Events()
		errorsCh := stream.Errors()
		for {
			select {
			case <-done:
				return
			case item, ok := <-eventsCh:
				if !ok {
					eventsCh = nil
					if errorsCh == nil {
						return
					}
					continue
				}
				var overflow *apiclient.RunStreamOverflow
				if item.Overflow != nil {
					overflow = &apiclient.RunStreamOverflow{
						Reason: item.Overflow.Reason,
					}
				}
				if !sendInProcessClientRunStreamItem(done, items, apiclient.RunStreamItem{
					Event:    item.Event,
					Overflow: overflow,
				}) {
					return
				}
			case err, ok := <-errorsCh:
				if !ok {
					errorsCh = nil
					if eventsCh == nil {
						return
					}
					continue
				}
				if err != nil && !sendInProcessClientRunStreamError(done, errs, err) {
					return
				}
			}
		}
	}()

	return &inProcessClientRunStream{
		items:         items,
		errors:        errs,
		done:          done,
		closeUpstream: stream.Close,
	}
}

func (s *inProcessClientRunStream) Items() <-chan apiclient.RunStreamItem {
	return s.items
}

func (s *inProcessClientRunStream) Errors() <-chan error {
	return s.errors
}

func (s *inProcessClientRunStream) Close() error {
	if s == nil {
		return nil
	}
	var err error
	s.closeOnce.Do(func() {
		close(s.done)
		if s.closeUpstream != nil {
			err = s.closeUpstream()
		}
	})
	return err
}

func sendInProcessClientRunStreamItem(
	done <-chan struct{},
	items chan<- apiclient.RunStreamItem,
	item apiclient.RunStreamItem,
) bool {
	select {
	case <-done:
		return false
	case items <- item:
		return true
	}
}

func sendInProcessClientRunStreamError(
	done <-chan struct{},
	errs chan<- error,
	err error,
) bool {
	select {
	case <-done:
		return false
	case errs <- err:
		return true
	}
}

type stubCoreRunStream struct {
	events    chan apicore.RunStreamItem
	errors    chan error
	closeFunc func() error
}

var _ apicore.RunStream = (*stubCoreRunStream)(nil)

func (s *stubCoreRunStream) Events() <-chan apicore.RunStreamItem {
	if s == nil {
		return nil
	}
	return s.events
}

func (s *stubCoreRunStream) Errors() <-chan error {
	if s == nil {
		return nil
	}
	return s.errors
}

func (s *stubCoreRunStream) Close() error {
	if s == nil || s.closeFunc == nil {
		return nil
	}
	return s.closeFunc()
}

func TestNewInProcessClientRunStreamStopsForwarderOnClose(t *testing.T) {
	t.Parallel()

	upstreamClosed := make(chan struct{})
	stream := &stubCoreRunStream{
		events: make(chan apicore.RunStreamItem, 2),
		errors: make(chan error, 1),
		closeFunc: func() error {
			close(upstreamClosed)
			return nil
		},
	}
	stream.events <- apicore.RunStreamItem{
		Event: &eventspkg.Event{
			Kind:      eventspkg.EventKindRunStarted,
			Timestamp: time.Date(2026, 4, 18, 12, 30, 0, 0, time.UTC),
		},
	}
	stream.events <- apicore.RunStreamItem{
		Event: &eventspkg.Event{
			Kind:      eventspkg.EventKindRunCompleted,
			Timestamp: time.Date(2026, 4, 18, 12, 30, 1, 0, time.UTC),
		},
	}
	close(stream.events)
	close(stream.errors)

	wrapped := newInProcessClientRunStream(stream)
	select {
	case _, ok := <-wrapped.Items():
		if !ok {
			t.Fatal("expected first forwarded event before close")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first forwarded event")
	}

	if err := wrapped.Close(); err != nil {
		t.Fatalf("wrapped.Close() error = %v", err)
	}

	select {
	case <-upstreamClosed:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for upstream close")
	}

	select {
	case _, ok := <-wrapped.Items():
		if ok {
			t.Fatal("expected items channel to close after Close")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for items channel close")
	}

	select {
	case _, ok := <-wrapped.Errors():
		if ok {
			t.Fatal("expected errors channel to close after Close")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for errors channel close")
	}
}
