package builtin

const windowManagerWorkspaceSchema = `"workspace_id":{"type":"string","minLength":1}`

const windowManagerRevisionSchema = `"expected_revision":{"type":"integer","minimum":0}`

const windowManagerClientSchema = `"client_id":{"type":"string","minLength":1}`

const windowManagerOriginSchema = `"origin":{"type":"string","maxLength":256}`

const windowManagerRectSchema = `{
	"type":"object",
	"properties":{
		"x":{"type":"number","minimum":0,"maximum":1},
		"y":{"type":"number","minimum":0,"maximum":1},
		"width":{"type":"number","exclusiveMinimum":0,"maximum":1},
		"height":{"type":"number","exclusiveMinimum":0,"maximum":1}
	},
	"required":["x","y","width","height"],
	"additionalProperties":false
}`

const windowManagerRouteSchema = `{
	"type":"object",
	"properties":{
		"pathname":{"type":"string","pattern":"^/(?:$|[^/?#][^?#]*)$"},
		"search":{"type":"object","additionalProperties":true}
	},
	"required":["pathname","search"],
	"additionalProperties":false
}`

const windowManagerRebaseSchema = `{
	"type":"object",
	"properties":{
		"window_id":{"type":"string","minLength":1},
		"source_node_id":{"type":"string","minLength":1},
		"target_node_id":{"type":"string","minLength":1},
		"split_id":{"type":"string","minLength":1},
		"boundary_index":{"type":"integer","minimum":0}
	},
	"additionalProperties":false
}`

const windowManagerReadInputSchema = `{
	"type":"object",
	"properties":{` + windowManagerWorkspaceSchema + `},
	"additionalProperties":false
}`

const windowManagerDesktopCreateInputSchema = `{
	"type":"object",
	"properties":{
		` + windowManagerWorkspaceSchema + `,
		` + windowManagerRevisionSchema + `,
		"desktop_id":{"type":"string","minLength":1},
		"name":{"type":"string","minLength":1,"maxLength":128},
		"purpose":{"type":"string","enum":["standard","focus"]},
		"focus_owner":{"type":"string","minLength":1},
		"after_id":{"type":"string","minLength":1},
		` + windowManagerOriginSchema + `
	},
	"required":["expected_revision"],
	"additionalProperties":false
}`

const windowManagerDesktopUpdateInputSchema = `{
	"type":"object",
	"properties":{
		` + windowManagerWorkspaceSchema + `,
		` + windowManagerRevisionSchema + `,
		"desktop_id":{"type":"string","minLength":1},
		"name":{"type":"string","minLength":1,"maxLength":128},
		` + windowManagerOriginSchema + `
	},
	"required":["expected_revision","desktop_id","name"],
	"additionalProperties":false
}`

const windowManagerDesktopReorderInputSchema = `{
	"type":"object",
	"properties":{
		` + windowManagerWorkspaceSchema + `,
		` + windowManagerRevisionSchema + `,
		"desktop_id":{"type":"string","minLength":1},
		"order":{"type":"integer","minimum":0},
		` + windowManagerOriginSchema + `
	},
	"required":["expected_revision","desktop_id","order"],
	"additionalProperties":false
}`

const windowManagerDesktopSwitchInputSchema = `{
	"type":"object",
	"properties":{
		` + windowManagerWorkspaceSchema + `,
		` + windowManagerRevisionSchema + `,
		` + windowManagerClientSchema + `,
		"desktop_id":{"type":"string","minLength":1},
		` + windowManagerOriginSchema + `
	},
	"required":["expected_revision","client_id","desktop_id"],
	"additionalProperties":false
}`

const windowManagerDesktopDeleteInputSchema = `{
	"type":"object",
	"properties":{
		` + windowManagerWorkspaceSchema + `,
		` + windowManagerRevisionSchema + `,
		"desktop_id":{"type":"string","minLength":1},
		"destination_id":{"type":"string","minLength":1},
		` + windowManagerOriginSchema + `
	},
	"required":["expected_revision","desktop_id"],
	"additionalProperties":false
}`

const windowManagerWindowListInputSchema = `{
	"type":"object",
	"properties":{
		` + windowManagerWorkspaceSchema + `,
		"desktop_id":{"type":"string","minLength":1},
		"include_minimized":{"type":"boolean"}
	},
	"additionalProperties":false
}`

const windowManagerWindowOpenInputSchema = `{
	"type":"object",
	"properties":{
		` + windowManagerWorkspaceSchema + `,
		` + windowManagerRevisionSchema + `,
		` + windowManagerClientSchema + `,
		"window_id":{"type":"string","minLength":1},
		"app":{"type":"string","minLength":1,"maxLength":256},
		"instance_key":{"type":"string","minLength":1,"maxLength":256},
		"desktop_id":{"type":"string","minLength":1},
		"route":` + windowManagerRouteSchema + `,
		"floating_rect":` + windowManagerRectSchema + `,
		"insert_tiled":{"type":"boolean"},
		"restore_window_id":{"type":"string","minLength":1},
		"rebase":` + windowManagerRebaseSchema + `,
		` + windowManagerOriginSchema + `
	},
	"required":["expected_revision"],
	"additionalProperties":false
}`

