package builtin

import (
	"encoding/json"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func sessionInputsClearDescriptor() toolspkg.Descriptor {
	descriptor := nativeDescriptor(
		toolspkg.ToolIDSessionInputsClear, "session_inputs_clear", "Session Inputs Clear",
		"Clear parked session input with attributed per-entry transcript traces; in-flight dispatch remains intact.",
		sessionIDInputSchema, toolspkg.RiskMutating, false, false, false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, "inputs", sessionsQueueKey}, []string{"clear queued session input"},
	)
	descriptor.OutputSchema = json.RawMessage(`{
		"type":"object","required":["inputs","cleared_count","queue_generation"],
		"properties":{
			"inputs":{"type":"array","items":` + sessionInputPayloadSchema + `},
			"cleared_count":{"type":"integer","minimum":0},
			"queue_generation":{"type":"integer"}
		},"additionalProperties":false
	}`)
	return descriptor
}
