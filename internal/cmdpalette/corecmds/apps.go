package corecmds

import (
	"github.com/compozy/compozy/internal/cmdpalette"
	"github.com/compozy/compozy/internal/windowmanager"
)

var appOnlyDefinitions = []domainDefinition{
	{id: "dashboard", title: "Home", icon: "home"},
	{id: "session", title: "Session", icon: coreIconTerminal, keywords: []string{"agent"}},
	{id: "terminal", title: "Terminal", icon: coreIconTerminal, keywords: []string{"terminal", "journal", "pty"}},
	{id: coreAppFiles, title: "Files", commandTitle: "Open project files", icon: "folder-tree",
		keywords: []string{"files", "explorer", "browse", "project"}},
	{id: "new-tab", title: "New tab", icon: coreIconPlus},
	{id: "usage", title: "Usage", icon: "gauge", keywords: []string{"provider usage", "limits", "quota"}},
	{id: coreSettingsKey, title: "Settings", icon: coreSettingsKey},
}

func appCommands() []cmdpalette.Descriptor {
	definitions := append([]domainDefinition(nil), appOnlyDefinitions...)
	definitions = append(definitions, sharedAppViewDomains...)
	commands := make([]cmdpalette.Descriptor, 0, len(definitions))
	for _, app := range definitions {
		title := "Open " + app.title
		if app.commandTitle != "" {
			title = app.commandTitle
		}
		command := coreDescriptor(
			cmdpalette.CommandID("app.open."+app.id),
			title,
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

// RetiredCommandID translates public aliases during their named removal window.
func RetiredCommandID(id cmdpalette.CommandID) (cmdpalette.CommandID, bool) {
	if replacement, retired := windowmanager.CommandAlias(string(id)); retired {
		return cmdpalette.CommandID(replacement), true
	}
	switch id {
	case "app.open.jobs", "app.open.triggers":
		return "app.open.automations", true
	case "palette.view.jobs", "palette.view.triggers":
		return "palette.view.automations", true
	default:
		return id, false
	}
}

func RetiredCommandRemovalRelease(id cmdpalette.CommandID) string {
	if id == "window.stack.reorder" || id == "window.stack.set_active" {
		return "v0.8.0"
	}
	return "v0.5.0"
}
