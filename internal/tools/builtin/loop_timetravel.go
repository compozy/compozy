package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var loopTimeTravelTools = []toolspkg.Descriptor{
	nativeLoopDescriptor(toolspkg.ToolIDLoopDiff, "loop_diff", "Loop Diff",
		"Compare generations or runs of the same Loop.", loopDiffInputSchema,
		toolspkg.RiskRead, true, false, []string{loopKey, "diff", historyTag},
		[]string{"compare loop generations", "diff loop runs"}),
	nativeLoopDescriptor(toolspkg.ToolIDLoopRerun, "loop_rerun", "Loop Rerun",
		"Rerun one settled node and its dependents.", loopRerunInputSchema,
		toolspkg.RiskMutating, false, false, []string{loopKey, "rerun", historyTag},
		[]string{"rerun loop node", "retry from node"}),
	nativeLoopDescriptor(toolspkg.ToolIDLoopFork, "loop_fork", "Loop Fork",
		"Create a linked run from one historical generation.", loopForkInputSchema,
		toolspkg.RiskMutating, false, false, []string{loopKey, "fork", historyTag},
		[]string{"fork loop run", "start from generation"}),
}

const loopDiffInputSchema = `{"type":"object","required":["run_id"],"additionalProperties":false,"properties":{"workspace":{"type":"string"},"run_id":{"type":"string","minLength":1},"generation":{"type":"integer","minimum":1},"against_generation":{"type":"integer","minimum":1},"against_run":{"type":"string","minLength":1}}}`

const loopRerunInputSchema = `{"type":"object","required":["run_id","from_node"],"additionalProperties":false,"properties":{"workspace":{"type":"string"},"run_id":{"type":"string","minLength":1},"from_node":{"type":"string","minLength":1},"item_index":{"type":"integer","minimum":0},"reason":{"type":"string"},"request_id":{"type":"string"}}}`

const loopForkInputSchema = `{"type":"object","required":["run_id","generation"],"additionalProperties":false,"properties":{"workspace":{"type":"string"},"run_id":{"type":"string","minLength":1},"generation":{"type":"integer","minimum":1},"inputs":{"type":"object"},"reason":{"type":"string"},"request_id":{"type":"string"}}}`

const loopDiffEndpointSchema = `{
	"type":"object",
	"required":["run_id","generation","status"],
	"properties":{
		"run_id":{"type":"string","minLength":1},
		"generation":{"type":"integer","minimum":1},
		"status":{"type":"string"},
		"as_of":{"type":"boolean"}
	},
	"additionalProperties":false
}`

const loopDiffValueSchema = `{
	"type":"object",
	"properties":{
		"inline":{},
		"size":{"type":"integer","minimum":0},
		"hash":{"type":"string"}
	},
	"additionalProperties":false
}`

const loopDiffOutputSchema = `{
	"type":"object",
	"required":["kind","base","against","inputs","nodes"],
	"properties":{
		"kind":{"type":"string"},
		"base":` + loopDiffEndpointSchema + `,
		"against":` + loopDiffEndpointSchema + `,
		"inputs":{"type":"array","items":{
			"type":"object",
			"required":["key","base","against"],
			"properties":{"key":{"type":"string"},"base":` + loopDiffValueSchema + `,"against":` + loopDiffValueSchema + `},
			"additionalProperties":false
		}},
		"nodes":{"type":"array","items":{
			"type":"object",
			"required":["node_id","change"],
			"properties":{
				"node_id":{"type":"string","minLength":1},
				"item_index":{"type":"integer","minimum":0},
				"change":{"type":"string"},
				"base":` + loopDiffValueSchema + `,
				"against":` + loopDiffValueSchema + `,
				"cause":{"type":"string"}
			},
			"additionalProperties":false
		}},
		"terminal":{
			"type":"object","required":["base","against"],
			"properties":{"base":{"type":"string"},"against":{"type":"string"}},
			"additionalProperties":false
		},
		"definition_divergence":{"type":"boolean"}
	},
	"additionalProperties":false
}`

const loopRerunOutputSchema = `{
	"type":"object",
	"required":["run_id","generation","parent_generation","rerun_nodes","carried"],
	"properties":{
		"run_id":{"type":"string","minLength":1},
		"generation":{"type":"integer","minimum":1},
		"parent_generation":{"type":"integer","minimum":1},
		"rerun_nodes":{"type":"array","items":{"type":"string","minLength":1}},
		"carried":{"type":"integer","minimum":0},
		"replayed":{"type":"boolean"}
	},
	"additionalProperties":false
}`

const loopForkOutputSchema = `{
	"type":"object",
	"required":["run"],
	"properties":{
		"run":{
			"type":"object",
			"required":["id","workspace_id","loop_name","status","completion_state","generation"],
			"additionalProperties":true
		},
		"replayed":{"type":"boolean"}
	},
	"additionalProperties":false
}`
