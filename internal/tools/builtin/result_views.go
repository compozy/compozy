package builtin

import (
	"encoding/json"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

const resultViewDescription = ` Returns the compact result; pass result_view "full" for the complete record.`

func withResultViews(d toolspkg.Descriptor, compact, full string) toolspkg.Descriptor {
	var schema map[string]json.RawMessage
	if err := json.Unmarshal(d.InputSchema, &schema); err != nil {
		panic("invariant: invalid native input schema: " + err.Error())
	}
	properties := map[string]json.RawMessage{}
	if raw := schema["properties"]; len(raw) > 0 {
		if err := json.Unmarshal(raw, &properties); err != nil {
			panic("invariant: invalid native input properties: " + err.Error())
		}
	}
	properties[toolspkg.ResultViewInputKey] = json.RawMessage(toolspkg.ResultViewSelectorSchema)
	schema["properties"] = mustResultViewJSON(properties)
	d.InputSchema = mustResultViewJSON(schema)
	d.InputSchemaDigest = ""
	d.OutputSchemaDigest = ""
	d.SetResultViews(&toolspkg.ResultViews{
		Compact: toolspkg.ResultViewContract{Schema: json.RawMessage(compact)},
		Full:    toolspkg.ResultViewContract{Schema: json.RawMessage(full)},
	})
	d.OutputSchema = json.RawMessage(compact)
	d.Description += resultViewDescription
	return d
}

func mustResultViewJSON(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		panic("invariant: native schema cannot be encoded: " + err.Error())
	}
	return raw
}

type SingleResultReason string

const (
	SingleResultContent       SingleResultReason = "content"
	SingleResultMinimal       SingleResultReason = "minimal"
	SingleResultCallerBounded SingleResultReason = "caller_bounded"
)

var singleResultFamilies = []map[toolspkg.ToolID]SingleResultReason{
	singleResultSession,
	singleResultSubagent,
	singleResultMonitor,
	singleResultClarify,
	singleResultNotify,
	singleResultAgent,
	singleResultAutomation,
	singleResultLoop,
	singleResultGoal,
	singleResultTask,
	singleResultWorkspace,
	singleResultWorktree,
	singleResultProfile,
	singleResultConfig,
	singleResultHooks,
	singleResultExtensions,
	singleResultMarketplace,
	singleResultProvider,
	singleResultResources,
	singleResultVault,
	singleResultGateway,
	singleResultMcp,
	singleResultObserve,
	singleResultLogs,
	singleResultWindow,
	singleResultDesktop,
	singleResultLayout,
	singleResultTerminal,
	singleResultTool,
	singleResultSkill,
	singleResultCmd,
	singleResultCommand,
}

func SingleResultReasonFor(id toolspkg.ToolID) (SingleResultReason, bool) {
	for _, family := range singleResultFamilies {
		if reason, ok := family[id]; ok {
			return reason, true
		}
	}
	return "", false
}
