package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

func automationSuggestionDescriptors() []toolspkg.Descriptor {
	return []toolspkg.Descriptor{
		withResultViews(nativeAutomationDescriptor(
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
		), automationSuggestionsListCompactOutputSchema, `{"type":"object"}`),
		withResultViews(nativeAutomationDescriptor(
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
		), automationSuggestionAcceptCompactOutputSchema, `{"type":"object"}`),
		withResultViews(nativeAutomationDescriptor(
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
		), automationSuggestionCompactOutputSchema, `{"type":"object"}`),
	}
}

const automationSuggestionsListInputSchema = `{
	"type":"object",
	"properties":{
		"workspace":{"type":"string"},
		"status":{"type":"string","enum":["pending","accepted","dismissed"]}
	},
	"additionalProperties":false
}`

const automationSuggestionActionInputSchema = `{
	"type":"object",
	"required":["suggestion_id"],
	"properties":{
		"workspace":{"type":"string"},
		"suggestion_id":{"type":"string"}
	},
	"additionalProperties":false
}`
