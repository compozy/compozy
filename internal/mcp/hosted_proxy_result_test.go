package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/tools"
	sdkmcp "github.com/mark3labs/mcp-go/mcp"
)

func TestHostedToolResultContract(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve hosted MCP error flag", func(t *testing.T) {
		t.Parallel()

		result, err := hostedToolResult(tools.ToolResult{
			Content: []tools.ToolContent{{Type: "text", Text: "denied"}},
			Metadata: map[string]json.RawMessage{
				toolResultIsErrorKey: json.RawMessage(`true`),
			},
		})
		if err != nil {
			t.Fatalf("hostedToolResult() error = %v", err)
		}
		if result == nil || !result.IsError {
			t.Fatalf("hostedToolResult() = %#v, want error result", result)
		}
	})

	t.Run("Should preserve hosted MCP media content", func(t *testing.T) {
		t.Parallel()

		result, err := hostedToolResult(tools.ToolResult{
			Content: []tools.ToolContent{
				{Type: "image", Data: json.RawMessage(`"aW1n"`), MIMEType: "image/png"},
				{Type: "audio", Data: json.RawMessage(`"YXVkaW8="`), MIMEType: "audio/mpeg"},
			},
		})
		if err != nil {
			t.Fatalf("hostedToolResult() error = %v", err)
		}
		if result == nil || len(result.Content) != 2 {
			t.Fatalf("hostedToolResult() = %#v, want two media blocks", result)
		}

		imageContent, ok := result.Content[0].(sdkmcp.ImageContent)
		if !ok {
			t.Fatalf("result.Content[0] type = %T, want sdkmcp.ImageContent", result.Content[0])
		}
		if imageContent.Data != "aW1n" || imageContent.MIMEType != "image/png" {
			t.Fatalf("image content = %#v, want data and MIME type preserved", imageContent)
		}

		audioContent, ok := result.Content[1].(sdkmcp.AudioContent)
		if !ok {
			t.Fatalf("result.Content[1] type = %T, want sdkmcp.AudioContent", result.Content[1])
		}
		if audioContent.Data != "YXVkaW8=" || audioContent.MIMEType != "audio/mpeg" {
			t.Fatalf("audio content = %#v, want data and MIME type preserved", audioContent)
		}
	})

	t.Run("Should expose artifact references and the native page-back tool", func(t *testing.T) {
		t.Parallel()

		ref := tools.ArtifactRef{
			URI:      tools.ToolArtifactURIPrefix + "art_" + strings.Repeat("a", 64),
			Name:     tools.ToolArtifactName,
			MIMEType: tools.ToolArtifactMIMEType,
			Bytes:    2048,
			SHA256:   strings.Repeat("a", 64),
		}
		result, err := hostedToolResult(tools.ToolResult{
			Preview:   "bounded preview",
			Artifacts: []tools.ArtifactRef{ref},
			Truncated: true,
		})
		if err != nil {
			t.Fatalf("hostedToolResult() error = %v", err)
		}
		if result == nil || result.Meta == nil || len(result.Content) != 2 {
			t.Fatalf("hostedToolResult() = %#v, want preview plus artifact projection", result)
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		if !strings.Contains(string(encoded), ref.URI) ||
			!strings.Contains(string(encoded), tools.ToolIDToolArtifactRead.String()) {
			t.Fatalf("hosted artifact result = %s, want URI and native read tool", encoded)
		}
	})

	t.Run("Should preserve bounded partial results on persistence failure", func(t *testing.T) {
		t.Parallel()

		toolErr := tools.NewToolError(
			tools.ErrorCodeResultPersistenceFailed,
			tools.ToolIDSkillView,
			"tool result could not be retained",
			tools.ErrToolResultPersistence,
			tools.ReasonResultPersistenceFailed,
		).WithPartialResult(tools.ToolResult{
			Content: []tools.ToolContent{{Type: "text", Text: "bounded preview"}},
		})
		result, ok, err := hostedToolPartialErrorResult(toolErr)
		if err != nil {
			t.Fatalf("hostedToolPartialErrorResult() error = %v", err)
		}
		if !ok || result == nil || !result.IsError || len(result.Content) != 2 {
			t.Fatalf("hostedToolPartialErrorResult() = %#v, %t, want MCP error with partial content", result, ok)
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		if !strings.Contains(string(encoded), "bounded preview") ||
			!strings.Contains(string(encoded), string(tools.ReasonResultPersistenceFailed)) {
			t.Fatalf("hosted partial result = %s, want preview and typed reason", encoded)
		}
	})
}
