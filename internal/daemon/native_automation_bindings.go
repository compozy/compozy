package daemon

import toolspkg "github.com/compozy/agh/internal/tools"

func (n *daemonNativeTools) automationToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDAutomationSuggestionsList: {
			call: n.automationSuggestionsList, availability: availability,
		},
		toolspkg.ToolIDAutomationSuggestionsAccept: {
			call: n.automationSuggestionsAccept, availability: availability,
		},
		toolspkg.ToolIDAutomationSuggestionsDismiss: {
			call: n.automationSuggestionsDismiss, availability: availability,
		},
		toolspkg.ToolIDAutomationJobsList: {
			call: n.automationJobsList, availability: availability,
		},
		toolspkg.ToolIDAutomationJobsGet: {
			call: n.automationJobsGet, availability: availability,
		},
		toolspkg.ToolIDAutomationJobsCreate: {
			call: n.automationJobsCreate, availability: availability,
		},
		toolspkg.ToolIDAutomationJobsUpdate: {
			call: n.automationJobsUpdate, availability: availability,
		},
		toolspkg.ToolIDAutomationJobsDelete: {
			call: n.automationJobsDelete, availability: availability,
		},
		toolspkg.ToolIDAutomationJobsEnable: {
			call: n.automationJobsEnable, availability: availability,
		},
		toolspkg.ToolIDAutomationJobsDisable: {
			call: n.automationJobsDisable, availability: availability,
		},
		toolspkg.ToolIDAutomationJobsTrigger: {
			call: n.automationJobsTrigger, availability: availability,
		},
		toolspkg.ToolIDAutomationJobsHistory: {
			call: n.automationJobsHistory, availability: availability,
		},
		toolspkg.ToolIDAutomationTriggersList: {
			call: n.automationTriggersList, availability: availability,
		},
		toolspkg.ToolIDAutomationTriggersGet: {
			call: n.automationTriggersGet, availability: availability,
		},
		toolspkg.ToolIDAutomationTriggersCreate: {
			call: n.automationTriggersCreate, availability: availability,
		},
		toolspkg.ToolIDAutomationTriggersUpdate: {
			call: n.automationTriggersUpdate, availability: availability,
		},
		toolspkg.ToolIDAutomationTriggersDelete: {
			call: n.automationTriggersDelete, availability: availability,
		},
		toolspkg.ToolIDAutomationTriggersEnable: {
			call: n.automationTriggersEnable, availability: availability,
		},
		toolspkg.ToolIDAutomationTriggersDisable: {
			call: n.automationTriggersDisable, availability: availability,
		},
		toolspkg.ToolIDAutomationTriggersHistory: {
			call: n.automationTriggersHistory, availability: availability,
		},
		toolspkg.ToolIDAutomationRunsList: {
			call: n.automationRunsList, availability: availability,
		},
		toolspkg.ToolIDAutomationRunsGet: {
			call: n.automationRunsGet, availability: availability,
		},
	}
}
