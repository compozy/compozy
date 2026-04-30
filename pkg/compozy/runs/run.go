package runs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	apiclient "github.com/compozy/compozy/internal/api/client"
	"github.com/compozy/compozy/internal/api/contract"
	apicore "github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

var (
	// ErrIncompatibleSchemaVersion reports an event schema the reader cannot decode.
	ErrIncompatibleSchemaVersion = errors.New("runs: incompatible schema version")
	// ErrPartialEventLine reports a truncated final JSON line in events.jsonl.
	ErrPartialEventLine = errors.New("runs: partial final event line")
	// ErrDaemonUnavailable reports that the public reader could not reach a ready daemon.
	ErrDaemonUnavailable = errors.New("runs: daemon unavailable")
)

const (
	publicRunStatusRunning   = "running"
	publicRunStatusCompleted = "completed"
	publicRunStatusFailed    = "failed"
	publicRunStatusCancelled = "cancel" + "led"
	publicRunStatusCrashed   = "crashed"

	defaultRunListQueryLimit = 500
	defaultRunEventPageLimit = 500
)

var resolveRunsDaemonReader = newDefaultDaemonRunReader

// SchemaVersionError reports an unsupported event schema version.
type SchemaVersionError struct {
	Version string
}

// Error implements the error interface.
func (e *SchemaVersionError) Error() string {
	if e == nil {
		return ErrIncompatibleSchemaVersion.Error()
	}
	return fmt.Sprintf("%s %q", ErrIncompatibleSchemaVersion.Error(), e.Version)
}

// Unwrap exposes the sentinel error for errors.Is checks.
func (e *SchemaVersionError) Unwrap() error {
	return ErrIncompatibleSchemaVersion
}

// Run is a handle over one daemon-backed run reader.
type Run struct {
	summary RunSummary
	client  daemonRunReader
}

type remoteRunEventPage struct {
	Events     []events.Event
	NextCursor *RemoteCursor
	HasMore    bool
}

type daemonRunReader interface {
	OpenRun(context.Context, string, string) (RunSummary, error)
	ListRuns(context.Context, string, ListOptions) ([]RunSummary, error)
	GetRunSnapshot(context.Context, string) (RemoteRunSnapshot, error)
	ListRunEvents(context.Context, string, RemoteCursor, int) (remoteRunEventPage, error)
	OpenRunStream(context.Context, string, RemoteCursor) (RemoteRunStream, error)
}

type daemonInfoRecord struct {
	SocketPath string `json:"socket_path,omitempty"`
	HTTPPort   int    `json:"http_port,omitempty"`
}

type daemonRunPayload = apicore.Run
type daemonRunJobState = apicore.RunJobState

type defaultDaemonRunReader struct {
	daemon    *apiclient.Client
	homePaths compozyconfig.HomePaths
}

// Open loads one run and prepares replay access through the daemon transport.
func Open(workspaceRoot, runID string) (*Run, error) {
	cleanRoot := cleanWorkspaceRoot(workspaceRoot)
	trimmedRunID := strings.TrimSpace(runID)
	if trimmedRunID == "" {
		return nil, errors.New("open run: missing run id")
	}

	client, err := resolveRunsDaemonReader()
	if err != nil {
		return nil, err
	}

	summary, err := client.OpenRun(context.Background(), cleanRoot, trimmedRunID)
	if err != nil {
		return nil, err
	}

	return &Run{
		summary: summary,
		client:  client,
	}, nil
}

// Summary returns the loaded run metadata.
func (r *Run) Summary() RunSummary {
	if r == nil {
		return RunSummary{}
	}
	return r.summary
}

func newDefaultDaemonRunReader() (daemonRunReader, error) {
	homePaths, err := compozyconfig.ResolveHomePaths()
	if err != nil {
		return nil, fmt.Errorf("resolve run reader home paths: %w", err)
	}

	info, err := readRunsDaemonInfo(homePaths.InfoPath)
	if err != nil {
		return nil, wrapDaemonUnavailable("resolve daemon info", err)
	}

	client, err := newDaemonClient(info)
	if err != nil {
		return nil, wrapDaemonUnavailable("build daemon client", err)
	}

	return &defaultDaemonRunReader{
		daemon:    client,
		homePaths: homePaths,
	}, nil
}

func newDaemonClient(info daemonInfoRecord) (*apiclient.Client, error) {
	return apiclient.New(apiclient.Target{
		SocketPath: strings.TrimSpace(info.SocketPath),
		HTTPPort:   info.HTTPPort,
	})
}

