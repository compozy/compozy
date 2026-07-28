package events

func info(name string, family string, component string) Metadata {
	return metadata(name, family, component, OutcomeInfo)
}

func success(name string, family string, component string) Metadata {
	return metadata(name, family, component, OutcomeSuccess)
}

func failure(name string, family string, component string) Metadata {
	return metadata(name, family, component, OutcomeFailure)
}

func warning(name string, family string, component string) Metadata {
	return metadata(name, family, component, OutcomeWarning)
}

func metadata(name string, family string, component string, outcome Outcome) Metadata {
	return Metadata{
		Name:        name,
		Family:      family,
		Component:   component,
		Outcome:     outcome,
		EmitsToLogs: true,
	}
}

func notify(entry Metadata) Metadata {
	entry.NotificationEligible = true
	return entry
}

func global(entry Metadata) Metadata {
	entry.GlobalScope = true
	return entry
}
