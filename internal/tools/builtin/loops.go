package builtin

import toolspkg "github.com/compozy/agh/internal/tools"

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
		[]string{loopKey, "validate", "lint"},
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
		[]string{loopKey, goalKey, "turns", "audit"},
		[]string{"goal turns", "loop turn audit", "goal history"},
	),
	nativeLoopDescriptor(
		toolspkg.ToolIDLoopStop,
		"loop_stop",
		"Loop Stop",
		"Stop one active Loop run.",
		loopRunIDInputSchema,
		toolspkg.RiskDestructive,
		false,
		true,
		[]string{loopKey, "stop", "destructive"},
		[]string{"stop loop", "cancel loop run"},
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
		[]string{loopKey, "delete", "destructive"},
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
	if id == toolspkg.ToolIDLoopApprove {
		return withRequiredCapabilities(descriptor, "loops.approve")
	}
	return descriptor
}

const loopListInputSchema = `{
	"type":"object",
	"additionalProperties":false,
	"properties":{
		"workspace_id":{"type":"string"},
		"q":{"type":"string"},
		"kind":{"type":"string","enum":["read_only","workspace"]},
		"category":{"type":"string"},
		"status":{
			"type":"string",
			"enum":[
				"queued","running","watching","needs-approval","paused","done",
				"no-op","blocked","failed","exhausted","stalled"
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
		"workspace_id":{"type":"string"},
		"status":{"type":"string","enum":["complete","blocked"]},
		"evidence":{"type":"string","maxLength":16384}
	}
}`

const loopNameInputSchema = `{
	"type":"object",
	"required":["name"],
	"additionalProperties":false,
	"properties":{
		"workspace_id":{"type":"string"},
		"name":{"type":"string","minLength":1}
	}
}`

const loopValidateInputSchema = `{
	"type":"object",
	"required":["definition"],
	"additionalProperties":false,
	"properties":{
		"workspace_id":{"type":"string"},
		"name":{"type":"string"},
		"definition":{"type":"object","additionalProperties":true}
	}
}`

const loopCreateInputSchema = `{
	"type":"object",
	"additionalProperties":false,
	"properties":{
		"workspace_id":{"type":"string"},
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
		"workspace_id":{"type":"string"},
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
		"workspace_id":{"type":"string"},
		"run_id":{"type":"string","minLength":1}
	}
}`

const loopRunsInputSchema = `{
	"type":"object",
	"additionalProperties":false,
	"properties":{
		"workspace_id":{"type":"string"},
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
		"workspace_id":{"type":"string"},
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
		"workspace_id":{"type":"string"},
		"name":{"type":"string","minLength":1},
		"config":{"type":"object","additionalProperties":true}
	}
}`

const loopApproveInputSchema = `{
	"type":"object",
	"required":["run_id","gate_id","decision"],
	"additionalProperties":false,
	"properties":{
		"workspace_id":{"type":"string"},
		"run_id":{"type":"string","minLength":1},
		"gate_id":{"type":"string","minLength":1},
		"decision":{"type":"string","enum":["approve","request_changes","reject"]},
		"approval_token_hash":{"type":"string","pattern":"^sha256:[0-9a-f]{64}$"}
	}
}`
