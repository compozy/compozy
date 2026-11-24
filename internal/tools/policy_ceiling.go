package tools

import (
	"encoding/json"
	"strings"
)

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
// subagents, and answering a child's prompt for an always-on lineage.
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
		ToolIDSessionArchive, ToolIDSessionArchiveStopped, ToolIDWorkspaceArchive, ToolIDTaskExecutionProfileSet,
		ToolIDMCPAllow, ToolIDMCPImport:
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

// inputSkipsPermissionCeiling reports calls whose input qualifies for approval without asking.
func (e *EffectivePolicyEvaluator) inputSkipsPermissionCeiling(d Descriptor, input json.RawMessage) bool {
	switch e.permissionMode() {
	case PermissionModeApproveReads:
		return isInstanceSelfServiceCall(d, input) || classifySessionCleanupCall(d, input) == sessionCleanupReads ||
			isExtensionSetupRead(d, input) || isSessionArchiveStoppedPreview(d, input)
	case PermissionModeApproveEdits, PermissionModeAuto:
		return classifySessionCleanupCall(d, input) != sessionCleanupDeletes ||
			isExtensionSetupRead(d, input) || isSessionArchiveStoppedPreview(d, input)
	default:
		return false
	}
}

// Session cleanup actions; the plan action deletes unless dry_run stays true.
const (
	SessionCleanupActionPlan   = "plan"
	SessionCleanupActionStatus = "status"
	SessionCleanupActionStop   = "stop"
)

// sessionCleanupCall is what one session_cleanup call does to history.
type sessionCleanupCall int

const (
	// sessionCleanupDeletes starts a delete, or is not a session_cleanup call.
	sessionCleanupDeletes sessionCleanupCall = iota
	// sessionCleanupReads previews a plan or reads the current run.
	sessionCleanupReads
	// sessionCleanupStops ends a delete already approved, as a coordination
	// mutation does.
	sessionCleanupStops
)

// classifySessionCleanupCall reads one session_cleanup call's input. Only a
// plan with dry_run=false deletes; a call without input (a catalog view)
// counts as that delete, so the tool advertises its approval.
func classifySessionCleanupCall(d Descriptor, input json.RawMessage) sessionCleanupCall {
	if d.Source.Kind != SourceBuiltin || d.ID != ToolIDSessionCleanup || len(input) == 0 {
		return sessionCleanupDeletes
	}
	var call struct {
		Action string `json:"action"`
		DryRun *bool  `json:"dry_run"`
	}
	if json.Unmarshal(input, &call) != nil {
		return sessionCleanupDeletes
	}
	switch strings.TrimSpace(call.Action) {
	case SessionCleanupActionStatus:
		return sessionCleanupReads
	case SessionCleanupActionStop:
		return sessionCleanupStops
	case "", SessionCleanupActionPlan:
		if call.DryRun == nil || *call.DryRun {
			return sessionCleanupReads
		}
	}
	return sessionCleanupDeletes
}

// isExtensionSetupRead reports an extensions_setup call that writes no inputs: only an inputs entry
// mutates. Like session cleanup, an evaluation without input (a catalog view) keeps the mutating class,
// so the tool advertises the approval its writes need.
func isExtensionSetupRead(d Descriptor, input json.RawMessage) bool {
	if d.Source.Kind != SourceBuiltin || d.ID != ToolIDExtensionsSetup || len(input) == 0 {
		return false
	}
	var call struct {
		Inputs map[string]json.RawMessage `json:"inputs"`
	}
	if json.Unmarshal(input, &call) != nil {
		return false
	}
	return len(call.Inputs) == 0
}

// isSessionArchiveStoppedPreview reports an Archive stopped call with
// dry_run=true; dry_run defaults to false, so a call without it archives.
func isSessionArchiveStoppedPreview(d Descriptor, input json.RawMessage) bool {
	if d.Source.Kind != SourceBuiltin || d.ID != ToolIDSessionArchiveStopped || len(input) == 0 {
		return false
	}
	var call struct {
		DryRun bool `json:"dry_run"`
	}
	return json.Unmarshal(input, &call) == nil && call.DryRun
}
