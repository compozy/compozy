package config

func cloneAutomationConfig(source AutomationConfig) AutomationConfig {
	cloned := source
	if source.Jobs != nil {
		cloned.Jobs = make([]AutomationJob, len(source.Jobs))
		for index, job := range source.Jobs {
			cloned.Jobs[index] = job
			cloned.Jobs[index].Task = cloneParsedJobTaskConfig(job.Task)
			cloned.Jobs[index].LoopTarget = cloneAutomationLoopTarget(job.LoopTarget)
		}
	}
	if source.Triggers != nil {
		cloned.Triggers = make([]AutomationTrigger, len(source.Triggers))
		for index, trigger := range source.Triggers {
			cloned.Triggers[index] = trigger
			cloned.Triggers[index].Filter = mergeStringMaps(nil, trigger.Filter)
			cloned.Triggers[index].LoopTarget = cloneAutomationLoopTarget(trigger.LoopTarget)
		}
	}
	return cloned
}
