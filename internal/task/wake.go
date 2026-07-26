package task

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/compozy/agh/internal/diagnostics"
)

const wakeSummaryMaxRunes = 512
const wakeEventCacheMaxEntries = 4096

const (
	// WakeReasonTerminal wakes the creator after a run reaches a terminal state.
	WakeReasonTerminal WakeReason = "terminal"
	// WakeReasonBlocked wakes the creator after a task block is created.
	WakeReasonBlocked WakeReason = "blocked"
	// WakeReasonNeedsAttention wakes the creator after the task escalates to needs_attention.
	WakeReasonNeedsAttention WakeReason = "needs_attention"
)

const (
	wakeSuppressionSessionNotLive = "session_not_live"
	wakeSuppressionOptOut         = "wake_creator_disabled"
	wakeSuppressionSelfWake       = "self_wake"
	wakeSuppressionDeliveryFailed = "delivery_failed"
)

// WakeReason identifies the transition class that woke a task creator.
type WakeReason string

// WakeEvent is the task-domain wake payload handed to the daemon bridge.
type WakeEvent struct {
	WakeEventID string     `json:"wake_event_id"`
	WorkspaceID string     `json:"workspace_id,omitempty"`
	TaskID      string     `json:"task_id"`
	RunID       string     `json:"run_id,omitempty"`
	Reason      WakeReason `json:"reason"`
	Summary     string     `json:"summary"`
}

// WakeNotifier delivers one wake event to a creator session.
type WakeNotifier interface {
	WakeCreator(ctx context.Context, creatorSessionID string, ev WakeEvent) error
}

type noopWakeNotifier struct{}

func defaultWakeNotifier(notifier WakeNotifier) WakeNotifier {
	if notifier != nil {
		return notifier
	}
	return noopWakeNotifier{}
}

func (noopWakeNotifier) WakeCreator(context.Context, string, WakeEvent) error {
	return nil
}

type wakeDispatchRequest struct {
	taskRecord         Task
	runID              string
	executingSessionID string
	wakeEventID        string
	reason             WakeReason
	summary            string
	actor              ActorContext
}

type wakeTaskPayload struct {
	WakeEventID        string     `json:"wake_event_id"`
	WorkspaceID        string     `json:"workspace_id,omitempty"`
	Reason             WakeReason `json:"reason"`
	Status             Status     `json:"status,omitempty"`
	RunID              string     `json:"run_id,omitempty"`
	CreatorSessionID   string     `json:"creator_session_id,omitempty"`
	ExecutingSessionID string     `json:"executing_session_id,omitempty"`
	Summary            string     `json:"summary,omitempty"`
	SuppressionReason  string     `json:"suppression_reason,omitempty"`
	Error              string     `json:"error,omitempty"`
}

// Normalize returns the canonical wake reason value.
func (r WakeReason) Normalize() WakeReason {
	return WakeReason(strings.ToLower(strings.TrimSpace(string(r))))
}

// Validate reports whether the wake reason is supported.
func (r WakeReason) Validate(path string) error {
	switch r.Normalize() {
	case WakeReasonTerminal, WakeReasonBlocked, WakeReasonNeedsAttention:
		return nil
	default:
		return fmt.Errorf("%w: invalid %s: %q", ErrValidation, path, r)
	}
}

// Normalize returns a trimmed wake event with bounded redacted summary text.
func (e WakeEvent) Normalize() WakeEvent {
	return WakeEvent{
		WakeEventID: strings.TrimSpace(e.WakeEventID),
		WorkspaceID: strings.TrimSpace(e.WorkspaceID),
		TaskID:      strings.TrimSpace(e.TaskID),
		RunID:       strings.TrimSpace(e.RunID),
		Reason:      e.Reason.Normalize(),
		Summary:     boundWakeSummary(e.Summary),
	}
}

// Validate reports whether the wake event has the minimum delivery identity.
func (e WakeEvent) Validate() error {
	normalized := e.Normalize()
	if normalized.WakeEventID == "" {
		return fmt.Errorf("%w: wake_event_id is required", ErrValidation)
	}
	if normalized.TaskID == "" {
		return fmt.Errorf("%w: wake_event.task_id is required", ErrValidation)
	}
	if err := normalized.Reason.Validate("wake_event.reason"); err != nil {
		return err
	}
	if normalized.Summary == "" {
		return fmt.Errorf("%w: wake_event.summary is required", ErrValidation)
	}
	return nil
}

