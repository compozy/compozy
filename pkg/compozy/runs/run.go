package runs

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
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

type daemonRunPayload = contract.Run
type daemonRunJobState = contract.RunJobState
type daemonRunSnapshotPayload = contract.RunSnapshotResponse
type daemonRunEventPagePayload = contract.RunEventPageResponse
type transportErrorPayload = contract.TransportError

type defaultDaemonRunReader struct {
	baseURL    string
	httpClient *http.Client
	homePaths  compozyconfig.HomePaths
}

type sseFrame struct {
	id    string
	event string
	data  bytes.Buffer
}

type heartbeatPayload = contract.HeartbeatPayload
type overflowPayload = contract.OverflowPayload

type daemonRunStream struct {
	items     chan RemoteRunStreamItem
	errors    chan error
	cancel    context.CancelFunc
	body      io.Closer
	closeOnce sync.Once
	readDone  chan struct{}
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

	client, baseURL, err := newDaemonHTTPClient(info)
	if err != nil {
		return nil, wrapDaemonUnavailable("build daemon client", err)
	}

	return &defaultDaemonRunReader{
		baseURL:    baseURL,
		httpClient: client,
		homePaths:  homePaths,
	}, nil
}

func newDaemonHTTPClient(info daemonInfoRecord) (*http.Client, string, error) {
	socketPath := strings.TrimSpace(info.SocketPath)
	if socketPath != "" {
		transport := &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var dialer net.Dialer
				return dialer.DialContext(ctx, "unix", socketPath)
			},
		}
		return &http.Client{Transport: transport}, "http://daemon", nil
	}
	if info.HTTPPort <= 0 || info.HTTPPort > 65535 {
		return nil, "", errors.New("daemon transport target is invalid")
	}
	return &http.Client{}, "http://127.0.0.1:" + strconv.Itoa(info.HTTPPort), nil
}

