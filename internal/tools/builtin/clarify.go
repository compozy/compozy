package builtin

import (
	"encoding/json"

	toolspkg "github.com/compozy/agh/internal/tools"
)

const clarifyInputSchema = `{
	"type":"object",
	"required":["question"],
	"properties":{
		"question":{"type":"string","minLength":1},
		"choices":{
			"type":"array",
			"maxItems":4,
			"uniqueItems":true,
			"items":{"type":"string","minLength":1}
		}
	},
	"additionalProperties":false
}`

const clarifyOutputSchema = `{
	"type":"object",
	"required":["choice","text","fallback"],
	"properties":{
		"choice":{"type":["integer","null"],"minimum":0},
		"text":{"type":"string"},
		"fallback":{"type":"boolean"}
	},
	"additionalProperties":false
}`

func clarifyDescriptors() []toolspkg.Descriptor {
	descriptor := nativeDescriptor(
		toolspkg.ToolIDClarify,
		"clarify",
		"Clarify",
		"Ask the user one bounded question and wait for the answer or configured timeout.",
		clarifyInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDClarify},
		[]string{"clarify", "question", "human input"},
		[]string{"ask user", "choose option", "request clarification"},
	)
	descriptor.OutputSchema = json.RawMessage(clarifyOutputSchema)
	descriptor.RequiresInteraction = false
	return []toolspkg.Descriptor{descriptor}
}