func (m *Service) dispatchTerminalWake(ctx context.Context, taskRecord Task, run Run, actor ActorContext) {
	status := run.Status.Normalize()
	m.dispatchWakeCreator(ctx, &wakeDispatchRequest{
		taskRecord:         taskRecord,
		runID:              run.ID,
		executingSessionID: run.SessionID,
		wakeEventID:        wakeEventID("terminal", taskRecord.ID, run.ID, status.String()),
		reason:             WakeReasonTerminal,
		summary:            terminalWakeSummary(taskRecord, run),
		actor:              actor,
	})
}

func (m *Service) dispatchBlockedWake(
	ctx context.Context,
	taskRecord Task,
	block TaskBlock,
	actor ActorContext,
	release *BlockTaskAndReleaseRunResult,
) {
	runID := ""
	executingSessionID := ""
	if release != nil {
		runID = strings.TrimSpace(release.Run.ID)
		executingSessionID = strings.TrimSpace(release.PreviousRun.SessionID)
	}
	m.dispatchWakeCreator(ctx, &wakeDispatchRequest{
		taskRecord:         taskRecord,
		runID:              runID,
		executingSessionID: executingSessionID,
		wakeEventID:        wakeEventID("blocked", taskRecord.ID, block.ID),
		reason:             WakeReasonBlocked,
		summary:            blockedWakeSummary(taskRecord, block),
		actor:              actor,
	})
}

func (m *Service) dispatchNeedsAttentionWake(
	ctx context.Context,
	taskRecord Task,
	block TaskBlock,
	reason string,
	actor ActorContext,
	release *BlockTaskAndReleaseRunResult,
) {
	runID := ""
	executingSessionID := ""
	if release != nil {
		runID = strings.TrimSpace(release.Run.ID)
		executingSessionID = strings.TrimSpace(release.PreviousRun.SessionID)
	}
	m.dispatchWakeCreator(ctx, &wakeDispatchRequest{
		taskRecord:         taskRecord,
		runID:              runID,
		executingSessionID: executingSessionID,
		wakeEventID:        wakeEventID("needs_attention", taskRecord.ID, block.ID),
		reason:             WakeReasonNeedsAttention,
		summary:            needsAttentionWakeSummary(taskRecord, reason),
		actor:              actor,
	})
}

func (m *Service) dispatchWakeCreator(ctx context.Context, req *wakeDispatchRequest) {
	if m == nil || req == nil {
		return
	}
	taskRecord := req.taskRecord
	if taskRecord.CreatedBy.Kind.Normalize() != ActorKindAgentSession {
		return
	}
	creatorSessionID := strings.TrimSpace(taskRecord.CreatedBy.Ref)
	event := WakeEvent{
		WakeEventID: strings.TrimSpace(req.wakeEventID),
		WorkspaceID: strings.TrimSpace(taskRecord.WorkspaceID),
		TaskID:      strings.TrimSpace(taskRecord.ID),
		RunID:       strings.TrimSpace(req.runID),
		Reason:      req.reason,
		Summary:     req.summary,
	}.Normalize()
	if err := event.Validate(); err != nil {
		m.logWakeDispatchError("task: invalid wake event", err, event, taskRecord.ID)
		return
	}
	reserved, err := m.reserveWakeEvent(ctx, event.TaskID, event.WakeEventID)
	if err != nil {
		m.logWakeDispatchError("task: wake dedupe check failed", err, event, taskRecord.ID)
		return
	}
	if !reserved {
		return
	}
	if !taskRecord.WakeCreator {
		m.recordWakeSuppressed(ctx, taskRecord, event, req, wakeSuppressionOptOut, nil)
		return
	}
	if creatorSessionID == "" {
		m.recordWakeSuppressed(ctx, taskRecord, event, req, wakeSuppressionSessionNotLive, nil)
		return
	}
	if strings.TrimSpace(req.executingSessionID) == creatorSessionID {
		m.recordWakeSuppressed(ctx, taskRecord, event, req, wakeSuppressionSelfWake, nil)
		return
	}
	notifier := m.wakeNotifier
	if notifier == nil {
		notifier = noopWakeNotifier{}
	}
	if err := notifier.WakeCreator(ctx, creatorSessionID, event); err != nil {
		if errors.Is(err, ErrSessionNotLive) {
			m.recordWakeSuppressed(ctx, taskRecord, event, req, wakeSuppressionSessionNotLive, nil)
			return
		}
		m.recordWakeSuppressed(ctx, taskRecord, event, req, wakeSuppressionDeliveryFailed, err)
		return
	}
	m.recordWakeDelivered(ctx, taskRecord, event, req)
}

