package builtin

import (
	"encoding/json"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func cmdPaletteDescriptors() []toolspkg.Descriptor {
	list := nativeDescriptor(
		toolspkg.ToolIDCmdPaletteList,
		"cmd_palette_list",
		"Command Palette List",
		"List command palette commands with their current availability.",
		cmdPaletteListInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDCatalog},
		[]string{"commands", "palette", descriptorKeywordCatalog},
		[]string{"list command palette", "available commands"},
	)
	list.OutputSchema = json.RawMessage(cmdPaletteListOutputSchema)
	invoke := nativeDescriptor(
		toolspkg.ToolIDCmdPaletteInvoke,
		"cmd_palette_invoke",
		"Command Palette Invoke",
		"Invoke one command palette command through its declared execution policy.",
		cmdPaletteInvokeInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDCatalog},
		[]string{"commands", "palette", "invoke"},
		[]string{"run command palette command", "invoke command"},
	)
	invoke.OutputSchema = json.RawMessage(cmdPaletteInvokeOutputSchema)
	return []toolspkg.Descriptor{list, invoke}
}

const cmdPaletteListInputSchema = `{
	"type":"object",
	"properties":{
		"workspace":{"type":"string"},
		"source":{"type":"string"},
		"client":{"type":"string"}
	},
	"additionalProperties":false
}`

const cmdPaletteInvokeInputSchema = `{
	"type":"object",
	"required":["id"],
	"properties":{
		"id":{"type":"string"},
		"workspace":{"type":"string"},
		"args":{"type":"object"},
		"client":{"type":"string"}
	},
	"additionalProperties":false
}`

const cmdPaletteListOutputSchema = `{
	"type":"object",
	"required":["commands"],
	"properties":{
		"commands":{"type":"array","items":{"type":"object"}}
	},
	"additionalProperties":false
}`

const cmdPaletteInvokeOutputSchema = `{
	"type":"object",
	"required":["status"],
	"properties":{
		"status":{"type":"string"},
		"result":{"type":"object","additionalProperties":true},
		"approval_id":{"type":"string"},
		"invocation_id":{"type":"string"}
	},
	"additionalProperties":false
}`
