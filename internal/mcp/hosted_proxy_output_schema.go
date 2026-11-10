package mcp

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/tools"
)

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
	"properties":{
		"error":{
			"type":"object",
			"required":["code","message"],
			"properties":{"code":{"type":"string"},"message":{"type":"string"}}
		}
	}
}`

// hostedOutputSchema accepts each declared view, the result continuation, and typed tool errors.
func hostedOutputSchema(descriptor tools.Descriptor) any {
	schemas := []json.RawMessage{descriptor.OutputSchema}
	if views := descriptor.ResultViews(); views != nil {
		schemas = []json.RawMessage{views.Compact.Schema, views.Full.Schema}
	}
	if len(schemas[0]) == 0 {
		return nil
	}
	wrapped := map[string]any{"type": "object"}
	branches := make([]any, 0, len(schemas)+2)
	for index, schema := range schemas {
		decoded, err := decodeHostedJSON(schema)
		if err != nil {
			return cloneRaw(schema)
		}
		declared, ok := decoded.(map[string]any)
		if !ok {
			return cloneRaw(schema)
		}
		if dialect, present := declared["$schema"]; present {
			if index == 0 {
				wrapped["$schema"] = dialect
			}
			delete(declared, "$schema")
		}
		rebaseHostedSchemaRefs(declared, "#/anyOf/"+strconv.Itoa(index))
		branches = append(branches, declared)
	}
	wrapped["anyOf"] = append(branches,
		json.RawMessage(hostedResultEnvelopeSchema),
		json.RawMessage(hostedErrorEnvelopeSchema),
	)
	return wrapped
}

// rebaseHostedSchemaRefs rewrites local "#" and "#/..." references of one schema object in place.
// Plain-name anchors ("#name") are location-independent and stay unchanged.
func rebaseHostedSchemaRefs(schema map[string]any, pointer string) {
	if _, embedded := schema["$id"]; embedded {
		return
	}
	for keyword, value := range schema {
		switch keyword {
		case "$ref", "$dynamicRef":
			if ref, ok := value.(string); ok && (ref == "#" || strings.HasPrefix(ref, "#/")) {
				schema[keyword] = pointer + strings.TrimPrefix(ref, "#")
			}
		case "const", "enum", "default", "examples":
			// Instance values, not subschemas.
		case "properties", "patternProperties", "$defs", "definitions", "dependentSchemas":
			if children, ok := value.(map[string]any); ok {
				for _, child := range children {
					rebaseHostedSchemaValue(child, pointer)
				}
			}
		default:
			rebaseHostedSchemaValue(value, pointer)
		}
	}
}

func rebaseHostedSchemaValue(value any, pointer string) {
	switch typed := value.(type) {
	case map[string]any:
		rebaseHostedSchemaRefs(typed, pointer)
	case []any:
		for _, item := range typed {
			rebaseHostedSchemaValue(item, pointer)
		}
	}
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
