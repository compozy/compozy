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

// RetiredCommandID accepts public aliases until removal in v0.5.0.
func RetiredCommandID(id cmdpalette.CommandID) (cmdpalette.CommandID, bool) {
	switch id {
	case "app.open.jobs", "app.open.triggers":
		return "app.open.automations", true
	case "palette.view.jobs", "palette.view.triggers":
		return "palette.view.automations", true
	default:
		return id, false
	}
}
