package builtin

import (
	"encoding/json"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func sessionOrchestrationDescriptors() []toolspkg.Descriptor {
	return []toolspkg.Descriptor{
		orchestrationDescriptor(
			toolspkg.ToolIDSessionWait, "session_wait", "Session Wait",
			"Wait for one bounded badge transition on a same-workspace session.",
			sessionWaitInputSchema, sessionWaitOutputSchema, toolspkg.RiskRead, true, false,
		),
		orchestrationDescriptor(
			toolspkg.ToolIDSessionStop, "session_stop", "Session Stop",
			"Stop one same-workspace session other than the caller.",
			sessionTargetInputSchema, sessionStopOutputSchema, toolspkg.RiskDestructive, false, true,
		),
		orchestrationDescriptor(
			toolspkg.ToolIDSessionApprove, "session_approve", "Session Approve",
			"Resolve one pending permission request on another same-workspace session.",
			sessionApproveInputSchema, sessionApproveOutputSchema, toolspkg.RiskMutating, false, false,
		),
		orchestrationDescriptor(
			toolspkg.ToolIDSessionClarifyAnswer, "session_clarify_answer", "Session Clarify Answer",
			"Answer one pending clarification on another same-workspace session.",
			sessionClarifyAnswerInputSchema, sessionClarifyAnswerOutputSchema, toolspkg.RiskMutating, false, false,
		),
		orchestrationDescriptor(
			toolspkg.ToolIDSessionPromptCancel, "session_prompt_cancel", "Session Prompt Cancel",
			"Cancel one in-flight prompt on another same-workspace session without stopping it.",
			sessionTargetInputSchema, sessionPromptCancelOutputSchema, toolspkg.RiskMutating, false, false,
		),
	}
}

func orchestrationDescriptor(
	id toolspkg.ToolID,
	nativeName string,
	title string,
	description string,
	inputSchema string,
	outputSchema string,
	risk toolspkg.RiskClass,
	readOnly bool,
	destructive bool,
) toolspkg.Descriptor {
	descriptor := nativeDescriptor(
		id, nativeName, title, description, inputSchema, risk, readOnly, destructive, false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, "orchestration"},
		[]string{nativeName, title},
	)
	descriptor.OutputSchema = json.RawMessage(outputSchema)
	return descriptor
}

const sessionTargetInputSchema = `{
	"type":"object",
	"required":["session_id"],
	"properties":{"session_id":{"type":"string","minLength":1},"subtree":{"type":"boolean"},"reason":{"type":"string"}},
	"additionalProperties":false
}`

const sessionWaitInputSchema = `{
	"type":"object",
	"required":["session_id"],
	"properties":{
		"session_id":{"type":"string","minLength":1},
		"until":{
			"type":"array",
			"items":{
				"type":"string",
				"enum":[
					"waiting-for-input","waiting-for-auth","idle","done","running",
					"stopped","failed","hung","unhealthy"
				]
			},
			"uniqueItems":true
		},
		"timeout_ms":{"type":"integer","minimum":1,"maximum":1800000}
	},
	"additionalProperties":false
}`

const sessionWaitOutputSchema = `{
	"type":"object",
	"required":["session_id","outcome","waited_ms"],
	"properties":{
		"session_id":{"type":"string"},
		"outcome":{"type":"string","enum":["state-reached","timeout","session-gone","canceled","overflow"]},
		"state":{"type":"string"},
		"waited_ms":{"type":"integer","minimum":0},
		"revision":{"type":"integer","minimum":0}
	},
	"additionalProperties":false
}`

const sessionStopOutputSchema = `{
	"type":"object",
	"required":["session_id","state"],
	"properties":{"session_id":{"type":"string"},"state":{"const":"stopped"},"outcome":{"const":"already-stopped"},"stopped_children":{"type":"integer","minimum":0},"closed_calls":{"type":"integer","minimum":0},"preserved_results":{"type":"integer","minimum":0}},
	"additionalProperties":false
}`

const sessionApproveInputSchema = `{
	"type":"object",
	"required":["session_id","request_id","decision"],
	"properties":{
		"session_id":{"type":"string","minLength":1},"request_id":{"type":"string","minLength":1},
		"decision":{"type":"string","enum":["allow-once","allow-always","reject-once","reject-always"]}
	},
	"additionalProperties":false
}`

const sessionApproveOutputSchema = `{
	"type":"object",
	"required":["outcome","request_id"],
	"properties":{
		"outcome":{"type":"string","enum":["applied","already-resolved","resolved-after-restart","queue-full"]},
		"request_id":{"type":"string"},"decision":{"type":"string"},"resolved_decision":{"type":"string"}
	},
	"additionalProperties":false
}`

const sessionClarifyAnswerInputSchema = `{
	"type":"object",
	"required":["session_id","request_id"],
	"properties":{
		"session_id":{"type":"string","minLength":1},"request_id":{"type":"string","minLength":1},
		"choice":{"type":"integer","minimum":1},"text":{"type":"string","minLength":1,"maxLength":4000}
	},
	"not":{"anyOf":[
		{"required":["choice","text"]},
		{"allOf":[{"not":{"required":["choice"]}},{"not":{"required":["text"]}}]}
	]},
	"additionalProperties":false
}`

const sessionClarifyAnswerOutputSchema = `{
	"type":"object",
	"required":["outcome","request_id"],
	"properties":{
		"outcome":{"type":"string","enum":["answered","already-resolved","resolved-after-restart","queue-full"]},
		"request_id":{"type":"string"},"resolved_answer":{"type":"string"}
	},
	"additionalProperties":false
}`

const sessionPromptCancelOutputSchema = `{
	"type":"object",
	"required":["session_id","outcome"],
	"properties":{
		"session_id":{"type":"string"},"outcome":{"type":"string","enum":["canceled","nothing-in-flight"]},
		"turn_id":{"type":"string"}
	},
	"additionalProperties":false
}`
