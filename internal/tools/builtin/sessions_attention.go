package builtin

import (
	"encoding/json"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func sessionAttentionDescriptors() []toolspkg.Descriptor {
	descriptor := nativeDescriptor(
		toolspkg.ToolIDNotify,
		"notify",
		"Notify Operator",
		"Send one sanitized, rate-limited notification from the bound session to live operator clients.",
		notifyInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, "notify", descriptorKeywordAgent},
		[]string{"notify operator", "operator notification"},
	)
	descriptor.OutputSchema = json.RawMessage(notifyOutputSchema)
	return []toolspkg.Descriptor{descriptor}
}

const notifyInputSchema = `{
	"type":"object",
	"required":["title"],
	"properties":{
		"title":{"type":"string","minLength":1,"maxLength":80},
		"body":{"type":"string","maxLength":240}
	},
	"additionalProperties":false
}`

const notifyOutputSchema = `{
	"type":"object",
	"required":["outcome"],
	"properties":{
		"outcome":{"type":"string","enum":["delivered","no-client","muted-workspace","rate-limited"]},
		"retry_after_ms":{"type":"integer","minimum":0}
	},
	"additionalProperties":false
}`