const windowManagerWindowNavigateInputSchema = `{
	"type":"object",
	"properties":{
		` + windowManagerWorkspaceSchema + `,
		` + windowManagerRevisionSchema + `,
		` + windowManagerClientSchema + `,
		"window_id":{"type":"string","minLength":1},
		"route":` + windowManagerRouteSchema + `,
		"rebase":` + windowManagerRebaseSchema + `,
		` + windowManagerOriginSchema + `
	},
	"required":["expected_revision","window_id","route"],
	"additionalProperties":false
}`

const windowManagerWindowCloseInputSchema = `{
	"type":"object",
	"properties":{
		` + windowManagerWorkspaceSchema + `,
		` + windowManagerRevisionSchema + `,
		"window_id":{"type":"string","minLength":1},
		"minimize":{"type":"boolean"},
		"rebase":` + windowManagerRebaseSchema + `,
		` + windowManagerOriginSchema + `
	},
	"required":["expected_revision","window_id"],
	"additionalProperties":false
}`

const windowManagerWindowFocusInputSchema = `{
	"type":"object",
	"properties":{
		` + windowManagerWorkspaceSchema + `,
		` + windowManagerRevisionSchema + `,
		` + windowManagerClientSchema + `,
		"window_id":{"type":"string","minLength":1},
		"direction":{"type":"string","enum":["left","right","up","down"]},
		` + windowManagerOriginSchema + `
	},
	"required":["expected_revision","client_id"],
	"additionalProperties":false
}`

const windowManagerWindowMoveInputSchema = `{
	"type":"object",
	"properties":{
		` + windowManagerWorkspaceSchema + `,
		` + windowManagerRevisionSchema + `,
		` + windowManagerClientSchema + `,
		"window_id":{"type":"string","minLength":1},
		"destination_desktop_id":{"type":"string","minLength":1},
		"target_window_id":{"type":"string","minLength":1},
		"placement":{"type":"string","enum":["floating","before","after","left","right","top","bottom","center"]},
		"floating_rect":` + windowManagerRectSchema + `,
		"move_group":{"type":"boolean"},
		"rebase":` + windowManagerRebaseSchema + `,
		` + windowManagerOriginSchema + `
	},
	"required":["expected_revision","window_id","destination_desktop_id"],
	"if":{
		"properties":{"move_group":{"enum":[true]}},
		"required":["move_group"]
	},
	"then":{
		"not":{"anyOf":[
			{"required":["target_window_id"]},
			{"required":["placement"]},
			{"required":["floating_rect"]}
		]}
	},
	"else":{"required":["placement"]},
	"additionalProperties":false
}`

const windowManagerWindowSwapInputSchema = `{
	"type":"object",
	"properties":{
		` + windowManagerWorkspaceSchema + `,
		` + windowManagerRevisionSchema + `,
		"first_window_id":{"type":"string","minLength":1},
		"second_window_id":{"type":"string","minLength":1},
		"rebase":` + windowManagerRebaseSchema + `,
		` + windowManagerOriginSchema + `
	},
	"required":["expected_revision","first_window_id","second_window_id"],
	"additionalProperties":false
}`

const windowManagerWindowFloatInputSchema = `{
	"type":"object",
	"properties":{
		` + windowManagerWorkspaceSchema + `,
		` + windowManagerRevisionSchema + `,
		"window_id":{"type":"string","minLength":1},
		"floating_rect":` + windowManagerRectSchema + `,
		"rebase":` + windowManagerRebaseSchema + `,
		` + windowManagerOriginSchema + `
	},
	"required":["expected_revision","window_id"],
	"additionalProperties":false
}`

const windowManagerWindowZoomInputSchema = `{
	"type":"object",
	"properties":{
		` + windowManagerWorkspaceSchema + `,
		` + windowManagerRevisionSchema + `,
		` + windowManagerClientSchema + `,
		"window_id":{"type":"string","minLength":1},
		"rebase":` + windowManagerRebaseSchema + `,
		` + windowManagerOriginSchema + `
	},
	"required":["expected_revision","client_id","window_id"],
	"additionalProperties":false
}`