func (d *defaultDaemonRunReader) OpenRun(
	ctx context.Context,
	workspaceRoot string,
	runID string,
) (RunSummary, error) {
	snapshot, err := d.daemon.GetRunSnapshot(ctx, strings.TrimSpace(runID))
	if err != nil {
		return RunSummary{}, adaptDaemonClientError("open run", err)
	}

	summary := d.summaryFromRun(workspaceRoot, snapshot.Run, snapshot.Jobs)
	page, err := d.ListRunEvents(ctx, strings.TrimSpace(runID), RemoteCursor{}, 32)
	if err == nil {
		applySummaryEventDetails(&summary, page.Events)
	}
	if summary.Status == "" {
		summary.Status = defaultRunStatus()
	}
	return summary, nil
}

func (d *defaultDaemonRunReader) ListRuns(
	ctx context.Context,
	workspaceRoot string,
	opts ListOptions,
) ([]RunSummary, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = defaultRunListQueryLimit
	}
	if limit < defaultRunListQueryLimit {
		limit = defaultRunListQueryLimit
	}

	runs, err := d.daemon.ListRuns(ctx, apiclient.RunListOptions{
		Workspace: strings.TrimSpace(workspaceRoot),
		Limit:     limit,
	})
	if err != nil {
		return nil, adaptDaemonClientError("/api/runs", err)
	}

	summaries := make([]RunSummary, 0, len(runs))
	for i := range runs {
		summaries = append(summaries, d.summaryFromRun(workspaceRoot, runs[i], nil))
	}
	return summaries, nil
}

func (d *defaultDaemonRunReader) GetRunSnapshot(
	ctx context.Context,
	runID string,
) (RemoteRunSnapshot, error) {
	snapshot, err := d.daemon.GetRunSnapshot(ctx, strings.TrimSpace(runID))
	if err != nil {
		return RemoteRunSnapshot{}, adaptDaemonClientError("/api/runs/"+strings.TrimSpace(runID)+"/snapshot", err)
	}
	return adaptRemoteRunSnapshot(snapshot), nil
}

func (d *defaultDaemonRunReader) ListRunEvents(
	ctx context.Context,
	runID string,
	after RemoteCursor,
	limit int,
) (remoteRunEventPage, error) {
	page, err := d.daemon.ListRunEvents(ctx, strings.TrimSpace(runID), apicore.StreamCursor{
		Timestamp: after.Timestamp,
		Sequence:  after.Sequence,
	}, limit)
	if err != nil {
		return remoteRunEventPage{}, adaptDaemonClientError("/api/runs/"+strings.TrimSpace(runID)+"/events", err)
	}
	return adaptRemoteRunEventPage(page), nil
}

func (d *defaultDaemonRunReader) OpenRunStream(
	ctx context.Context,
	runID string,
	after RemoteCursor,
) (RemoteRunStream, error) {
	stream, err := d.daemon.OpenRunStream(ctx, strings.TrimSpace(runID), apicore.StreamCursor{
		Timestamp: after.Timestamp,
		Sequence:  after.Sequence,
	})
	if err != nil {
		return nil, adaptDaemonClientError("open remote run stream", err)
	}
	if stream == nil {
		return nil, nil
	}
	return newDaemonRunStreamAdapter(stream), nil
}

func (d *defaultDaemonRunReader) summaryFromRun(
	workspaceRoot string,
	run daemonRunPayload,
	jobs []daemonRunJobState,
) RunSummary {
	summary := RunSummary{
		RunID:         strings.TrimSpace(run.RunID),
		ParentRunID:   strings.TrimSpace(run.ParentRunID),
		Status:        normalizeStatus(run.Status),
		Mode:          strings.TrimSpace(run.Mode),
		WorkspaceRoot: cleanWorkspaceRoot(workspaceRoot),
		StartedAt:     run.StartedAt.UTC(),
		EndedAt:       utcTimePointer(run.EndedAt),
		ArtifactsDir:  filepath.Join(d.homePaths.RunsDir, strings.TrimSpace(run.RunID)),
	}

	for i := range jobs {
		if jobs[i].Summary == nil {
			continue
		}
		summary.IDE = firstNonEmpty(summary.IDE, jobs[i].Summary.IDE)
		summary.Model = firstNonEmpty(summary.Model, jobs[i].Summary.Model)
	}
	if summary.Status == "" {
		summary.Status = defaultRunStatus()
	}
	return summary
}

func readRunsDaemonInfo(path string) (daemonInfoRecord, error) {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return daemonInfoRecord{}, errors.New("daemon info path is required")
	}

	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return daemonInfoRecord{}, err
	}

	var info daemonInfoRecord
	if err := json.Unmarshal(data, &info); err != nil {
		return daemonInfoRecord{}, err
	}
	return info, nil
}

