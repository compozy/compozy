package hooks

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"
)

// HookRunWriter persists hook run records into an active session-scoped store.
type HookRunWriter interface {
	RecordHookRun(context.Context, HookRunRecord) error
}

// TelemetrySink persists hook run records when no active writer is attached to
// the dispatch context.
type TelemetrySink interface {
	WriteHookRecord(ctx context.Context, sessionID string, record HookRunRecord) error
}

type hookRunWriterContextKey struct{}

type dispatchMetricKey struct {
	Event   HookEvent
	Source  HookSource
	Mode    HookMode
	Outcome HookRunOutcome
}

type hookMetrics struct {
	mu sync.Mutex

	dispatchCounts              map[dispatchMetricKey]int64
	dispatchLatency             map[dispatchMetricKey]time.Duration
	pipelineCounts              map[HookEvent]int64
	pipelineLatency             map[HookEvent]time.Duration
	asyncDropCount              int64
	asyncQueueDepth             int
	permissionEscalationBlocks  int64
	dispatchDepthViolationCount int64
	registryReloadCount         int64
	registryReloadLatency       time.Duration
	registryReloadLastHookDelta int
}

type hookTraceEntry struct {
	Hook     string          `json:"hook"`
	Outcome  HookRunOutcome  `json:"outcome"`
	Duration time.Duration   `json:"duration"`
	Required bool            `json:"required,omitempty"`
	Error    string          `json:"error,omitempty"`
	Patch    json.RawMessage `json:"patch,omitempty"`
}

type dispatchReport struct {
	Trace          []hookTraceEntry
	Denied         bool
	DenySource     string
	DenyReason     string
	FailedHook     string
	FailedRequired bool
}

// WithHookRunWriter attaches a direct hook-run persistence writer to the context.
func WithHookRunWriter(ctx context.Context, writer HookRunWriter) context.Context {
	if ctx == nil || writer == nil {
		return ctx
	}
	return context.WithValue(ctx, hookRunWriterContextKey{}, writer)
}

// HookRunWriterFromContext resolves the attached hook-run writer, if any.
func HookRunWriterFromContext(ctx context.Context) HookRunWriter {
	if ctx == nil {
		return nil
	}
	writer, ok := ctx.Value(hookRunWriterContextKey{}).(HookRunWriter)
	if !ok {
		return nil
	}
	return writer
}

func newHookMetrics() *hookMetrics {
	return &hookMetrics{
		dispatchCounts:  make(map[dispatchMetricKey]int64),
		dispatchLatency: make(map[dispatchMetricKey]time.Duration),
		pipelineCounts:  make(map[HookEvent]int64),
		pipelineLatency: make(map[HookEvent]time.Duration),
	}
}

func (m *hookMetrics) observeHookRun(record HookRunRecord) {
	if m == nil {
		return
	}
	key := dispatchMetricKey{
		Event:   record.Event,
		Source:  record.Source,
		Mode:    record.Mode,
		Outcome: record.Outcome,
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.dispatchCounts[key]++
	m.dispatchLatency[key] += record.Duration
}

func (m *hookMetrics) observePipeline(event HookEvent, duration time.Duration) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pipelineCounts[event]++
	m.pipelineLatency[event] += duration
}

func (m *hookMetrics) observeAsyncDrop(queueDepth int) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.asyncDropCount++
	if queueDepth > m.asyncQueueDepth {
		m.asyncQueueDepth = queueDepth
	}
}

func (m *hookMetrics) observeQueueDepth(queueDepth int) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if queueDepth > m.asyncQueueDepth {
		m.asyncQueueDepth = queueDepth
	}
}

func (m *hookMetrics) observePermissionEscalationBlock() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.permissionEscalationBlocks++
}

func (m *hookMetrics) observeDepthViolation() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dispatchDepthViolationCount++
}

func (m *hookMetrics) observeRegistryReload(duration time.Duration, hookDelta int) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.registryReloadCount++
	m.registryReloadLatency += duration
	m.registryReloadLastHookDelta = hookDelta
}

