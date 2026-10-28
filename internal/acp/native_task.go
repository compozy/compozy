package acp

import "unicode/utf8"

// EventTypeNativeTask is one provider-native task lifecycle observation for a
// Claude Agent/Task subagent. It never creates a subagent row or a tool result:
// session ingest joins it to the native row of its originating Agent/Task call.
const EventTypeNativeTask = "native_task"

// NativeTaskSourceClaudeSDK marks observations decoded from claude-agent-acp's
// filtered raw SDK task messages (`_claude/sdkMessage`).
const NativeTaskSourceClaudeSDK = "claude_sdk"

// NativeTaskKind is what one observation reports about a provider task.
type NativeTaskKind string

const (
	// NativeTaskKindSpawn reports a started task run.
	NativeTaskKindSpawn NativeTaskKind = "spawn"
	// NativeTaskKindProgress reports a progress line or usage for a running task.
	NativeTaskKindProgress NativeTaskKind = "progress"
	// NativeTaskKindState reports a status patch or a settled-task notification.
	NativeTaskKindState NativeTaskKind = "state"
)

// Provider task statuses carried by NativeTaskObservation.Status. SDK patches
// report killed; settled-task notifications report stopped.
const (
	NativeTaskStatusPending   = "pending"
	NativeTaskStatusRunning   = "running"
	NativeTaskStatusPaused    = "paused"
	NativeTaskStatusCompleted = "completed"
	NativeTaskStatusFailed    = "failed"
	NativeTaskStatusKilled    = "killed"
	NativeTaskStatusStopped   = "stopped"
)

// Field bounds applied to provider task payloads before admission. The summary
// bound is a byte safety cap above the default subagent result allowance; the
// configured result limit decides the delivered result, and a cut here is
// recorded as SummaryTruncated so result_truncated stays truthful.
const (
	NativeTaskSummaryMaxBytes = 1 << 20
	NativeTaskTextMaxBytes    = 64 << 10
	NativeTaskNameMaxBytes    = 256
)

// NativeTaskUsage is the provider's running usage for one task. It is
// informational: the parent's billed usage already includes it.
type NativeTaskUsage struct {
	TotalTokens int64
	ToolUses    int64
	DurationMs  int64
}

// NativeTaskObservation is the typed, immutable payload of EventTypeNativeTask.
// The owning AgentEvent carries the session and turn; ToolCallID is the
// Agent/Task tool call that launched the task, never a Compozy subagent id.
type NativeTaskObservation struct {
	Source     string
	Kind       NativeTaskKind
	TaskID     string
	RunID      string
	ToolCallID string
	// Status is the provider status, empty when the observation carries none.
	Status string
	// Summary is nil when absent, distinguishing it from an explicit empty summary.
	Summary *string
	// SummaryTruncated records that Summary was cut before storage.
	SummaryTruncated bool
	Error            string
	Description      string
	LastToolName     string
	Usage            *NativeTaskUsage
	// Background is nil when the observation says nothing about backgrounding.
	Background *bool
	EventUUID  string
}

// Clone returns an isolated copy of the observation.
func (o *NativeTaskObservation) Clone() *NativeTaskObservation {
	if o == nil {
		return nil
	}
	cloned := *o
	if o.Summary != nil {
		cloned.Summary = new(*o.Summary)
	}
	if o.Usage != nil {
		usage := *o.Usage
		cloned.Usage = &usage
	}
	if o.Background != nil {
		cloned.Background = new(*o.Background)
	}
	return &cloned
}

// Terminal reports whether the observation settles its task run.
func (o *NativeTaskObservation) Terminal() bool {
	if o == nil || o.Kind != NativeTaskKindState {
		return false
	}
	switch o.Status {
	case NativeTaskStatusCompleted, NativeTaskStatusFailed, NativeTaskStatusKilled, NativeTaskStatusStopped:
		return true
	default:
		return false
	}
}

// WithNativeTask returns an event carrying an isolated native task observation.
func (e AgentEvent) WithNativeTask(observation *NativeTaskObservation) AgentEvent {
	payload := e.clonePayload()
	payload.nativeTask = observation.Clone()
	e.payload = normalizeAgentEventPayload(payload)
	return e
}

// NativeTask returns an isolated copy of the event's native task observation.
func (e AgentEvent) NativeTask() *NativeTaskObservation {
	if e.payload == nil {
		return nil
	}
	return e.payload.nativeTask.Clone()
}

// bound enforces the field bounds before admission. A cut summary records
// SummaryTruncated; the other fields are display text the codec bounds again.
func (o *NativeTaskObservation) bound() {
	if o == nil {
		return
	}
	if o.Summary != nil {
		if bounded, cut := truncateNativeTaskText(*o.Summary, NativeTaskSummaryMaxBytes); cut {
			o.Summary = &bounded
			o.SummaryTruncated = true
		}
	}
	o.Error, _ = truncateNativeTaskText(o.Error, NativeTaskTextMaxBytes)
	o.Description, _ = truncateNativeTaskText(o.Description, NativeTaskTextMaxBytes)
	o.LastToolName, _ = truncateNativeTaskText(o.LastToolName, NativeTaskNameMaxBytes)
	o.EventUUID, _ = truncateNativeTaskText(o.EventUUID, NativeTaskNameMaxBytes)
}

// payloadBytes is the retained size of the observation's variable fields.
func (o *NativeTaskObservation) payloadBytes() int {
	if o == nil {
		return 0
	}
	size := len(o.Source) + len(o.Kind) + len(o.TaskID) + len(o.RunID) + len(o.ToolCallID) + len(o.Status) +
		len(o.Error) + len(o.Description) + len(o.LastToolName) + len(o.EventUUID)
	if o.Summary != nil {
		size += len(*o.Summary)
	}
	return size
}

// truncateNativeTaskText returns the longest valid prefix of value within
// limit bytes and whether anything was cut.
func truncateNativeTaskText(value string, limit int) (string, bool) {
	if len(value) <= limit {
		return value, false
	}
	end := limit
	for end > 0 && !utf8.RuneStart(value[end]) {
		end--
	}
	return value[:end], true
}
