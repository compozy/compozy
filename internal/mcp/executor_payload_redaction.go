package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/toolmeta"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func redactMCPToolResult(result toolspkg.ToolResult) (toolspkg.ToolResult, error) {
	redactor := mcpPayloadRedactor{
		redactions: append([]toolspkg.Redaction(nil), result.Redactions...),
	}
	redacted := result
	redacted.Preview = redactor.text(result.Preview, "$.preview")

	var err error
	redacted.Structured, err = redactor.rawJSON(result.Structured, "$.structured")
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	redacted.Metadata, err = redactor.rawMap(result.Metadata, "$.metadata")
	if err != nil {
		return toolspkg.ToolResult{}, err
	}

	redacted.Content = append([]toolspkg.ToolContent(nil), result.Content...)
	for index := range redacted.Content {
		path := fmt.Sprintf("$.content[%d]", index)
		redacted.Content[index].Type = redactor.text(result.Content[index].Type, path+".type")
		redacted.Content[index].Text = redactor.text(result.Content[index].Text, path+".text")
		redacted.Content[index].MIMEType = redactor.text(result.Content[index].MIMEType, path+".mime_type")
		redacted.Content[index].Data, err = redactor.rawJSON(result.Content[index].Data, path+".data")
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		redacted.Content[index].Metadata, err = redactor.rawMap(result.Content[index].Metadata, path+".metadata")
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
	}

	redacted.Artifacts = append([]toolspkg.ArtifactRef(nil), result.Artifacts...)
	for index := range redacted.Artifacts {
		path := fmt.Sprintf("$.artifacts[%d]", index)
		redacted.Artifacts[index].URI = redactor.text(result.Artifacts[index].URI, path+".uri")
		redacted.Artifacts[index].Name = redactor.text(result.Artifacts[index].Name, path+".name")
		redacted.Artifacts[index].MIMEType = redactor.text(result.Artifacts[index].MIMEType, path+".mime_type")
		redacted.Artifacts[index].SHA256 = redactor.text(result.Artifacts[index].SHA256, path+".sha256")
	}
	safeRedactions := make([]toolspkg.Redaction, 0, len(redactor.redactions))
	for index := range redactor.redactions {
		redaction := redactor.redactions[index]
		if diagnostics.RedactExactBinary(string(redaction.Reason)) != string(redaction.Reason) {
			continue
		}
		redaction.Path = diagnostics.RedactExactBinary(redaction.Path)
		safeRedactions = append(safeRedactions, redaction)
	}
	redacted.Redactions = safeRedactions
	return redacted, nil
}

func redactMCPToolDescriptor(
	descriptor toolspkg.MCPToolDescriptor,
) (toolspkg.MCPToolDescriptor, error) {
	if mcpDescriptorIdentityContainsSecret(descriptor) {
		return toolspkg.MCPToolDescriptor{}, toolspkg.NewValidationError(
			"identity",
			toolspkg.ReasonSecretMetadata,
			"mcp tool identity contains credential material",
		)
	}

	redacted := descriptor
	redacted.Title = diagnostics.RedactExactBinary(descriptor.Title)
	redacted.FriendlyVerb = diagnostics.RedactExactBinary(descriptor.FriendlyVerb)
	redacted.Preview = diagnostics.RedactExactBinary(descriptor.Preview)
	redacted.Description = diagnostics.RedactExactBinary(descriptor.Description)
	var err error
	redacted.InputSchema, err = diagnostics.RedactExactBinaryJSON(descriptor.InputSchema)
	if err != nil {
		return toolspkg.MCPToolDescriptor{}, fmt.Errorf("mcp: redact input schema: %w", err)
	}
	redacted.OutputSchema, err = diagnostics.RedactExactBinaryJSON(descriptor.OutputSchema)
	if err != nil {
		return toolspkg.MCPToolDescriptor{}, fmt.Errorf("mcp: redact output schema: %w", err)
	}
	if err := toolmeta.ValidateDescriptorMetadata(redacted.FriendlyVerb, redacted.Preview); err != nil {
		return toolspkg.MCPToolDescriptor{}, fmt.Errorf("mcp: invalid redacted tool presentation metadata: %w", err)
	}
	redacted.Source.RawToolName = redacted.RawName
	return redacted, nil
}

func mcpDescriptorIdentityContainsSecret(descriptor toolspkg.MCPToolDescriptor) bool {
	identityFields := []string{
		string(descriptor.ID),
		descriptor.RawName,
		descriptor.Source.Kind.String(),
		descriptor.Source.Owner,
		descriptor.Source.RawServerName,
		descriptor.Source.RawToolName,
		descriptor.Source.ResourceID,
		descriptor.Source.ResourceVersion,
		descriptor.Source.WorkspaceID,
		descriptor.Source.Scope,
	}
	for _, value := range identityFields {
		if diagnostics.RedactExactBinary(value) != value {
			return true
		}
	}
	return false
}

type mcpPayloadRedactor struct {
	redactions []toolspkg.Redaction
}

func (r *mcpPayloadRedactor) text(value string, path string) string {
	redacted := diagnostics.RedactExactBinary(value)
	if redacted != value {
		r.record(path, len(value))
	}
	return redacted
}

func (r *mcpPayloadRedactor) rawJSON(raw json.RawMessage, path string) (json.RawMessage, error) {
	redacted, err := diagnostics.RedactExactBinaryJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("mcp: redact result field %s: %w", path, err)
	}
	if !bytes.Equal(redacted, raw) {
		r.record(path, len(raw))
	}
	return redacted, nil
}

func (r *mcpPayloadRedactor) rawMap(
	values map[string]json.RawMessage,
	path string,
) (map[string]json.RawMessage, error) {
	if values == nil {
		return nil, nil
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("mcp: encode result field %s: %w", path, err)
	}
	redacted, err := diagnostics.RedactExactBinaryJSON(encoded)
	if err != nil {
		return nil, fmt.Errorf("mcp: redact result field %s: %w", path, err)
	}
	decoded := make(map[string]json.RawMessage, len(values))
	if err := json.Unmarshal(redacted, &decoded); err != nil {
		return nil, fmt.Errorf("mcp: decode redacted result field %s: %w", path, err)
	}
	if !bytes.Equal(redacted, encoded) {
		r.record(path, len(encoded))
	}
	return decoded, nil
}

func (r *mcpPayloadRedactor) record(path string, byteCount int) {
	if r == nil {
		return
	}
	if byteCount < 0 {
		byteCount = 0
	}
	r.redactions = append(r.redactions, toolspkg.Redaction{
		Path:   path,
		Reason: toolspkg.ReasonSecretMetadata,
		Bytes:  int64(byteCount),
	})
}