func (m *Service) reserveWakeEvent(ctx context.Context, taskID string, wakeEventID string) (bool, error) {
	key := wakeEventKey(taskID, wakeEventID)
	if key == "" {
		return false, fmt.Errorf("%w: wake event key is required", ErrValidation)
	}
	m.wakeMu.Lock()
	if _, ok := m.wakeEventIDs[key]; ok {
		m.wakeMu.Unlock()
		return false, nil
	}
	m.wakeMu.Unlock()

	recorded, err := m.hasWakeAuditEvent(ctx, taskID, wakeEventID)
	if err != nil {
		return false, err
	}

	m.wakeMu.Lock()
	defer m.wakeMu.Unlock()
	if _, ok := m.wakeEventIDs[key]; ok {
		return false, nil
	}
	if recorded {
		m.rememberWakeEventKeyLocked(key)
		return false, nil
	}
	m.rememberWakeEventKeyLocked(key)
	return true, nil
}

func (m *Service) rememberWakeEventKeyLocked(key string) {
	if strings.TrimSpace(key) == "" {
		return
	}
	if _, ok := m.wakeEventIDs[key]; ok {
		return
	}
	for len(m.wakeEventIDs) >= wakeEventCacheMaxEntries && len(m.wakeEventOrder) > 0 {
		evicted := m.wakeEventOrder[0]
		delete(m.wakeEventIDs, evicted)
		copy(m.wakeEventOrder, m.wakeEventOrder[1:])
		m.wakeEventOrder[len(m.wakeEventOrder)-1] = ""
		m.wakeEventOrder = m.wakeEventOrder[:len(m.wakeEventOrder)-1]
	}
	if len(m.wakeEventIDs) >= wakeEventCacheMaxEntries {
		clear(m.wakeEventIDs)
		m.wakeEventOrder = m.wakeEventOrder[:0]
	}
	m.wakeEventIDs[key] = struct{}{}
	m.wakeEventOrder = append(m.wakeEventOrder, key)
}

func (m *Service) hasWakeAuditEvent(ctx context.Context, taskID string, wakeEventID string) (bool, error) {
	return m.store.TaskWakeEventExists(ctx, taskID, wakeEventID)
}

func (m *Service) recordWakeDelivered(
	ctx context.Context,
	taskRecord Task,
	event WakeEvent,
	req *wakeDispatchRequest,
) {
	if err := m.recordTaskEvent(
		ctx,
		event.TaskID,
		event.RunID,
		taskEventWakeDelivered,
		req.actor,
		wakePayload(taskRecord, event, req, "", nil),
	); err != nil {
		m.logWakeDispatchError("task: wake delivered audit failed", err, event, taskRecord.ID)
	}
}

func (m *Service) recordWakeSuppressed(
	ctx context.Context,
	taskRecord Task,
	event WakeEvent,
	req *wakeDispatchRequest,
	reason string,
	deliveryErr error,
) {
	if err := m.recordTaskEvent(
		ctx,
		event.TaskID,
		event.RunID,
		taskEventWakeSuppressed,
		req.actor,
		wakePayload(taskRecord, event, req, reason, deliveryErr),
	); err != nil {
		m.logWakeDispatchError("task: wake suppressed audit failed", err, event, taskRecord.ID)
	}
}

func wakePayload(
	taskRecord Task,
	event WakeEvent,
	req *wakeDispatchRequest,
	suppressionReason string,
	deliveryErr error,
) wakeTaskPayload {
	payload := wakeTaskPayload{
		WakeEventID:        event.WakeEventID,
		WorkspaceID:        event.WorkspaceID,
		Reason:             event.Reason,
		Status:             taskRecord.Status.Normalize(),
		RunID:              event.RunID,
		CreatorSessionID:   strings.TrimSpace(taskRecord.CreatedBy.Ref),
		ExecutingSessionID: strings.TrimSpace(req.executingSessionID),
		Summary:            event.Summary,
		SuppressionReason:  strings.TrimSpace(suppressionReason),
	}
	if deliveryErr != nil {
		payload.Error = boundWakeSummary(deliveryErr.Error())
	}
	return payload
}

