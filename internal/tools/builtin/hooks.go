package builtin

import toolspkg "github.com/compozy/agh/internal/tools"

const (
	hooksHooksKey = "hooks"
)

const (
	hooksCatalogKey  = "catalog"
	hooksEventsKey   = "events"
	hooksMutationKey = "mutation"
	hooksRunsKey     = "runs"
)

var hookTools = []toolspkg.Descriptor{
	nativeDescriptor(
		toolspkg.ToolIDHooksList,
		"hooks_list",
		"Hooks List",
		"List resolved hooks through the live hook catalog.",
		hooksListInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDHooks},
		[]string{hooksHooksKey, hooksCatalogKey},
		[]string{"hook catalog", "resolved hooks"},
	),
	nativeDescriptor(
		toolspkg.ToolIDHooksInfo,
		"hooks_info",
		"Hooks Info",
		"Read one resolved hook by name.",
		hooksInfoInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDHooks},
		[]string{hooksHooksKey, hooksCatalogKey},
		[]string{"hook details", "hook info"},
	),
	nativeDescriptor(
		toolspkg.ToolIDHooksEvents,
		"hooks_events",
		"Hooks Events",
		"List supported hook events and payload metadata.",
		hooksEventsInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDHooks},
		[]string{hooksHooksKey, hooksEventsKey},
		[]string{"hook events", "event catalog"},
	),
	nativeDescriptor(
		toolspkg.ToolIDHooksRuns,
		"hooks_runs",
		"Hooks Runs",
		"List hook run audit records.",
		hooksRunsInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDHooks},
		[]string{hooksHooksKey, hooksRunsKey, "audit"},
		[]string{"hook run history", "hook audit"},
	),
	nativeDescriptor(
		toolspkg.ToolIDHooksCreate,
		"hooks_create",
		"Hooks Create",
		"Create one config-backed hook declaration through the validated config writer.",
		hooksCreateInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDHooks},
		[]string{hooksHooksKey, hooksMutationKey},
		[]string{"create hook", "add hook declaration"},
	),
	nativeDescriptor(
		toolspkg.ToolIDHooksUpdate,
		"hooks_update",
		"Hooks Update",
		"Update one config-backed hook declaration through the validated config writer.",
		hooksUpdateInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDHooks},
		[]string{hooksHooksKey, hooksMutationKey},
		[]string{"update hook", "edit hook declaration"},
	),
	nativeDescriptor(
		toolspkg.ToolIDHooksDelete,
		"hooks_delete",
		"Hooks Delete",
		"Delete one config-backed hook declaration through the validated config writer.",
		hooksNameMutationInputSchema,
		toolspkg.RiskDestructive,
		false,
		true,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDHooks},
		[]string{hooksHooksKey, hooksMutationKey},
		[]string{"delete hook", "remove hook declaration"},
	),
	nativeDescriptor(
		toolspkg.ToolIDHooksEnable,
		"hooks_enable",
		"Hooks Enable",
		"Enable one config-backed hook declaration through the validated config writer.",
		hooksNameMutationInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDHooks},
		[]string{hooksHooksKey, hooksMutationKey},
		[]string{"enable hook", "activate hook declaration"},
	),
	nativeDescriptor(
		toolspkg.ToolIDHooksDisable,
		"hooks_disable",
		"Hooks Disable",
		"Disable one config-backed hook declaration through the validated config writer.",
		hooksNameMutationInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDHooks},
		[]string{hooksHooksKey, hooksMutationKey},
		[]string{"disable hook", "deactivate hook declaration"},
	),
}

func hookDescriptors() []toolspkg.Descriptor {
	return hookTools
}

const hooksListInputSchema = `{
	"type":"object",
	"properties":{
		"workspace_root":{"type":"string"},
		"workspace_id":{"type":"string"},
		"agent":{"type":"string"},
		"event":{"type":"string"},
		"source":{"type":"string"},
		"mode":{"type":"string"}
	},
	"additionalProperties":false
}`

const hooksInfoInputSchema = `{
	"type":"object",
	"required":["name"],
	"properties":{
		"name":{"type":"string"},
		"workspace_root":{"type":"string"},
		"workspace_id":{"type":"string"},
		"agent":{"type":"string"},
		"event":{"type":"string"},
		"source":{"type":"string"},
		"mode":{"type":"string"}
	},
	"additionalProperties":false
}`

const hooksEventsInputSchema = `{
	"type":"object",
	"properties":{
		"family":{"type":"string"},
		"sync_only":{"type":"boolean"}
	},
	"additionalProperties":false
}`

const hooksRunsInputSchema = `{
	"type":"object",
	"required":["workspace_id","session_id"],
	"properties":{
		"workspace_id":{"type":"string"},
		"session_id":{"type":"string"},
		"event":{"type":"string"},
		"outcome":{"type":"string"},
		"since":{"type":"string"},
		"last":{"type":"integer"}
	},
	"additionalProperties":false
}`

const hooksCreateInputSchema = `{
	"type":"object",
	"required":["name","event","command"],
	"properties":` + hooksMutationProperties + `,
	"additionalProperties":false
}`

const hooksUpdateInputSchema = `{
	"type":"object",
	"required":["name"],
	"properties":` + hooksMutationProperties + `,
	"additionalProperties":false
}`

const hooksNameMutationInputSchema = `{
	"type":"object",
	"required":["name"],
	"properties":{
		"name":{"type":"string"},
		"scope":{"type":"string"},
		"workspace_root":{"type":"string"}
	},
	"additionalProperties":false
}`

const hooksMutationProperties = `{
	"name":{"type":"string"},
	"scope":{"type":"string"},
	"workspace_root":{"type":"string"},
	"event":{"type":"string"},
	"mode":{"type":"string"},
	"required":{"type":"boolean"},
	"priority":{"type":"integer"},
	"timeout":{"type":"string"},
	"matcher":{},
	"command":{"type":"string"},
	"args":{},
	"env":{},
	"secret_env":{},
	"enabled":{"type":"boolean"},
	"source":{"type":"string"}
}`
