package acp

import (
	"cmp"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/compozy/internal/redact"
)

// claudeSDKMessageMethod is claude-agent-acp's raw SDK message extension
// notification. Its params are {sessionId, message}; the adapter forwards only
// messages matching the session's emitRawSDKMessages filters.
const claudeSDKMessageMethod = "_claude/sdkMessage"

// The SDK system message subtypes that carry the lifecycle of a provider task,
// including Agent/Task subagents. Nothing else is requested.
const (
	claudeTaskStarted      = "task_started"
	claudeTaskProgress     = "task_progress"
	claudeTaskUpdated      = "task_updated"
	claudeTaskNotification = "task_notification"
	claudeSDKSystemType    = "system"
)

var claudeTaskSubtypes = [...]string{claudeTaskStarted, claudeTaskProgress, claudeTaskUpdated, claudeTaskNotification}

// claudeTaskTrackLimit bounds the per-process task correlation state.
const claudeTaskTrackLimit = 256

// claudeTaskIDLimit bounds provider identifiers accepted from the wire.
const claudeTaskIDLimit = 256

// claudeTaskRawSDKFilters is the `_meta.claudeCode.emitRawSDKMessages` value.
func claudeTaskRawSDKFilters() []any {
	filters := make([]any, 0, len(claudeTaskSubtypes))
	for _, subtype := range claudeTaskSubtypes {
		filters = append(filters, map[string]any{promptCacheTypeKey: claudeSDKSystemType, "subtype": subtype})
	}
	return filters
}

type wireClaudeSDKMessage struct {
	SessionID acpsdk.SessionId `json:"sessionId"`
	Message   json.RawMessage  `json:"message"`
}

type wireClaudeTaskUsage struct {
	TotalTokens int64 `json:"total_tokens"`
	ToolUses    int64 `json:"tool_uses"`
	DurationMs  int64 `json:"duration_ms"`
}

type wireClaudeTaskPatch struct {
	Status         string `json:"status"`
	Error          string `json:"error"`
	Description    string `json:"description"`
	IsBackgrounded *bool  `json:"is_backgrounded"`
}

// wireClaudeTaskMessage is the union of SDKTaskStartedMessage,
// SDKTaskProgressMessage, SDKTaskUpdatedMessage, and SDKTaskNotificationMessage.
type wireClaudeTaskMessage struct {
	Type           string               `json:"type"`
	Subtype        string               `json:"subtype"`
	TaskID         string               `json:"task_id"`
	RunID          string               `json:"run_id"`
	ToolUseID      string               `json:"tool_use_id"`
	Description    string               `json:"description"`
	SubagentType   string               `json:"subagent_type"`
	TaskType       string               `json:"task_type"`
	IsBackgrounded *bool                `json:"is_backgrounded"`
	Status         string               `json:"status"`
	Summary        *string              `json:"summary"`
	LastToolName   string               `json:"last_tool_name"`
	Usage          *wireClaudeTaskUsage `json:"usage"`
	Patch          *wireClaudeTaskPatch `json:"patch"`
	UUID           string               `json:"uuid"`
}

// claudeTaskTracker correlates SDK task ids and runs to the Agent/Task tool call
// that launched them. It is per process: a replaced process starts empty, so an
// old process's task can never bind to a newer turn.
type claudeTaskTracker struct {
	enabled atomic.Bool
	mu      sync.Mutex
	tasks   map[string]*claudeTrackedTask
	order   []string
}

type claudeTrackedTask struct {
	runID     string
	toolUseID string
	agent     bool
}

// handleClaudeSDKMessage decodes the four requested task subtypes into typed
// native-task events. The caller already rejected non-bound sessions.
func (p *AgentProcess) handleClaudeSDKMessage(params json.RawMessage) error {
	if !p.claudeTasks.enabled.Load() {
		return nil
	}
	var notification wireClaudeSDKMessage
	if err := json.Unmarshal(params, &notification); err != nil {
		return fmt.Errorf("acp: decode %s: %w", claudeSDKMessageMethod, err)
	}
	var message wireClaudeTaskMessage
	if err := json.Unmarshal(notification.Message, &message); err != nil {
		p.logClaudeTaskDrop(notification.SessionID, "", "malformed")
		return nil
	}
	observation, reason := p.claudeTasks.observe(message)
	if reason != "" {
		p.logClaudeTaskDrop(notification.SessionID, message.TaskID, reason)
		return nil
	}
	p.emitPromptEvent(AgentEvent{
		Type:       EventTypeNativeTask,
		SessionID:  string(notification.SessionID),
		TurnID:     p.activeTurnID(),
		Timestamp:  timeNowUTC(),
		ToolCallID: observation.ToolCallID,
	}.WithNativeTask(observation))
	return nil
}

func (p *AgentProcess) logClaudeTaskDrop(sessionID acpsdk.SessionId, taskID, reason string) {
	if p.logger == nil {
		return
	}
	p.logger.Debug(
		"acp.claude_task.dropped",
		"session_id", sessionID,
		"task_id", redact.ClaimTokens(taskID),
		"reason", reason,
	)
}

