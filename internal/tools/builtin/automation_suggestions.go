package builtin

import toolspkg "github.com/compozy/agh/internal/tools"

func automationSuggestionDescriptors() []toolspkg.Descriptor {
	return []toolspkg.Descriptor{
		nativeAutomationDescriptor(
			toolspkg.ToolIDAutomationSuggestionsList,
			"automation_suggestions_list",
			"Automation Suggestions List",
			"List consent-first automation suggestions for one workspace.",
			automationSuggestionsListInputSchema,
			toolspkg.RiskRead,
			true,
			false,
			[]string{automationAutomationKey, automationSuggestionsKey, descriptorKeywordCatalog},
			[]string{"automation suggestions", "suggested jobs"},
		),
		nativeAutomationDescriptor(
			toolspkg.ToolIDAutomationSuggestionsAccept,
			"automation_suggestions_accept",
			"Automation Suggestions Accept",
			"Accept one workspace suggestion and create its validated automation Job.",
			automationSuggestionActionInputSchema,
			toolspkg.RiskMutating,
			false,
			false,
			[]string{automationAutomationKey, automationSuggestionsKey, automationMutationKey},
			[]string{"accept automation suggestion", "create suggested job"},
		),
		nativeAutomationDescriptor(
			toolspkg.ToolIDAutomationSuggestionsDismiss,
			"automation_suggestions_dismiss",
			"Automation Suggestions Dismiss",
			"Durably dismiss one workspace suggestion so it is not emitted again.",
			automationSuggestionActionInputSchema,
			toolspkg.RiskDestructive,
			false,
			true,
			[]string{automationAutomationKey, automationSuggestionsKey, automationMutationKey},
			[]string{"dismiss automation suggestion", "reject suggested job"},
		),
	}
}

const automationSuggestionsListInputSchema = `{
	"type":"object",
	"required":["workspace_id"],
	"properties":{
		"workspace_id":{"type":"string"},
		"status":{"type":"string","enum":["pending","accepted","dismissed"]}
	},
	"additionalProperties":false
}`

const automationSuggestionActionInputSchema = `{
	"type":"object",
	"required":["workspace_id","suggestion_id"],
	"properties":{
		"workspace_id":{"type":"string"},
		"suggestion_id":{"type":"string"}
	},
	"additionalProperties":false
}`
