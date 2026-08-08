package builtin

import (
	"encoding/json"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

const (
	loopKey = "loop"
	goalKey = "goal"
)

var loopTools = []toolspkg.Descriptor{
	nativeLoopDescriptor(
		toolspkg.ToolIDGoalGet,
		"goal_get",
		"Goal Get",
		"Read the visible Goal projection for the caller session, including terminal state until clear.",
		goalGetInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		[]string{goalKey, descriptorKeywordStatus, "session"},
		[]string{"goal status", "current goal", "session goal"},
	),
	nativeLoopDescriptor(
		toolspkg.ToolIDGoalReport,
		"goal_report",
		"Goal Report",
		"Record a completion or blocker intent for the caller session's current Goal prompt.",
		goalReportInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		[]string{goalKey, "report", descriptorKeywordUpdate},
		[]string{"report goal complete", "report goal blocked", "goal evidence"},
	),
	nativeLoopDescriptor(
		toolspkg.ToolIDLoopList,
		"loop_list",
		"Loop List",
		"List Loop definitions available in the caller workspace.",
		loopListInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		[]string{loopKey, descriptorKeywordCatalog},
		[]string{"loop catalog", "list loops"},
	),
	nativeLoopDescriptor(
		toolspkg.ToolIDLoopInspect,
		"loop_inspect",
		"Loop Inspect",
		"Read one Loop definition with inputs, contract, start bindings, and version.",
		loopNameInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		[]string{loopKey, descriptorKeywordStatus},
		[]string{"loop inspect", "loop describe", "loop inputs", "loop contract"},
	),
	nativeLoopDescriptor(
		toolspkg.ToolIDLoopValidate,
		"loop_validate",
		"Loop Validate",
		"Lint and compile one Loop definition without saving it.",
		loopValidateInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		[]string{loopKey, descriptorKeywordValidate, "lint"},
		[]string{"loop validate", "loop lint", "compile loop"},
	),
	nativeLoopDescriptor(
		toolspkg.ToolIDLoopCreate,
		"loop_create",
		"Loop Create",
		"Create or fork one user-authored Loop definition with version checks.",
		loopCreateInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		[]string{loopKey, descriptorKeywordCreate, "definition"},
		[]string{"create loop", "fork loop", "publish loop definition"},
	),
	nativeLoopDescriptor(
		toolspkg.ToolIDLoopRun,
		"loop_run",
		"Loop Run",
		"Start or dry-run one Loop with structured inputs.",
		loopRunInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		[]string{loopKey, "run", "dry-run"},
		[]string{"run loop", "dry run loop", "start loop"},
	),
	nativeLoopDescriptor(
		toolspkg.ToolIDLoopStatus,
		"loop_status",
		"Loop Status",
		"Read one Loop run status with generation detail.",
		loopRunIDInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		[]string{loopKey, descriptorKeywordStatus},
		[]string{"loop run status", "inspect loop run"},
	),
	nativeLoopDescriptor(
		toolspkg.ToolIDLoopRuns,
		"loop_runs",
		"Loop Runs",
		"List workspace-scoped Loop runs with filters and aggregates.",
		loopRunsInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		[]string{loopKey, "runs", descriptorKeywordStatus},
		[]string{"loop runs", "list loop runs"},
	),
	nativeLoopDescriptor(
		toolspkg.ToolIDLoopTurns,
		"loop_turns",
		"Loop Turns",
		"List one Loop run's total-order Goal turn audit with cursor and node/item filters.",
		loopTurnsInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		[]string{loopKey, goalKey, "turns", descriptorKeywordAudit},
		[]string{"goal turns", "loop turn audit", "goal history"},
	),
	nativeLoopDescriptor(
		toolspkg.ToolIDLoopCancel,
		"loop_cancel",
		"Loop Cancel",
		"Request cooperative cancellation for one active Loop run.",
		loopRunIDInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		[]string{loopKey, descriptorKeywordCancel},
		[]string{"cancel loop", "cancel loop run"},
	),
	nativeLoopDescriptor(
		toolspkg.ToolIDLoopKill,
		"loop_kill",
		"Loop Kill",
		"Immediately terminalize one active Loop run.",
		loopRunIDInputSchema,
		toolspkg.RiskDestructive,
		false,
		true,
		[]string{loopKey, "kill", descriptorKeywordDestructive},
		[]string{"kill loop", "kill loop run"},
	),
	nativeLoopDescriptor(
		toolspkg.ToolIDLoopNodes,
		"loop_nodes",
		"Loop Nodes",
		"List workspace-scoped Loop node state with stable pagination.",
		loopNodesInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		[]string{loopKey, "nodes", descriptorKeywordStatus},
		[]string{"loop nodes", "waiting nodes", "quarantined nodes"},
	),
	nativeLoopDescriptor(
		toolspkg.ToolIDLoopNodePause,
		"loop_node_pause",
		"Loop Node Pause",
		"Pause one authored node across its live cells.",
		loopNodePauseInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		[]string{loopKey, descriptorKeywordNode, "pause"},
		[]string{"pause loop node"},
	),
	nativeLoopDescriptor(
		toolspkg.ToolIDLoopNodeResume,
		"loop_node_resume",
		"Loop Node Resume",
		"Resume one paused node or admit one manual wait payload.",
		loopNodeResumeInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		[]string{loopKey, descriptorKeywordNode, "resume"},
		[]string{"resume loop node", "resume loop wait"},
	),
	nativeLoopDescriptor(
		toolspkg.ToolIDLoopNodeCancel,
		"loop_node_cancel",
		"Loop Node Cancel",
		"Request cooperative cancellation for one authored node.",
		loopNodeMutationInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		[]string{loopKey, descriptorKeywordNode, descriptorKeywordCancel},
		[]string{"cancel loop node"},
	),
	nativeLoopDescriptor(
		toolspkg.ToolIDLoopNodeKill,
		"loop_node_kill",
		"Loop Node Kill",
		"Immediately fence and stop one authored node.",
		loopNodeMutationInputSchema,
		toolspkg.RiskDestructive,
		false,
		true,
		[]string{loopKey, descriptorKeywordNode, "kill", descriptorKeywordDestructive},
		[]string{"kill loop node"},
	),
	nativeLoopDescriptor(
		toolspkg.ToolIDLoopNodeRequeue,
		"loop_node_requeue",
		"Loop Node Requeue",
		"Clear one node quarantine and reserve its next generation.",
		loopNodeMutationInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		[]string{loopKey, descriptorKeywordNode, "requeue"},
		[]string{"requeue loop node", "clear node quarantine"},
	),
	nativeLoopDescriptor(
		toolspkg.ToolIDLoopPause,
		"loop_pause",
		"Loop Pause",
		"Request a generation-boundary pause for one running Loop run.",
		loopRunIDInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		[]string{loopKey, "pause"},
		[]string{"pause loop", "pause loop run"},
	),
	nativeLoopDescriptor(
		toolspkg.ToolIDLoopResume,
		"loop_resume",
		"Loop Resume",
		"Resume one paused Loop run.",
		loopRunIDInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		[]string{loopKey, "resume"},
		[]string{"resume loop", "resume loop run"},
	),
	nativeLoopDescriptor(
		toolspkg.ToolIDLoopConfigure,
		"loop_configure",
		"Loop Configure",
		"Write per-loop runtime config overrides.",
		loopConfigureInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		[]string{loopKey, "config", descriptorKeywordUpdate},
		[]string{"configure loop", "loop config"},
	),
	nativeLoopDescriptor(
		toolspkg.ToolIDLoopApprove,
		"loop_approve",
		"Loop Approve",
		"Apply one human-gate decision without allowing self-approval.",
		loopApproveInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		[]string{loopKey, "approve", "gate"},
		[]string{"approve loop gate", "human gate decision"},
	),
	nativeLoopDescriptor(
		toolspkg.ToolIDLoopDelete,
		"loop_delete",
		"Loop Delete",
		"Delete one workspace-authored Loop definition.",
		loopNameInputSchema,
		toolspkg.RiskDestructive,
		false,
		true,
		[]string{loopKey, "delete", descriptorKeywordDestructive},
		[]string{"delete loop", "remove loop definition"},
	),
}

func loopDescriptors() []toolspkg.Descriptor {
	return loopTools
}

func nativeLoopDescriptor(
	id toolspkg.ToolID,
	nativeName string,
	title string,
	description string,
	inputSchema string,
	risk toolspkg.RiskClass,
	readOnly bool,
	destructive bool,
	tags []string,
	searchHints []string,
) toolspkg.Descriptor {
	descriptor := nativeDescriptor(
		id, nativeName, title, description, inputSchema, risk, readOnly, destructive, false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDLoops}, tags, searchHints,
	)
	switch id {
	case toolspkg.ToolIDLoopStatus:
		descriptor.OutputSchema = json.RawMessage(loopStatusOutputSchema)
	case toolspkg.ToolIDLoopRuns:
		descriptor.OutputSchema = json.RawMessage(loopRunsOutputSchema)
	case toolspkg.ToolIDLoopNodes:
		descriptor.OutputSchema = json.RawMessage(loopNodesOutputSchema)
	case toolspkg.ToolIDLoopCancel,
		toolspkg.ToolIDLoopKill,
		toolspkg.ToolIDLoopNodePause,
		toolspkg.ToolIDLoopNodeResume,
		toolspkg.ToolIDLoopNodeCancel,
		toolspkg.ToolIDLoopNodeKill,
		toolspkg.ToolIDLoopNodeRequeue:
		descriptor.OutputSchema = json.RawMessage(loopMutationOutputSchema)
	}
	if id == toolspkg.ToolIDLoopApprove {
		return withRequiredCapabilities(descriptor, "loops.approve")
	}
	return descriptor
}

