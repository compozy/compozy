package corecmds

import "github.com/compozy/compozy/internal/cmdpalette"

type appDefinition struct {
	id       string
	title    string
	icon     string
	keywords []string
}

var appDefinitions = []appDefinition{
	{id: "dashboard", title: "Home", icon: "home"},
	{id: "session", title: "Session", icon: coreIconTerminal, keywords: []string{"agent", "terminal"}},
	{id: "new-tab", title: "New tab", icon: coreIconPlus},
	{id: coreAppAgents, title: "Agents", icon: "bot"},
	{id: coreNetworkKey, title: "Network", icon: coreIconGlobe},
	{id: coreAppTasks, title: "Tasks", icon: "list-checks"},
	{id: coreAppLoops, title: "Loops", icon: "repeat-2"},
	{id: coreAppJobs, title: "Jobs", icon: "clock-3"},
	{id: coreAppTriggers, title: "Triggers", icon: coreIconZap},
	{id: coreAppMarketplace, title: "Marketplace", icon: "store"},
	{id: coreAppBridges, title: "Bridges", icon: "waypoints"},
	{id: coreAppKnowledge, title: "Knowledge", icon: "book-open"},
	{id: "sandbox", title: "Sandbox", icon: "boxes"},
	{id: coreAppVault, title: "Vault", icon: "key-round"},
	{id: coreSettingsKey, title: "Settings", icon: coreSettingsKey},
}

func appCommands() []cmdpalette.Descriptor {
	commands := make([]cmdpalette.Descriptor, 0, len(appDefinitions))
	for _, app := range appDefinitions {
		command := coreDescriptor(
			cmdpalette.CommandID("app.open."+app.id),
			"Open "+app.title,
			"Apps",
			app.icon,
			cmdpalette.Action{Kind: cmdpalette.ActionKindNavigate, App: app.id},
		)
		command.Keywords = append([]string(nil), app.keywords...)
		commands = append(commands, command)
	}
	return commands
}
