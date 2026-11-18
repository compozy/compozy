package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

func automationRunDescriptors() []toolspkg.Descriptor {
	return []toolspkg.Descriptor{
		nativeAutomationDescriptor(
			toolspkg.ToolIDAutomationRunsList,
			"automation_runs_list",
			"Automation Runs List",
			"List persisted automation run records.",
			automationRunsListInputSchema,
			toolspkg.RiskRead,
			true,
			false,
			[]string{automationAutomationKey, automationRunsKey, historyTag},
			[]string{"automation runs", "run history"},
		),
		nativeAutomationDescriptor(
			toolspkg.ToolIDAutomationRunsGet,
			"automation_runs_get",
			"Automation Runs Get",
			"Read one persisted automation run record.",
			automationRunIDInputSchema,
			toolspkg.RiskRead,
			true,
			false,
			[]string{automationAutomationKey, automationRunsKey},
			[]string{"automation run details", "run record"},
		),
		nativeAutomationDescriptor(
			toolspkg.ToolIDAutomationRunsCancel, "automation_runs_cancel", "Automation Runs Cancel",
			"Cancel an orphaned run whose session is missing or stopped, preserving history. "+
				"Live sessions, delegated executions and admissions without a session are refused; "+
				"stop them through their owning lifecycle first. Terminal runs are unchanged.",
			automationRunIDInputSchema, toolspkg.RiskMutating, false, false,
			[]string{automationAutomationKey, automationRunsKey, automationMutationKey},
			[]string{"cancel automation run", "recover stuck run"},
		),
	}
}
