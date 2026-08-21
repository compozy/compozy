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
	{id: coreAppJobs, title: "Jobs", icon: "clock-3"},
	{id: coreAppTriggers, title: "Triggers", icon: coreIconZap},
	{id: coreAppMarketplace, title: "Marketplace", icon: "store"},
	{id: coreAppBridges, title: "Bridges", icon: "waypoints"},
	{id: coreAppKnowledge, title: "Knowledge", icon: "book-open"},
	{id: coreAppVault, title: "Vault", icon: "key-round"},
}
