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
	attachCLIRunUI         = defaultAttachCLIRunUI
	attachStartedCLIRunUI  = defaultAttachStartedCLIRunUI
	watchCLIRun            = defaultWatchCLIRun
	openCLIRemoteUISession = uipkg.AttachRemote
)

var errRunSettledBeforeUIAttach = errors.New("run settled before ui attach")

const (
	defaultUIAttachSnapshotTimeout      = 300 * time.Millisecond
	defaultUIAttachSnapshotPollInterval = 10 * time.Millisecond
	defaultOwnedRunCancelTimeout        = 5 * time.Second
)

func defaultAttachCLIRunUI(ctx context.Context, client daemonCommandClient, runID string) error {
	return attachRemoteCLIRunUI(ctx, client, runID, false)
}

func defaultAttachStartedCLIRunUI(ctx context.Context, client daemonCommandClient, runID string) error {
	return attachRemoteCLIRunUI(ctx, client, runID, true)
}

func attachRemoteCLIRunUI(
	ctx context.Context,
	client daemonCommandClient,
	runID string,
	cancelOwnedRunOnExit bool,
) error {
	trimmedRunID := strings.TrimSpace(runID)
	if client == nil {
		return errors.New("daemon client is required")
	}
	if trimmedRunID == "" {
		return errors.New("run id is required")
	}

	snapshot, err := loadUIAttachSnapshot(
		ctx,
		client,
		trimmedRunID,
		defaultUIAttachSnapshotTimeout,
		defaultUIAttachSnapshotPollInterval,
	)
	if err != nil {
		return err
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
	var cancelRequested atomic.Bool
	if cancelOwnedRunOnExit {
		session.SetQuitHandler(func(uipkg.QuitRequest) {
			cancelRequested.Store(true)
			session.Shutdown()
		})
	}

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
		if err := cancelOwnedDaemonRun(ctx, client, trimmedRunID); err != nil {
			return err
		}
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return nil
	}
	return nil
}

func cancelOwnedDaemonRun(ctx context.Context, client daemonCommandClient, runID string) error {
	if client == nil {
		return errors.New("daemon client is required")
	}
	detachedCtx := context.Background()
	if ctx != nil {
		detachedCtx = context.WithoutCancel(ctx)
	}
	cancelCtx, cancel := context.WithTimeout(detachedCtx, defaultOwnedRunCancelTimeout)
	defer cancel()
	if err := client.CancelRun(cancelCtx, runID); err != nil {
		return fmt.Errorf("cancel daemon run after ui exit: %w", err)
	}
	return nil
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

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return snapshot, err
		}
		time.Sleep(pollInterval)

		snapshot, err = client.GetRunSnapshot(ctx, runID)
		if err != nil {
			return apicore.RunSnapshot{}, err
		}
		if !runSnapshotNeedsWarmup(snapshot) {
			return snapshot, nil
		}
	}
	return snapshot, nil
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
			line := renderObservedRunEvent(event)
			if strings.TrimSpace(line) == "" {
				continue
			}
			if _, err := io.WriteString(dst, line); err != nil {
				return fmt.Errorf("write run watch output: %w", err)
			}
		}
	}
	return nil
}

func renderObservedRunEvent(event eventspkg.Event) string {
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
		Status:     snapshot.Run.Status,
		NextCursor: remoteCursorPointer(snapshot.NextCursor),
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
