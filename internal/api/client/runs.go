package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/pkg/compozy/events"
)

var streamHeartbeatGapTolerance = contract.HeartbeatGapTolerance

// RunListOptions filters the daemon-backed run list query.
type RunListOptions struct {
	Workspace string
	Status    string
	Mode      string
	Limit     int
}

// RunStreamHeartbeat reports one idle heartbeat frame from the daemon stream.
type RunStreamHeartbeat struct {
	Cursor    apicore.StreamCursor
	Timestamp time.Time
}

// RunStreamSnapshot reports one snapshot frame from the daemon stream contract.
type RunStreamSnapshot struct {
	Snapshot apicore.RunSnapshot
}

// RunStreamOverflow reports that the client must reconnect from the last acknowledged cursor.
type RunStreamOverflow struct {
	Cursor    apicore.StreamCursor
	Reason    string
	Timestamp time.Time
}

// RunStreamItem is one parsed SSE delivery from the daemon.
type RunStreamItem struct {
	Event     *events.Event
	Snapshot  *RunStreamSnapshot
	Heartbeat *RunStreamHeartbeat
	Overflow  *RunStreamOverflow
}

// RunStream consumes daemon SSE frames until EOF, cancellation, or Close.
type RunStream interface {
	Items() <-chan RunStreamItem
	Errors() <-chan error
	Close() error
}

type clientRunStream struct {
	client     *Client
	runID      string
	items      chan RunStreamItem
	errors     chan error
	ctx        context.Context
	cancel     context.CancelFunc
	closeOnce  sync.Once
	readDone   chan struct{}
	cursorMu   sync.Mutex
	lastCursor apicore.StreamCursor
	body       io.Closer
}

type sseFrame struct {
	id    string
	event string
	data  bytes.Buffer
}

type streamEnvelope struct {
	item   RunStreamItem
	cursor apicore.StreamCursor
}

type streamConnection struct {
	ctx    context.Context
	items  chan streamEnvelope
	errors chan error
	done   chan struct{}
}

type heartbeatPayload = contract.HeartbeatPayload
type overflowPayload = contract.OverflowPayload
// ListRuns lists daemon-managed runs for the requested workspace and filters.
func (c *Client) ListRuns(ctx context.Context, opts RunListOptions) ([]apicore.Run, error) {
	if c == nil {
		return nil, ErrDaemonClientRequired
	}

	values := url.Values{}
	if workspace := strings.TrimSpace(opts.Workspace); workspace != "" {
		values.Set("workspace", workspace)
	}
	if status := strings.TrimSpace(opts.Status); status != "" {
		values.Set("status", status)
	}
	if mode := strings.TrimSpace(opts.Mode); mode != "" {
		values.Set("mode", mode)
	}
	if opts.Limit > 0 {
		values.Set("limit", fmt.Sprintf("%d", opts.Limit))
	}

	var response contract.RunListResponse
	path := "/api/runs"
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	if _, err := c.doJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return nil, err
	}
	return response.Runs, nil
}

// GetRun loads the latest daemon-backed run summary for one run.
func (c *Client) GetRun(ctx context.Context, runID string) (apicore.Run, error) {
	if c == nil {
		return apicore.Run{}, ErrDaemonClientRequired
	}

	trimmedRunID := strings.TrimSpace(runID)
	if trimmedRunID == "" {
		return apicore.Run{}, ErrRunIDRequired
	}

	var response contract.RunResponse
	path := "/api/runs/" + url.PathEscape(trimmedRunID)
	if _, err := c.doJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return apicore.Run{}, err
	}
	return response.Run, nil
}

// CancelRun requests cancellation for one daemon-backed run.
func (c *Client) CancelRun(ctx context.Context, runID string) error {
	if c == nil {
		return ErrDaemonClientRequired
	}

	trimmedRunID := strings.TrimSpace(runID)
	if trimmedRunID == "" {
		return ErrRunIDRequired
	}

	path := "/api/runs/" + url.PathEscape(trimmedRunID) + "/cancel"
	_, err := c.doJSON(ctx, http.MethodPost, path, nil, nil)
	return err
}

