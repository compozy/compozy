package loop

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/tools"
)

// OutputSchemaFor selects the node's output contract.
func (s ToolSchemaSnapshot) OutputSchemaFor(view tools.ResultView) (json.RawMessage, string) {
	if view == tools.ResultViewFull && s.FullOutputSchemaDigest != "" {
		return s.FullOutputSchema, s.FullOutputSchemaDigest
	}
	return s.OutputSchema, s.OutputSchemaDigest
}

func nodeResultView(node dsl.Node) tools.ResultView {
	if node.Params[tools.ResultViewInputKey] == string(tools.ResultViewFull) {
		return tools.ResultViewFull
	}
	return tools.ResultViewCompact
}

func pinnedActionResultView(
	descriptor tools.Descriptor, view tools.ResultView, pinned bool,
) tools.ResultView {
	if !pinned || descriptor.ResultViews() == nil {
		return ""
	}
	if view == "" {
		return tools.ResultViewCompact
	}
	return view
}

func validatePinnedActionContract(
	kind string, descriptor tools.Descriptor, expected *ToolSchemaSnapshot, view tools.ResultView,
) error {
	current := ToolSchemaSnapshot{
		InputSchemaDigest:  descriptor.InputSchemaDigest,
		OutputSchemaDigest: descriptor.OutputSchemaDigest,
	}
	if views := descriptor.ResultViews(); views != nil {
		current.FullOutputSchemaDigest = views.Full.Digest
	}
	if pinnedView := pinnedActionResultView(descriptor, view, true); pinnedView != "" {
		view = pinnedView
	}
	_, currentOutput := current.OutputSchemaFor(view)
	currentInput := current.InputSchemaDigest
	meta := map[string]string{actionKindMetaKey: kind}
	if expected != nil {
		_, expectedOutput := expected.OutputSchemaFor(view)
		if strings.TrimSpace(expected.ToolID) == kind &&
			strings.TrimSpace(expected.InputSchemaDigest) == strings.TrimSpace(currentInput) &&
			strings.TrimSpace(expectedOutput) == strings.TrimSpace(currentOutput) {
			return nil
		}
		meta["expected_input_schema_digest"] = strings.TrimSpace(expected.InputSchemaDigest)
		meta["expected_output_schema_digest"] = strings.TrimSpace(expectedOutput)
	}
	meta["current_input_schema_digest"] = strings.TrimSpace(currentInput)
	meta["current_output_schema_digest"] = strings.TrimSpace(currentOutput)
	return reasonError(ReasonCodeActionContractStale,
		fmt.Errorf("%w: action tool contract changed after Start", ErrActionInvalidOutput), meta)
}

func reviewToolInputSchema(snapshot ToolSchemaSnapshot) (json.RawMessage, error) {
	if snapshot.FullOutputSchemaDigest == "" {
		return cloneRawMessage(snapshot.InputSchema), nil
	}
	var schema map[string]any
	if err := json.Unmarshal(snapshot.InputSchema, &schema); err != nil {
		return nil, fmt.Errorf("loop: decode review input schema: %w", err)
	}
	if properties, ok := schema[jsonSchemaPropertiesKey].(map[string]any); ok {
		delete(properties, tools.ResultViewInputKey)
	}
	return json.Marshal(schema)
}