// observe returns the normalized observation or the reason it was dropped.
func (t *claudeTaskTracker) observe(message wireClaudeTaskMessage) (*NativeTaskObservation, string) {
	if message.Type != claudeSDKSystemType || !validClaudeTaskID(message.TaskID) ||
		len(message.RunID) > claudeTaskIDLimit || len(message.ToolUseID) > claudeTaskIDLimit {
		return nil, "malformed"
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	tracked := t.tasks[message.TaskID]
	// Raw forwarding precedes the adapter's own stale-run filter: run ids of one
	// task sort in run order, and an older run must never settle a newer one.
	if tracked != nil && message.RunID != "" && tracked.runID != "" && message.RunID < tracked.runID {
		return nil, "stale_run"
	}
	if message.Subtype == claudeTaskStarted {
		tracked = &claudeTrackedTask{
			runID:     message.RunID,
			toolUseID: message.ToolUseID,
			agent:     message.SubagentType != "" || message.TaskType == "local_agent",
		}
		t.track(message.TaskID, tracked)
		if !tracked.agent {
			return nil, "not_agent"
		}
	} else {
		var reason string
		if tracked, reason = t.continueRun(tracked, message); reason != "" {
			return nil, reason
		}
	}
	observation := &NativeTaskObservation{
		Source:     NativeTaskSourceClaudeSDK,
		TaskID:     message.TaskID,
		RunID:      message.RunID,
		ToolCallID: cmp.Or(message.ToolUseID, tracked.toolUseID),
		EventUUID:  message.UUID,
	}
	if observation.ToolCallID == "" {
		return nil, "uncorrelated"
	}
	return observation, fillClaudeTaskObservation(observation, message)
}

// continueRun resolves the tracked task for a non-start subtype. A frame of a
// newer run rebinds the task to that run's explicit tool call, if any.
func (t *claudeTaskTracker) continueRun(
	tracked *claudeTrackedTask,
	message wireClaudeTaskMessage,
) (*claudeTrackedTask, string) {
	if tracked == nil {
		// task_updated never names its tool call; progress names the subagent
		// type and notifications name the call, which session ingest verifies.
		if message.Subtype == claudeTaskUpdated || message.ToolUseID == "" ||
			message.Subtype == claudeTaskProgress && message.SubagentType == "" {
			return nil, "unknown_task"
		}
		tracked = &claudeTrackedTask{runID: message.RunID, toolUseID: message.ToolUseID, agent: true}
		t.track(message.TaskID, tracked)
	}
	if !tracked.agent {
		return nil, "not_agent"
	}
	if message.RunID != "" && message.RunID != tracked.runID {
		tracked.runID = message.RunID
		tracked.toolUseID = message.ToolUseID
	} else if message.ToolUseID != "" {
		tracked.toolUseID = message.ToolUseID
	}
	return tracked, ""
}

func fillClaudeTaskObservation(observation *NativeTaskObservation, message wireClaudeTaskMessage) string {
	switch message.Subtype {
	case claudeTaskStarted:
		observation.Kind = NativeTaskKindSpawn
		observation.Description = message.Description
		observation.Background = message.IsBackgrounded
	case claudeTaskProgress:
		observation.Kind = NativeTaskKindProgress
		observation.Description = message.Description
		observation.Summary = message.Summary
		observation.LastToolName = message.LastToolName
		observation.Usage = nativeTaskUsage(message.Usage)
	case claudeTaskUpdated:
		if message.Patch == nil || !validClaudePatchStatus(message.Patch.Status) ||
			message.Patch.Status == "" && message.Patch.IsBackgrounded == nil {
			return "no_lifecycle_patch"
		}
		observation.Kind = NativeTaskKindState
		observation.Status = message.Patch.Status
		observation.Error = message.Patch.Error
		observation.Description = message.Patch.Description
		observation.Background = message.Patch.IsBackgrounded
	case claudeTaskNotification:
		switch message.Status {
		case NativeTaskStatusCompleted, NativeTaskStatusFailed, NativeTaskStatusStopped:
		default:
			return "malformed"
		}
		observation.Kind = NativeTaskKindState
		observation.Status = message.Status
		observation.Summary = message.Summary
		observation.Usage = nativeTaskUsage(message.Usage)
	default:
		return "unrequested_subtype"
	}
	return ""
}

func validClaudePatchStatus(status string) bool {
	switch status {
	case "", NativeTaskStatusPending, NativeTaskStatusRunning, NativeTaskStatusPaused,
		NativeTaskStatusCompleted, NativeTaskStatusFailed, NativeTaskStatusKilled:
		return true
	default:
		return false
	}
}

func validClaudeTaskID(id string) bool {
	return id != "" && len(id) <= claudeTaskIDLimit
}

func nativeTaskUsage(usage *wireClaudeTaskUsage) *NativeTaskUsage {
	if usage == nil {
		return nil
	}
	return &NativeTaskUsage{TotalTokens: usage.TotalTokens, ToolUses: usage.ToolUses, DurationMs: usage.DurationMs}
}

// track records a task, evicting the oldest one beyond the bound. Callers hold mu.
func (t *claudeTaskTracker) track(taskID string, task *claudeTrackedTask) {
	if t.tasks == nil {
		t.tasks = make(map[string]*claudeTrackedTask)
	}
	if _, exists := t.tasks[taskID]; !exists {
		t.order = append(t.order, taskID)
	}
	t.tasks[taskID] = task
	for len(t.order) > claudeTaskTrackLimit {
		delete(t.tasks, t.order[0])
		t.order = t.order[1:]
	}
}
