package tools

import "strings"

func (e *EffectivePolicyEvaluator) applyPermissionCeiling(d Descriptor, decision *EffectiveToolDecision) {
	switch e.permissionMode() {
	case PermissionModeApproveAll:
		return
	case PermissionModeApproveEdits, PermissionModeAuto:
		if isLineageCoordinationTool(d, e.inputs.AlwaysOnLineage) || isOwnAgentMemoryWrite(d) ||
			isReadOnlyAutoApprovable(d) || isCoordinationMutation(d) {
			return
		}
		requireApproval(decision, e.inputs.ApprovalAvailable)
	case PermissionModeApproveReads:
		// Session lineage and terminal scope authorize terminal input without a
		// second permission handoff; deny-all still asks for every tool call.
		if d.ID == ToolIDTerminalWrite || isOwnAgentMemoryWrite(d) || isReadOnlyAutoApprovable(d) {
			return
		}
		requireApproval(decision, e.inputs.ApprovalAvailable)
	case PermissionModeDenyAll:
		requireApproval(decision, e.inputs.ApprovalAvailable)
	default:
		denyDecision(decision, ReasonPolicyDenied)
	}
}

// isLineageCoordinationTool lists builtin tools that act only inside the
// caller's own lineage although their declared risk would ask: delegated
// subagents, and answering a child's prompt for a stays-on lineage.
func isLineageCoordinationTool(d Descriptor, alwaysOnLineage bool) bool {
	if d.Source.Kind != SourceBuiltin {
		return false
	}
	switch d.ID {
	case ToolIDSubagentDelegate, ToolIDSubagentCancel:
		return true
	case ToolIDSessionApprove, ToolIDSessionClarifyAnswer:
		return alwaysOnLineage
	default:
		return false
	}
}

// isCoordinationMutation reports a builtin tool whose declared risk is a plain
// mutation of Compozy's own coordination and bookkeeping state: not
// destructive, not open-world, and not a change to what any agent may do.
// Extension and MCP tools never qualify; their owners' declarations are not
// trusted to skip approval.
func isCoordinationMutation(d Descriptor) bool {
	if d.Source.Kind != SourceBuiltin || d.Risk != RiskMutating || d.ReadOnly ||
		d.Destructive || d.OpenWorld || d.RequiresInteraction {
		return false
	}
	return !asksAtEditLevel(d.ID)
}

// selfEscalationToolPrefixes are builtin families that change Compozy's own
// config, approvals, hooks, extensions, or model catalog, plus Loops, whose
// authored command criteria run shell commands outside the terminal approval,
// and Automations, which persist schedules that run other agents at their own
// levels.
var selfEscalationToolPrefixes = []string{
	"compozy__config_", "compozy__tool_approvals_", "compozy__hooks_", "compozy__extensions_",
	"compozy__provider_models_", "compozy__loop_", "compozy__automation_",
}

// asksAtEditLevel lists builtin mutations that keep asking at approve-edits and
// auto whatever their declared risk: terminal input or execution, answering
// another session's prompts, reshaping lineage, authoring or running agent
// definitions or the agent a task runs, dispatching arbitrary palette commands,
// archiving sessions or workspaces, and the families above.
func asksAtEditLevel(id ToolID) bool {
	switch id {
	case ToolIDTerminalExec, ToolIDTerminalWrite, ToolIDTerminalOpen, ToolIDTerminalSignal, ToolIDTerminalClose,
		ToolIDSessionApprove, ToolIDSessionClarifyAnswer,
		ToolIDSessionReparent, ToolIDAgentCreate, ToolIDAgentInstanceCreate, ToolIDAgentInstanceEnsureMain,
		ToolIDAgentInstancePause, ToolIDAgentInstanceResume, ToolIDCmdPaletteInvoke,
		ToolIDSessionArchive, ToolIDWorkspaceArchive, ToolIDTaskExecutionProfileSet:
		return true
	}
	for _, prefix := range selfEscalationToolPrefixes {
		if strings.HasPrefix(id.String(), prefix) {
			return true
		}
	}
	return false
}

func isOwnAgentMemoryWrite(d Descriptor) bool {
	if d.Source.Kind != SourceBuiltin || d.Risk != RiskMutating || d.Destructive || d.OpenWorld ||
		d.RequiresInteraction {
		return false
	}
	switch d.ID {
	case ToolIDAgentMemoryAdd, ToolIDAgentMemoryReplace, ToolIDAgentMemoryRemove:
		return true
	default:
		return false
	}
}
