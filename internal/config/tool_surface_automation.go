package config

func automationToolPathKinds() map[string]ValueKind {
	return map[string]ValueKind{
		"automation.enabled":                 ConfigValueBool,
		"automation.timezone":                ConfigValueString,
		"automation.max_concurrent_jobs":     ConfigValueInt,
		"automation.suggestions.pending_cap": ConfigValueInt,
	}
}