// GetRunSnapshot loads the dense attach snapshot for one run.
func (c *Client) GetRunSnapshot(ctx context.Context, runID string) (apicore.RunSnapshot, error) {
	if c == nil {
		return apicore.RunSnapshot{}, ErrDaemonClientRequired
	}

	trimmedRunID := strings.TrimSpace(runID)
	if trimmedRunID == "" {
		return apicore.RunSnapshot{}, ErrRunIDRequired
	}

	var payload contract.RunSnapshotResponse
	path := "/api/runs/" + url.PathEscape(trimmedRunID) + "/snapshot"
	if _, err := c.doJSON(ctx, http.MethodGet, path, nil, &payload); err != nil {
		return apicore.RunSnapshot{}, err
	}

	snapshot, err := payload.Decode()
	if err != nil {
		return apicore.RunSnapshot{}, err
	}
	return snapshot, nil
}

// ListRunEvents pages through persisted daemon-backed events for one run.
func (c *Client) ListRunEvents(
	ctx context.Context,
	runID string,
	after apicore.StreamCursor,
	limit int,
) (apicore.RunEventPage, error) {
	if c == nil {
		return apicore.RunEventPage{}, ErrDaemonClientRequired
	}

	trimmedRunID := strings.TrimSpace(runID)
	if trimmedRunID == "" {
		return apicore.RunEventPage{}, ErrRunIDRequired
	}

	values := url.Values{}
	if after.Sequence > 0 {
		values.Set("after", apicore.FormatCursor(after.Timestamp, after.Sequence))
	}
	if limit > 0 {
		values.Set("limit", fmt.Sprintf("%d", limit))
	}

	var response contract.RunEventPageResponse
	path := "/api/runs/" + url.PathEscape(trimmedRunID) + "/events"
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	if _, err := c.doJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return apicore.RunEventPage{}, err
	}

	page, err := response.Decode()
	if err != nil {
		return apicore.RunEventPage{}, err
	}
	return page, nil
}

// OpenRunStream opens the daemon SSE stream for one run after the supplied cursor.
func (c *Client) OpenRunStream(
	ctx context.Context,
	runID string,
	after apicore.StreamCursor,
) (RunStream, error) {
	if c == nil {
		return nil, ErrDaemonClientRequired
	}

	trimmedRunID := strings.TrimSpace(runID)
	if trimmedRunID == "" {
		return nil, ErrRunIDRequired
	}
	if ctx == nil {
		ctx = context.Background()
	}

	streamCtx, cancel := context.WithCancel(ctx)
	body, err := c.openRunStreamBody(streamCtx, trimmedRunID, after)
	if err != nil {
		cancel()
		return nil, err
	}

	stream := &clientRunStream{
		client:     c,
		runID:      trimmedRunID,
		items:      make(chan RunStreamItem, 32),
		errors:     make(chan error, 4),
		ctx:        streamCtx,
		cancel:     cancel,
		readDone:   make(chan struct{}),
		lastCursor: after,
	}
	go stream.run(body)
	return stream, nil
}

func (c *Client) openRunStreamBody(
	ctx context.Context,
	runID string,
	after apicore.StreamCursor,
) (io.ReadCloser, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("build daemon stream request: %w", err)
	}
	path := "/api/runs/" + url.PathEscape(runID) + "/stream"
	if err := applyRequestPath(request, path); err != nil {
		return nil, err
	}
	if after.Sequence > 0 {
		request.Header.Set("Last-Event-ID", contract.FormatCursor(after.Timestamp, after.Sequence))
	}

	response, err := c.roundTrip(request)
	if err != nil {
		return nil, err
	}

	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return response.Body, nil
	}

	payload, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil {
		return nil, fmt.Errorf("read daemon stream response: %w", readErr)
	}
	if err := c.handleStatus(path, response.StatusCode, payload, nil); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("unexpected daemon stream status: %d", response.StatusCode)
}

func (s *clientRunStream) Items() <-chan RunStreamItem {
	if s == nil {
		return nil
	}
	return s.items
}

func (s *clientRunStream) Errors() <-chan error {
	if s == nil {
		return nil
	}
	return s.errors
}

func (s *clientRunStream) Close() error {
	if s == nil {
		return nil
	}

	s.closeOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		s.closeCurrentBody()
		<-s.readDone
	})
	return nil
}

