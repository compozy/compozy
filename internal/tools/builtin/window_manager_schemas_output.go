package builtin

const windowManagerChangesSchema = `{
	"type":"object",
	"properties":{
		"desktop_ids":{"type":"array","items":{"type":"string"}},
		"window_ids":{"type":"array","items":{"type":"string"}},
		"group_ids":{"type":"array","items":{"type":"string"}},
 "tab_ids":{"type":"array","items":{"type":"string"}},
 "tab_split":{"type":"array","items":{"type":"string"}},
 "tab_unsplit":{"type":"array","items":{"type":"string"}},
		"node_ids":{"type":"array","items":{"type":"string"}},
		"client_ids":{"type":"array","items":{"type":"string"}},
		"stack_grouped":{"type":"array","items":{"type":"string"}},
		"stack_ungrouped":{"type":"array","items":{"type":"string"}}
	},
	"additionalProperties":false
}`

const windowManagerDiagnosticsSchema = `{
	"type":"array",
	"items":{
		"type":"object",
		"properties":{
			"code":{"type":"string"},
			"path":{"type":"string"},
			"message":{"type":"string"}
		},
		"required":["code","message"],
		"additionalProperties":false
	}
}`

const windowManagerReadOutputSchema = `{
	"type":"object",
	"properties":{
		"workspace_id":{"type":"string"},
		"revision":{"type":"integer","minimum":0},
		"desktops":{"type":"array"},
		"windows":{"type":"array"},
		"clients":{"type":"array"},
		"snapshot":{"type":"object"},
		"document":{"type":"object"},
		"valid":{"type":"boolean"},
		"deprecations":` + windowManagerDeprecationsSchema + `,
		"diagnostics":` + windowManagerDiagnosticsSchema + `
	},
	"required":["workspace_id","revision"],
	"additionalProperties":false
}`

const windowManagerCommandOutputSchema = `{
	"type":"object",
	"properties":{
		"workspace_id":{"type":"string"},
		"command_id":{"type":"string"},
		"revision":{"type":"integer","minimum":0},
		"applied":{"type":"boolean"},
		"changes":` + windowManagerChangesSchema + `,
		"diagnostics":` + windowManagerDiagnosticsSchema + `,
 "deprecations":` + windowManagerDeprecationsSchema + `,
		"client":{"type":"object"},
		"rebased_from":{"type":"integer","minimum":0}
	},
	"required":["workspace_id","command_id","revision","applied","changes","diagnostics","deprecations"],
	"additionalProperties":false
}`

const windowManagerPreviewOutputSchema = `{
	"type":"object",
	"properties":{
		"workspace_id":{"type":"string"},
		"command_id":{"type":"string"},
		"revision":{"type":"integer","minimum":0},
		"changed":{"type":"boolean"},
		"changes":` + windowManagerChangesSchema + `,
		"diagnostics":` + windowManagerDiagnosticsSchema + `,
 "deprecations":` + windowManagerDeprecationsSchema + `,
		"client":{"type":"object"},
		"snapshot":{"type":"object"}
	},
	"required":["workspace_id","command_id","revision","changed","changes","diagnostics","deprecations","snapshot"],
	"additionalProperties":false
}`

const windowManagerDeprecationsSchema = `{
 "type":"array","items":{"type":"object","properties":{
 "code":{"type":"string"},"surface":{"type":"string"},"replacement":{"type":"string"},"removal_release":{"type":"string"},"message":{"type":"string"}},
 "required":["code","surface","removal_release","message"],"additionalProperties":false}
}`