const loopListInputSchema = `{
	"type":"object",
	"additionalProperties":false,
	"properties":{
		"workspace":{"type":"string"},
		"q":{"type":"string"},
		"kind":{"type":"string","enum":["read_only","workspace"]},
		"category":{"type":"string"},
		"status":{
			"type":"string",
			"enum":[
				"queued","running","watching","needs-approval","paused","done",
				"no-op","blocked","failed","exhausted","stalled","canceled"
			]
		},
		"sort":{"type":"string","enum":["name"]},
		"cursor":{"type":"string"},
		"limit":{"type":"integer","minimum":1,"maximum":200}
	}
}`

const goalGetInputSchema = loopListInputSchema

const goalReportInputSchema = `{
	"type":"object",
	"required":["status"],
	"additionalProperties":false,
	"properties":{
		"workspace":{"type":"string"},
		"status":{"type":"string","enum":["complete","blocked"]},
		"evidence":{"type":"string","maxLength":16384}
	}
}`

const loopNameInputSchema = `{
	"type":"object",
	"required":["name"],
	"additionalProperties":false,
	"properties":{
		"workspace":{"type":"string"},
		"name":{"type":"string","minLength":1}
	}
}`

const loopValidateInputSchema = `{
	"type":"object",
	"required":["definition"],
	"additionalProperties":false,
	"properties":{
		"workspace":{"type":"string"},
		"name":{"type":"string"},
		"definition":{"type":"object","additionalProperties":true}
	}
}`

