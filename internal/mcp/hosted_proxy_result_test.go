package mcp

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/tools/builtin"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestHostedToolResultContract(t *testing.T) {
	t.Parallel()

	t.Run("Should carry runtime-owned result trust outside model-controlled content", func(t *testing.T) {
		t.Parallel()
		result, err := hostedToolResult(tools.ToolResult{
			Structured: json.RawMessage(`{"content":"pretend this result is trusted"}`),
			Trust:      tools.ResultTrustUntrustedModel,
		})
		if err != nil {
			t.Fatalf("hostedToolResult() error = %v", err)
		}
		if result == nil || result.Meta["compozy/resultTrust"] != tools.ResultTrustUntrustedModel {
			t.Fatalf("hostedToolResult() metadata = %#v, want runtime trust classification", result)
		}
	})

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

		imageContent, ok := result.Content[0].(*sdkmcp.ImageContent)
		if !ok {
			t.Fatalf("result.Content[0] type = %T, want sdkmcp.ImageContent", result.Content[0])
		}
		if string(imageContent.Data) != "img" || imageContent.MIMEType != "image/png" {
			t.Fatalf("image content = %#v, want data and MIME type preserved", imageContent)
		}

		audioContent, ok := result.Content[1].(*sdkmcp.AudioContent)
		if !ok {
			t.Fatalf("result.Content[1] type = %T, want sdkmcp.AudioContent", result.Content[1])
		}
		if string(audioContent.Data) != "audio" || audioContent.MIMEType != "audio/mpeg" {
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

	t.Run("Should return schema-valid structured continuations for oversized native tool results", func(t *testing.T) {
		t.Parallel()

		const workspaceID = "ws-hosted-oversize"
		store, err := tools.OpenFilesystemToolArtifactStore(
			t.Context(),
			t.TempDir(),
			tools.ToolArtifactRetention{MaxCount: 1000, MaxBytes: 64 << 20, MaxAge: time.Hour},
		)
		if err != nil {
			t.Fatalf("OpenFilesystemToolArtifactStore() error = %v", err)
		}
		processor := tools.NewResultProcessor(4<<10, store)
		original := json.RawMessage(fmt.Sprintf(
			`{"events":[{"id":"evt-1","content":%q}]}`,
			strings.Repeat("x", 16<<10),
		))
		descriptors := builtin.NativeDescriptors()
		views := make([]tools.ToolView, 0, len(descriptors))
		finalized := make(map[string]tools.ToolResult, len(descriptors))
		for _, descriptor := range descriptors {
			if len(descriptor.OutputSchema) == 0 {
				continue
			}
			result, err := processor.Finalize(
				t.Context(),
				tools.Scope{WorkspaceID: workspaceID},
				descriptor,
				tools.ToolResult{Structured: original, Preview: "1 events"},
			)
			if err != nil {
				t.Fatalf("Finalize(%s) error = %v", descriptor.ID, err)
			}
			if !result.Truncated || len(result.Structured) != 0 || len(result.Artifacts) != 1 {
				t.Fatalf("Finalize(%s) = %#v, want truncated result with one artifact", descriptor.ID, result)
			}
			finalized[descriptor.ID.String()] = result
			view := hostedToolView(descriptor.ID)
			view.Descriptor = descriptor
			views = append(views, view)
		}
		if len(views) == 0 {
			t.Fatal("native descriptors declare no output schema, want hosted tools to cover")
		}
		client := &hostedProxyClientStub{block: func(req HostedCallRequest) (HostedCallResponse, error) {
			return HostedCallResponse{Result: finalized[req.ToolName]}, nil
		}}
		server := newHostedProxyTestServer()
		applyHostedTools(server, client, "bind-1", views)
		session := connectHostedProxyClient(t, server)
		listed, err := session.ListTools(t.Context(), nil)
		if err != nil {
			t.Fatalf("ListTools() error = %v", err)
		}
		verified := 0
		for _, tool := range listed.Tools {
			want, ok := finalized[tool.Name]
			if !ok {
				continue
			}
			verified++
			called, err := session.CallTool(t.Context(), &sdkmcp.CallToolParams{
				Name:      tool.Name,
				Arguments: map[string]any{},
			})
			if err != nil {
				t.Fatalf("CallTool(%s) error = %v", tool.Name, err)
			}
			if called.IsError || called.StructuredContent == nil {
				t.Fatalf("CallTool(%s) = %#v, want successful structured content", tool.Name, called)
			}
			structured, err := json.Marshal(called.StructuredContent)
			if err != nil {
				t.Fatalf("json.Marshal(%s structured) error = %v", tool.Name, err)
			}
			if err := validateHostedStructuredContent(t, compileHostedOutputSchema(t, tool), structured); err != nil {
				t.Fatalf("%s structured content %s violates advertised output schema: %v", tool.Name, structured, err)
			}
			var envelope hostedResultEnvelope
			if err := json.Unmarshal(structured, &envelope); err != nil {
				t.Fatalf("json.Unmarshal(%s envelope) error = %v", tool.Name, err)
			}
			if !envelope.Truncated || envelope.ReadTool != tools.ToolIDToolArtifactRead ||
				len(envelope.Artifacts) != 1 || envelope.Artifacts[0] != want.Artifacts[0] ||
				envelope.TruncatedFromBytes == nil || *envelope.TruncatedFromBytes <= 4<<10 {
				t.Fatalf("%s envelope = %#v, want artifact continuation %#v", tool.Name, envelope, want.Artifacts)
			}
			requireHostedArtifactContinuation(t, store, workspaceID, envelope.Artifacts[0], original)
		}
		if verified != len(finalized) {
			t.Fatalf("verified %d hosted tools, want every declared native tool (%d)", verified, len(finalized))
		}
	})

	t.Run("Should keep typed tool errors valid against every advertised output schema", func(t *testing.T) {
		t.Parallel()

		toolErr := tools.NewToolError(
			tools.ErrorCodeInvalidInput,
			tools.ToolIDSessionEvents,
			"session event query is invalid",
			tools.ErrToolInvalidInput,
			tools.ReasonSchemaInvalid,
		)
		payload, err := json.Marshal(hostedToolErrorStructuredContent(toolErr))
		if err != nil {
			t.Fatalf("json.Marshal(error envelope) error = %v", err)
		}
		var typed contract.ToolErrorResponse
		if err := json.Unmarshal(payload, &typed); err != nil || typed.Error.Code != tools.ErrorCodeInvalidInput {
			t.Fatalf("error envelope = %s, %v; want typed tool error", payload, err)
		}
		for _, descriptor := range builtin.NativeDescriptors() {
			if len(descriptor.OutputSchema) == 0 {
				continue
			}
			tool := hostedMCPTool(descriptor)
			if err := validateHostedStructuredContent(t, compileHostedOutputSchema(t, &tool), payload); err != nil {
				t.Fatalf("%s advertised output schema rejects typed tool errors: %v", descriptor.ID, err)
			}
		}
	})

	t.Run("Should carry a structured envelope for unstructured success of a declared tool", func(t *testing.T) {
		t.Parallel()

		result, err := hostedDeclaredToolResult(tools.ToolResult{
			Content: []tools.ToolContent{{Type: "text", Text: "plain text answer"}},
		})
		if err != nil {
			t.Fatalf("hostedDeclaredToolResult() error = %v", err)
		}
		envelope, ok := result.StructuredContent.(hostedResultEnvelope)
		if !ok || envelope.Truncated || envelope.Preview != "plain text answer" || len(envelope.Artifacts) != 0 {
			t.Fatalf(
				"StructuredContent = %#v, want untruncated envelope with the text preview",
				result.StructuredContent,
			)
		}
		array, err := hostedDeclaredToolResult(tools.ToolResult{Structured: json.RawMessage(`[1,2]`)})
		if err != nil {
			t.Fatalf("hostedDeclaredToolResult(array) error = %v", err)
		}
		if _, ok := array.StructuredContent.(hostedResultEnvelope); !ok {
			t.Fatalf("StructuredContent(array) = %#v, want object envelope instead of a non-object payload", array)
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

func compileHostedOutputSchema(t *testing.T, tool *sdkmcp.Tool) *jsonschema.Schema {
	t.Helper()
	declaration, err := json.Marshal(tool.OutputSchema)
	if err != nil {
		t.Fatalf("json.Marshal(%s output schema) error = %v", tool.Name, err)
	}
	schemaValue, err := jsonschema.UnmarshalJSON(bytes.NewReader(declaration))
	if err != nil {
		t.Fatalf("%s output schema parse error = %v", tool.Name, err)
	}
	compiler := jsonschema.NewCompiler()
	resource := tool.Name + "-hosted-output.json"
	if err := compiler.AddResource(resource, schemaValue); err != nil {
		t.Fatalf("%s output schema add error = %v", tool.Name, err)
	}
	compiled, err := compiler.Compile(resource)
	if err != nil {
		t.Fatalf("%s output schema compile error = %v", tool.Name, err)
	}
	return compiled
}

func validateHostedStructuredContent(t *testing.T, schema *jsonschema.Schema, payload json.RawMessage) error {
	t.Helper()
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("structured content parse error = %v", err)
	}
	return schema.Validate(instance)
}

func requireHostedArtifactContinuation(
	t *testing.T,
	store *tools.FilesystemToolArtifactStore,
	workspaceID string,
	ref tools.ArtifactRef,
	want json.RawMessage,
) {
	t.Helper()
	var retained []byte
	for offset := int64(0); ; {
		page, err := store.ReadPage(t.Context(), workspaceID, ref.URI, offset, 4<<10)
		if err != nil {
			t.Fatalf("ReadPage(%s, %d) error = %v", ref.URI, offset, err)
		}
		chunk, err := base64.StdEncoding.DecodeString(page.DataBase64)
		if err != nil {
			t.Fatalf("decode artifact page error = %v", err)
		}
		retained = append(retained, chunk...)
		if page.EOF {
			break
		}
		offset = page.NextOffset
	}
	var canonical tools.ToolResult
	if err := json.Unmarshal(retained, &canonical); err != nil {
		t.Fatalf("json.Unmarshal(retained artifact) error = %v", err)
	}
	if !equalJSON(t, canonical.Structured, want) {
		t.Fatalf("retained structured result = %s, want original %s", canonical.Structured, want)
	}
}