func (s *clientRunStream) run(body io.ReadCloser) {
	defer close(s.readDone)
	defer close(s.items)
	defer close(s.errors)

	for {
		connection := newStreamConnection(s.ctx, body)
		s.setCurrentBody(body)
		reconnect, stop := s.consumeConnection(connection)
		s.setCurrentBody(nil)
		if stop {
			return
		}
		if !reconnect {
			return
		}
		nextBody, err := s.client.openRunStreamBody(s.ctx, s.runID, s.lastAckCursor())
		if err != nil {
			s.sendError(err)
			return
		}
		body = nextBody
	}
}

func (s *clientRunStream) consumeConnection(connection *streamConnection) (bool, bool) {
	timer := time.NewTimer(streamHeartbeatGapTolerance)
	defer timer.Stop()

	itemCh := connection.items
	errCh := connection.errors

	for itemCh != nil || errCh != nil {
		select {
		case <-s.ctx.Done():
			s.closeCurrentBody()
			<-connection.done
			return false, true
		case <-timer.C:
			s.closeCurrentBody()
			<-connection.done
			return true, false
		case envelope, ok := <-itemCh:
			if !ok {
				itemCh = nil
				continue
			}
			if envelope.cursor.Sequence > 0 {
				s.storeAckCursor(envelope.cursor)
			}
			if err := s.sendItem(envelope.item); err != nil {
				s.closeCurrentBody()
				<-connection.done
				return false, true
			}
			if envelope.item.Overflow != nil {
				s.closeCurrentBody()
				<-connection.done
				return true, false
			}
			resetStreamTimer(timer, streamHeartbeatGapTolerance)
		case err, ok := <-errCh:
			if !ok {
				errCh = nil
				continue
			}
			if err != nil {
				s.closeCurrentBody()
				<-connection.done
				s.sendError(err)
				return false, true
			}
		}
	}

	return true, false
}

func newStreamConnection(ctx context.Context, body io.ReadCloser) *streamConnection {
	connection := &streamConnection{
		ctx:    ctx,
		items:  make(chan streamEnvelope, 32),
		errors: make(chan error, 4),
		done:   make(chan struct{}),
	}
	go connection.read(body)
	return connection
}

func (c *streamConnection) read(body io.Reader) {
	defer close(c.done)
	defer close(c.items)
	defer close(c.errors)

	reader := bufio.NewReader(body)
	frame := sseFrame{}
	for {
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			c.sendError(fmt.Errorf("read daemon stream: %w", err))
			return
		}

		if line != "" {
			if consumed, dispatchErr := c.consumeLine(&frame, line); dispatchErr != nil {
				c.sendError(dispatchErr)
				return
			} else if consumed {
				frame = sseFrame{}
			}
		}

		if errors.Is(err, io.EOF) {
			if frame.data.Len() > 0 || frame.event != "" || frame.id != "" {
				if dispatchErr := c.dispatchFrame(frame); dispatchErr != nil {
					c.sendError(dispatchErr)
				}
			}
			return
		}
	}
}

func (c *streamConnection) consumeLine(frame *sseFrame, line string) (bool, error) {
	trimmed := strings.TrimRight(line, "\r\n")
	if trimmed == "" {
		if frame == nil || (frame.data.Len() == 0 && frame.event == "" && frame.id == "") {
			return true, nil
		}
		return true, c.dispatchFrame(*frame)
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

func (c *streamConnection) dispatchFrame(frame sseFrame) error {
	switch strings.TrimSpace(frame.event) {
	case apicore.RunHeartbeatSSEEvent, "heartbeat":
		return c.dispatchHeartbeat(frame.data.Bytes())
	case apicore.RunOverflowSSEEvent, "overflow":
		return c.dispatchOverflow(frame.data.Bytes())
	case apicore.RunSnapshotSSEEvent:
		return c.dispatchSnapshot(frame.data.Bytes())
	case "error":
		return c.dispatchStreamError(frame.data.Bytes())
	default:
		return c.dispatchEvent(frame.data.Bytes())
	}
}

func (c *streamConnection) dispatchSnapshot(raw []byte) error {
	var payload contract.RunSnapshotResponse
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("decode snapshot frame: %w", err)
	}
	snapshot, err := payload.Decode()
	if err != nil {
		return fmt.Errorf("decode snapshot cursor: %w", err)
	}

	envelope := streamEnvelope{
		item: RunStreamItem{
			Snapshot: &RunStreamSnapshot{Snapshot: snapshot},
		},
	}
	if snapshot.NextCursor != nil {
		envelope.cursor = *snapshot.NextCursor
	}
	return c.sendItem(envelope)
}

func (c *streamConnection) dispatchHeartbeat(raw []byte) error {
	var payload heartbeatPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("decode heartbeat frame: %w", err)
	}
	cursor, err := contract.ParseCursor(payload.Cursor)
	if err != nil {
		return fmt.Errorf("decode heartbeat cursor: %w", err)
	}
	return c.sendItem(streamEnvelope{
		item: RunStreamItem{
			Heartbeat: &RunStreamHeartbeat{
				Cursor:    cursor,
				Timestamp: payload.TS,
			},
		},
		cursor: cursor,
	})
}

