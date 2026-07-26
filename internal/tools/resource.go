package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/resources"
)

const (
	// ToolResourceKind is the canonical desired-state resource kind for tool records.
	ToolResourceKind     resources.ResourceKind = "tool"
	toolResourceMaxBytes                        = 256 << 10
)

// NewResourceCodec builds the canonical tool resource codec.
func NewResourceCodec() (resources.KindCodec[Tool], error) {
	return resources.NewJSONCodec(ToolResourceKind, toolResourceMaxBytes, validateToolSpec)
}

func validateToolSpec(_ context.Context, scope resources.ResourceScope, spec Tool) (Tool, error) {
	normalizedScope := scope.Normalize()
	if err := normalizedScope.Validate("scope"); err != nil {
		return Tool{}, err
	}
	presentation := spec.Presentation()

	normalized := Tool{
		ID:           ToolID(strings.TrimSpace(spec.ID.String())),
		Backend:      normalizeBackendRef(spec.Backend),
		DisplayTitle: strings.TrimSpace(spec.DisplayTitle),
		ToolPresentation: NewToolPresentation(
			strings.TrimSpace(presentation.FriendlyVerb),
			strings.TrimSpace(presentation.Preview),
		),
		Description:         strings.TrimSpace(spec.Description),
		InputSchema:         cloneRawMessage(spec.InputSchema),
		OutputSchema:        cloneRawMessage(spec.OutputSchema),
		Source:              normalizeSourceRef(spec.Source),
		Visibility:          spec.Visibility,
		Risk:                spec.Risk,
		ReadOnly:            spec.ReadOnly,
		Destructive:         spec.Destructive,
		OpenWorld:           spec.OpenWorld,
		RequiresInteraction: spec.RequiresInteraction,
		ConcurrencySafe:     spec.ConcurrencySafe,
		MaxResultBytes:      spec.MaxResultBytes,
		Toolsets:            normalizeToolsets(spec.Toolsets),
		Tags:                normalizeStrings(spec.Tags),
		SearchHints:         normalizeStrings(spec.SearchHints),
	}

	inputSchema, err := canonicalSchema("tool.input_schema", normalized.InputSchema, true)
	if err != nil {
		return Tool{}, err
	}
	normalized.InputSchema = inputSchema

	outputSchema, err := canonicalSchema("tool.output_schema", normalized.OutputSchema, false)
	if err != nil {
		return Tool{}, err
	}
	normalized.OutputSchema = outputSchema

	if err := normalized.Validate(); err != nil {
		return Tool{}, fmt.Errorf("%w: %v", resources.ErrValidation, err)
	}

	return normalized, nil
}

func canonicalSchema(field string, raw json.RawMessage, required bool) (json.RawMessage, error) {
	if len(raw) == 0 {
		if required {
			return nil, fmt.Errorf("%w: %s is required", resources.ErrValidation, field)
		}
		return nil, nil
	}

	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("%w: %s: %v", resources.ErrValidation, field, err)
	}
	if decoded == nil {
		if required {
			return nil, fmt.Errorf("%w: %s must be a JSON object", resources.ErrValidation, field)
		}
		return nil, nil
	}

	canonical, err := json.Marshal(decoded)
	if err != nil {
		return nil, fmt.Errorf("tools: canonicalize %s: %w", field, err)
	}
	return append(json.RawMessage(nil), canonical...), nil
}

func normalizeBackendRef(ref BackendRef) BackendRef {
	return BackendRef{
		Kind:                 ref.Kind,
		ExtensionID:          strings.TrimSpace(ref.ExtensionID),
		Handler:              strings.TrimSpace(ref.Handler),
		MCPServer:            strings.TrimSpace(ref.MCPServer),
		MCPTool:              strings.TrimSpace(ref.MCPTool),
		NativeName:           strings.TrimSpace(ref.NativeName),
		RequiresCapabilities: normalizeStrings(ref.RequiresCapabilities),
	}
}

func normalizeSourceRef(ref SourceRef) SourceRef {
	return SourceRef{
		Kind:            ref.Kind,
		Owner:           strings.TrimSpace(ref.Owner),
		RawServerName:   strings.TrimSpace(ref.RawServerName),
		RawToolName:     strings.TrimSpace(ref.RawToolName),
		ResourceID:      strings.TrimSpace(ref.ResourceID),
		ResourceVersion: strings.TrimSpace(ref.ResourceVersion),
		WorkspaceID:     strings.TrimSpace(ref.WorkspaceID),
		Scope:           strings.TrimSpace(ref.Scope),
	}
}

func normalizeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func normalizeToolsets(values []ToolsetID) []ToolsetID {
	if len(values) == 0 {
		return nil
	}
	normalized := make([]ToolsetID, 0, len(values))
	for _, value := range values {
		trimmed := ToolsetID(strings.TrimSpace(value.String()))
		if trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}
