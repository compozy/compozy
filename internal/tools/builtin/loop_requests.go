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

const loopRequestInputSchema = `{"type":"object","required":["run_id","node_id"],"additionalProperties":false,"properties":{"workspace":{"type":"string"},"run_id":{"type":"string","minLength":1},"node_id":{"type":"string","minLength":1},"item_index":{"type":"integer","minimum":0}}}`

const loopRespondInputSchema = `{"type":"object","required":["run_id","node_id"],"additionalProperties":false,"properties":{"workspace":{"type":"string"},"run_id":{"type":"string","minLength":1},"node_id":{"type":"string","minLength":1},"item_index":{"type":"integer","minimum":0},"decision":{"type":"string","enum":["respond","approve","edit","reject"]},"payload":{"description":"Required for ask, respond, and edit decisions; omitted for approve and reject."},"note":{"type":"string"}}}`

const loopNodeAmendInputSchema = `{"type":"object","required":["run_id","node_id","payload"],"additionalProperties":false,"properties":{"workspace":{"type":"string"},"run_id":{"type":"string","minLength":1},"node_id":{"type":"string","minLength":1},"item_index":{"type":"integer","minimum":0},"payload":{},"reason":{"type":"string"}}}`

const loopRequestsOutputSchema = `{"type":"object","required":["items","aggregates","next_cursor"]}`
const loopRequestOutputSchema = `{"type":"object","required":["loop_run_id","node_id","state","context"]}`
const loopRespondOutputSchema = `{"type":"object","required":["ok","run_id","node_id","decision","state","provenance"]}`
const loopNodeAmendOutputSchema = `{"type":"object","required":["ok","amendment"]}`
