package builtin

import (
	"encoding/json"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func sessionContinueDescriptor() toolspkg.Descriptor {
	descriptor := nativeDescriptor(
		toolspkg.ToolIDSessionContinue,
		"session_continue",
		"Session Continue",
		"Continue a user session with another agent, runtime, or declared route. The new session "+
			"starts with the source conversation carried over; the source is never changed. "+
			"Retrying with the same idempotency_key returns the recorded result.",
		sessionContinueInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, "continue", "handoff"},
		[]string{"continue session", "handoff session", "switch agent"},
	)
	descriptor.OutputSchema = json.RawMessage(sessionDeriveOutputSchema)
	return descriptor
}

func sessionForkDescriptor() toolspkg.Descriptor {
	descriptor := nativeDescriptor(
		toolspkg.ToolIDSessionFork,
		"session_fork",
		"Session Fork",
		"Fork a user session with the same agent, runtime, and account: the whole conversation, or "+
			"through message_id and its turn (the agent's reply and tool work included). When the agent "+
			"can clone its own session the new session loads that clone on its first prompt. The source "+
			"is never changed. Retrying with the same idempotency_key returns the recorded result.",
		sessionForkInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, "fork", "branch"},
		[]string{"fork session", "branch session", "fork from message"},
	)
	descriptor.OutputSchema = json.RawMessage(sessionDeriveOutputSchema)
	return descriptor
}

const sessionForkInputSchema = `{
	"type":"object",
	"required":["session_id","idempotency_key"],
	"properties":{
		"workspace":{"type":"string"},
		"session_id":{"type":"string","minLength":1},
		"message_id":{"type":"string","minLength":1},
		"name":{"type":"string"},
		"idempotency_key":{"type":"string","minLength":1},
		"expected_epoch":{"type":"integer","minimum":0},
		"expected_generation":{"type":"integer","minimum":0},
		"expected_max_sequence":{"type":"integer","minimum":0}
	},
	"additionalProperties":false
}`

const sessionContinueInputSchema = `{
	"type":"object",
	"required":["session_id","agent","idempotency_key"],
	"properties":{
		"workspace":{"type":"string"},
		"session_id":{"type":"string","minLength":1},
		"agent":{"type":"string","minLength":1},
		"runtime":{
			"type":"object",
			"required":["provider"],
			"properties":{
				"provider":{"type":"string","minLength":1},
				"model":{"type":"string"},
				"reasoning_effort":{"type":"string"},
				"speed":{"type":"string","enum":["normal","fast"]},
				"acp_options":{
					"type":"array",
					"items":{
						"type":"object",
						"required":["id"],
						"properties":{
							"id":{"type":"string","minLength":1},
							"value_id":{"type":"string"},
							"bool_value":{"type":"boolean"}
						},
						"additionalProperties":false
					}
				}
			},
			"additionalProperties":false
		},
		"route":{"type":"integer","minimum":1},
		"name":{"type":"string"},
		"message":{"type":"string"},
		"idempotency_key":{"type":"string","minLength":1},
		"expected_epoch":{"type":"integer","minimum":0},
		"expected_generation":{"type":"integer","minimum":0},
		"expected_max_sequence":{"type":"integer","minimum":0}
	},
	"not":{"required":["runtime","route"]},
	"additionalProperties":false
}`

const sessionDeriveOutputSchema = `{
	"type":"object",
	"required":["derived"],
	"properties":{
		"session":{"type":"object"},
		"derived":{
			"type":"object",
			"required":[
				"kind","source_session_id","origin_agent_name","through_turn_id","seed",
				"truncated","omitted_count","source_turn_in_progress","first_prompt","replayed","child_session_id"
			],
			"properties":{
				"kind":{"type":"string","enum":["continue","fork"]},
				"source_session_id":{"type":"string"},
				"origin_agent_name":{"type":"string"},
				"origin_message_id":{"type":"string"},
				"through_turn_id":{"type":"string"},
				"seed":{"type":"string","enum":["replay","native_fork"]},
				"native_state":{"type":"string"},
				"acp_session_id":{"type":"string"},
				"native_fork_error":{"type":"string"},
				"replay_message_count":{"type":"integer"},
				"replay_bytes":{"type":"integer"},
				"truncated":{"type":"boolean"},
				"omitted_count":{"type":"integer"},
				"source_turn_in_progress":{"type":"boolean"},
				"first_prompt":{"type":"string","enum":["staged","admitted"]},
				"replayed":{"type":"boolean"},
				"child_deleted":{"type":"boolean"},
				"child_session_id":{"type":"string"}
			},
			"additionalProperties":false
		}
	},
	"additionalProperties":false
}`
