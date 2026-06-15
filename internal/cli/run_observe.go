package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"time"

	apiclient "github.com/compozy/compozy/internal/api/client"
	apicore "github.com/compozy/compozy/internal/api/core"
	uipkg "github.com/compozy/compozy/internal/core/run/ui"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
	runspkg "github.com/compozy/compozy/pkg/compozy/runs"
)

var (
	attachCLIRunUI                 = newAttachCLIRunUI()
	attachStartedCLIRunUI          = newAttachStartedCLIRunUI()
	watchCLIRun                    = defaultWatchCLIRun
	openCLIRemoteUISession         = uipkg.AttachRemote
	openCLIRemoteMultiRunUISession = uipkg.AttachRemoteMultiple
	openCLIRemoteReviewWatchUI     = uipkg.AttachRemoteReviewWatch
)

var errRunSettledBeforeUIAttach = errors.New("run settled before ui attach")

const (
	defaultUIAttachSnapshotTimeout      = 300 * time.Millisecond
	defaultUIAttachSnapshotPollInterval = 10 * time.Millisecond
	defaultOwnedRunCancelTimeout        = 5 * time.Second
	daemonRunModeTaskMulti              = "task_multi"
	daemonRunModeReviewWatch            = "review_watch"
)

type cliRunObserveConfig struct {
	attachSnapshotTimeout      time.Duration
	attachSnapshotPollInterval time.Duration
	ownedRunCancelTimeout      time.Duration
}

type cliRunObserveOption func(*cliRunObserveConfig)

func defaultCLIRunObserveConfig() cliRunObserveConfig {
	return cliRunObserveConfig{
		attachSnapshotTimeout:      defaultUIAttachSnapshotTimeout,
		attachSnapshotPollInterval: defaultUIAttachSnapshotPollInterval,
		ownedRunCancelTimeout:      defaultOwnedRunCancelTimeout,
	}
}

func newCLIRunObserveConfig(options ...cliRunObserveOption) cliRunObserveConfig {
	cfg := defaultCLIRunObserveConfig()
	for _, option := range options {
		if option != nil {
			option(&cfg)
		}
	}
	return cfg
}

func withUIAttachSnapshotTimeout(timeout time.Duration) cliRunObserveOption {
	return func(cfg *cliRunObserveConfig) {
		cfg.attachSnapshotTimeout = timeout
	}
}

func withUIAttachSnapshotPollInterval(interval time.Duration) cliRunObserveOption {
	return func(cfg *cliRunObserveConfig) {
		cfg.attachSnapshotPollInterval = interval
	}
}

func withOwnedRunCancelTimeout(timeout time.Duration) cliRunObserveOption {
	return func(cfg *cliRunObserveConfig) {
		cfg.ownedRunCancelTimeout = timeout
	}
}

func newAttachCLIRunUI(options ...cliRunObserveOption) func(context.Context, daemonCommandClient, string) error {
	cfg := newCLIRunObserveConfig(options...)
	return func(ctx context.Context, client daemonCommandClient, runID string) error {
		return attachRemoteCLIRunUI(ctx, client, runID, false, cfg)
	}
}

func newAttachStartedCLIRunUI(options ...cliRunObserveOption) func(context.Context, daemonCommandClient, string) error {
	cfg := newCLIRunObserveConfig(options...)
	return func(ctx context.Context, client daemonCommandClient, runID string) error {
		return attachRemoteCLIRunUI(ctx, client, runID, true, cfg)
	}
}

func defaultCLIRunObserveOptions() []cliRunObserveOption {
	return []cliRunObserveOption{
		withUIAttachSnapshotTimeout(defaultUIAttachSnapshotTimeout),
		withUIAttachSnapshotPollInterval(defaultUIAttachSnapshotPollInterval),
		withOwnedRunCancelTimeout(defaultOwnedRunCancelTimeout),
	}
}

func defaultAttachCLIRunUI(ctx context.Context, client daemonCommandClient, runID string) error {
	return newAttachCLIRunUI(defaultCLIRunObserveOptions()...)(ctx, client, runID)
}

func defaultAttachStartedCLIRunUI(ctx context.Context, client daemonCommandClient, runID string) error {
	return newAttachStartedCLIRunUI(defaultCLIRunObserveOptions()...)(ctx, client, runID)
}

