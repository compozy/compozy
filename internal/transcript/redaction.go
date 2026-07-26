package transcript

import (
	"encoding/json"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/diagnostics"
	redactpkg "github.com/compozy/agh/internal/redact"
	"github.com/compozy/agh/internal/store"
)

const transcriptStdoutFieldKey = "stdout"

var structuralRedactor = redactpkg.New(redactpkg.Options{Disabled: true})

// RedactAgentEvent removes displayable secret material before an ACP event is
// stored, replayed, or streamed to a caller.
func RedactAgentEvent(event acp.AgentEvent) acp.AgentEvent {
	redacted := event
	redacted.Text = redactDisplayString(event.Text)
	redacted.Title = redactDisplayString(event.Title)
	redacted.ToolCallID = redactStructuralString(event.ToolCallID)
	redacted = redacted.WithToolDetail(
		redactStructuralString(event.ToolName()),
		redactRawMessage(event.ToolInput()),
		event.ToolError(),
		redactDisplayString(event.ToolErrorDetail()),
	).WithToolKind(redactStructuralString(event.ToolKind()))
	redacted.StopReason = redactStructuralString(event.StopReason)
	redacted.PromptStopReason = acp.PromptStopReason(redactStructuralString(string(event.PromptStopReason)))
	redacted.Action = redactStructuralString(event.Action)
	redacted.Resource = redactStructuralString(event.Resource)
	redacted.Decision = redactStructuralString(event.Decision)
	redacted.Error = redactDisplayString(event.Error)
	redacted.Failure = redactSessionFailure(event.Failure)
	redacted.Synthetic = redactPromptSyntheticMeta(event.Synthetic)
	redacted.Goal = redactGoalPromptMeta(event.Goal)
	if event.AvailableCommands != nil {
		redacted.AvailableCommands = acp.NewAvailableCommandSet(event.AvailableCommands.Values())
	}
	redacted.Runtime = redactRuntimeActivity(event.Runtime)
	redacted.Raw = redactRawMessage(event.Raw)
	return redacted
}

func redactCanonicalPayload(payload *canonicalEventPayload) {
	if payload == nil {
		return
	}
	payload.Text = redactDisplayString(payload.Text)
	payload.AuthoredText = redactDisplayString(payload.AuthoredText)
	payload.Title = redactDisplayString(payload.Title)
	payload.ToolName = redactStructuralString(payload.ToolName)
	payload.ToolCallID = redactStructuralString(payload.ToolCallID)
	payload.ToolInput = redactRawMessage(payload.ToolInput)
	payload.ToolResult = redactTranscriptToolResult(payload.ToolResult)
	payload.StopReason = redactStructuralString(payload.StopReason)
	payload.PromptStopReason = acp.PromptStopReason(redactStructuralString(string(payload.PromptStopReason)))
	payload.Action = redactStructuralString(payload.Action)
	payload.Resource = redactStructuralString(payload.Resource)
	payload.Decision = redactStructuralString(payload.Decision)
	payload.Error = redactDisplayString(payload.Error)
	payload.Failure = redactSessionFailure(payload.Failure)
	payload.Synthetic = redactPromptSyntheticMeta(payload.Synthetic)
	payload.Goal = redactGoalPromptMeta(payload.Goal)
	payload.Runtime = redactRuntimeActivity(payload.Runtime)
	payload.Raw = redactRawMessage(payload.Raw)
}

func redactTranscriptEvent(parsed event) event {
	parsed.Text = redactDisplayString(parsed.Text)
	parsed.StopReason = redactStructuralString(parsed.StopReason)
	parsed.Error = redactDisplayString(parsed.Error)
	parsed.Failure = redactSessionFailure(parsed.Failure)
	parsed.Runtime = redactRuntimeActivity(parsed.Runtime)
	parsed.Marker = redactMarker(parsed.Marker)
	parsed.ToolCallID = redactStructuralString(parsed.ToolCallID)
	parsed.ToolName = redactStructuralString(parsed.ToolName)
	parsed.ToolInput = redactRawMessage(parsed.ToolInput)
	parsed.ToolResult = redactTranscriptToolResult(parsed.ToolResult)
	return parsed
}

