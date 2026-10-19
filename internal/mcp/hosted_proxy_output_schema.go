package mcp

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/tools"
)

// hostedSchemaRootKeywords are document-level keywords that must stay on the advertised root so
// "#/..." references and dialect selection inside the declared schema keep resolving after wrapping.
var hostedSchemaRootKeywords = []string{"$schema", "$id", "$defs", "definitions"}

// hostedResultEnvelopeSchema accepts the structured continuation emitted when the canonical
// payload is unavailable, most commonly because the result exceeded the tool result budget.
const hostedResultEnvelopeSchema = `{
	"type":"object",
	"description":"Returned instead of the declared payload when the full structured result is unavailable. ` +
	`When truncated is true, read the complete result by calling read_tool with an artifacts[].uri.",
	"required":["truncated"],
	"properties":{
		"truncated":{"type":"boolean"},
		"truncated_from_bytes":{"type":"integer","minimum":0},
		"preview":{"type":"string"},
		"artifacts":{
			"type":"array",
			"items":{
				"type":"object",
				"required":["uri"],
				"properties":{
					"uri":{"type":"string"},
					"name":{"type":"string"},
					"mime_type":{"type":"string"},
					"bytes":{"type":"integer","minimum":0},
					"sha256":{"type":"string"}
				}
			}
		},
		"read_tool":{"type":"string"}
	},
	"additionalProperties":false
}`

// hostedErrorEnvelopeSchema accepts the typed `{"error":{...}}` payload carried by hosted tool errors.
const hostedErrorEnvelopeSchema = `{
	"type":"object",
	"required":["error"],
	"properties":{"error":{"type":"object"}}
}`

// hostedOutputSchema widens a descriptor output schema so every hosted result shape validates:
// the declared payload, the structured result continuation, or a typed tool error. MCP clients
// validate structuredContent against the advertised schema, including on error results.
func hostedOutputSchema(schema json.RawMessage) any {
	if len(schema) == 0 {
		return nil
	}
	decoded, err := decodeHostedJSON(schema)
	if err != nil {
		return cloneRaw(schema)
	}
	declared, ok := decoded.(map[string]any)
	if !ok {
		return cloneRaw(schema)
	}
	wrapped := map[string]any{"type": "object"}
	for _, keyword := range hostedSchemaRootKeywords {
		if value, present := declared[keyword]; present {
			wrapped[keyword] = value
			delete(declared, keyword)
		}
	}
	wrapped["anyOf"] = []any{
		declared,
		json.RawMessage(hostedResultEnvelopeSchema),
		json.RawMessage(hostedErrorEnvelopeSchema),
	}
	return wrapped
}

// hostedResultEnvelope is the structured continuation for results without an object payload.
type hostedResultEnvelope struct {
	Truncated          bool                `json:"truncated"`
	TruncatedFromBytes *int64              `json:"truncated_from_bytes,omitempty"`
	Preview            string              `json:"preview,omitempty"`
	Artifacts          []tools.ArtifactRef `json:"artifacts,omitempty"`
	ReadTool           tools.ToolID        `json:"read_tool,omitempty"`
}

func newHostedResultEnvelope(result tools.ToolResult) hostedResultEnvelope {
	envelope := hostedResultEnvelope{
		Truncated: result.Truncated,
		Preview:   hostedEnvelopePreview(result),
	}
	if raw, ok := result.Metadata["truncated_from_bytes"]; ok {
		if parsed, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64); err == nil && parsed >= 0 {
			envelope.TruncatedFromBytes = &parsed
		}
	}
	if len(result.Artifacts) > 0 {
		envelope.Artifacts = append([]tools.ArtifactRef(nil), result.Artifacts...)
		envelope.ReadTool = tools.ToolIDToolArtifactRead
	}
	return envelope
}

func hostedEnvelopePreview(result tools.ToolResult) string {
	if strings.TrimSpace(result.Preview) != "" {
		return result.Preview
	}
	parts := make([]string, 0, len(result.Content))
	for _, block := range result.Content {
		if strings.TrimSpace(block.Text) != "" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// hostedStructuredObject reports whether structured content is a JSON object, the only shape
// MCP accepts for CallToolResult.structuredContent.
func hostedStructuredObject(value any) bool {
	_, ok := value.(map[string]any)
	return ok
}