func attachRemoteCLIRunUI(
	ctx context.Context,
	client daemonCommandClient,
	runID string,
	cancelOwnedRunOnExit bool,
	cfg cliRunObserveConfig,
) error {
	trimmedRunID, err := normalizeRemoteRunRequest(client, runID)
	if err != nil {
		return err
	}

	snapshot, err := loadUIAttachSnapshot(
		ctx,
		client,
		trimmedRunID,
		cfg.attachSnapshotTimeout,
		cfg.attachSnapshotPollInterval,
	)
	if err != nil {
		return err
	}
	if isTaskMultiRunSnapshot(snapshot) {
		return attachRemoteCLIMultiRunUI(ctx, client, trimmedRunID, cancelOwnedRunOnExit, cfg)
	}
	if isReviewWatchRunSnapshot(snapshot) {
		return attachRemoteCLIReviewWatchUI(ctx, client, trimmedRunID, snapshot, cancelOwnedRunOnExit, cfg)
	}
	if runSnapshotSettledBeforeUIAttach(snapshot) {
		return errRunSettledBeforeUIAttach
	}

	session, err := openCLIRemoteUISession(ctx, uipkg.RemoteAttachOptions{
		Snapshot:     snapshot,
		OwnerSession: cancelOwnedRunOnExit,
		LoadSnapshot: func(loadCtx context.Context) (apicore.RunSnapshot, error) {
			return client.GetRunSnapshot(loadCtx, trimmedRunID)
		},
		OpenStream: func(streamCtx context.Context, after apicore.StreamCursor) (apiclient.RunStream, error) {
			return client.OpenRunStream(streamCtx, trimmedRunID, after)
		},
	})
	if err != nil {
		return err
	}
	return waitRemoteCLIRunUI(ctx, client, trimmedRunID, session, cancelOwnedRunOnExit, cfg.ownedRunCancelTimeout)
}

func normalizeRemoteRunRequest(client daemonCommandClient, runID string) (string, error) {
	trimmedRunID := strings.TrimSpace(runID)
	if client == nil {
		return "", errors.New("daemon client is required")
	}
	if trimmedRunID == "" {
		return "", errors.New("run id is required")
	}
	return trimmedRunID, nil
}

func isTaskMultiRunSnapshot(snapshot apicore.RunSnapshot) bool {
	return snapshot.Run.Mode == daemonRunModeTaskMulti
}

func isReviewWatchRunSnapshot(snapshot apicore.RunSnapshot) bool {
	return snapshot.Run.Mode == daemonRunModeReviewWatch
}

func attachRemoteCLIMultiRunUI(
	ctx context.Context,
	client daemonCommandClient,
	runID string,
	cancelOwnedRunOnExit bool,
	cfg cliRunObserveConfig,
) error {
	parentSnapshot, err := loadUIAttachTaskRunMultipleSnapshot(
		ctx,
		client,
		runID,
		cfg.attachSnapshotTimeout,
		cfg.attachSnapshotPollInterval,
	)
	if err != nil {
		return err
	}
	session, err := openCLIRemoteMultiRunUISession(ctx, uipkg.RemoteMultiRunAttachOptions{
		Snapshot:     parentSnapshot,
		OwnerSession: cancelOwnedRunOnExit,
		LoadSnapshot: func(loadCtx context.Context) (apicore.TaskRunMultipleSnapshot, error) {
			return client.GetTaskRunMultipleSnapshot(loadCtx, runID)
		},
		LoadChildSnapshot: func(loadCtx context.Context, childRunID string) (apicore.RunSnapshot, error) {
			return client.GetRunSnapshot(loadCtx, childRunID)
		},
		OpenParentStream: func(streamCtx context.Context, after apicore.StreamCursor) (apiclient.RunStream, error) {
			return client.OpenRunStream(streamCtx, runID, after)
		},
		OpenChildStream: func(
			streamCtx context.Context,
			childRunID string,
			after apicore.StreamCursor,
		) (apiclient.RunStream, error) {
			return client.OpenRunStream(streamCtx, childRunID, after)
		},
	})
	if err != nil {
		return err
	}
	return waitRemoteCLIRunUI(ctx, client, runID, session, cancelOwnedRunOnExit, cfg.ownedRunCancelTimeout)
}

