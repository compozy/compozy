package corecmds

import "github.com/compozy/compozy/internal/cmdpalette"

var viewOnlyDefinitions = []domainDefinition{
	{id: "sessions", title: "Sessions", icon: coreIconTerminal},
	{id: "worktrees", title: "Worktrees", icon: "git-branch"},
	{id: "network-channels", title: "Network channels", icon: coreIconGlobe},
	{id: coreAppExtensions, title: "Extensions", icon: "blocks"},
	{id: "profiles", title: "Profiles", icon: "users-round"},
}

func viewCommands() []cmdpalette.Descriptor {
	definitions := append([]domainDefinition(nil), viewOnlyDefinitions...)
	definitions = append(definitions, sharedAppViewDomains...)
	commands := make([]cmdpalette.Descriptor, 0, len(definitions))
	for _, view := range definitions {
		commands = append(commands, coreDescriptor(
			cmdpalette.CommandID("palette.view."+view.id), view.title, coreSectionViews, view.icon,
			cmdpalette.Action{Kind: cmdpalette.ActionKindView, View: view.id},
		))
	}
	return commands
}
