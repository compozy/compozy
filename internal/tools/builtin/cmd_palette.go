package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

func cmdPaletteDescriptors() []toolspkg.Descriptor {
	return []toolspkg.Descriptor{
		nativeDescriptor(
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
			[]string{"commands", "palette", hooksCatalogKey},
			[]string{"list command palette", "available commands"},
		),
		nativeDescriptor(
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
		),
	}
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