func (d *defaultDaemonRunReader) OpenRun(
	ctx context.Context,
	workspaceRoot string,
	runID string,
) (RunSummary, error) {
	snapshot, err := d.getRunSnapshotPayload(ctx, strings.TrimSpace(runID))
	if err != nil {
		return RunSummary{}, err
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
	values := url.Values{}
	if workspace := strings.TrimSpace(workspaceRoot); workspace != "" {
		values.Set("workspace", workspace)
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = defaultRunListQueryLimit
	}
	if limit < defaultRunListQueryLimit {
		limit = defaultRunListQueryLimit
	}
	values.Set("limit", strconv.Itoa(limit))

	var payload struct {
		Runs []daemonRunPayload `json:"runs"`
	}
	if err := d.doJSON(ctx, "/api/runs?"+values.Encode(), &payload); err != nil {
		return nil, err
	}

	summaries := make([]RunSummary, 0, len(payload.Runs))
	for i := range payload.Runs {
		summaries = append(summaries, d.summaryFromRun(workspaceRoot, payload.Runs[i], nil))
	}
	return summaries, nil
}

func (d *defaultDaemonRunReader) GetRunSnapshot(
	ctx context.Context,
	runID string,
) (RemoteRunSnapshot, error) {
	payload, err := d.getRunSnapshotPayload(ctx, strings.TrimSpace(runID))
	if err != nil {
		return RemoteRunSnapshot{}, err
	}
	snapshot, err := payload.Decode()
	if err != nil {
		return RemoteRunSnapshot{}, err
	}

	result := RemoteRunSnapshot{
		Status: strings.TrimSpace(snapshot.Run.Status),
	}
	if cursor := remoteCursorFromPointer(snapshot.NextCursor); cursor != nil {
		result.NextCursor = cursor
	}
	return result, nil
}

func (d *defaultDaemonRunReader) ListRunEvents(
	ctx context.Context,
	runID string,
	after RemoteCursor,
	limit int,
) (remoteRunEventPage, error) {
	values := url.Values{}
	if after.Sequence > 0 {
		values.Set("after", formatRemoteCursor(after))
	}
	if limit > 0 {
		values.Set("limit", strconv.Itoa(limit))
	}

	path := "/api/runs/" + url.PathEscape(strings.TrimSpace(runID)) + "/events"
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var payload daemonRunEventPagePayload
	if err := d.doJSON(ctx, path, &payload); err != nil {
		return remoteRunEventPage{}, err
	}
	page, err := payload.Decode()
	if err != nil {
		return remoteRunEventPage{}, err
	}

	result := remoteRunEventPage{
		Events:  page.Events,
		HasMore: page.HasMore,
	}
	if cursor := remoteCursorFromPointer(page.NextCursor); cursor != nil {
		result.NextCursor = cursor
	}
	return result, nil
}

func (d *defaultDaemonRunReader) OpenRunStream(
	ctx context.Context,
	runID string,
	after RemoteCursor,
) (RemoteRunStream, error) {
	ctx, cancel := withRequestTimeout(
		ctx,
		contract.DefaultTimeout(
			contract.TimeoutClassForRoute(
				http.MethodGet,
				"/api/runs/"+url.PathEscape(strings.TrimSpace(runID))+"/stream",
			),
		),
	)
	defer cancel()
	request, err := d.newRequest(ctx, "/api/runs/"+url.PathEscape(strings.TrimSpace(runID))+"/stream")
	if err != nil {
		return nil, err
	}
	if after.Sequence > 0 {
		request.Header.Set("Last-Event-ID", formatRemoteCursor(after))
	}

	response, err := d.httpClient.Do(request)
	if err != nil {
		return nil, wrapDaemonRequestError("open remote run stream", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		defer response.Body.Close()
		payload, readErr := io.ReadAll(response.Body)
		if readErr != nil {
			return nil, wrapDaemonRequestError("open remote run stream", readErr)
		}
		return nil, d.decodeTransportError("open remote run stream", response.StatusCode, payload)
	}

	stream := &daemonRunStream{
		items:    make(chan RemoteRunStreamItem, 32),
		errors:   make(chan error, 4),
		body:     response.Body,
		readDone: make(chan struct{}),
	}
	go stream.read(ctx, response.Body)
	return stream, nil
}

func (d *defaultDaemonRunReader) getRunSnapshotPayload(
	ctx context.Context,
	runID string,
) (daemonRunSnapshotPayload, error) {
	var payload daemonRunSnapshotPayload
	path := "/api/runs/" + url.PathEscape(strings.TrimSpace(runID)) + "/snapshot"
	if err := d.doJSON(ctx, path, &payload); err != nil {
		return daemonRunSnapshotPayload{}, err
	}
	return payload, nil
}

func (d *defaultDaemonRunReader) summaryFromRun(
	workspaceRoot string,
	run daemonRunPayload,
	jobs []daemonRunJobState,
) RunSummary {
	summary := RunSummary{
		RunID:         strings.TrimSpace(run.RunID),
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

func (d *defaultDaemonRunReader) doJSON(ctx context.Context, path string, dst any) error {
	ctx, cancel := withRequestTimeout(ctx, contract.DefaultTimeout(contract.TimeoutClassForRoute(http.MethodGet, path)))
	defer cancel()

	request, err := d.newRequest(ctx, path)
	if err != nil {
		return err
	}

	response, err := d.httpClient.Do(request)
	if err != nil {
		return wrapDaemonRequestError(path, err)
	}
	defer response.Body.Close()

	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return wrapDaemonRequestError(path, err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return d.decodeTransportError(path, response.StatusCode, payload)
	}
	if len(payload) == 0 || dst == nil {
		return nil
	}
	if err := json.Unmarshal(payload, dst); err != nil {
		return fmt.Errorf("%s: decode daemon response: %w", path, err)
	}
	return nil
}

func (d *defaultDaemonRunReader) newRequest(ctx context.Context, path string) (*http.Request, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, d.baseURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("build daemon request: %w", err)
	}
	targetPath := strings.TrimSpace(path)
	if !strings.HasPrefix(targetPath, "/") {
		targetPath = "/" + targetPath
	}
	parsedPath, err := url.ParseRequestURI(targetPath)
	if err != nil {
		return nil, fmt.Errorf("validate daemon request path: %w", err)
	}
	if parsedPath.IsAbs() || parsedPath.Host != "" || !strings.HasPrefix(parsedPath.Path, "/api/") {
		return nil, fmt.Errorf("validate daemon request path: %q is not a daemon API path", path)
	}

	targetURL, err := url.Parse(d.baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse daemon base URL: %w", err)
	}
	targetURL.Path = parsedPath.Path
	targetURL.RawPath = parsedPath.EscapedPath()
	targetURL.RawQuery = parsedPath.RawQuery
	request.URL = targetURL
	request.Host = targetURL.Host
	return request, nil
}

func withRequestTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout <= 0 {
		return ctx, func() {}
	}
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

func (d *defaultDaemonRunReader) decodeTransportError(path string, statusCode int, payload []byte) error {
	var transport transportErrorPayload
	if err := json.Unmarshal(payload, &transport); err == nil {
		message := strings.TrimSpace(transport.Message)
		if message == "" {
			message = strings.TrimSpace(transport.Code)
		}
		if message == "" {
			message = http.StatusText(statusCode)
		}
		err := errors.New(message)
		if statusCode == http.StatusServiceUnavailable {
			return wrapDaemonUnavailable(path, err)
		}
		return fmt.Errorf("%s: %w", path, err)
	}
	if statusCode == http.StatusServiceUnavailable {
		return wrapDaemonUnavailable(path, errors.New(http.StatusText(statusCode)))
	}
	return fmt.Errorf("%s: daemon request failed with status %d", path, statusCode)
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

func wrapDaemonUnavailable(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w: %v", op, ErrDaemonUnavailable, err)
}

func wrapDaemonRequestError(op string, err error) error {
	if err == nil {
		return nil
	}

	var netErr *net.OpError
	if errors.As(err, &netErr) || errors.Is(err, context.DeadlineExceeded) {
		return wrapDaemonUnavailable(op, err)
	}
	return err
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

func remoteCursorFromPointer(cursor *contract.StreamCursor) *RemoteCursor {
	if cursor == nil || cursor.Sequence == 0 || cursor.Timestamp.IsZero() {
		return nil
	}
	result := remoteCursorFromContract(*cursor)
	return &result
}

type daemonRunStreamError struct {
	message string
}

func (e daemonRunStreamError) Error() string {
	return strings.TrimSpace(e.message)
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
	if s == nil {
		return nil
	}

	s.closeOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		if s.body != nil {
			_ = s.body.Close()
		}
		<-s.readDone
	})
	return nil
}

func (s *daemonRunStream) read(ctx context.Context, body io.Reader) {
	defer close(s.readDone)
	defer close(s.items)
	defer close(s.errors)

	reader := bufio.NewReader(body)
	frame := sseFrame{}
	for {
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			s.sendError(fmt.Errorf("read daemon stream: %w", err))
			return
		}

		if line != "" {
			if consumed, dispatchErr := s.consumeLine(&frame, line); dispatchErr != nil {
				s.sendError(dispatchErr)
				return
			} else if consumed {
				frame = sseFrame{}
			}
		}

		if errors.Is(err, io.EOF) {
			if frame.data.Len() > 0 || frame.event != "" || frame.id != "" {
				if dispatchErr := s.dispatchFrame(frame); dispatchErr != nil {
					s.sendError(dispatchErr)
				}
			}
			return
		}
		if ctx.Err() != nil {
			return
		}
	}
}

func (s *daemonRunStream) consumeLine(frame *sseFrame, line string) (bool, error) {
	trimmed := strings.TrimRight(line, "\r\n")
	if trimmed == "" {
		if frame == nil || (frame.data.Len() == 0 && frame.event == "" && frame.id == "") {
			return true, nil
		}
		return true, s.dispatchFrame(*frame)
	}

	switch {
	case strings.HasPrefix(trimmed, "id:"):
		frame.id = strings.TrimSpace(strings.TrimPrefix(trimmed, "id:"))
	case strings.HasPrefix(trimmed, "event:"):
		frame.event = strings.TrimSpace(strings.TrimPrefix(trimmed, "event:"))
	case strings.HasPrefix(trimmed, "data:"):
		if frame.data.Len() > 0 {
			frame.data.WriteByte('\n')
		}
		frame.data.WriteString(strings.TrimSpace(strings.TrimPrefix(trimmed, "data:")))
	}
	return false, nil
}

func (s *daemonRunStream) dispatchFrame(frame sseFrame) error {
	switch strings.TrimSpace(frame.event) {
	case "heartbeat":
		return s.dispatchHeartbeat(frame.data.Bytes())
	case "overflow":
		return s.dispatchOverflow(frame.data.Bytes())
	case "error":
		return s.dispatchStreamError(frame.data.Bytes())
	default:
		return s.dispatchEvent(frame.data.Bytes())
	}
}

func (s *daemonRunStream) dispatchHeartbeat(raw []byte) error {
	var payload heartbeatPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("decode heartbeat frame: %w", err)
	}
	cursor, err := parseRemoteCursor(payload.Cursor)
	if err != nil {
		return fmt.Errorf("decode heartbeat cursor: %w", err)
	}
	return s.sendItem(RemoteRunStreamItem{
		HeartbeatCursor: &cursor,
	})
}

func (s *daemonRunStream) dispatchOverflow(raw []byte) error {
	var payload overflowPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("decode overflow frame: %w", err)
	}
	cursor, err := parseRemoteCursor(payload.Cursor)
	if err != nil {
		return fmt.Errorf("decode overflow cursor: %w", err)
	}
	return s.sendItem(RemoteRunStreamItem{
		OverflowCursor: &cursor,
	})
}

func (s *daemonRunStream) dispatchStreamError(raw []byte) error {
	var payload transportErrorPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("decode stream error frame: %w", err)
	}
	message := strings.TrimSpace(payload.Message)
	if message == "" {
		message = strings.TrimSpace(payload.Code)
	}
	if message == "" {
		message = "daemon stream error"
	}
	return daemonRunStreamError{message: message}
}

func (s *daemonRunStream) dispatchEvent(raw []byte) error {
	var item events.Event
	if err := json.Unmarshal(raw, &item); err != nil {
		return fmt.Errorf("decode daemon event frame: %w", err)
	}
	return s.sendItem(RemoteRunStreamItem{Event: &item})
}

func (s *daemonRunStream) sendItem(item RemoteRunStreamItem) error {
	select {
	case s.items <- item:
		return nil
	default:
		return errors.New("client run stream buffer is full")
	}
}

func (s *daemonRunStream) sendError(err error) {
	if err == nil {
		return
	}
	select {
	case s.errors <- err:
	default:
	}
}