func attachRemoteCLIReviewWatchUI(
	ctx context.Context,
	client daemonCommandClient,
	runID string,
	snapshot apicore.RunSnapshot,
	cancelOwnedRunOnExit bool,
	cfg cliRunObserveConfig,
) error {
	session, err := openCLIRemoteReviewWatchUI(ctx, uipkg.RemoteReviewWatchAttachOptions{
		Snapshot:     snapshot,
		OwnerSession: cancelOwnedRunOnExit,
		LoadSnapshot: func(loadCtx context.Context) (apicore.RunSnapshot, error) {
			return client.GetRunSnapshot(loadCtx, runID)
		},
		LoadChildSnapshot: func(loadCtx context.Context, childRunID string) (apicore.RunSnapshot, error) {
			return client.GetRunSnapshot(loadCtx, childRunID)
		},
		OpenParentStream: func(streamCtx context.Context, after apicore.StreamCursor) (apiclient.RunStream, error) {
			return client.OpenRunStream(streamCtx, runID, after)
		},
		OpenChildStream: func(
			streamCtx context.Context,
			childRunID string,
			after apicore.StreamCursor,
		) (apiclient.RunStream, error) {
			return client.OpenRunStream(streamCtx, childRunID, after)
		},
	})
	if err != nil {
		return err
	}
	return waitRemoteCLIRunUI(ctx, client, runID, session, cancelOwnedRunOnExit, cfg.ownedRunCancelTimeout)
}

func waitRemoteCLIRunUI(
	ctx context.Context,
	client daemonCommandClient,
	runID string,
	session uipkg.Session,
	cancelOwnedRunOnExit bool,
	cancelTimeout time.Duration,
) error {
	var cancelRequested atomic.Bool
	installRemoteUIQuitHandler(session, cancelOwnedRunOnExit, &cancelRequested)
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			if cancelOwnedRunOnExit {
				cancelRequested.Store(true)
			}
			session.Shutdown()
		case <-done:
		}
	}()

	waitErr := session.Wait()
	close(done)
	if waitErr != nil {
		return waitErr
	}
	if cancelOwnedRunOnExit && cancelRequested.Load() {
		if err := cancelOwnedDaemonRun(ctx, client, runID, cancelTimeout); err != nil {
			return err
		}
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return nil
	}
	return nil
}

func installRemoteUIQuitHandler(session uipkg.Session, cancelOwnedRunOnExit bool, cancelRequested *atomic.Bool) {
	if !cancelOwnedRunOnExit {
		return
	}
	session.SetQuitHandler(func(uipkg.QuitRequest) {
		cancelRequested.Store(true)
		session.Shutdown()
	})
}

func cancelOwnedDaemonRun(
	ctx context.Context,
	client daemonCommandClient,
	runID string,
	timeout time.Duration,
) error {
	if client == nil {
		return errors.New("daemon client is required")
	}
	detachedCtx := context.Background()
	if ctx != nil {
		detachedCtx = context.WithoutCancel(ctx)
	}
	cancelCtx, cancel := context.WithTimeout(detachedCtx, timeout)
	defer cancel()
	if err := client.CancelRun(cancelCtx, runID); err != nil {
		return fmt.Errorf("cancel daemon run after ui exit: %w", err)
	}
	return nil
}

func loadUIAttachTaskRunMultipleSnapshot(
	ctx context.Context,
	client daemonCommandClient,
	runID string,
	timeout time.Duration,
	pollInterval time.Duration,
) (apicore.TaskRunMultipleSnapshot, error) {
	snapshot, err := client.GetTaskRunMultipleSnapshot(ctx, runID)
	if err != nil {
		return apicore.TaskRunMultipleSnapshot{}, err
	}
	if len(snapshot.Items) > 0 || isTerminalObservedRunStatus(snapshot.Run.Status) ||
		timeout <= 0 || pollInterval <= 0 {
		return snapshot, nil
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return snapshot, ctx.Err()
		case <-timer.C:
			return snapshot, nil
		case <-ticker.C:
			snapshot, err = client.GetTaskRunMultipleSnapshot(ctx, runID)
			if err != nil {
				return apicore.TaskRunMultipleSnapshot{}, err
			}
			if len(snapshot.Items) > 0 || isTerminalObservedRunStatus(snapshot.Run.Status) {
				return snapshot, nil
			}
		}
	}
}

func loadUIAttachSnapshot(
	ctx context.Context,
	client daemonCommandClient,
	runID string,
	timeout time.Duration,
	pollInterval time.Duration,
) (apicore.RunSnapshot, error) {
	snapshot, err := client.GetRunSnapshot(ctx, runID)
	if err != nil {
		return apicore.RunSnapshot{}, err
	}
	if !runSnapshotNeedsWarmup(snapshot) || timeout <= 0 || pollInterval <= 0 {
		return snapshot, nil
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return snapshot, ctx.Err()
		case <-timer.C:
			return snapshot, nil
		case <-ticker.C:
			snapshot, err = client.GetRunSnapshot(ctx, runID)
			if err != nil {
				return apicore.RunSnapshot{}, err
			}
			if !runSnapshotNeedsWarmup(snapshot) {
				return snapshot, nil
			}
		}
	}
}

func runSnapshotNeedsWarmup(snapshot apicore.RunSnapshot) bool {
	if runSnapshotSettledBeforeUIAttach(snapshot) {
		return false
	}
	return len(snapshot.Jobs) == 0
}

