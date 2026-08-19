package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var loopRequestTools = []toolspkg.Descriptor{
	nativeLoopDescriptor(toolspkg.ToolIDLoopRequests, "loop_requests", "Loop Requests",
		"List pending or resolved human requests in the caller workspace.", loopRequestsInputSchema,
		toolspkg.RiskRead, true, false, []string{loopKey, "requests", descriptorKeywordStatus},
		[]string{"human requests", "pending loop requests"}),
	nativeLoopDescriptor(toolspkg.ToolIDLoopRequest, "loop_request", "Loop Request",
		"Read one human request with its full redacted context.", loopRequestInputSchema,
		toolspkg.RiskRead, true, false, []string{loopKey, "request", descriptorKeywordStatus},
		[]string{"inspect loop request", "request context"}),
	nativeLoopDescriptor(toolspkg.ToolIDLoopRespond, "loop_respond", "Loop Respond",
		"Admit one validated answer or review decision for a human request.", loopRespondInputSchema,
		toolspkg.RiskMutating, false, false, []string{loopKey, "request", "respond"},
		[]string{"answer loop request", "respond to ask", "review loop action"}),
	nativeLoopDescriptor(toolspkg.ToolIDLoopNodeAmend, "loop_node_amend", "Loop Node Amend",
		"Append a validated amendment to one parked Loop node output.", loopNodeAmendInputSchema,
		toolspkg.RiskMutating, false, false, []string{loopKey, "node", "amend"},
		[]string{"amend loop output", "correct parked node"}),
}

const loopRequestsInputSchema = `{"type":"object","additionalProperties":false,"properties":{"workspace":{"type":"string"},"run_id":{"type":"string"},"state":{"type":"string","enum":["pending","resolved"]},"cursor":{"type":"string"},"limit":{"type":"integer","minimum":1,"maximum":200}}}`

const loopRequestInputSchema = `{"type":"object","required":["run_id","generation","node_id"],"additionalProperties":false,"properties":{"workspace":{"type":"string"},"run_id":{"type":"string","minLength":1},"generation":{"type":"integer","minimum":1},"node_id":{"type":"string","minLength":1},"item_index":{"type":"integer","minimum":0}}}`

const loopRespondInputSchema = `{"type":"object","required":["run_id","generation","node_id"],"additionalProperties":false,"properties":{"workspace":{"type":"string"},"run_id":{"type":"string","minLength":1},"generation":{"type":"integer","minimum":1},"node_id":{"type":"string","minLength":1},"item_index":{"type":"integer","minimum":0},"decision":{"type":"string","enum":["respond","approve","edit","reject"]},"payload":{"description":"Required for ask, respond, and edit decisions; omitted for approve and reject."},"note":{"type":"string"}}}`

const loopNodeAmendInputSchema = `{"type":"object","required":["run_id","node_id","payload"],"additionalProperties":false,"properties":{"workspace":{"type":"string"},"run_id":{"type":"string","minLength":1},"node_id":{"type":"string","minLength":1},"item_index":{"type":"integer","minimum":0},"payload":{},"reason":{"type":"string"}}}`

const loopRequestPayloadSchema = `{
	"type":"object",
	"required":[
		"loop_run_id","generation","node_id","item_index","kind","state","prompt",
		"context","decisions","agents","opened_at"
	],
	"properties":{
		"loop_run_id":{"type":"string","minLength":1},
		"loop_name":{"type":"string"},
		"generation":{"type":"integer","minimum":1},
		"node_id":{"type":"string","minLength":1},
		"item_index":{"type":"integer","minimum":0},
		"kind":{"type":"string","enum":["ask","review"]},
		"state":{"type":"string","enum":["pending","answered","expired","canceled"]},
		"prompt":{"type":"string"},
		"context":{},
		"expect":{},
		"edit_schema":{},
		"respond_schema":{},
		"proposed_preview":{},
		"decisions":{"type":"array","items":{"type":"string"}},
		"agents":{"type":"string","enum":["allow","deny"]},
		"answered_decision":{"type":"string"},
		"actor_kind":{"type":"string"},
		"actor_id":{"type":"string"},
		"opened_at":{"type":"string","format":"date-time"},
		"answered_at":{"type":"string","format":"date-time"},
		"resolved_at":{"type":"string","format":"date-time"},
		"expires_at":{"type":"string","format":"date-time"}
	},
	"additionalProperties":false
}`

const loopRequestsOutputSchema = `{
	"type":"object",
	"required":["items","aggregates","next_cursor"],
	"properties":{
		"items":{"type":"array","items":` + loopRequestPayloadSchema + `},
		"aggregates":{
			"type":"object",
			"required":["pending"],
			"properties":{"pending":{"type":"integer","minimum":0}},
			"additionalProperties":false
		},
		"next_cursor":{"type":"string"}
	},
	"additionalProperties":false
}`

const loopRequestOutputSchema = loopRequestPayloadSchema

const loopRespondOutputSchema = `{
	"type":"object",
	"required":["ok","run_id","node_id","decision","state","provenance"],
	"properties":{
		"ok":{"type":"boolean"},
		"run_id":{"type":"string","minLength":1},
		"node_id":{"type":"string","minLength":1},
		"decision":{"type":"string","enum":["respond","approve","edit","reject"]},
		"state":{"type":"string","enum":["answered"]},
		"provenance":{
			"type":"object",
			"required":["actor_kind","actor_id","answered_at"],
			"properties":{
				"actor_kind":{"type":"string"},
				"actor_id":{"type":"string"},
				"answered_at":{"type":"string","format":"date-time"}
			},
			"additionalProperties":false
		}
	},
	"additionalProperties":false
}`

const loopAmendmentPayloadSchema = `{
	"type":"object",
	"required":["loop_run_id","generation","node_id","item_index","amendment_seq","actor_kind","actor_id","created_at"],
	"properties":{
		"loop_run_id":{"type":"string","minLength":1},
		"generation":{"type":"integer","minimum":1},
		"node_id":{"type":"string","minLength":1},
		"item_index":{"type":"integer","minimum":0},
		"amendment_seq":{"type":"integer","minimum":1},
		"original":{},
		"amended":{},
		"original_summary":{
			"type":"object","required":["byte_size","content_hash"],
			"properties":{"byte_size":{"type":"integer","minimum":0},"content_hash":{"type":"string"}},
			"additionalProperties":false
		},
		"amended_summary":{
			"type":"object","required":["byte_size","content_hash"],
			"properties":{"byte_size":{"type":"integer","minimum":0},"content_hash":{"type":"string"}},
			"additionalProperties":false
		},
		"actor_kind":{"type":"string"},
		"actor_id":{"type":"string"},
		"reason":{"type":"string"},
		"created_at":{"type":"string","format":"date-time"}
	},
	"additionalProperties":false
}`

const loopNodeAmendOutputSchema = `{
	"type":"object",
	"required":["ok","amendment"],
	"properties":{"ok":{"type":"boolean"},"amendment":` + loopAmendmentPayloadSchema + `},
	"additionalProperties":false
}`
