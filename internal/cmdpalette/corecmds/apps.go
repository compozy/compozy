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
	{id: "session", title: "Session", icon: "square-terminal", keywords: []string{"agent", "terminal"}},
	{id: "new-tab", title: "New tab", icon: "plus"},
	{id: "agents", title: "Agents", icon: "bot"},
	{id: "network", title: "Network", icon: "globe"},
	{id: "tasks", title: "Tasks", icon: "list-checks"},
	{id: "loops", title: "Loops", icon: "repeat-2"},
	{id: "jobs", title: "Jobs", icon: "clock-3"},
	{id: "triggers", title: "Triggers", icon: "zap"},
	{id: "marketplace", title: "Marketplace", icon: "store"},
	{id: "bridges", title: "Bridges", icon: "waypoints"},
	{id: "knowledge", title: "Knowledge", icon: "book-open"},
	{id: "sandbox", title: "Sandbox", icon: "boxes"},
	{id: "vault", title: "Vault", icon: "key-round"},
	{id: "settings", title: "Settings", icon: "settings"},
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