func runSnapshotSettledBeforeUIAttach(snapshot apicore.RunSnapshot) bool {
	if isTerminalObservedRunStatus(snapshot.Run.Status) {
		return true
	}
	if len(snapshot.Jobs) == 0 {
		return false
	}
	for _, job := range snapshot.Jobs {
		if !isTerminalObservedJobStatus(job.Status) {
			return false
		}
	}
	return true
}

func defaultWatchCLIRun(ctx context.Context, dst io.Writer, client daemonCommandClient, runID string) error {
	if dst == nil {
		dst = io.Discard
	}

	eventsCh, errsCh := runspkg.WatchRemote(ctx, cliRemoteWatchClient{daemon: client}, runID)
	sawTerminalEvent := false
	for eventsCh != nil || errsCh != nil {
		select {
		case <-ctx.Done():
			return nil
		case err, ok := <-errsCh:
			if !ok {
				errsCh = nil
				continue
			}
			if err != nil {
				return err
			}
		case event, ok := <-eventsCh:
			if !ok {
				eventsCh = nil
				continue
			}
			if isTerminalDaemonEvent(event.Kind) {
				sawTerminalEvent = true
			}
			line := renderObservedRunEvent(event)
			if strings.TrimSpace(line) == "" {
				continue
			}
			if _, err := io.WriteString(dst, line); err != nil {
				return fmt.Errorf("write run watch output: %w", err)
			}
		}
	}
	if sawTerminalEvent {
		_, err := waitForTerminalDaemonRunSnapshot(ctx, client, runID)
		return err
	}
	return nil
}

func watchCLIRunUntilTerminalSuccess(
	ctx context.Context,
	dst io.Writer,
	client daemonCommandClient,
	runID string,
) error {
	if dst == nil {
		dst = io.Discard
	}

	eventsCh, errsCh := runspkg.WatchRemote(ctx, cliRemoteWatchClient{daemon: client}, runID)
	for eventsCh != nil || errsCh != nil {
		select {
		case <-ctx.Done():
			return nil
		case err, ok := <-errsCh:
			if !ok {
				errsCh = nil
				continue
			}
			if err != nil {
				return mapDaemonCommandError(err)
			}
		case event, ok := <-eventsCh:
			if !ok {
				eventsCh = nil
				continue
			}
			done, err := writeObservedEventAndCheckTerminal(ctx, dst, client, runID, event)
			if err != nil || done {
				return err
			}
		}
	}

	return currentTerminalRunSuccess(ctx, client, runID)
}

func writeObservedEventAndCheckTerminal(
	ctx context.Context,
	dst io.Writer,
	client daemonCommandClient,
	runID string,
	event eventspkg.Event,
) (bool, error) {
	line := renderObservedRunEvent(event)
	if strings.TrimSpace(line) != "" {
		if _, err := io.WriteString(dst, line); err != nil {
			return false, withExitCode(2, fmt.Errorf("write run watch output: %w", err))
		}
	}
	if !isTerminalDaemonEvent(event.Kind) {
		return false, nil
	}
	terminal, err := waitForTerminalDaemonRunSnapshot(ctx, client, runID)
	if err != nil {
		return false, mapDaemonCommandError(err)
	}
	return true, terminalRunSuccessError(terminal)
}

func currentTerminalRunSuccess(ctx context.Context, client daemonCommandClient, runID string) error {
	snapshot, err := client.GetRunSnapshot(ctx, runID)
	if err != nil {
		return mapDaemonCommandError(err)
	}
	if isTerminalDaemonRun(snapshot.Run.Status) {
		return terminalRunSuccessError(snapshot.Run)
	}
	return nil
}

func terminalRunSuccessError(run apicore.Run) error {
	if !isTerminalFailureStatus(run) {
		return nil
	}
	status := strings.TrimSpace(run.Status)
	message := strings.TrimSpace(run.ErrorText)
	if message == "" {
		message = fmt.Sprintf("run ended with status %s", status)
	} else {
		message = fmt.Sprintf("run ended with status %s: %s", status, message)
	}
	return withExitCode(1, errors.New(message))
}

func renderObservedRunEvent(event eventspkg.Event) string {
	if line, handled := renderObservedTaskMultiLifecycle(event); handled {
		return line
	}
	if line, handled := renderObservedRunLifecycle(event); handled {
		return line
	}
	if line, handled := renderObservedJobLifecycle(event); handled {
		return line
	}
	if line, handled := renderObservedSessionLifecycle(event); handled {
		return line
	}
	return ""
}

