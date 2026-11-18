package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

const extensionsRestoreKey = "restore"

// extensionBuiltinTools restore a permanently removed built-in and list those removals. Permanent removal
// has no tool on purpose (Q5 a): an agent can recover a built-in but cannot delete one permanently.
var extensionBuiltinTools = []toolspkg.Descriptor{
	nativeExtensionDescriptorWithOutput(
		toolspkg.ToolIDExtensionsRestore,
		"extensions_restore",
		"Extensions Restore",
		"Restore a permanently removed built-in extension at its bundled version, enabled.",
		extensionBuiltinRestoreInputSchema,
		extensionBuiltinRestoreOutputSchema,
		toolspkg.RiskMutating,
		false,
		[]string{extensionsExtensionsKey, extensionsRestoreKey, "builtin", extensionsMutationKey},
		[]string{"restore builtin extension", "restore built-in extension"},
	),
	nativeExtensionDescriptorWithOutput(
		toolspkg.ToolIDExtensionsRemovedList,
		"extensions_removed_list",
		"Extensions Removed List",
		"List built-in extensions that were removed permanently and can be restored.",
		extensionBuiltinRemovedListInputSchema,
		extensionBuiltinRemovedListOutputSchema,
		toolspkg.RiskRead,
		true,
		[]string{extensionsExtensionsKey, "builtin", "removed", extensionsCatalogKey},
		[]string{"removed builtin extensions", "removed built-in extensions"},
	),
}

const extensionBuiltinRestoreInputSchema = `{
	"type":"object",
	"required":["name"],
	"properties":{
		"name":{"type":"string","minLength":1,"description":"Built-in extension name to restore."}
	},
	"additionalProperties":false
}`

const extensionBuiltinRemovedListInputSchema = `{
	"type":"object",
	"properties":{},
	"additionalProperties":false
}`

const extensionBuiltinRestoreOutputSchema = `{
	"type":"object",
	"required":["name","status","source","version","enabled","path"],
	"properties":{
		"name":{"type":"string"},
		"status":{"type":"string","enum":["restored"]},
		"source":{"type":"string"},
		"version":{"type":"string"},
		"enabled":{"type":"boolean"},
		"path":{"type":"string"}
	},
	"additionalProperties":false
}`

const extensionBuiltinRemovedListOutputSchema = `{
	"type":"object",
	"required":["removed"],
	"properties":{
		"removed":{
			"type":"array",
			"items":{
				"type":"object",
				"required":["name","removed_at","removed_by"],
				"properties":{
					"name":{"type":"string"},
					"removed_at":{"type":"string"},
					"removed_by":{"type":"string"}
				},
				"additionalProperties":false
			}
		}
	},
	"additionalProperties":false
}`
