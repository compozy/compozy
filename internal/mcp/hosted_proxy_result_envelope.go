package mcp

import (
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/tools"
)

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

// Non-object payloads retain their established hosted text/envelope representation.
func hostedStructuredObject(value any) bool {
	_, ok := value.(map[string]any)
	return ok
}