func renderObservedTaskMultiLifecycle(event eventspkg.Event) (string, bool) {
	switch event.Kind {
	case eventspkg.EventKindTaskRunMultipleStarted:
		payload, ok := decodeObservedPayload[kinds.TaskRunMultiplePayload](event)
		if !ok {
			return "task queue started\n", true
		}
		total := payload.Total
		if total <= 0 {
			total = len(payload.Slugs)
		}
		return fmt.Sprintf(
			"task queue started | mode=%s total=%d\n",
			firstNonEmpty(payload.Mode, "enqueued"),
			total,
		), true
	case eventspkg.EventKindTaskRunMultipleItemQueued:
		return renderObservedTaskMultiItem(event, "queued")
	case eventspkg.EventKindTaskRunMultipleChildStarted:
		return renderObservedTaskMultiItem(event, "started")
	case eventspkg.EventKindTaskRunMultipleChildCompleted:
		return renderObservedTaskMultiItem(event, "completed")
	case eventspkg.EventKindTaskRunMultipleChildFailed:
		return renderObservedTaskMultiItem(event, "failed")
	case eventspkg.EventKindTaskRunMultipleItemCanceled:
		return renderObservedTaskMultiItem(event, "canceled")
	case eventspkg.EventKindTaskRunMultipleQueueCompleted:
		payload, ok := decodeObservedPayload[kinds.TaskRunMultiplePayload](event)
		if !ok {
			return "task queue completed\n", true
		}
		if payload.Total > 0 {
			return fmt.Sprintf("task queue completed | total=%d\n", payload.Total), true
		}
		return "task queue completed\n", true
	case eventspkg.EventKindTaskRunMultipleQueueCanceled:
		payload, ok := decodeObservedPayload[kinds.TaskRunMultiplePayload](event)
		if !ok {
			return "task queue canceled\n", true
		}
		if message := strings.TrimSpace(payload.Error); message != "" {
			return fmt.Sprintf("task queue canceled | %s\n", message), true
		}
		return "task queue canceled\n", true
	default:
		return "", false
	}
}

func renderObservedTaskMultiItem(event eventspkg.Event, fallbackStatus string) (string, bool) {
	payload, ok := decodeObservedPayload[kinds.TaskRunMultiplePayload](event)
	if !ok {
		return fmt.Sprintf("task %s\n", fallbackStatus), true
	}
	status := firstNonEmpty(payload.Status, fallbackStatus)
	segments := []string{fmt.Sprintf("%s %s", observedTaskMultiLabel(payload), status)}
	if childRunID := strings.TrimSpace(payload.ChildRunID); childRunID != "" {
		segments = append(segments, "run="+childRunID)
	}
	if worktreePath := strings.TrimSpace(payload.WorktreePath); worktreePath != "" {
		segments = append(segments, "worktree="+worktreePath)
	}
	if message := strings.TrimSpace(payload.Error); message != "" {
		segments = append(segments, message)
	}
	return strings.Join(segments, " | ") + "\n", true
}

// writeTaskRunMultipleHandoff fetches the terminal parent snapshot and writes a
// per-child handoff summary so non-UI callers can locate every preserved
// worktree after the queue settles.
func writeTaskRunMultipleHandoff(
	ctx context.Context,
	dst io.Writer,
	client daemonCommandClient,
	runID string,
) error {
	snapshot, err := client.GetTaskRunMultipleSnapshot(ctx, runID)
	if err != nil {
		return mapDaemonCommandError(err)
	}
	return renderTaskRunMultipleHandoff(dst, snapshot)
}

func renderTaskRunMultipleHandoff(dst io.Writer, snapshot apicore.TaskRunMultipleSnapshot) error {
	if dst == nil {
		return nil
	}
	for _, line := range formatTaskRunMultipleHandoff(snapshot) {
		if _, err := io.WriteString(dst, line); err != nil {
			return withExitCode(2, fmt.Errorf("write task multi-run handoff: %w", err))
		}
	}
	return nil
}

// formatTaskRunMultipleHandoff renders the children in requested (snapshot) order.
func formatTaskRunMultipleHandoff(snapshot apicore.TaskRunMultipleSnapshot) []string {
	if len(snapshot.Items) == 0 {
		return nil
	}
	lines := make([]string, 0, len(snapshot.Items)+1)
	lines = append(lines, "task multi-run handoff:\n")
	for i := range snapshot.Items {
		lines = append(lines, formatTaskRunMultipleHandoffItem(&snapshot.Items[i]))
	}
	return lines
}