func (m *Service) logWakeDispatchError(message string, err error, event WakeEvent, taskID string) {
	if err == nil {
		return
	}
	slog.Error(
		message,
		"error", err,
		"task_id", strings.TrimSpace(taskID),
		"run_id", strings.TrimSpace(event.RunID),
		"wake_event_id", strings.TrimSpace(event.WakeEventID),
		"reason", event.Reason.Normalize(),
	)
}

func terminalWakeSummary(taskRecord Task, run Run) string {
	label := wakeTaskLabel(taskRecord)
	switch run.Status.Normalize() {
	case TaskRunStatusCompleted:
		return boundWakeSummary(fmt.Sprintf("Task %s completed.", label))
	case TaskRunStatusFailed:
		if !isTerminalTaskStatus(taskRecord.Status.Normalize()) {
			if errText := strings.TrimSpace(run.Error); errText != "" {
				return boundWakeSummary(fmt.Sprintf("Run for task %s failed: %s", label, errText))
			}
			return boundWakeSummary(fmt.Sprintf("Run for task %s failed.", label))
		}
		if errText := strings.TrimSpace(run.Error); errText != "" {
			return boundWakeSummary(fmt.Sprintf("Task %s failed: %s", label, errText))
		}
		return boundWakeSummary(fmt.Sprintf("Task %s failed.", label))
	case TaskRunStatusCanceled:
		if !isTerminalTaskStatus(taskRecord.Status.Normalize()) {
			return boundWakeSummary(fmt.Sprintf("Run for task %s was canceled.", label))
		}
		return boundWakeSummary(fmt.Sprintf("Task %s was canceled.", label))
	default:
		return boundWakeSummary(fmt.Sprintf("Task %s reached %s.", label, run.Status.Normalize()))
	}
}

func blockedWakeSummary(taskRecord Task, block TaskBlock) string {
	label := wakeTaskLabel(taskRecord)
	reason := strings.TrimSpace(block.Reason)
	if reason == "" {
		return boundWakeSummary(fmt.Sprintf("Task %s is blocked.", label))
	}
	return boundWakeSummary(fmt.Sprintf("Task %s is blocked: %s", label, reason))
}

func needsAttentionWakeSummary(taskRecord Task, reason string) string {
	label := wakeTaskLabel(taskRecord)
	trimmedReason := strings.TrimSpace(reason)
	if trimmedReason == "" {
		return boundWakeSummary(fmt.Sprintf("Task %s needs attention.", label))
	}
	return boundWakeSummary(fmt.Sprintf("Task %s needs attention: %s", label, trimmedReason))
}

func wakeTaskLabel(taskRecord Task) string {
	title := strings.TrimSpace(taskRecord.Title)
	if title != "" {
		return fmt.Sprintf("%q", title)
	}
	id := strings.TrimSpace(taskRecord.ID)
	if id != "" {
		return id
	}
	return "task"
}

func wakeEventID(parts ...string) string {
	cleaned := make([]string, 0, len(parts)+1)
	cleaned = append(cleaned, "wake")
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		trimmed = strings.ReplaceAll(trimmed, " ", "_")
		cleaned = append(cleaned, trimmed)
	}
	return strings.Join(cleaned, "-")
}

func wakeEventKey(taskID string, wakeEventID string) string {
	trimmedTaskID := strings.TrimSpace(taskID)
	trimmedWakeEventID := strings.TrimSpace(wakeEventID)
	if trimmedTaskID == "" || trimmedWakeEventID == "" {
		return ""
	}
	return trimmedTaskID + "/" + trimmedWakeEventID
}

func boundWakeSummary(value string) string {
	redacted := strings.TrimSpace(diagnostics.Redact(RedactClaimTokens(value)))
	if redacted == "" {
		return ""
	}
	runes := []rune(redacted)
	if len(runes) <= wakeSummaryMaxRunes {
		return redacted
	}
	if wakeSummaryMaxRunes <= 3 {
		return string(runes[:wakeSummaryMaxRunes])
	}
	return string(runes[:wakeSummaryMaxRunes-3]) + "..."
}
