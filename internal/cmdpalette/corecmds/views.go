package corecmds

import "github.com/compozy/compozy/internal/cmdpalette"

func viewCommands() []cmdpalette.Descriptor {
	views := []struct {
		id    string
		title string
		icon  string
	}{
		{id: "sessions", title: "Sessions", icon: coreIconTerminal},
		{id: "worktrees", title: "Worktrees", icon: "git-branch"},
		{id: coreAppTasks, title: "Tasks", icon: "list-checks"},
		{id: coreAppLoops, title: "Loops", icon: "repeat-2"},
		{id: coreAppJobs, title: "Jobs", icon: "clock-3"},
		{id: coreAppTriggers, title: "Triggers", icon: coreIconZap},
		{id: coreAppAgents, title: "Agents", icon: "bot"},
		{id: coreAppBridges, title: "Bridges", icon: "waypoints"},
		{id: coreAppKnowledge, title: "Knowledge", icon: "book-open"},
		{id: coreAppVault, title: "Vault", icon: "key-round"},
		{id: "network-channels", title: "Network channels", icon: coreIconGlobe},
		{id: coreAppMarketplace, title: "Marketplace", icon: "store"},
		{id: coreAppExtensions, title: "Extensions", icon: "blocks"},
	}
	commands := make([]cmdpalette.Descriptor, 0, len(views))
	for _, view := range views {
		commands = append(commands, coreDescriptor(
			cmdpalette.CommandID("palette.view."+view.id), view.title, "Views", view.icon,
			cmdpalette.Action{Kind: cmdpalette.ActionKindView, View: view.id},
		))
	}
	return commands
}
