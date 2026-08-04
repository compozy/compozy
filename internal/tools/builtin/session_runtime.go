package builtin

import (
	"encoding/json"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func sessionRuntimeSetDescriptor() toolspkg.Descriptor {
	descriptor := nativeDescriptor(
		toolspkg.ToolIDSessionRuntimeSet,
		"session_runtime_set",
		"Session Runtime Set",
		"Select the default runtime for future prompts without starting or reconfiguring the session.",
		sessionRuntimeSetInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, sessionsRuntimeKey, "set"},
		[]string{"select session runtime", "set default runtime"},
	)
	descriptor.OutputSchema = json.RawMessage(sessionCreateOutputSchema)
	return descriptor
}

func sessionRuntimeClearDescriptor() toolspkg.Descriptor {
	descriptor := nativeDescriptor(
		toolspkg.ToolIDSessionRuntimeClear,
		"session_runtime_clear",
		"Session Runtime Clear",
		"Clear the default runtime for future prompts without starting or reconfiguring the session.",
		sessionRuntimeClearInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, sessionsRuntimeKey, "clear"},
		[]string{"clear session runtime", "clear default runtime"},
	)
	descriptor.OutputSchema = json.RawMessage(sessionCreateOutputSchema)
	return descriptor
}

const sessionRuntimeSelectionSchema = `{
	"type":"object",
	"required":["provider"],
	"properties":{
		"provider":{"type":"string","minLength":1},
		"model":{"type":"string"},
		"reasoning_effort":{"type":"string","enum":["none","minimal","low","medium","high","xhigh","max"]},
		"speed":{"type":"string","enum":["normal","fast"]}
	},
	"additionalProperties":false
}`

const sessionRuntimeSetInputSchema = `{
	"type":"object",
	"required":["session_id","runtime"],
	"properties":{
		"workspace":{"type":"string"},
		"session_id":{"type":"string","minLength":1},
		"runtime":` + sessionRuntimeSelectionSchema + `,
		"expected_revision":{"type":"integer","minimum":0}
	},
	"additionalProperties":false
}`

const sessionRuntimeClearInputSchema = `{
	"type":"object",
	"required":["session_id"],
	"properties":{
		"workspace":{"type":"string"},
		"session_id":{"type":"string","minLength":1},
		"expected_revision":{"type":"integer","minimum":0}
	},
	"additionalProperties":false
}`
