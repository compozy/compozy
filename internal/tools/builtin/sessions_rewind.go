package builtin

import (
	"encoding/json"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func sessionRewindDescriptor() toolspkg.Descriptor {
	descriptor := nativeDescriptor(
		toolspkg.ToolIDSessionRewind,
		"session_rewind",
		"Session Rewind",
		"Rewind an idle user session before one durable user message. "+
			"This does not undo files, tools, network actions, or memory.",
		sessionRewindInputSchema,
		toolspkg.RiskDestructive,
		false,
		true,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, "rewind"},
		[]string{"rewind session", "rollback conversation"},
	)
	descriptor.OutputSchema = json.RawMessage(sessionRewindOutputSchema)
	return descriptor
}

const sessionRewindInputSchema = `{
	"type":"object",
	"required":[
		"session_id","message_id","idempotency_key",
		"expected_epoch","expected_generation","expected_max_sequence"
	],
	"properties":{
		"workspace":{"type":"string"},
		"session_id":{"type":"string","minLength":1},
		"message_id":{"type":"string","minLength":1},
		"idempotency_key":{"type":"string","minLength":1},
		"expected_epoch":{"type":"integer","minimum":0},
		"expected_generation":{"type":"integer","minimum":0},
		"expected_max_sequence":{"type":"integer","minimum":0}
	},
	"additionalProperties":false
}`

const sessionRewindOutputSchema = `{
	"type":"object",
	"required":["session","rewind"],
	"properties":{
		"session":{"type":"object"},
		"rewind":{
			"type":"object",
			"required":["transcript_epoch","target_message_id","draft_text","generation","max_sequence"],
			"properties":{
				"transcript_epoch":{"type":"integer"},
				"target_message_id":{"type":"string"},
				"archived_from_sequence":{"type":"integer"},
				"archived_through_sequence":{"type":"integer"},
				"archived_event_count":{"type":"integer"},
				"generation":{"type":"integer"},
				"max_sequence":{"type":"integer"},
				"draft_text":{"type":"string"},
				"replayed":{"type":"boolean"}
			},
			"additionalProperties":false
		}
	},
	"additionalProperties":false
}`