func redactMarker(marker *Marker) *Marker {
	if marker == nil {
		return nil
	}
	redacted := marker.Normalize()
	return &redacted
}

func redactTranscriptToolResult(result *ToolResult) *ToolResult {
	if result == nil {
		return nil
	}
	redacted := cloneToolResult(result)
	redacted.Stdout = redactDisplayString(redacted.Stdout)
	redacted.Stderr = redactDisplayString(redacted.Stderr)
	redacted.FilePath = redactStructuralString(redacted.FilePath)
	redacted.Content = redactDisplayString(redacted.Content)
	redacted.StructuredPatch = redactRawMessage(redacted.StructuredPatch)
	redacted.Error = redactDisplayString(redacted.Error)
	redacted.RawOutput = redactRawMessage(redacted.RawOutput)
	return redacted
}

func redactSessionFailure(failure *store.SessionFailure) *store.SessionFailure {
	if failure == nil {
		return nil
	}
	redacted := failure.Normalize()
	redacted.Summary = redactDisplayString(redacted.Summary)
	redacted.CrashBundlePath = redactStructuralString(redacted.CrashBundlePath)
	return &redacted
}

func redactPromptSyntheticMeta(meta *acp.PromptSyntheticMeta) *acp.PromptSyntheticMeta {
	if meta == nil {
		return nil
	}
	redacted := meta.Normalize()
	redacted.TaskID = redactStructuralString(redacted.TaskID)
	redacted.TaskRunID = redactStructuralString(redacted.TaskRunID)
	redacted.WorkflowID = redactStructuralString(redacted.WorkflowID)
	redacted.CoordinatorSessionID = redactStructuralString(redacted.CoordinatorSessionID)
	redacted.Reason = redactDisplayString(redacted.Reason)
	redacted.Summary = redactDisplayString(redacted.Summary)
	redacted.WakeEventID = redactStructuralString(redacted.WakeEventID)
	redacted.Goal = redactGoalPromptMeta(redacted.Goal)
	return &redacted
}

func redactGoalPromptMeta(meta *acp.GoalPromptMeta) *acp.GoalPromptMeta {
	redacted := acp.CloneGoalPromptMeta(meta)
	if redacted == nil {
		return nil
	}
	redacted.RunID = redactStructuralString(redacted.RunID)
	redacted.NodeID = redactStructuralString(redacted.NodeID)
	redacted.PromptID = redactStructuralString(redacted.PromptID)
	return redacted
}

func redactRuntimeActivity(activity *acp.RuntimeActivity) *acp.RuntimeActivity {
	redacted := cloneRuntimeActivity(activity)
	if redacted == nil {
		return nil
	}
	redacted.TurnID = redactStructuralString(redacted.TurnID)
	redacted.TurnSource = redactStructuralString(redacted.TurnSource)
	redacted.LastActivityKind = redactStructuralString(redacted.LastActivityKind)
	redacted.LastActivityDetail = redactDisplayString(redacted.LastActivityDetail)
	redacted.CurrentTool = redactStructuralString(redacted.CurrentTool)
	redacted.ToolCallID = redactStructuralString(redacted.ToolCallID)
	return redacted
}

func redactRawMessage(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	if json.Valid(raw) {
		engine := redactpkg.New(redactpkg.Options{Disabled: !redactpkg.Enabled()})
		return acp.CloneRawMessage(engine.RedactJSON(raw, displayJSONFields))
	}
	redacted := diagnostics.Redact(string(raw))
	if json.Valid([]byte(redacted)) {
		return acp.CloneRawMessage(json.RawMessage(redacted))
	}
	return rawMessageFromValue(redacted)
}

var displayJSONFields = []string{
	"", "authored_text", "body", "command", "content", "description", "detail", "error",
	"message", "output", "payload", "raw_input", "raw_output", "reason",
	"result", "stderr", transcriptStdoutFieldKey, "summary", "text", "title", "tool_input",
	"tool_result",
}

func redactDisplayString(value string) string {
	return diagnostics.Redact(value)
}

func redactStructuralString(value string) string {
	return structuralRedactor.RedactString(value)
}