func applySummaryEventDetails(summary *RunSummary, items []events.Event) {
	if summary == nil {
		return
	}

	for i := range items {
		applyRunSummaryEvent(summary, items[i])
	}
}

func applyRunSummaryEvent(summary *RunSummary, item events.Event) {
	switch item.Kind {
	case events.EventKindRunQueued:
		applyRunQueuedSummary(summary, item.Payload)
	case events.EventKindRunStarted:
		applyRunStartedSummary(summary, item.Payload)
	case events.EventKindRunCompleted:
		applyRunTerminalSummary(summary, item.Timestamp, item.Payload, func(payload kinds.RunCompletedPayload) string {
			return payload.ArtifactsDir
		})
	case events.EventKindRunFailed:
		applyRunTerminalSummary(summary, item.Timestamp, item.Payload, func(payload kinds.RunFailedPayload) string {
			return payload.ArtifactsDir
		})
	case events.EventKindRunCrashed:
		applyRunTerminalSummary(summary, item.Timestamp, item.Payload, func(payload kinds.RunCrashedPayload) string {
			return payload.ArtifactsDir
		})
	case events.EventKindJobQueued:
		applyJobQueuedSummary(summary, item.Payload)
	case events.EventKindJobStarted:
		applyJobStartedSummary(summary, item.Payload)
	}
}

func applyRunQueuedSummary(summary *RunSummary, payloadJSON []byte) {
	var payload kinds.RunQueuedPayload
	if json.Unmarshal(payloadJSON, &payload) != nil {
		return
	}
	summary.WorkspaceRoot = firstNonEmpty(summary.WorkspaceRoot, payload.WorkspaceRoot)
	summary.IDE = firstNonEmpty(summary.IDE, payload.IDE)
	summary.Model = firstNonEmpty(summary.Model, payload.Model)
}

func applyRunStartedSummary(summary *RunSummary, payloadJSON []byte) {
	var payload kinds.RunStartedPayload
	if json.Unmarshal(payloadJSON, &payload) != nil {
		return
	}
	summary.WorkspaceRoot = firstNonEmpty(summary.WorkspaceRoot, payload.WorkspaceRoot)
	summary.IDE = firstNonEmpty(summary.IDE, payload.IDE)
	summary.Model = firstNonEmpty(summary.Model, payload.Model)
	summary.ArtifactsDir = firstNonEmpty(summary.ArtifactsDir, payload.ArtifactsDir)
}

func applyRunTerminalSummary[T any](
	summary *RunSummary,
	timestamp time.Time,
	payloadJSON []byte,
	artifactsDir func(T) string,
) {
	var payload T
	if json.Unmarshal(payloadJSON, &payload) != nil {
		return
	}
	summary.ArtifactsDir = firstNonEmpty(summary.ArtifactsDir, artifactsDir(payload))
	if summary.EndedAt == nil {
		summary.EndedAt = timePointer(timestamp)
	}
}

func applyJobQueuedSummary(summary *RunSummary, payloadJSON []byte) {
	var payload kinds.JobQueuedPayload
	if json.Unmarshal(payloadJSON, &payload) != nil {
		return
	}
	summary.IDE = firstNonEmpty(summary.IDE, payload.IDE)
	summary.Model = firstNonEmpty(summary.Model, payload.Model)
}

func applyJobStartedSummary(summary *RunSummary, payloadJSON []byte) {
	var payload kinds.JobStartedPayload
	if json.Unmarshal(payloadJSON, &payload) != nil {
		return
	}
	summary.IDE = firstNonEmpty(summary.IDE, payload.IDE)
	summary.Model = firstNonEmpty(summary.Model, payload.Model)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func utcTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copyValue := value.UTC()
	return &copyValue
}

func sortRunSummaries(items []RunSummary) {
	slices.SortFunc(items, func(left, right RunSummary) int {
		switch {
		case left.StartedAt.Equal(right.StartedAt):
			if left.RunID == right.RunID {
				return 0
			}
			if left.RunID > right.RunID {
				return -1
			}
			return 1
		case left.StartedAt.After(right.StartedAt):
			return -1
		default:
			return 1
		}
	})
}

func parseRemoteCursor(raw string) (RemoteCursor, error) {
	cursor, err := contract.ParseCursor(raw)
	if err != nil {
		return RemoteCursor{}, err
	}
	return remoteCursorFromContract(cursor), nil
}

func formatRemoteCursor(cursor RemoteCursor) string {
	return contract.FormatCursor(cursor.Timestamp, cursor.Sequence)
}

