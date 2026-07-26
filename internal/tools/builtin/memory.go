package builtin

import toolspkg "github.com/compozy/agh/internal/tools"

const (
	memoryListKey = "list"
)

var memoryTools = []toolspkg.Descriptor{
	nativeDescriptor(
		toolspkg.ToolIDMemoryList,
		"memory_list",
		"Memory List",
		"List Memory v2 headers visible for a scope.",
		memoryListInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDMemory},
		[]string{memoryAdminMemoryKey, memoryListKey},
		[]string{"memory list", "memory headers"},
	),
	nativeDescriptor(
		toolspkg.ToolIDMemoryShow,
		"memory_show",
		"Memory Show",
		"Show one Memory v2 document through the current memory store.",
		memoryShowInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDMemory},
		[]string{memoryAdminMemoryKey, "show"},
		[]string{"memory show", "memory document"},
	),
	nativeDescriptor(
		toolspkg.ToolIDMemorySearch,
		"memory_search",
		"Memory Search",
		"Recall Memory v2 entries through the active provider-backed recall path.",
		memorySearchInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDMemory},
		[]string{memoryAdminMemoryKey, "search"},
		[]string{"memory search", "recall memory"},
	),
	nativeDescriptor(
		toolspkg.ToolIDMemoryPropose,
		"memory_propose",
		"Memory Propose",
		"Submit one Memory v2 write/delete or an atomic document-body operations batch through the write controller.",
		memoryProposeInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDMemory},
		[]string{memoryAdminMemoryKey, "propose"},
		[]string{"memory propose", "memory write", "memory update", "memory delete"},
	),
	nativeDescriptor(
		toolspkg.ToolIDMemoryNote,
		"memory_note",
		"Memory Note",
		"Submit an ad-hoc Memory v2 note through the write controller.",
		memoryNoteInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDMemory},
		[]string{memoryAdminMemoryKey, "note"},
		[]string{"memory note", "ad hoc memory"},
	),
}

func memoryDescriptors() []toolspkg.Descriptor {
	return memoryTools
}

const memoryListInputSchema = `{
	"type":"object",
	"properties":{
		"scope":{"type":"string","enum":["global","workspace","agent"]},
		"workspace":{"type":"string"},
		"agent_name":{"type":"string"},
		"agent_tier":{"type":"string","enum":["workspace","global"]},
		"type":{"type":"string","enum":["user","feedback","project","reference"]},
		"sort":{"type":"string","enum":["recent","name"]},
		"cursor":{"type":"string"},
		"limit":{"type":"integer","minimum":1,"maximum":200},
		"include_system":{"type":"boolean"}
	},
	"additionalProperties":false
}`

const memoryShowInputSchema = `{
	"type":"object",
	"required":["filename"],
	"properties":{
		"filename":{"type":"string"},
		"scope":{"type":"string","enum":["global","workspace","agent"]},
		"workspace":{"type":"string"},
		"agent_name":{"type":"string"},
		"agent_tier":{"type":"string","enum":["workspace","global"]}
	},
	"additionalProperties":false
}`

const memorySearchInputSchema = `{
	"type":"object",
	"properties":{
		"query":{"type":"string"},
		"q":{"type":"string"},
		"scope":{"type":"string","enum":["global","workspace","agent"]},
		"workspace":{"type":"string"},
		"agent_name":{"type":"string"},
		"agent_tier":{"type":"string","enum":["workspace","global"]},
		"limit":{"type":"integer"}
	},
	"additionalProperties":false
}`

const memoryProposeInputSchema = `{
	"type":"object",
	"properties":{
		"operation":{"type":"string","enum":["add","update","delete"]},
		"operations":{
			"type":"array",
			"minItems":1,
			"description":"Atomic document-body operations validated against the final byte and line budget.",
			"items":{
				"type":"object",
				"required":["action"],
				"properties":{
					"action":{"type":"string","enum":["add","replace","remove"]},
					"content":{"type":"string"},
					"old_text":{"type":"string","description":"A substring that must occur exactly once for replace or remove."}
				},
				"additionalProperties":false
			}
		},
		"filename":{"type":"string"},
		"target_filename":{"type":"string"},
		"content":{"type":"string"},
		"name":{"type":"string"},
		"description":{"type":"string"},
		"type":{"type":"string","enum":["user","feedback","project","reference"]},
		"scope":{"type":"string","enum":["global","workspace","agent"]},
		"workspace":{"type":"string"},
		"agent_name":{"type":"string"},
		"agent_tier":{"type":"string","enum":["workspace","global"]},
		"entity":{"type":"string"},
		"attribute":{"type":"string"}
	},
	"additionalProperties":false
}`

const memoryNoteInputSchema = `{
	"type":"object",
	"required":["content"],
	"properties":{
		"content":{"type":"string"},
		"slug":{"type":"string"},
		"scope":{"type":"string","enum":["global","workspace","agent"]},
		"workspace":{"type":"string"},
		"agent_name":{"type":"string"},
		"agent_tier":{"type":"string","enum":["workspace","global"]},
		"tags":{"type":"array","items":{"type":"string"}}
	},
	"additionalProperties":false
}`
