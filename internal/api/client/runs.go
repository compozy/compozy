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

	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

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

// RunStreamOverflow reports that the client must reconnect from the last acknowledged cursor.
type RunStreamOverflow struct {
	Cursor    apicore.StreamCursor
	Reason    string
	Timestamp time.Time
}

// RunStreamItem is one parsed SSE delivery from the daemon.
type RunStreamItem struct {
	Event     *events.Event
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
	items     chan RunStreamItem
	errors    chan error
	ctx       context.Context
	cancel    context.CancelFunc
	body      io.Closer
	closeOnce sync.Once
	readDone  chan struct{}
}

type sseFrame struct {
	id    string
	event string
	data  bytes.Buffer
}

type heartbeatPayload struct {
	Cursor string    `json:"cursor"`
	TS     time.Time `json:"ts"`
}

type overflowPayload struct {
	Cursor string    `json:"cursor"`
	Reason string    `json:"reason"`
	TS     time.Time `json:"ts"`
}

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

	response := struct {
		Runs []apicore.Run `json:"runs"`
	}{}
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

	response := struct {
		Run apicore.Run `json:"run"`
	}{}
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

	var payload struct {
		Run        apicore.Run                    `json:"run"`
		Jobs       []apicore.RunJobState          `json:"jobs,omitempty"`
		Transcript []apicore.RunTranscriptMessage `json:"transcript,omitempty"`
		Usage      kinds.Usage                    `json:"usage,omitempty"`
		Shutdown   *apicore.RunShutdownState      `json:"shutdown,omitempty"`
		NextCursor string                         `json:"next_cursor,omitempty"`
	}
	path := "/api/runs/" + url.PathEscape(trimmedRunID) + "/snapshot"
	if _, err := c.doJSON(ctx, http.MethodGet, path, nil, &payload); err != nil {
		return apicore.RunSnapshot{}, err
	}

	nextCursor, err := apicore.ParseCursor(payload.NextCursor)
	if err != nil {
		return apicore.RunSnapshot{}, fmt.Errorf("decode snapshot cursor: %w", err)
	}

	snapshot := apicore.RunSnapshot{
		Run:        payload.Run,
		Jobs:       payload.Jobs,
		Transcript: payload.Transcript,
		Usage:      payload.Usage,
		Shutdown:   payload.Shutdown,
	}
	if nextCursor.Sequence > 0 {
		snapshot.NextCursor = &nextCursor
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

	response := struct {
		Events     []events.Event `json:"events"`
		NextCursor string         `json:"next_cursor,omitempty"`
		HasMore    bool           `json:"has_more"`
	}{}
	path := "/api/runs/" + url.PathEscape(trimmedRunID) + "/events"
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	if _, err := c.doJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return apicore.RunEventPage{}, err
	}

	nextCursor, err := apicore.ParseCursor(response.NextCursor)
	if err != nil {
		return apicore.RunEventPage{}, fmt.Errorf("decode events cursor: %w", err)
	}

	page := apicore.RunEventPage{
		Events:  response.Events,
		HasMore: response.HasMore,
	}
	if nextCursor.Sequence > 0 {
		page.NextCursor = &nextCursor
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
	request, err := http.NewRequestWithContext(streamCtx, http.MethodGet, c.baseURL, http.NoBody)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("build daemon stream request: %w", err)
	}
	if err := applyRequestPath(request, "/api/runs/"+url.PathEscape(trimmedRunID)+"/stream"); err != nil {
		cancel()
		return nil, err
	}
	if after.Sequence > 0 {
		request.Header.Set("Last-Event-ID", apicore.FormatCursor(after.Timestamp, after.Sequence))
	}

	response, err := c.roundTrip(request)
	if err != nil {
		cancel()
		return nil, err
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		payload, readErr := io.ReadAll(response.Body)
		_ = response.Body.Close()
		cancel()
		if readErr != nil {
			return nil, fmt.Errorf("read daemon stream response: %w", readErr)
		}
		if err := c.handleStatus(request.URL.Path, response.StatusCode, payload, nil); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("unexpected daemon stream status: %d", response.StatusCode)
	}

	stream := &clientRunStream{
		items:    make(chan RunStreamItem, 32),
		errors:   make(chan error, 4),
		ctx:      streamCtx,
		cancel:   cancel,
		body:     response.Body,
		readDone: make(chan struct{}),
	}
	go stream.read(response.Body)
	return stream, nil
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
		if s.body != nil {
			_ = s.body.Close()
		}
		<-s.readDone
	})
	return nil
}

func (s *clientRunStream) read(body io.Reader) {
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
	}
}

func (s *clientRunStream) consumeLine(frame *sseFrame, line string) (bool, error) {
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

func (s *clientRunStream) dispatchFrame(frame sseFrame) error {
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

func (s *clientRunStream) dispatchHeartbeat(raw []byte) error {
	var payload heartbeatPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("decode heartbeat frame: %w", err)
	}
	cursor, err := apicore.ParseCursor(payload.Cursor)
	if err != nil {
		return fmt.Errorf("decode heartbeat cursor: %w", err)
	}
	return s.sendItem(RunStreamItem{
		Heartbeat: &RunStreamHeartbeat{
			Cursor:    cursor,
			Timestamp: payload.TS,
		},
	})
}

func (s *clientRunStream) dispatchOverflow(raw []byte) error {
	var payload overflowPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("decode overflow frame: %w", err)
	}
	cursor, err := apicore.ParseCursor(payload.Cursor)
	if err != nil {
		return fmt.Errorf("decode overflow cursor: %w", err)
	}
	return s.sendItem(RunStreamItem{
		Overflow: &RunStreamOverflow{
			Cursor:    cursor,
			Reason:    strings.TrimSpace(payload.Reason),
			Timestamp: payload.TS,
		},
	})
}

func (s *clientRunStream) dispatchStreamError(raw []byte) error {
	var payload apicore.TransportError
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

func (s *clientRunStream) dispatchEvent(raw []byte) error {
	var item events.Event
	if err := json.Unmarshal(raw, &item); err != nil {
		return fmt.Errorf("decode daemon event frame: %w", err)
	}
	return s.sendItem(RunStreamItem{Event: &item})
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
