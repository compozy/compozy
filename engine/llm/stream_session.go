package llm

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm/telemetry"
	"github.com/compozy/compozy/engine/streaming"
	"github.com/compozy/compozy/pkg/logger"
)

type streamSession struct {
	opts      *StreamOptions
	publisher streaming.Publisher
	mu        sync.Mutex
	chunkSeq  int64
	buffer    strings.Builder
}

type streamSessionKey struct{}

func withStreamSession(ctx context.Context, session *streamSession) context.Context {
	if session == nil {
		return ctx
	}
	return context.WithValue(ctx, streamSessionKey{}, session)
}

func streamSessionFromContext(ctx context.Context) (*streamSession, bool) {
	if ctx == nil {
		return nil, false
	}
	session, ok := ctx.Value(streamSessionKey{}).(*streamSession)
	if !ok || session == nil {
		return nil, false
	}
	return session, true
}

func newStreamSession(opts *StreamOptions) *streamSession {
	if opts == nil || opts.Publisher == nil || opts.ExecID.IsZero() {
		return nil
	}
	clone := opts.clone()
	return &streamSession{opts: clone, publisher: clone.Publisher}
}

func (s *streamSession) streamingHandler(ctx context.Context, chunk []byte) error {
	if s == nil || len(chunk) == 0 {
		return nil
	}
	text := string(chunk)
	if strings.TrimSpace(text) == "" {
		return nil
	}
	seq := s.nextSequence()
	s.appendStructured(text)
	s.publishChunkEvent(ctx, seq, text)
	if s.opts.Structured {
		s.publishStructuredSnapshot(ctx, seq, false)
	}
	return nil
}

func (s *streamSession) finalize(ctx context.Context, output *core.Output) {
	if s == nil {
		return
	}
	currentSeq := s.sequence()
	if s.opts.Structured {
		snapshot := s.snapshotStructured()
		if snapshot != "" {
			seq := s.nextSequence()
			s.publishStructuredContent(ctx, seq, snapshot, true)
			return
		}
		if currentSeq > 0 {
			return
		}
	}
	if !s.opts.Structured && currentSeq > 0 {
		return
	}
	fallback := extractTextFromOutput(output)
	if strings.TrimSpace(fallback) == "" {
		return
	}
	s.publishFallbackChunks(ctx, fallback)
}

func (s *streamSession) emitError(ctx context.Context, err error) {
	if s == nil || err == nil {
		return
	}
	data := s.baseData()
	data["message"] = core.RedactError(err)
	s.tryPublish(ctx, streaming.EventTypeError, data)
}

func (s *streamSession) observer() telemetry.Observer {
	if s == nil {
		return nil
	}
	return &streamObserver{session: s}
}

func (s *streamSession) publishStatus(ctx context.Context, stage string, iteration int, meta map[string]any) {
	if s == nil {
		return
	}
	data := s.baseData()
	data["stage"] = stage
	if iteration >= 0 {
		data["iteration"] = iteration
	}
	if len(meta) > 0 {
		data["metadata"] = meta
	}
	s.tryPublish(ctx, streaming.EventTypeStatus, data)
}

func (s *streamSession) publishWarning(ctx context.Context, code string, meta map[string]any) {
	if s == nil {
		return
	}
	data := s.baseData()
	data["code"] = code
	if len(meta) > 0 {
		data["details"] = meta
	}
	s.tryPublish(ctx, streaming.EventTypeWarning, data)
}

func (s *streamSession) publishTool(ctx context.Context, entry *telemetry.ToolLogEntry) {
	if s == nil || entry == nil {
		return
	}
	data := s.baseData()
	data["tool_call_id"] = entry.ToolCallID
	data["tool_name"] = entry.ToolName
	data["status"] = entry.Status
	data["duration_ms"] = float64(entry.Duration) / float64(time.Millisecond)
	if entry.Metadata != nil {
		data["metadata"] = entry.Metadata
	}
	if entry.Input != "" {
		data["input"] = entry.Input
	}
	if entry.Output != "" {
		data["output"] = entry.Output
	}
	if entry.Error != "" {
		data["error"] = entry.Error
	}
	data["redacted"] = entry.Redacted
	s.tryPublish(ctx, streaming.EventTypeToolCall, data)
}

func (s *streamSession) publishChunkEvent(ctx context.Context, seq int64, raw string) {
	sanitized := core.RedactString(raw)
	if strings.TrimSpace(sanitized) == "" {
		return
	}
	data := s.baseData()
	data["sequence"] = seq
	data["content"] = sanitized
	s.tryPublish(ctx, streaming.EventTypeLLMChunk, data)
}

