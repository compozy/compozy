package builtin

import (
	"encoding/json"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func agentMemoryDescriptors() []toolspkg.Descriptor {
	result := make([]toolspkg.Descriptor, 0, 4)
	for _, item := range []struct {
		id                               toolspkg.ToolID
		name, title, description, schema string
		read                             bool
	}{
		{toolspkg.ToolIDAgentMemoryAdd, "agent_memory_add", "Add Agent Memory", "Save a dated, source-scoped note in the calling stays-on agent's memory. Experimental.", `{"type":"object","required":["text"],"properties":{"text":{"type":"string","minLength":1,"maxLength":1000},"until":{"type":"string","format":"date"}},"additionalProperties":false}`, false},
		{toolspkg.ToolIDAgentMemoryReplace, "agent_memory_replace", "Replace Agent Memory", "Replace matching text in the calling stays-on agent's memory. Experimental.", `{"type":"object","required":["match","text"],"properties":{"match":{"type":"string","minLength":1},"text":{"type":"string","minLength":1,"maxLength":1000}},"additionalProperties":false}`, false},
		{toolspkg.ToolIDAgentMemoryRemove, "agent_memory_remove", "Remove Agent Memory", "Remove matching notes from the calling stays-on agent's memory. Experimental.", `{"type":"object","required":["match"],"properties":{"match":{"type":"string","minLength":1}},"additionalProperties":false}`, false},
		{toolspkg.ToolIDAgentMemorySearch, "agent_memory_search", "Search Agent Memory", "Search the calling stays-on agent's current and archived notes. Experimental.", `{"type":"object","required":["query"],"properties":{"query":{"type":"string","minLength":1},"include_sessions":{"type":"boolean","default":false}},"additionalProperties":false}`, true},
	} {
		risk := toolspkg.RiskMutating
		if item.read {
			risk = toolspkg.RiskRead
		}
		descriptor := nativeDescriptor(item.id, item.name, item.title, item.description, item.schema, risk, item.read, false, false, []toolspkg.ToolsetID{toolspkg.ToolsetIDAuthoredContext}, []string{"agent", "memory"}, []string{"agent memory", "remember notes"})
		// Memory identity is always derived from the caller, never selected by input.
		descriptor.InputSchema = json.RawMessage(item.schema)
		switch item.id {
		case toolspkg.ToolIDAgentMemoryAdd:
			descriptor.OutputSchema = json.RawMessage(`{"type":"object","required":["digest","budget"],"properties":{"digest":{"type":"string"},"budget":{"type":"object","required":["lines","max_lines","bytes","max_bytes"],"properties":{"lines":{"type":"integer"},"max_lines":{"type":"integer"},"bytes":{"type":"integer"},"max_bytes":{"type":"integer"}},"additionalProperties":false}},"additionalProperties":false}`)
		case toolspkg.ToolIDAgentMemoryReplace:
			descriptor.OutputSchema = json.RawMessage(`{"type":"object","required":["digest"],"properties":{"digest":{"type":"string"}},"additionalProperties":false}`)
		case toolspkg.ToolIDAgentMemoryRemove:
			descriptor.OutputSchema = json.RawMessage(`{"type":"object","required":["digest","removed"],"properties":{"digest":{"type":"string"},"removed":{"type":"integer"}},"additionalProperties":false}`)
		case toolspkg.ToolIDAgentMemorySearch:
			descriptor.OutputSchema = json.RawMessage(`{"type":"object","required":["results"],"properties":{"results":{"type":"array","items":{"type":"object","required":["source","text"],"properties":{"source":{"type":"string"},"text":{"type":"string"},"file":{"type":"string"},"line":{"type":"integer"},"score":{"type":"number"},"session_id":{"type":"string"}}}}},"additionalProperties":false}`)
		}
		result = append(result, descriptor)
	}
	return result
}