func formatTaskRunMultipleHandoffItem(item *apicore.TaskRunMultipleItem) string {
	segments := []string{
		fmt.Sprintf("  %s %s", firstNonEmpty(item.Slug, "(unknown)"), firstNonEmpty(item.Status, "unknown")),
		"run=" + handoffValueOrDash(item.RunID),
		"worktree=" + handoffValueOrDash(item.WorktreePath),
	}
	if branch := strings.TrimSpace(item.BaseBranch); branch != "" {
		segments = append(segments, "branch="+branch)
	}
	if message := strings.TrimSpace(item.ErrorText); message != "" {
		segments = append(segments, message)
	}
	return strings.Join(segments, " | ") + "\n"
}

// handoffValueOrDash renders an empty/unknown field as "-" for backward
// compatibility with snapshots that predate worktree metadata.
func handoffValueOrDash(value string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	return "-"
}

func observedTaskMultiLabel(payload kinds.TaskRunMultiplePayload) string {
	slug := strings.TrimSpace(payload.Slug)
	index := payload.Index + 1
	if index <= 0 {
		index = 1
	}
	if payload.Total > 0 {
		if slug != "" {
			return fmt.Sprintf("task[%d/%d] %s", index, payload.Total, slug)
		}
		return fmt.Sprintf("task[%d/%d]", index, payload.Total)
	}
	if slug != "" {
		return fmt.Sprintf("task[%d] %s", index, slug)
	}
	return fmt.Sprintf("task[%d]", index)
}

func isTerminalObservedRunStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case execStatusCompleted, execStatusFailed, execStatusCanceled, execStatusCrashed:
		return true
	default:
		return false
	}
}

func isTerminalObservedJobStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case execStatusCompleted, execStatusFailed, execStatusCanceled:
		return true
	default:
		return false
	}
}

func renderObservedRunLifecycle(event eventspkg.Event) (string, bool) {
	switch event.Kind {
	case eventspkg.EventKindRunStarted:
		return renderObservedRunStarted(event), true
	case eventspkg.EventKindRunCompleted:
		return renderObservedRunCompleted(event), true
	case eventspkg.EventKindRunFailed:
		return renderObservedRunFailed(event), true
	case eventspkg.EventKindRunCancelled:
		return renderObservedRunCancelled(event), true
	case eventspkg.EventKindRunCrashed:
		return renderObservedRunCrashed(event), true
	default:
		return "", false
	}
}

func renderObservedJobLifecycle(event eventspkg.Event) (string, bool) {
	switch event.Kind {
	case eventspkg.EventKindJobQueued:
		return renderObservedJobQueued(event), true
	case eventspkg.EventKindJobStarted:
		return renderObservedJobStarted(event), true
	case eventspkg.EventKindJobRetryScheduled:
		return renderObservedJobRetryScheduled(event), true
	case eventspkg.EventKindJobCompleted:
		return renderObservedJobCompleted(event), true
	case eventspkg.EventKindJobFailed:
		return renderObservedJobFailed(event), true
	case eventspkg.EventKindJobCancelled:
		return renderObservedJobCancelled(event), true
	default:
		return "", false
	}
}

func renderObservedRunStarted(event eventspkg.Event) string {
	payload, ok := decodeObservedPayload[kinds.RunStartedPayload](event)
	if !ok || payload.JobsTotal <= 0 {
		return "run started\n"
	}
	return fmt.Sprintf("run started | jobs=%d\n", payload.JobsTotal)
}

func renderObservedRunCompleted(event eventspkg.Event) string {
	payload, ok := decodeObservedPayload[kinds.RunCompletedPayload](event)
	if !ok {
		return "run completed\n"
	}
	message := strings.TrimSpace(payload.SummaryMessage)
	if message != "" {
		return fmt.Sprintf("run completed | %s\n", message)
	}
	return fmt.Sprintf(
		"run completed | succeeded=%d failed=%d canceled=%d\n",
		payload.JobsSucceeded,
		payload.JobsFailed,
		payload.JobsCancelled,
	)
}

func renderObservedRunFailed(event eventspkg.Event) string {
	payload, ok := decodeObservedPayload[kinds.RunFailedPayload](event)
	if !ok {
		return "run failed\n"
	}
	if message := strings.TrimSpace(payload.Error); message != "" {
		return fmt.Sprintf("run failed | %s\n", message)
	}
	return "run failed\n"
}

func renderObservedRunCancelled(event eventspkg.Event) string {
	payload, ok := decodeObservedPayload[kinds.RunCancelledPayload](event)
	if !ok {
		return "run canceled\n"
	}
	if reason := strings.TrimSpace(payload.Reason); reason != "" {
		return fmt.Sprintf("run canceled | %s\n", reason)
	}
	return "run canceled\n"
}

