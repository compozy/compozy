package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var loopTimeTravelTools = []toolspkg.Descriptor{
	nativeLoopDescriptor(toolspkg.ToolIDLoopDiff, "loop_diff", "Loop Diff",
		"Compare generations or runs of the same Loop.", loopDiffInputSchema,
		toolspkg.RiskRead, true, false, []string{loopKey, "diff", "history"},
		[]string{"compare loop generations", "diff loop runs"}),
	nativeLoopDescriptor(toolspkg.ToolIDLoopRerun, "loop_rerun", "Loop Rerun",
		"Rerun one settled node and its dependents.", loopRerunInputSchema,
		toolspkg.RiskMutating, false, false, []string{loopKey, "rerun", "history"},
		[]string{"rerun loop node", "retry from node"}),
	nativeLoopDescriptor(toolspkg.ToolIDLoopFork, "loop_fork", "Loop Fork",
		"Create a linked run from one historical generation.", loopForkInputSchema,
		toolspkg.RiskMutating, false, false, []string{loopKey, "fork", "history"},
		[]string{"fork loop run", "start from generation"}),
}

const loopDiffInputSchema = `{"type":"object","required":["run_id"],"additionalProperties":false,"properties":{"workspace":{"type":"string"},"run_id":{"type":"string","minLength":1},"generation":{"type":"integer","minimum":1},"against_generation":{"type":"integer","minimum":1},"against_run":{"type":"string","minLength":1}}}`

const loopRerunInputSchema = `{"type":"object","required":["run_id","from_node"],"additionalProperties":false,"properties":{"workspace":{"type":"string"},"run_id":{"type":"string","minLength":1},"from_node":{"type":"string","minLength":1},"item_index":{"type":"integer","minimum":0},"reason":{"type":"string"},"request_id":{"type":"string"}}}`

const loopForkInputSchema = `{"type":"object","required":["run_id","generation"],"additionalProperties":false,"properties":{"workspace":{"type":"string"},"run_id":{"type":"string","minLength":1},"generation":{"type":"integer","minimum":1},"inputs":{"type":"object"},"reason":{"type":"string"},"request_id":{"type":"string"}}}`

const loopDiffOutputSchema = `{"type":"object","required":["kind","base","against","inputs","nodes"]}`
const loopRerunOutputSchema = `{"type":"object","required":["run_id","generation","parent_generation","rerun_nodes","carried"]}`
const loopForkOutputSchema = `{"type":"object","required":["run"]}`
