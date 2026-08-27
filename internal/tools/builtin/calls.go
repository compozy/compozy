package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

func callDescriptors() []toolspkg.Descriptor {
	return []toolspkg.Descriptor{
		callDescriptor(toolspkg.ToolIDAgentCall, "agent_call", "Agent Call",
			"Call one named agent or existing child asynchronously. Use agent_list for the current roster.",
			callCreateInputSchema, toolspkg.RiskMutating, false),
		callDescriptor(toolspkg.ToolIDCallReturn, "call_return", "Call Return",
			"Return one structured JSON result from the child session bound to a call.",
			callReturnInputSchema, toolspkg.RiskMutating, false),
		callDescriptor(toolspkg.ToolIDCallAwait, "call_await", "Call Await",
			"Wait for one or more calls for a bounded interval.", callAwaitInputSchema, toolspkg.RiskRead, true),
		callDescriptor(toolspkg.ToolIDCallCancel, "call_cancel", "Call Cancel",
			"Cancel one caller-owned call idempotently.", callCancelInputSchema, toolspkg.RiskMutating, false),
		callDescriptor(toolspkg.ToolIDCallResult, "call_result", "Call Result",
			"Read the complete stored JSON result for one settled call.", callIDInputSchema, toolspkg.RiskRead, true),
		callDescriptor(toolspkg.ToolIDCallPublish, "call_publish", "Call Publish",
			"Publish one completed result into a participating Network channel thread.",
			callPublishInputSchema, toolspkg.RiskMutating, false),
		callDescriptor(toolspkg.ToolIDAgentMessage, "agent_message", "Agent Message",
			"Send inert text to one lineage session without creating a new call.",
			callMessageInputSchema, toolspkg.RiskMutating, false),
	}
}

func callDescriptor(
	id toolspkg.ToolID,
	nativeName, title, description, schema string,
	risk toolspkg.RiskClass,
	readOnly bool,
) toolspkg.Descriptor {
	return nativeDescriptor(
		id, nativeName, title, description, schema, risk, readOnly, risk == toolspkg.RiskDestructive, false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDCalls}, []string{"calls", agentsSegment}, []string{nativeName, title},
	)
}

const callCreateTaskSchema = `{
	"type":"object",
	"properties":{
		"agent":{"type":"string","minLength":1},"session_id":{"type":"string","minLength":1},
		"prompt":{"type":"string","minLength":1},"expect":{},
		"idle_ttl_seconds":{"type":"integer","minimum":1},
		"deadline_seconds":{
			"description":"Positive integer seconds; invalid values return call_deadline_invalid."
		},
		"strict":{"type":"boolean"},
		"result_budget":{"type":"string"},"result_overflow":{"type":"string","enum":["store","reject"]},
		"idempotency_key":{"type":"string"},
		"runtime":{"type":"object","properties":{
			"provider":{"type":"string"},"model":{"type":"string"},
			"reasoning_effort":{"type":"string"},"speed":{"type":"string","enum":["normal","fast"]}
		},"additionalProperties":false},
		"narrow":{"type":"object","properties":{
			"tools":{"type":"array","items":{"type":"string"}},
			"skills":{"type":"array","items":{"type":"string"}},
			"mcp_servers":{"type":"array","items":{"type":"string"}},
			"workspace_paths":{"type":"array","items":{"type":"string"}},
			"network_channels":{"type":"array","items":{"type":"string"}},
			"sandbox_profiles":{"type":"array","items":{"type":"string"}}
		},"additionalProperties":false}
	},
	"oneOf":[
		{"required":["agent","prompt"],"not":{"required":["session_id"]}},
		{"required":["session_id","prompt"],"not":{"required":["agent"]}}
	],
	"additionalProperties":false
}`

const callCreateInputSchema = `{
	"type":"object",
	"properties":{
		"agent":{"type":"string","minLength":1},"session_id":{"type":"string","minLength":1},
		"prompt":{"type":"string","minLength":1},"expect":{},
		"idle_ttl_seconds":{"type":"integer","minimum":1},
		"deadline_seconds":{
			"description":"Positive integer seconds; invalid values return call_deadline_invalid."
		},
		"strict":{"type":"boolean"},
		"result_budget":{"type":"string"},"result_overflow":{"type":"string","enum":["store","reject"]},
		"idempotency_key":{"type":"string"},
		"runtime":{"type":"object","properties":{
			"provider":{"type":"string"},"model":{"type":"string"},
			"reasoning_effort":{"type":"string"},"speed":{"type":"string","enum":["normal","fast"]}
		},"additionalProperties":false},
		"narrow":{"type":"object","properties":{
			"tools":{"type":"array","items":{"type":"string"}},
			"skills":{"type":"array","items":{"type":"string"}},
			"mcp_servers":{"type":"array","items":{"type":"string"}},
			"workspace_paths":{"type":"array","items":{"type":"string"}},
			"network_channels":{"type":"array","items":{"type":"string"}},
			"sandbox_profiles":{"type":"array","items":{"type":"string"}}
		},"additionalProperties":false},
		"tasks":{"type":"array","items":` + callCreateTaskSchema + `}
	},
	"additionalProperties":false
}`

const callReturnInputSchema = `{
	"type":"object",
	"properties":{"call_id":{"type":"string"},"result":{},"final_text":{"type":"string"}},
	"additionalProperties":false
}`

const callAwaitInputSchema = `{
	"type":"object","required":["call_ids"],
	"properties":{
		"call_ids":{"type":"array","items":{"type":"string"},"minItems":1,"uniqueItems":true},
		"timeout_ms":{"type":"integer","minimum":0,"maximum":1800000},
		"resume":{"type":"string"}
	},
	"additionalProperties":false
}`

const callCancelInputSchema = `{
	"type":"object","required":["call_id"],
	"properties":{"call_id":{"type":"string","minLength":1},"reason":{"type":"string"}},
	"additionalProperties":false
}`

const callIDInputSchema = `{"type":"object","required":["call_id"],"properties":{"call_id":{"type":"string","minLength":1}},"additionalProperties":false}`

const callPublishInputSchema = `{
	"type":"object","required":["call_id","channel"],
	"properties":{
		"call_id":{"type":"string","minLength":1},
		"channel":{"type":"string","minLength":1},"thread_id":{"type":"string"}
	},
	"additionalProperties":false
}`

const callMessageInputSchema = `{
	"type":"object","required":["to","text"],
	"properties":{
		"to":{
			"type":"string","minLength":1,
			"description":"parent or a granted session id inside the caller lineage"
		},
		"text":{"type":"string","minLength":1},"call_id":{"type":"string"}
	},
	"additionalProperties":false
}`