func (h *Hooks) emitHookRun(
	ctx context.Context,
	payload any,
	hook RegisteredHook,
	outcome HookRunOutcome,
	duration time.Duration,
	rawPatch json.RawMessage,
	err error,
	depth int,
) {
	if h == nil {
		return
	}

	record := HookRunRecord{
		HookName:      hook.Name,
		Event:         hook.Event,
		Source:        hook.Source,
		Mode:          hook.Mode,
		Duration:      duration,
		Outcome:       outcome,
		DispatchDepth: depth,
		PatchApplied:  h.persistedPatchForEvent(hook.Event, rawPatch),
		Error:         strings.TrimSpace(errorString(err)),
		Required:      hook.Required,
		RecordedAt:    h.now().UTC(),
	}

	h.metrics.observeHookRun(record)

	if writer := HookRunWriterFromContext(ctx); writer != nil {
		if writeErr := writer.RecordHookRun(ctx, record); writeErr != nil {
			h.logger.WarnContext(
				ctx,
				"hook.dispatch.telemetry_write_failed",
				"hook",
				hook.Name,
				"event",
				hook.Event.String(),
				"error",
				writeErr,
			)
		}
		return
	}

	if h.telemetrySink == nil {
		return
	}

	sessionID := sessionIDFromPayload(payload)
	if strings.TrimSpace(sessionID) == "" {
		return
	}

	if writeErr := h.telemetrySink.WriteHookRecord(ctx, sessionID, record); writeErr != nil {
		h.logger.WarnContext(
			ctx,
			"hook.dispatch.telemetry_write_failed",
			"hook",
			hook.Name,
			"event",
			hook.Event.String(),
			"session_id",
			sessionID,
			"error",
			writeErr,
		)
	}
}

func (h *Hooks) persistedPatchForEvent(event HookEvent, rawPatch json.RawMessage) json.RawMessage {
	if len(rawPatch) == 0 || !shouldPersistPatch(event, h.debugPatchAudit) {
		return nil
	}
	return cloneRawJSON(rawPatch)
}

func shouldPersistPatch(event HookEvent, debug bool) bool {
	switch event.Family() {
	case HookEventFamilyPermission, HookEventFamilyPrompt, HookEventFamilyTool, HookEventFamilyInput:
		return true
	default:
		return debug
	}
}

func cloneRawJSON(src json.RawMessage) json.RawMessage {
	if len(src) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), src...)
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

type sessionContextCarrier interface {
	hookSessionContext() SessionContext
}

func sessionIDFromPayload(payload any) string {
	carrier, ok := payload.(sessionContextCarrier)
	if !ok {
		return ""
	}
	return strings.TrimSpace(carrier.hookSessionContext().SessionID)
}

func traceStrings(trace []hookTraceEntry) []string {
	if len(trace) == 0 {
		return nil
	}

	out := make([]string, 0, len(trace))
	for _, entry := range trace {
		item := entry.Hook + ":" + string(entry.Outcome)
		if entry.Error != "" {
			item += ":" + entry.Error
		}
		out = append(out, item)
	}
	return out
}

func (h *Hooks) recordDepthViolation(ctx context.Context, event HookEvent, err error) {
	if h == nil || err == nil {
		return
	}
	h.metrics.observeDepthViolation()
	h.logger.WarnContext(
		ctx,
		"hook.dispatch.depth_exceeded",
		"event", event.String(),
		"error", err,
		"event_chain", hookEventChainStrings(currentDispatchChain(ctx), event),
	)
}

func (h *Hooks) enterDispatch(ctx context.Context, event HookEvent) (context.Context, int, error) {
	dispatchCtx, depth, err := enterDispatch(ctx, event)
	if err != nil {
		h.recordDepthViolation(ctx, event, err)
	}
	return dispatchCtx, depth, err
}

func hookEventChainStrings(chain []HookEvent, next HookEvent) []string {
	items := make([]string, 0, len(chain)+1)
	for _, event := range chain {
		items = append(items, event.String())
	}
	if next != "" {
		items = append(items, next.String())
	}
	return items
}