func renderObservedRunCrashed(event eventspkg.Event) string {
	payload, ok := decodeObservedPayload[kinds.RunCrashedPayload](event)
	if !ok {
		return "run crashed\n"
	}
	if message := strings.TrimSpace(payload.Error); message != "" {
		return fmt.Sprintf("run crashed | %s\n", message)
	}
	return "run crashed\n"
}

func renderObservedJobQueued(event eventspkg.Event) string {
	payload, ok := decodeObservedPayload[kinds.JobQueuedPayload](event)
	if !ok {
		return "job queued\n"
	}
	label := firstNonEmpty(payload.TaskTitle, payload.CodeFile, payload.SafeName)
	if label != "" {
		return fmt.Sprintf("%s queued | %s\n", observedJobLabel(payload.Index), label)
	}
	return fmt.Sprintf("%s queued\n", observedJobLabel(payload.Index))
}

func renderObservedJobStarted(event eventspkg.Event) string {
	payload, ok := decodeObservedPayload[kinds.JobStartedPayload](event)
	if !ok {
		return "job started\n"
	}
	return fmt.Sprintf(
		"%s started | attempt %d/%d\n",
		observedJobLabel(payload.Index),
		max(payload.Attempt, 1),
		max(payload.MaxAttempts, max(payload.Attempt, 1)),
	)
}

func renderObservedJobRetryScheduled(event eventspkg.Event) string {
	payload, ok := decodeObservedPayload[kinds.JobRetryScheduledPayload](event)
	if !ok {
		return "job retry scheduled\n"
	}
	if reason := strings.TrimSpace(payload.Reason); reason != "" {
		return fmt.Sprintf(
			"%s retry scheduled | attempt %d/%d | %s\n",
			observedJobLabel(payload.Index),
			max(payload.Attempt, 1),
			max(payload.MaxAttempts, max(payload.Attempt, 1)),
			reason,
		)
	}
	return fmt.Sprintf(
		"%s retry scheduled | attempt %d/%d\n",
		observedJobLabel(payload.Index),
		max(payload.Attempt, 1),
		max(payload.MaxAttempts, max(payload.Attempt, 1)),
	)
}

func renderObservedJobCompleted(event eventspkg.Event) string {
	payload, ok := decodeObservedPayload[kinds.JobCompletedPayload](event)
	if !ok {
		return "job completed\n"
	}
	return fmt.Sprintf("%s completed | exit=%d\n", observedJobLabel(payload.Index), payload.ExitCode)
}

func renderObservedJobFailed(event eventspkg.Event) string {
	payload, ok := decodeObservedPayload[kinds.JobFailedPayload](event)
	if !ok {
		return "job failed\n"
	}
	message := strings.TrimSpace(payload.Error)
	if message != "" {
		return fmt.Sprintf(
			"%s failed | exit=%d | %s\n",
			observedJobLabel(payload.Index),
			payload.ExitCode,
			message,
		)
	}
	return fmt.Sprintf("%s failed | exit=%d\n", observedJobLabel(payload.Index), payload.ExitCode)
}

func renderObservedJobCancelled(event eventspkg.Event) string {
	payload, ok := decodeObservedPayload[kinds.JobCancelledPayload](event)
	if !ok {
		return "job canceled\n"
	}
	if reason := strings.TrimSpace(payload.Reason); reason != "" {
		return fmt.Sprintf("%s canceled | %s\n", observedJobLabel(payload.Index), reason)
	}
	return fmt.Sprintf("%s canceled\n", observedJobLabel(payload.Index))
}

func renderObservedSessionLifecycle(event eventspkg.Event) (string, bool) {
	switch event.Kind {
	case eventspkg.EventKindSessionStarted:
		payload, ok := decodeObservedPayload[kinds.SessionStartedPayload](event)
		if !ok {
			return "session attached\n", true
		}
		if payload.Resumed {
			return fmt.Sprintf("%s session resumed\n", observedJobLabel(payload.Index)), true
		}
		return fmt.Sprintf("%s session attached\n", observedJobLabel(payload.Index)), true
	case eventspkg.EventKindSessionCompleted:
		payload, ok := decodeObservedPayload[kinds.SessionCompletedPayload](event)
		if !ok {
			return "session completed\n", true
		}
		return fmt.Sprintf("%s session completed\n", observedJobLabel(payload.Index)), true
	case eventspkg.EventKindSessionFailed:
		payload, ok := decodeObservedPayload[kinds.SessionFailedPayload](event)
		if !ok {
			return "session failed\n", true
		}
		if message := strings.TrimSpace(payload.Error); message != "" {
			return fmt.Sprintf("%s session failed | %s\n", observedJobLabel(payload.Index), message), true
		}
		return fmt.Sprintf("%s session failed\n", observedJobLabel(payload.Index)), true
	default:
		return "", false
	}
}