const loopCreateInputSchema = `{
	"type":"object",
	"additionalProperties":false,
	"properties":{
		"workspace":{"type":"string"},
		"definition":{"type":"object","additionalProperties":true},
		"fork_from_name":{"type":"string"},
		"expected_version":{"type":"integer","minimum":0}
	}
}`

const loopRunInputSchema = `{
	"type":"object",
	"required":["name"],
	"additionalProperties":false,
	"properties":{
		"workspace":{"type":"string"},
		"name":{"type":"string","minLength":1},
		"inputs":{"type":"object","additionalProperties":true},
		"parent_loop_run_id":{"type":"string"},
		"config_overrides":{"type":"object","additionalProperties":true},
		"network_participation":` + networkParticipationRequestSchema + `,
		"dry":{"type":"boolean"}
	}
}`

const loopRunIDInputSchema = `{
	"type":"object",
	"required":["run_id"],
	"additionalProperties":false,
	"properties":{
		"workspace":{"type":"string"},
		"run_id":{"type":"string","minLength":1}
	}
}`

const loopRunsInputSchema = `{
	"type":"object",
	"additionalProperties":false,
	"properties":{
		"workspace":{"type":"string"},
		"loop_name":{"type":"string"},
		"status":{"type":"string"},
		"limit":{"type":"integer","minimum":1}
	}
}`

const loopTurnsInputSchema = `{
	"type":"object",
	"required":["run_id"],
	"additionalProperties":false,
	"properties":{
		"workspace":{"type":"string"},
		"run_id":{"type":"string","minLength":1},
		"node":{"type":"string","minLength":1},
		"item":{"type":"integer","minimum":0},
		"after_seq":{"type":"integer","minimum":0},
		"limit":{"type":"integer","minimum":1,"maximum":200}
	}
}`

const loopConfigureInputSchema = `{
	"type":"object",
	"required":["name","config"],
	"additionalProperties":false,
	"properties":{
		"workspace":{"type":"string"},
		"name":{"type":"string","minLength":1},
		"config":{"type":"object","additionalProperties":true}
	}
}`

const loopApproveInputSchema = `{
	"type":"object",
	"required":["run_id","gate_id","decision"],
	"additionalProperties":false,
	"properties":{
		"workspace":{"type":"string"},
		"run_id":{"type":"string","minLength":1},
		"gate_id":{"type":"string","minLength":1},
		"decision":{"type":"string","enum":["approve","request_changes","reject"]},
		"approval_token_hash":{"type":"string","pattern":"^sha256:[0-9a-f]{64}$"}
	}
}`
