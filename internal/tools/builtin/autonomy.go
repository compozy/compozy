package builtin

import toolspkg "github.com/compozy/agh/internal/tools"

const (
	autonomyAutonomyKey = "autonomy"
	autonomyLeasesKey   = "leases"
	autonomyReviewsKey  = "reviews"
)

var autonomyTools = []toolspkg.Descriptor{
	nativeDescriptor(
		toolspkg.ToolIDTaskRunClaimNext,
		"task_run_claim_next",
		"Task Run Claim Next",
		"Claim the next eligible task run for the caller session.",
		autonomyClaimNextInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDAutonomy},
		[]string{autonomyAutonomyKey, tasksTasksKey, "runs"},
		[]string{"claim task run", "next work"},
	),
	nativeDescriptor(
		toolspkg.ToolIDTaskRunHeartbeat,
		"task_run_heartbeat",
		"Task Run Heartbeat",
		"Extend the caller session's active task-run lease.",
		autonomyHeartbeatInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDAutonomy},
		[]string{autonomyAutonomyKey, tasksTasksKey, autonomyLeasesKey},
		[]string{"heartbeat task run", "extend lease"},
	),
	nativeDescriptor(
		toolspkg.ToolIDTaskRunComplete,
		"task_run_complete",
		"Task Run Complete",
		"Complete the caller session's active task-run lease.",
		autonomyCompleteInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDAutonomy},
		[]string{autonomyAutonomyKey, tasksTasksKey, autonomyLeasesKey},
		[]string{"complete task run", "finish work"},
	),
	nativeDescriptor(
		toolspkg.ToolIDTaskRunFail,
		"task_run_fail",
		"Task Run Fail",
		"Fail the caller session's active task-run lease.",
		autonomyFailInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDAutonomy},
		[]string{autonomyAutonomyKey, tasksTasksKey, autonomyLeasesKey},
		[]string{"fail task run", "report failure"},
	),
	nativeDescriptor(
		toolspkg.ToolIDTaskRunRelease,
		"task_run_release",
		"Task Run Release",
		"Release the caller session's active task-run lease back to the queue.",
		autonomyReleaseInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDAutonomy},
		[]string{autonomyAutonomyKey, tasksTasksKey, autonomyLeasesKey},
		[]string{"release task run", "handoff work"},
	),
	nativeDescriptor(
		toolspkg.ToolIDTaskRunReviewSubmit,
		"submit_run_review",
		"Submit Run Review",
		"Submit the caller session's bound task-run review verdict through the task service.",
		autonomySubmitRunReviewInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDAutonomy},
		[]string{autonomyAutonomyKey, tasksTasksKey, autonomyReviewsKey},
		[]string{"submit_run_review", "review verdict", "task run review"},
	),
}

func autonomyDescriptors() []toolspkg.Descriptor {
	return autonomyTools
}

const autonomyClaimNextInputSchema = `{
	"type":"object",
	"properties":{
		"run_id":{"type":"string"},
		"workspace_id":{"type":"string"},
		"required_capabilities":{"type":"array","items":{"type":"string"}},
		"priority_min":{"type":"integer"},
		"lease_seconds":{"type":"integer"}
	},
	"additionalProperties":false
}`

const autonomyHeartbeatInputSchema = `{
	"type":"object",
	"required":["run_id"],
	"properties":{
		"run_id":{"type":"string"},
		"lease_seconds":{"type":"integer"}
	},
	"additionalProperties":false
}`

const autonomyCompleteInputSchema = `{
	"type":"object",
	"required":["run_id"],
	"properties":{
		"run_id":{"type":"string"},
		"result":{},
		"created_task_ids":{"type":"array","items":{"type":"string"}}
	},
	"additionalProperties":false
}`

const autonomyFailInputSchema = `{
	"type":"object",
	"required":["run_id","error"],
	"properties":{
		"run_id":{"type":"string"},
		"error":{"type":"string"},
		"metadata":{}
	},
	"additionalProperties":false
}`

const autonomyReleaseInputSchema = `{
	"type":"object",
	"required":["run_id"],
	"properties":{
		"run_id":{"type":"string"},
		"reason":{"type":"string"}
	},
	"additionalProperties":false
}`

const autonomySubmitRunReviewInputSchema = `{
	"type":"object",
	"required":["review_id","run_id","outcome","confidence","reason","missing_work","next_round_guidance","delivery_id"],
	"properties":{
		"review_id":{"type":"string"},
		"run_id":{"type":"string"},
		"outcome":{"type":"string","enum":["approved","rejected","blocked","error","timeout","invalid_output"]},
		"confidence":{"type":"number","minimum":0,"maximum":1},
		"reason":{"type":"string"},
		"missing_work":{"type":"array","items":{"type":"string"}},
		"next_round_guidance":{"type":"string"},
		"review_text":{"type":"string"},
		"delivery_id":{"type":"string"}
	},
	"additionalProperties":false
}`