func observedJobLabel(index int) string {
	return fmt.Sprintf("job[%d]", index+1)
}

func decodeObservedPayload[T any](event eventspkg.Event) (T, bool) {
	var payload T
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return payload, false
	}
	return payload, true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

type cliRemoteWatchClient struct {
	daemon daemonCommandClient
}

func (c cliRemoteWatchClient) GetRunSnapshot(ctx context.Context, runID string) (runspkg.RemoteRunSnapshot, error) {
	if c.daemon == nil {
		return runspkg.RemoteRunSnapshot{}, errors.New("daemon client is required")
	}
	snapshot, err := c.daemon.GetRunSnapshot(ctx, runID)
	if err != nil {
		return runspkg.RemoteRunSnapshot{}, err
	}
	return runspkg.RemoteRunSnapshot{
		Status:            snapshot.Run.Status,
		Incomplete:        snapshot.Incomplete,
		IncompleteReasons: append([]string(nil), snapshot.IncompleteReasons...),
		NextCursor:        remoteCursorPointer(snapshot.NextCursor),
	}, nil
}

func (c cliRemoteWatchClient) OpenRunStream(
	ctx context.Context,
	runID string,
	after runspkg.RemoteCursor,
) (runspkg.RemoteRunStream, error) {
	if c.daemon == nil {
		return nil, errors.New("daemon client is required")
	}
	stream, err := c.daemon.OpenRunStream(ctx, runID, apicore.StreamCursor{
		Timestamp: after.Timestamp,
		Sequence:  after.Sequence,
	})
	if err != nil {
		return nil, err
	}
	if stream == nil {
		return nil, nil
	}
	return newCLIRemoteRunStream(stream), nil
}

type cliRemoteRunStream struct {
	inner apiclient.RunStream
	items chan runspkg.RemoteRunStreamItem
	errs  chan error
}

func newCLIRemoteRunStream(inner apiclient.RunStream) *cliRemoteRunStream {
	stream := &cliRemoteRunStream{
		inner: inner,
		items: make(chan runspkg.RemoteRunStreamItem, 32),
		errs:  make(chan error, 4),
	}
	go stream.forward()
	return stream
}

func (s *cliRemoteRunStream) Items() <-chan runspkg.RemoteRunStreamItem {
	if s == nil {
		return nil
	}
	return s.items
}

func (s *cliRemoteRunStream) Errors() <-chan error {
	if s == nil {
		return nil
	}
	return s.errs
}

func (s *cliRemoteRunStream) Close() error {
	if s == nil || s.inner == nil {
		return nil
	}
	return s.inner.Close()
}

func (s *cliRemoteRunStream) forward() {
	if s == nil || s.inner == nil {
		return
	}

	defer close(s.items)
	defer close(s.errs)

	itemCh := s.inner.Items()
	errCh := s.inner.Errors()
	for itemCh != nil || errCh != nil {
		select {
		case item, ok := <-itemCh:
			if !ok {
				itemCh = nil
				continue
			}
			converted := runspkg.RemoteRunStreamItem{
				Event:           item.Event,
				HeartbeatCursor: remoteHeartbeatCursor(item.Heartbeat),
				OverflowCursor:  remoteOverflowCursor(item.Overflow),
			}
			s.items <- converted
		case err, ok := <-errCh:
			if !ok {
				errCh = nil
				continue
			}
			if err != nil {
				s.errs <- err
				return
			}
		}
	}
}

func remoteCursorPointer(cursor *apicore.StreamCursor) *runspkg.RemoteCursor {
	if cursor == nil {
		return nil
	}
	return &runspkg.RemoteCursor{
		Timestamp: cursor.Timestamp,
		Sequence:  cursor.Sequence,
	}
}

func remoteHeartbeatCursor(cursor *apiclient.RunStreamHeartbeat) *runspkg.RemoteCursor {
	if cursor == nil {
		return nil
	}
	return &runspkg.RemoteCursor{
		Timestamp: cursor.Cursor.Timestamp,
		Sequence:  cursor.Cursor.Sequence,
	}
}

func remoteOverflowCursor(cursor *apiclient.RunStreamOverflow) *runspkg.RemoteCursor {
	if cursor == nil {
		return nil
	}
	return &runspkg.RemoteCursor{
		Timestamp: cursor.Cursor.Timestamp,
		Sequence:  cursor.Cursor.Sequence,
	}
}
