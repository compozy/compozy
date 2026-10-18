package corecmds

type domainDefinition struct {
	id       string
	title    string
	icon     string
	keywords []string
}

var sharedAppViewDomains = []domainDefinition{
	{id: coreAppAgents, title: "Agents", icon: "bot"},
	{id: coreAppTasks, title: "Tasks", icon: "list-checks"},
	{id: coreAppLoops, title: "Loops", icon: "repeat-2"},
	{id: coreAppAutomations, title: "Automations", icon: coreIconZap,
		keywords: []string{"jobs", "triggers", coreStartSchedule, coreKeywordWebhook, coreStartEvent}},
	{id: coreAppMarketplace, title: "Marketplace", icon: "store"},
	{id: coreAppVault, title: "Vault", icon: "key-round"},
}