func (c *streamConnection) dispatchOverflow(raw []byte) error {
	var payload overflowPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("decode overflow frame: %w", err)
	}
	cursor, err := contract.ParseCursor(payload.Cursor)
	if err != nil {
		return fmt.Errorf("decode overflow cursor: %w", err)
	}
	return c.sendItem(streamEnvelope{
		item: RunStreamItem{
			Overflow: &RunStreamOverflow{
				Cursor:    cursor,
				Reason:    strings.TrimSpace(payload.Reason),
				Timestamp: payload.TS,
			},
		},
		cursor: cursor,
	})
}

func (c *streamConnection) dispatchStreamError(raw []byte) error {
	var payload contract.TransportError
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
	return errors.New(message)
}

func (c *streamConnection) dispatchEvent(raw []byte) error {
	var item events.Event
	if err := json.Unmarshal(raw, &item); err != nil {
		return fmt.Errorf("decode daemon event frame: %w", err)
	}
	cursor := contract.CursorFromEvent(item)
	return c.sendItem(streamEnvelope{
		item:   RunStreamItem{Event: &item},
		cursor: cursor,
	})
}

func (c *streamConnection) sendItem(item streamEnvelope) error {
	select {
	case c.items <- item:
		return nil
	case <-c.ctx.Done():
		return c.ctx.Err()
	}
}

func (c *streamConnection) sendError(err error) {
	if err == nil {
		return
	}
	select {
	case c.errors <- err:
	default:
	}
}

func (s *clientRunStream) sendItem(item RunStreamItem) error {
	if s == nil {
		return ErrDaemonClientRequired
	}
	var done <-chan struct{}
	if s.ctx != nil {
		done = s.ctx.Done()
	}
	select {
	case s.items <- item:
		return nil
	case <-done:
		return s.ctx.Err()
	}
}

func (s *clientRunStream) sendError(err error) {
	if err == nil {
		return
	}
	select {
	case s.errors <- err:
	default:
	}
}

func (s *clientRunStream) storeAckCursor(cursor apicore.StreamCursor) {
	s.cursorMu.Lock()
	defer s.cursorMu.Unlock()

	if cursor.Sequence == 0 || cursor.Timestamp.IsZero() {
		return
	}
	if cursorAfterOrEqual(cursor, s.lastCursor) {
		s.lastCursor = cursor
	}
}

func (s *clientRunStream) lastAckCursor() apicore.StreamCursor {
	s.cursorMu.Lock()
	defer s.cursorMu.Unlock()
	return s.lastCursor
}

func (s *clientRunStream) setCurrentBody(body io.Closer) {
	s.cursorMu.Lock()
	defer s.cursorMu.Unlock()
	s.body = body
}

func (s *clientRunStream) closeCurrentBody() {
	s.cursorMu.Lock()
	body := s.body
	s.body = nil
	s.cursorMu.Unlock()
	if body != nil {
		_ = body.Close()
	}
}

func cursorAfterOrEqual(left apicore.StreamCursor, right apicore.StreamCursor) bool {
	switch {
	case right.Sequence == 0 || right.Timestamp.IsZero():
		return true
	case left.Timestamp.After(right.Timestamp):
		return true
	case left.Timestamp.Before(right.Timestamp):
		return false
	default:
		return left.Sequence >= right.Sequence
	}
}

func resetStreamTimer(timer *time.Timer, interval time.Duration) {
	if timer == nil {
		return
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(interval)
}
