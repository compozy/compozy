package builtin

const taskListInputSchema = `{
	"type":"object",
	"properties":{
		"scope":{"type":"string"},
		"workspace":{"type":"string"},
		"status":{"type":"string"},
		"priority":{"type":"string"},
		"include_drafts":{"type":"boolean"},
		"include_loop":{"type":"boolean","description":"Include Loop execution records; omitted means false."},
		"loop_run_id":{"type":"string","description":"Filter to one Loop run; this also includes Loop records."},
		"approval_state":{"type":"string"},
		"owner_kind":{"type":"string"},
		"owner_ref":{"type":"string"},
		"parent_task_id":{"type":"string"},
		"worktree":{"type":"string"},
		"participation_channel":{"type":"string"},
		"search":{"type":"string"},
		"sort":{"type":"string","enum":["recent","priority"]},
		"cursor":{"type":"string"},
		"limit":{"type":"integer"}
	},
	"additionalProperties":false
}`
