package extensionpkg

import automationpkg "github.com/compozy/compozy/internal/automation"

func cloneExtensionAutomationJobs(values []automationpkg.Job) []automationpkg.Job {
	if len(values) == 0 {
		return nil
	}
	result := make([]automationpkg.Job, 0, len(values))
	for _, value := range values {
		result = append(result, automationpkg.CloneJob(value))
	}
	return result
}

func cloneExtensionAutomationTriggers(values []automationpkg.Trigger) []automationpkg.Trigger {
	if len(values) == 0 {
		return nil
	}
	result := make([]automationpkg.Trigger, 0, len(values))
	for _, value := range values {
		result = append(result, automationpkg.CloneTrigger(value))
	}
	return result
}
