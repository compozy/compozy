package builtin

import (
	"encoding/json"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

const subagentDelegateDescription = "Delegate one self-contained task to a subagent: a new agent session on any available provider/model that sees only `task`, not this conversation. Prefer mode=\"async\" for long work: when the subagent finishes, Compozy wakes this session with a message (steered into the running turn when supported, otherwise queued), so end your turn instead of polling or waiting in loops; call compozy__subagent_status only when you need the result mid-turn. For another review round, delegate again with the original brief, prior findings, and open objections — do not prompt the old child session. A subagent waiting for approval does not wake you: the operator approves it, or you call compozy__session_approve on child_session_id after compozy__subagent_status reports status \"waiting\"."

func subagentDescriptors() []toolspkg.Descriptor {
	return []toolspkg.Descriptor{
		subagentDescriptor(
			toolspkg.ToolIDSubagentCapabilities,
			"subagent_capabilities",
			"Subagent Capabilities",
			"Discover inherited runtime settings and available agents, providers, models and "+
				"constraints before delegating. Read-only and idempotent; requires an active caller turn.",
			emptyInputSchema,
			subagentCapabilitiesOutputSchema,
			toolspkg.RiskRead,
			true,
			false,
			false,
		),
		subagentDescriptor(
			toolspkg.ToolIDSubagentDelegate,
			"subagent_delegate",
			"Delegate Subagent",
			subagentDelegateDescription,
			subagentDelegateInputSchema,
			subagentOutputSchema,
			toolspkg.RiskDestructive,
			false,
			true,
			true,
		),
		subagentDescriptor(
			toolspkg.ToolIDSubagentStatus,
			"subagent_status",
			"Subagent Status",
			"Read one subagent owned by this session. Reading a terminal result acknowledges its "+
				"delivery (idempotent, not read-only). End your turn instead of polling: completion wakes "+
				"this session. Use session_history on child_session_id for the full result.",
			subagentStatusInputSchema,
			subagentOutputSchema,
			toolspkg.RiskMutating,
			false,
			false,
			false,
		),
		subagentDescriptor(
			toolspkg.ToolIDSubagentCancel,
			"subagent_cancel",
			"Cancel Subagent",
			"Cancel a delegated subagent and its descendants when the task is no longer needed. "+
				"Provider-native subagents cannot be canceled individually; stop the parent turn instead.",
			subagentCancelInputSchema,
			subagentCancelOutputSchema,
			toolspkg.RiskDestructive,
			false,
			true,
			false,
		),
	}
}

func subagentDescriptor(
	id toolspkg.ToolID,
	name, title, description, input, output string,
	risk toolspkg.RiskClass,
	readOnly, destructive, openWorld bool,
) toolspkg.Descriptor {
	descriptor := nativeDescriptor(
		id,
		name,
		title,
		description,
		input,
		risk,
		readOnly,
		destructive,
		openWorld,
		[]toolspkg.ToolsetID{
			toolspkg.ToolsetIDSessions,
		},
		[]string{"sessions", "subagent"},
		[]string{"delegate", "subagent"},
	)
	descriptor.InputErrorCode = toolspkg.ErrorCodeInvalidRequest
	descriptor.Idempotent = id == toolspkg.ToolIDSubagentCapabilities || id == toolspkg.ToolIDSubagentStatus
	descriptor.OutputSchema = json.RawMessage(output)
	return descriptor
}