const windowManagerLayoutPreviewInputSchema = `{
	"type":"object",
	"properties":{
		` + windowManagerWorkspaceSchema + `,
		` + windowManagerRevisionSchema + `,
		` + windowManagerClientSchema + `,
		"command_id":{"type":"string","enum":[
			"desktop.create","desktop.update","desktop.reorder","desktop.switch","desktop.delete",
			"window.open","window.navigate","window.close","window.focus","window.move","window.swap","window.toggle_floating",
			"window.zoom","layout.arrange","layout.resize","layout.balance","layout.undo","layout.redo"
		]},
		"payload":{"type":"object"},
		"rebase":` + windowManagerRebaseSchema + `,
		` + windowManagerOriginSchema + `
	},
	"required":["expected_revision","command_id","payload"],
	"if":{
		"properties":{"command_id":{"enum":["desktop.switch","window.focus","window.zoom"]}},
		"required":["command_id"]
	},
	"then":{"required":["client_id"]},
	"additionalProperties":false
}`

const windowManagerLayoutArrangeInputSchema = `{
	"type":"object",
	"properties":{
		` + windowManagerWorkspaceSchema + `,
		` + windowManagerRevisionSchema + `,
		"desktop_id":{"type":"string","minLength":1},
		"window_ids":{"type":"array","minItems":1,"uniqueItems":true,"items":{"type":"string","minLength":1}},
		"arrangement":{"type":"string","enum":["horizontal","vertical","grid","stack"]},
		"frame":` + windowManagerRectSchema + `,
		"group_id":{"type":"string","minLength":1},
		"resource_id":{"type":"string","minLength":1},
		"rebase":` + windowManagerRebaseSchema + `,
		` + windowManagerOriginSchema + `
	},
	"required":["expected_revision"],
	"additionalProperties":false
}`

const windowManagerLayoutResizeInputSchema = `{
	"type":"object",
	"properties":{
		` + windowManagerWorkspaceSchema + `,
		` + windowManagerRevisionSchema + `,
		"split_id":{"type":"string","minLength":1},
		"boundary_index":{"type":"integer","minimum":0},
		"delta":{"type":"number","exclusiveMinimum":-1,"exclusiveMaximum":1},
		"rebase":` + windowManagerRebaseSchema + `,
		` + windowManagerOriginSchema + `
	},
	"required":["expected_revision","split_id","boundary_index","delta"],
	"additionalProperties":false
}`

const windowManagerLayoutBalanceInputSchema = `{
	"type":"object",
	"properties":{
		` + windowManagerWorkspaceSchema + `,
		` + windowManagerRevisionSchema + `,
		"group_id":{"type":"string","minLength":1},
		"split_id":{"type":"string","minLength":1},
		"rebase":` + windowManagerRebaseSchema + `,
		` + windowManagerOriginSchema + `
	},
	"required":["expected_revision"],
	"additionalProperties":false
}`

const windowManagerLayoutHistoryInputSchema = `{
	"type":"object",
	"properties":{
		` + windowManagerWorkspaceSchema + `,
		` + windowManagerRevisionSchema + `,
		` + windowManagerOriginSchema + `
	},
	"required":["expected_revision"],
	"additionalProperties":false
}`

const windowManagerLayoutDocumentSchema = `{
	"type":"object",
	"properties":{
		"version":{"type":"integer","const":2},
		"workspace_id":{"type":"string","minLength":1},
		"desktops":{"type":"array","minItems":1,"items":{"type":"object"}},
		"windows":{"type":"object"},
		"overrides":{"type":"object"}
	},
	"required":["version","workspace_id","desktops","windows"],
	"additionalProperties":false
}`

const windowManagerLayoutValidateInputSchema = `{
	"type":"object",
	"properties":{
		` + windowManagerWorkspaceSchema + `,
		"document":` + windowManagerLayoutDocumentSchema + `
	},
	"required":["document"],
	"additionalProperties":false
}`

const windowManagerLayoutApplyInputSchema = `{
	"type":"object",
	"properties":{
		` + windowManagerWorkspaceSchema + `,
		` + windowManagerRevisionSchema + `,
		` + windowManagerClientSchema + `,
		"document":` + windowManagerLayoutDocumentSchema + `,
		` + windowManagerOriginSchema + `
	},
	"required":["expected_revision","document"],
	"additionalProperties":false
}`

const windowManagerChangesSchema = `{
	"type":"object",
	"properties":{
		"desktop_ids":{"type":"array","items":{"type":"string"}},
		"window_ids":{"type":"array","items":{"type":"string"}},
		"group_ids":{"type":"array","items":{"type":"string"}},
		"node_ids":{"type":"array","items":{"type":"string"}},
		"client_ids":{"type":"array","items":{"type":"string"}}
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
		"client":{"type":"object"},
		"rebased_from":{"type":"integer","minimum":0}
	},
	"required":["workspace_id","command_id","revision","applied","changes","diagnostics"],
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
		"client":{"type":"object"},
		"snapshot":{"type":"object"}
	},
	"required":["workspace_id","command_id","revision","changed","changes","diagnostics","snapshot"],
	"additionalProperties":false
}`
