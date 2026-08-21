package corecmds

import "github.com/compozy/compozy/internal/cmdpalette"

type settingsDestination struct {
	Slug     string
	Title    string
	Icon     string
	Keywords []string
}

var settingsDestinations = []settingsDestination{
	{
		Slug:     "general",
		Title:    "General",
		Icon:     "sliders-horizontal",
		Keywords: []string{"defaults", "permissions", "updates"},
	},
	{Slug: "appearance", Title: "Appearance", Icon: "palette", Keywords: []string{"wallpaper", "theme", "dock"}},
	{
		Slug:     "layouts",
		Title:    "Layouts",
		Icon:     coreIconPanelTop,
		Keywords: []string{"window manager", "desktops", "shortcuts"},
	},
	{Slug: "providers", Title: "Providers", Icon: "cpu", Keywords: []string{"models", "auth", "codex", "claude"}},
	{Slug: "memory", Title: "Memory", Icon: "brain", Keywords: []string{"recall", "ledger", "dream"}},
	{Slug: "roles", Title: "Roles", Icon: "route", Keywords: []string{"coordinator", "routing", "model"}},
	{Slug: "skills", Title: "Skills", Icon: "wrench", Keywords: []string{"registry", coreAppMarketplace, "install"}},
	{
		Slug:     "automation",
		Title:    "Automation",
		Icon:     coreIconZap,
		Keywords: []string{coreAppJobs, coreAppTriggers, "scheduler", "cron"},
	},
	{Slug: coreNetworkKey, Title: "Network", Icon: coreNetworkKey, Keywords: []string{"peers", "channels", "delivery"}},
	{Slug: "gateway", Title: "Gateway", Icon: "radio", Keywords: []string{"remote", "pairing", "devices"}},
	{Slug: "attention", Title: "Attention", Icon: "bell", Keywords: []string{"notifications", "sound", "mute"}},
	{
		Slug:     "observability",
		Title:    "Observability",
		Icon:     "activity",
		Keywords: []string{"logs", "capture", "support bundle"},
	},
	{Slug: "hooks", Title: "Hooks", Icon: "webhook", Keywords: []string{"lifecycle", "events", "presets"}},
	{Slug: "palette", Title: "Palette", Icon: coreIconCommand, Keywords: []string{"commands", "shortcuts", "search"}},
	{Slug: coreAppExtensions, Title: "Extensions", Icon: "puzzle", Keywords: []string{"policy", "registry", "trust"}},
}

func settingsCommands() []cmdpalette.Descriptor {
	commands := make([]cmdpalette.Descriptor, 0, len(settingsDestinations))
	for _, destination := range settingsDestinations {
		command := coreDescriptor(
			cmdpalette.CommandID("settings."+destination.Slug),
			"Settings → "+destination.Title,
			coreSectionSettings,
			destination.Icon,
			cmdpalette.Action{
				Kind: cmdpalette.ActionKindNavigate,
				App:  coreSettingsKey,
				Args: map[string]any{"pathname": "/settings/" + destination.Slug},
			},
		)
		command.Keywords = append([]string(nil), destination.Keywords...)
		commands = append(commands, command)
	}
	return commands
}
