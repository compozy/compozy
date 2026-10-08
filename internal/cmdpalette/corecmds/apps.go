package corecmds

import "github.com/compozy/compozy/internal/cmdpalette"

var appOnlyDefinitions = []domainDefinition{
	{id: "dashboard", title: "Home", icon: "home"},
	{id: "session", title: "Session", icon: coreIconTerminal, keywords: []string{"agent"}},
	{id: "terminal", title: "Terminal", icon: coreIconTerminal, keywords: []string{"terminal", "journal", "pty"}},
	{id: "new-tab", title: "New tab", icon: coreIconPlus},
	{id: coreSettingsKey, title: "Settings", icon: coreSettingsKey},
}

func appCommands() []cmdpalette.Descriptor {
	definitions := append([]domainDefinition(nil), appOnlyDefinitions...)
	definitions = append(definitions, sharedAppViewDomains...)
	commands := make([]cmdpalette.Descriptor, 0, len(definitions))
	for _, app := range definitions {
		command := coreDescriptor(
			cmdpalette.CommandID("app.open."+app.id),
			"Open "+app.title,
			coreSectionApps,
			app.icon,
			cmdpalette.Action{Kind: cmdpalette.ActionKindNavigate, App: app.id},
		)
		command.Keywords = append([]string(nil), app.keywords...)
		commands = append(commands, command)
	}
	return commands
}

// RegisteredApp reports membership in the daemon-owned OS app inventory.
func RegisteredApp(id string) bool {
	for _, app := range appOnlyDefinitions {
		if app.id == id {
			return true
		}
	}
	for _, app := range sharedAppViewDomains {
		if app.id == id {
			return true
		}
	}
	return false
}
