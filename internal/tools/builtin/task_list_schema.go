package builtin

const taskListInputSchema = `{
	"type":"object",
	"properties":{
		"scope":{"type":"string"},
		"workspace_id":{"type":"string"},
		"status":{"type":"string"},
		"priority":{"type":"string"},
		"include_drafts":{"type":"boolean"},
		"approval_state":{"type":"string"},
		"owner_kind":{"type":"string"},
		"owner_ref":{"type":"string"},
		"parent_task_id":{"type":"string"},
		"participation_channel":{"type":"string"},
		"search":{"type":"string"},
		"sort":{"type":"string","enum":["recent","priority"]},
		"cursor":{"type":"string"},
		"limit":{"type":"integer"}
	},
	"additionalProperties":false
}`