func remoteCursorFromContract(cursor contract.StreamCursor) RemoteCursor {
	return RemoteCursor{
		Timestamp: cursor.Timestamp,
		Sequence:  cursor.Sequence,
	}
}

func remoteCursorFromCore(cursor apicore.StreamCursor) RemoteCursor {
	return RemoteCursor{
		Timestamp: cursor.Timestamp,
		Sequence:  cursor.Sequence,
	}
}

func remoteCursorPointerFromCore(cursor *apicore.StreamCursor) *RemoteCursor {
	if cursor == nil || cursor.Sequence == 0 || cursor.Timestamp.IsZero() {
		return nil
	}
	value := remoteCursorFromCore(*cursor)
	return &value
}

func adaptRemoteRunSnapshot(snapshot apicore.RunSnapshot) RemoteRunSnapshot {
	return RemoteRunSnapshot{
		Status:            strings.TrimSpace(snapshot.Run.Status),
		Incomplete:        snapshot.Incomplete,
		IncompleteReasons: append([]string(nil), snapshot.IncompleteReasons...),
		NextCursor:        remoteCursorPointerFromCore(snapshot.NextCursor),
	}
}

func adaptRemoteRunEventPage(page apicore.RunEventPage) remoteRunEventPage {
	return remoteRunEventPage{
		Events:     page.Events,
		NextCursor: remoteCursorPointerFromCore(page.NextCursor),
		HasMore:    page.HasMore,
	}
}

func wrapDaemonUnavailable(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w: %v", op, ErrDaemonUnavailable, err)
}

func adaptDaemonClientError(op string, err error) error {
	if err == nil {
		return nil
	}

	var remoteErr *apiclient.RemoteError
	if errors.As(err, &remoteErr) {
		remoteMessage := errors.New(remoteErr.Error())
		if remoteErr.StatusCode == 0 || remoteErr.StatusCode == 503 {
			return wrapDaemonUnavailable(op, remoteMessage)
		}
		return fmt.Errorf("%s: %w", op, remoteMessage)
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return wrapDaemonUnavailable(op, err)
	}

	var netErr *net.OpError
	if errors.As(err, &netErr) {
		return wrapDaemonUnavailable(op, err)
	}

	return fmt.Errorf("%s: %w", op, err)
}

type daemonRunStream struct {
	inner  apiclient.RunStream
	items  chan RemoteRunStreamItem
	errors chan error
}

func newDaemonRunStreamAdapter(inner apiclient.RunStream) *daemonRunStream {
	stream := &daemonRunStream{
		inner:  inner,
		items:  make(chan RemoteRunStreamItem, 32),
		errors: make(chan error, 4),
	}
	go stream.forward()
	return stream
}

func (s *daemonRunStream) Items() <-chan RemoteRunStreamItem {
	if s == nil {
		return nil
	}
	return s.items
}

func (s *daemonRunStream) Errors() <-chan error {
	if s == nil {
		return nil
	}
	return s.errors
}

func (s *daemonRunStream) Close() error {
	if s == nil || s.inner == nil {
		return nil
	}
	return s.inner.Close()
}

func (s *daemonRunStream) forward() {
	if s == nil || s.inner == nil {
		return
	}

	defer close(s.items)
	defer close(s.errors)

	itemCh := s.inner.Items()
	errCh := s.inner.Errors()
	for itemCh != nil || errCh != nil {
		select {
		case item, ok := <-itemCh:
			if !ok {
				itemCh = nil
				continue
			}
			converted := RemoteRunStreamItem{
				Event: item.Event,
			}
			if item.Snapshot != nil {
				snapshot := RemoteRunSnapshot{
					Status:            normalizeStatus(item.Snapshot.Snapshot.Run.Status),
					Incomplete:        item.Snapshot.Snapshot.Incomplete,
					IncompleteReasons: append([]string(nil), item.Snapshot.Snapshot.IncompleteReasons...),
					NextCursor:        remoteCursorPointerFromCore(item.Snapshot.Snapshot.NextCursor),
				}
				converted.Snapshot = &snapshot
			}
			if item.Heartbeat != nil {
				cursor := remoteCursorFromCore(item.Heartbeat.Cursor)
				converted.HeartbeatCursor = &cursor
			}
			if item.Overflow != nil {
				cursor := remoteCursorFromCore(item.Overflow.Cursor)
				converted.OverflowCursor = &cursor
			}
			s.items <- converted
		case err, ok := <-errCh:
			if !ok {
				errCh = nil
				continue
			}
			if err != nil {
				s.errors <- err
			}
		}
	}
}
