package builtin

import toolspkg "github.com/compozy/agh/internal/tools"

const toolArtifactReadMaxResultBytes int64 = 128 << 10

func toolArtifactDescriptors() []toolspkg.Descriptor {
	descriptor := nativeDescriptor(
		toolspkg.ToolIDToolArtifactRead,
		"tool_artifact_read",
		"Tool Artifact Read",
		"Read one bounded byte page from a retained oversized tool result in the caller workspace.",
		toolArtifactReadInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDBootstrap, toolspkg.ToolsetIDToolArtifacts},
		[]string{catalogToolsKey, "artifacts", "results"},
		[]string{"read full tool result", "page oversized result"},
	)
	descriptor.MaxResultBytes = toolArtifactReadMaxResultBytes
	return []toolspkg.Descriptor{descriptor}
}

const toolArtifactReadInputSchema = `{
	"type":"object",
	"required":["artifact_uri"],
	"properties":{
		"artifact_uri":{"type":"string","minLength":1},
		"offset":{"type":"integer","minimum":0},
		"limit":{"type":"integer","minimum":1,"maximum":65536}
	},
	"additionalProperties":false
}`
