package builtin

const windowManagerWindowGroupInputSchema = `{
	"type":"object",
	"properties":{
		` + windowManagerWorkspaceSchema + `,
		` + windowManagerRevisionSchema + `,
		` + windowManagerClientSchema + `,
		"target_window_id":{"type":"string","minLength":1},
		"window_ids":{"type":"array","minItems":1,"uniqueItems":true,"items":{"type":"string","minLength":1}},
		"insert_index":{"type":"integer","minimum":0},
		"rebase":` + windowManagerRebaseSchema + `,
		` + windowManagerOriginSchema + `
	},
	"required":["expected_revision","target_window_id","window_ids"],
	"additionalProperties":false
}`

const windowManagerWindowReorderInputSchema = `{
	"type":"object",
	"properties":{
		` + windowManagerWorkspaceSchema + `,
		` + windowManagerRevisionSchema + `,
		` + windowManagerClientSchema + `,
		"window_id":{"type":"string","minLength":1},
		"index":{"type":"integer","minimum":0},
		"rebase":` + windowManagerRebaseSchema + `,
		` + windowManagerOriginSchema + `
	},
	"required":["expected_revision","window_id","index"],
	"additionalProperties":false
}`

const windowManagerWindowActivateInputSchema = `{
	"type":"object",
	"properties":{
		` + windowManagerWorkspaceSchema + `,
		` + windowManagerRevisionSchema + `,
		` + windowManagerClientSchema + `,
		"window_id":{"type":"string","minLength":1},
		"rebase":` + windowManagerRebaseSchema + `,
		` + windowManagerOriginSchema + `
	},
	"required":["expected_revision","window_id"],
	"additionalProperties":false
}`

const windowManagerWindowPinInputSchema = `{
	"type":"object",
	"properties":{
		` + windowManagerWorkspaceSchema + `,
		` + windowManagerRevisionSchema + `,
		` + windowManagerClientSchema + `,
		"window_id":{"type":"string","minLength":1},
		"pinned":{"type":"boolean"},
		"rebase":` + windowManagerRebaseSchema + `,
		` + windowManagerOriginSchema + `
	},
	"required":["expected_revision","window_id","pinned"],
	"additionalProperties":false
}`

const windowManagerWindowReopenInputSchema = `{
	"type":"object",
	"properties":{
		` + windowManagerWorkspaceSchema + `,
		` + windowManagerRevisionSchema + `,
		` + windowManagerClientSchema + `,
		"rebase":` + windowManagerRebaseSchema + `,
		` + windowManagerOriginSchema + `
	},
	"required":["expected_revision"],
	"additionalProperties":false
}`
