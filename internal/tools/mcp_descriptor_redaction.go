package tools

import (
	"fmt"

	"github.com/compozy/compozy/internal/redact"
	"github.com/compozy/compozy/internal/toolmeta"
)

// RedactMCPToolDescriptor protects metadata without changing a tool's dispatch identity.
func RedactMCPToolDescriptor(descriptor MCPToolDescriptor, snapshot *redact.SecretSnapshot) (MCPToolDescriptor, error) {
	identity := []string{
		string(descriptor.ID), descriptor.RawName, descriptor.Source.Kind.String(), descriptor.Source.Owner,
		descriptor.Source.RawServerName, descriptor.Source.RawToolName, descriptor.Source.ResourceID,
		descriptor.Source.ResourceVersion, descriptor.Source.WorkspaceID, descriptor.Source.ProfileID,
		descriptor.Source.Scope,
	}
	for _, value := range identity {
		if snapshot.ExactBinaryString(value) != value {
			return MCPToolDescriptor{}, NewValidationError(
				"identity", ReasonSecretMetadata, "mcp tool identity contains credential material",
			)
		}
	}

	redacted := descriptor
	redacted.Title = snapshot.ExactBinaryString(descriptor.Title)
	redacted.FriendlyVerb = snapshot.ExactBinaryString(descriptor.FriendlyVerb)
	redacted.Preview = snapshot.ExactBinaryString(descriptor.Preview)
	redacted.Description = snapshot.ExactBinaryString(descriptor.Description)
	var err error
	redacted.InputSchema, err = snapshot.ExactBinaryJSON(descriptor.InputSchema)
	if err != nil {
		return MCPToolDescriptor{}, fmt.Errorf("mcp: redact input schema: %w", err)
	}
	redacted.OutputSchema, err = snapshot.ExactBinaryJSON(descriptor.OutputSchema)
	if err != nil {
		return MCPToolDescriptor{}, fmt.Errorf("mcp: redact output schema: %w", err)
	}
	if err := toolmeta.ValidateDescriptorMetadata(redacted.FriendlyVerb, redacted.Preview); err != nil {
		return MCPToolDescriptor{}, fmt.Errorf("mcp: invalid redacted tool presentation metadata: %w", err)
	}
	if redacted.Source.RawToolName == "" {
		redacted.Source.RawToolName = redacted.RawName
	}
	return redacted, nil
}