func (s *streamSession) publishStructuredSnapshot(ctx context.Context, seq int64, complete bool) {
	s.publishStructuredContent(ctx, seq, s.snapshotStructured(), complete)
}

func (s *streamSession) publishStructuredContent(ctx context.Context, seq int64, content string, complete bool) {
	if s == nil || !s.opts.Structured {
		return
	}
	sanitized := strings.TrimSpace(content)
	if sanitized == "" {
		return
	}
	data := s.baseData()
	data["sequence"] = seq
	data["content"] = sanitized
	data["complete"] = complete
	s.tryPublish(ctx, streaming.EventTypeStructuredDelta, data)
}

func (s *streamSession) publishFallbackChunks(ctx context.Context, text string) {
	for _, line := range splitLines(text) {
		for _, segment := range segmentLine(line, fallbackSegmentLimit) {
			seq := s.nextSequence()
			s.publishChunkEvent(ctx, seq, segment)
		}
	}
}

func (s *streamSession) nextSequence() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.chunkSeq++
	return s.chunkSeq
}

func (s *streamSession) sequence() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.chunkSeq
}

func (s *streamSession) appendStructured(delta string) {
	if !s.opts.Structured {
		return
	}
	s.mu.Lock()
	s.buffer.WriteString(delta)
	s.mu.Unlock()
}

func (s *streamSession) snapshotStructured() string {
	if !s.opts.Structured {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return strings.TrimSpace(s.buffer.String())
}

func (s *streamSession) baseData() map[string]any {
	data := map[string]any{
		"component": s.opts.Component,
	}
	if s.opts.TaskID != "" {
		data["task_id"] = s.opts.TaskID
	}
	if !s.opts.WorkflowExecID.IsZero() {
		data["workflow_exec_id"] = s.opts.WorkflowExecID.String()
	}
	return data
}

func (s *streamSession) tryPublish(ctx context.Context, eventType streaming.EventType, data map[string]any) {
	if s.publisher == nil {
		return
	}
	_, err := s.publisher.Publish(ctx, s.opts.ExecID, streaming.Event{Type: eventType, Data: data})
	if err != nil {
		logger.FromContext(ctx).Warn(
			"Failed to publish stream event",
			"event_type", eventType,
			"exec_id", s.opts.ExecID.String(),
			"error", err,
		)
	}
}

const fallbackSegmentLimit = 200

func splitLines(text string) []string {
	if text == "" {
		return []string{""}
	}
	replaced := strings.ReplaceAll(text, "\r\n", "\n")
	replaced = strings.ReplaceAll(replaced, "\r", "\n")
	return strings.Split(replaced, "\n")
}

func segmentLine(line string, limit int) []string {
	if limit <= 0 || utf8.RuneCountInString(line) <= limit {
		return []string{line}
	}
	segments := make([]string, 0, utf8.RuneCountInString(line)/limit+1)
	var builder strings.Builder
	count := 0
	for _, r := range line {
		builder.WriteRune(r)
		count++
		if count >= limit {
			segments = append(segments, builder.String())
			builder.Reset()
			count = 0
		}
	}
	if builder.Len() > 0 {
		segments = append(segments, builder.String())
	}
	if len(segments) == 0 {
		segments = append(segments, line)
	}
	return segments
}

func extractTextFromOutput(output *core.Output) string {
	if output == nil {
		return ""
	}
	if response := firstString((*output)["response"]); response != "" {
		return response
	}
	if value := firstString((*output)[core.OutputRootKey]); value != "" {
		return value
	}
	for _, v := range *output {
		if s := firstString(v); s != "" {
			return s
		}
	}
	return ""
}

func firstString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case []byte:
		return string(v)
	default:
		return ""
	}
}

type streamObserver struct {
	session *streamSession
}

func (o *streamObserver) OnEvent(ctx context.Context, evt *telemetry.Event) {
	if o == nil || o.session == nil || evt == nil {
		return
	}
	switch evt.Stage {
	case "llm_request":
		o.session.publishStatus(ctx, evt.Stage, evt.Iteration, evt.Metadata)
	case "llm_response":
		o.session.publishStatus(ctx, evt.Stage, evt.Iteration, evt.Metadata)
	case "context_threshold":
		o.session.publishWarning(ctx, evt.Stage, evt.Metadata)
	case "loop_restart":
		o.session.publishStatus(ctx, evt.Stage, evt.Iteration, evt.Metadata)
	case "restart_threshold_clamped":
		o.session.publishWarning(ctx, evt.Stage, evt.Metadata)
	}
}

func (o *streamObserver) OnTool(ctx context.Context, entry *telemetry.ToolLogEntry) {
	if o == nil || o.session == nil {
		return
	}
	o.session.publishTool(ctx, entry)
}
